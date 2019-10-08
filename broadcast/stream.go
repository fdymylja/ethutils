package broadcast

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fdymylja/ethutils/errors"
	"github.com/fdymylja/ethutils/nodeop"
	"github.com/fdymylja/utils"
	"sync"
	"time"
)

// Node defines behaviour of ethereum queried nodes
type Node interface {
	nodeop.BlockPuller
	Close()
	SubscribeNewHead(ctx context.Context, headers chan<- *types.Header) (sub ethereum.Subscription, err error)
}

var DefaultStreamerOptions = &StreamerOptions{
	NodeOpTimeout:      15 * time.Second,
	MaxRetries:         50,
	WaitAfterHeader:    5 * time.Second,
	StreamBlocks:       true,
	StreamTransactions: true,
}

type TxWithBlock struct {
	Transaction *types.Transaction
	BlockNumber uint64
	Timestamp   uint64
}

type StreamerOptions struct {
	// NodeOpTimeout is the timeout for node query operations
	NodeOpTimeout time.Duration
	// MaxRetries is the number of retries to make while querying blocks content (-1 equals retry infinitely)
	MaxRetries int
	// WaitAfterHeader is how long the client waits before querying block contents
	WaitAfterHeader time.Duration
	// StreamBlocks is used to define if blocks should be forwarded to the parent
	StreamBlocks bool
	// StreamTransactions is used to define if transactions forwarded to the parent
	StreamTransactions bool
}

// NewStreamDefault creates a streamer instance with default options
func NewStreamDefault(endpoint string) *Stream {
	return NewStream(endpoint, DefaultStreamerOptions)
}

// NewStream creates a Stream instance with programmable options
func NewStream(endpoint string, options *StreamerOptions) *Stream {
	return &Stream{
		endpoint:     endpoint,
		options:      options,
		lastBlock:    0,
		ethClient:    nil,
		shutdown:     make(chan struct{}),
		errs:         make(chan error),
		transactions: make(chan *TxWithBlock),
		blocks:       make(chan *types.Header),
		shutdownOnce: &sync.Once{},
	}
}

// Stream streams transactions and or block headers coming from the ethereum blockchain
type Stream struct {
	endpoint     string             // endpoint is where the ethereum client connects to, must support wss connections
	options      *StreamerOptions   // options is the streamer options
	lastBlock    uint64             // the last block queried
	ethClient    Node               // ethClient is the client connected to an ethereum node
	shutdown     chan struct{}      // shutdown is used to close the streamer
	errs         chan error         // errs is used to forward errors coming from the streamer
	transactions chan *TxWithBlock  // transactions is the channel used to forward transactions to the parent
	blocks       chan *types.Header // blocks is the channel used to forward blocks to the parent
	shutdownOnce *sync.Once         // shutdownOnce guarantees that shutdown is only done once
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

func (s *Stream) listenBlockHeaders(headers chan *types.Header, sub ethereum.Subscription) {
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
		case err, ok := <-subErr:
			if !ok {
				subErr = nil // make this case forever blocking
				break
			}
			s.sendError(err)
		case newHeader, ok := <-headers:
			if !ok { // break due to error from sub
				headers = nil // make this case forever blocking
				break
			}
			s.onHeader(newHeader)
		}
	}
}

func (s *Stream) onHeader(header *types.Header) {
	// send header if required
	if s.options.StreamBlocks {
		select {
		case <-s.shutdown:
			return
		case s.blocks <- header:
		}
	}
	// pull transactions from block if required
	if !s.options.StreamTransactions {
		return
	}
	block, err := s.downloadBlock(header)
	if err != nil {
		s.sendError(err)
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
	defer utils.WrapErrorP(&err)
	// sleep before pulling the block, the Ethereum network is BIG, synchronization between thousands of nodes takes time
	// ethereum nodes might notify of a new incoming block but the node we have queried
	// might still be unaware of its presence.
	time.Sleep(s.options.WaitAfterHeader)
	for tryN := 0; ; {
		// check if during retries the streamer was shutdown
		select {
		case <-s.shutdown:
			return nil, errors.ErrShutdown
		default:
		}
		// loop breaks only if MaxRetries is bigger-equal than 0 and number of tries is bigger than MaxRetries
		if s.options.MaxRetries >= 0 && tryN > s.options.MaxRetries { // max retries reached
			return nil, errors.ErrMaxRetriesReached
		}
		ctx, cancel := s.ctx()
		block, err = nodeop.DownloadBlock(ctx, s.ethClient, header.Hash())
		cancel()
		if err == nil {
			return
		}
		tryN++
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
	close(s.blocks)
	close(s.transactions)
	// close eth client
	s.ethClient.Close()
}

func (s *Stream) sendError(err error) {
	select {
	case <-s.shutdown: // error was dropped due to shutdown
	case s.errs <- err: // error sent
	}
}

// Err returns a channel that forwards errors coming from the streamer
func (s *Stream) Err() <-chan error {
	return s.errs
}

// Block returns a channel that forwards block headers
func (s *Stream) Header() <-chan *types.Header {
	return s.blocks
}

// Transaction returns a channel that forwards incoming transactions
func (s *Stream) Transaction() <-chan *TxWithBlock {
	return s.transactions
}

func (s *Stream) Close() error {
	// shutdown once
	s.shutdownOnce.Do(func() {
		close(s.shutdown)
	})
	return nil
}
