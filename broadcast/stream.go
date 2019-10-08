package broadcast

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fdymylja/ethutils"
	"github.com/fdymylja/ethutils/errors"
	"github.com/fdymylja/ethutils/nodeop"
	"github.com/fdymylja/utils"
	"log"
	"sync"
	"time"
)

// ErrDownloadBlock defines a block download error
type ErrDownloadBlock struct {
	BlockNumber uint64
	Err         error
}

// Error implements error interface
func (e *ErrDownloadBlock) Error() string {
	return fmt.Sprintf("failure in pulling block %d: %s", e.BlockNumber, e.Err)
}

// DefaultStreamOptions defines the default options for the Stream
var DefaultStreamOptions = &StreamOptions{
	NodeOpTimeout:      15 * time.Second,
	MaxRetries:         50,
	RetryWait:          5 * time.Second,
	WaitAfterHeader:    5 * time.Second,
	StreamBlocks:       true,
	StreamTransactions: true,
}

// TxWithBlock represents the data structure forwarded by Stream on incoming transaction
type TxWithBlock struct {
	// Transaction is the transaction
	Transaction *types.Transaction
	// BlockNumber is the block number the tx was included in
	BlockNumber uint64
	// Timestamp is the timestamp of the block
	Timestamp uint64
}

// StreamOptions represents the parameters used
type StreamOptions struct {
	// NodeOpTimeout is the timeout for node query operations
	NodeOpTimeout time.Duration
	// MaxRetries is the number of retries to make while querying headers content (-1 equals retry infinitely)
	MaxRetries int
	// RetryWait is how long we wait before retrying again
	RetryWait time.Duration
	// WaitAfterHeader is how long the client waits before querying block contents
	WaitAfterHeader time.Duration
	// StreamBlocks is used to define if headers should be forwarded to the parent
	StreamBlocks bool
	// StreamTransactions is used to define if transactions forwarded to the parent
	StreamTransactions bool
}

// NewStreamDefault creates a streamer instance with default options
func NewStreamDefault(endpoint string) *Stream {
	return NewStream(endpoint, DefaultStreamOptions)
}

// NewStream creates a Stream instance with programmable options
func NewStream(endpoint string, options *StreamOptions) *Stream {
	return &Stream{
		endpoint:     endpoint,
		options:      options,
		lastBlock:    0,
		ethClient:    nil,
		transactions: make(chan *TxWithBlock),
		headers:      make(chan *types.Header),
		closed:       make(chan struct{}),
		shutdown:     make(chan struct{}),
		errs:         make(chan error),
		shutdownOnce: &sync.Once{},
	}
}

// Stream streams transactions and or block headers coming from the ethereum blockchain
type Stream struct {
	endpoint     string             // endpoint is where the ethereum client connects to, must support wss connections
	options      *StreamOptions     // options is the streamer options
	lastBlock    uint64             // the last block queried
	ethClient    ethutils.Node      // ethClient is the client connected to an ethereum node
	transactions chan *TxWithBlock  // transactions is the channel used to forward transactions to the parent
	headers      chan *types.Header // headers is the channel used to forward headers to the parent
	shutdown     chan struct{}      // shutdown is used to close the streamer
	errs         chan error         // errs is used to forward errors coming from the streamer
	shutdownOnce *sync.Once         // shutdownOnce guarantees that shutdown is only done once
	closed       chan struct{}      // closed sends a signal to Close() caller that close operations are done
}

// Connect runs the streamer
func (s *Stream) Connect() (err error) {
	defer utils.WrapErrorP(&err)

	// connect to the node
	client, err := ethclient.Dial(s.endpoint)
	if err != nil {
		return
	}
	s.ethClient = client
	// subscribe to headers
	headers := make(chan *types.Header)
	ctx, cancel := s.ctx()
	defer cancel()
	// sub
	sub, err := s.ethClient.SubscribeNewHead(ctx, headers)
	if err != nil {
		return
	}
	// listen to events
	go s.listenBlockHeaders(headers, sub)
	return
}

// listenBlockHeaders listens for new ethereum headers and errors coming from the subscriber
func (s *Stream) listenBlockHeaders(headers <-chan *types.Header, sub ethereum.Subscription) {
	defer func() {
		sub.Unsubscribe()
	}()
	// broadcast loop
	subErr := sub.Err()
	for {
		select {
		case <-s.shutdown:
			s.cleanup()
			return
		case err, ok := <-subErr: // only one error is sent from ethereum subscription
			if err != nil {
				s.sendError(err)
			}
			if !ok { // if this is closed and err is nil it means that the subscriber was shutdown
				subErr = nil // make this case forever blocking
				break
			}
		case newHeader, ok := <-headers:
			if newHeader != nil {
				s.onHeader(newHeader)
			}
			if !ok { // break due to error from sub
				headers = nil // make this case forever blocking
				break
			}
		}
	}
}

// onHeader handles operations when a new header comes from the ethereum network
func (s *Stream) onHeader(header *types.Header) {
	// send header if required
	if s.options.StreamBlocks {
		select {
		case <-s.shutdown:
			return
		case s.headers <- header:
		}
	}
	// pull transactions from block if required
	if !s.options.StreamTransactions {
		return
	}
	block, err := s.downloadBlock(header)
	if err != nil {
		// wrap error into download block error
		err = &ErrDownloadBlock{
			BlockNumber: header.Number.Uint64(),
			Err:         err,
		}
		s.sendError(err)
		return
	}
	// forward transactions to listener
	for _, tx := range block.Transactions() {
		select {
		case s.transactions <- &TxWithBlock{Transaction: tx, BlockNumber: block.NumberU64(), Timestamp: block.Time()}:
		case <-s.shutdown:
			return
		}
	}
}

// downloadBlock pulls block data from ethereum node including transactions, it retries a total amount of times
// equivalent to StreamOptions.MaxRetries before returning an error
func (s *Stream) downloadBlock(header *types.Header) (block *types.Block, err error) {
	// sleep before pulling the block, the Ethereum network is BIG, synchronization between thousands of nodes takes time
	// ethereum nodes might notify of a new incoming block but the node we have queried  might still be unaware of its presence.
	time.Sleep(s.options.WaitAfterHeader)
	var lastError error
	for tryN := 0; ; {
		log.Printf("try n: %d", tryN)
		// check if during retries the streamer was shutdown
		select {
		case <-s.shutdown:
			return nil, errors.ErrShutdown
		default:
		}
		// loop breaks only if MaxRetries is bigger-equal than 0 and number of tries is bigger than MaxRetries
		if s.options.MaxRetries != -1 && tryN >= s.options.MaxRetries { // max retries reached
			return nil, &errors.ErrMaxRetriesReached{
				LastError: lastError,
			}
		}
		ctx, cancel := s.ctx()
		block, lastError = nodeop.DownloadBlock(ctx, s.ethClient, header.Hash())
		cancel()
		if lastError == nil {
			return
		}
		tryN++
		time.Sleep(s.options.RetryWait)
	}
}

// ctx returns a context for operations based on StreamOptions.NodeOpTimeout
func (s *Stream) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.options.NodeOpTimeout)
}

// cleanup forces all goroutines to exit
func (s *Stream) cleanup() {
	// close channels
	close(s.errs)
	close(s.headers)
	close(s.transactions)
	// close eth client
	s.ethClient.Close()
	// send closed
	close(s.closed)
}

// sendError forwards errors to parent making sure to drop them in case of shutdown
func (s *Stream) sendError(err error) {
	select {
	case <-s.shutdown: // error was dropped due to shutdown
	case s.errs <- err: // error sent
	}
}

// Err returns a channel that forwards errors coming from the streamer, per ethclient, only one error is forwarded
// to this channel
func (s *Stream) Err() <-chan error {
	return s.errs
}

// Block returns a channel that forwards block headers
func (s *Stream) Header() <-chan *types.Header {
	return s.headers
}

// Transaction returns a channel that forwards incoming transactions
func (s *Stream) Transaction() <-chan *TxWithBlock {
	return s.transactions
}

// Close closes the streamer
func (s *Stream) Close() error {
	// shutdown once
	s.shutdownOnce.Do(func() {
		close(s.shutdown)
	})
	<-s.closed
	return nil
}
