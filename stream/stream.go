package stream

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/nodeop"
	"github.com/fdymylja/ethutils/status"
	"sync"
	"time"
)

// DefaultStreamOptions defines the default options for the Stream
var DefaultStreamOptions = &Options{
	NodeOpTimeout:      15 * time.Second,
	MaxRetries:         50,
	RetryWait:          5 * time.Second,
	WaitAfterHeader:    5 * time.Second,
	StreamHeaders:      true,
	StreamTransactions: true,
}

// Options represents the options used for Client
type Options struct {
	// NodeOpTimeout is the timeout for node query operations
	NodeOpTimeout time.Duration
	// MaxRetries is the number of retries to make while querying headers content (-1 equals retry infinitely)
	MaxRetries int
	// RetryWait is how long we wait before retrying again
	RetryWait time.Duration
	// WaitAfterHeader is how long the client waits before querying block contents
	WaitAfterHeader time.Duration
	// StreamBlocks is used to define if blocks should be forwarded to the parent
	StreamBlocks bool
	// StreamHeaders is used to define if headers should be forwarded to the parent
	StreamHeaders bool
	// StreamTransactions is used to define if transactions forwarded to the parent
	StreamTransactions bool
}

// NewClient generates a new Client instance taking the node endpoint and the options
func NewClient(endpoint string, options *Options) *Client {
	return &Client{
		mu:       new(sync.Mutex),
		endpoint: endpoint,
		options:  options,
	}
}

// NewClientDefault returns a client taking an endpoint and the default options
func NewClientDefault(endpoint string) *Client {
	return NewClient(endpoint, DefaultStreamOptions)
}

// Client is an ethereum events streamer, it connects to an ethereum node and pulls information regarding headers,
// transactions or entire blocks too. It is concurrency safe and can be used multiple times as long as Close() is called
// before the next Connect(). It forwards only one error, the error can come from the ethereum subscription or from
// trying to pull the block data. If Close() is called, and there were no prior errors, it will also forward a ErrShutdown
// to signal that the instance has exited due to shutdown. This error can be safely ignored.
type Client struct {
	blocks       chan *types.Block
	headers      chan *types.Header
	transactions chan *interfaces.TxWithBlock

	sendErrorOnce *sync.Once
	errs          chan error
	shutdown      chan struct{}
	loopExit      chan struct{}

	endpoint string
	options  *Options

	mu        *sync.Mutex
	connected bool
	client    interfaces.Node
}

// init instantiates the types necessary for the streamer to work, this function makes the streamer re-usable
func (c *Client) init() {
	c.headers = make(chan *types.Header)
	c.blocks = make(chan *types.Block)
	c.transactions = make(chan *interfaces.TxWithBlock)

	c.shutdown = make(chan struct{})
	c.loopExit = make(chan struct{})
	c.errs = make(chan error, 1)
	c.sendErrorOnce = new(sync.Once)
}

// context returns a Context and a CancelFunc, the context is used to query ethereum nodes with a specified timeout
func (c *Client) context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.options.NodeOpTimeout)
}

// Connect connects the instance to the ethereum network and starts the loop that forwards information
func (c *Client) Connect() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// check if closed
	if c.connected {
		return status.ErrAlreadyConnected
	}
	// if it is not closed init
	c.init()
	// connect to ethereum node
	ctx, cancel := c.context()
	defer cancel()
	c.client, err = ethclient.DialContext(ctx, c.endpoint)
	if err != nil {
		return
	}
	// subscribe to headers
	ctx, cancel = c.context()
	defer cancel()
	headers := make(chan *types.Header)
	sub, err := c.client.SubscribeNewHead(ctx, headers)
	if err != nil {
		return
	}
	// start main loop
	go c.loop(headers, sub)
	// at the end set closed to true
	c.connected = true
	// return
	return
}

// loop loops through node events using an ethereum subscription
func (c *Client) loop(headers <-chan *types.Header, sub ethereum.Subscription) {
	defer close(c.loopExit)
	defer sub.Unsubscribe()
	for {
		select {
		case <-c.shutdown:
			return
		case err := <-sub.Err():
			c.sendError(err)
			return
		case header := <-headers:
			err := c.onHeader(header)
			if err != nil {
				c.sendError(err)
				return
			}
		}
	}
}

// onHeader handles operations done when a header is received
func (c *Client) onHeader(header *types.Header) (err error) {
	// check if the instance has to stream headers
	if c.options.StreamHeaders {
		select {
		case <-c.shutdown:
			return
		case c.headers <- header:
		}
	}
	// check if the instance has to stream blocks or transactions
	if !(c.options.StreamBlocks || c.options.StreamTransactions) {
		return
	}
	// pull the block
	block, err := c.downloadBlock(header)
	if err != nil {
		// wrap error into download block error
		err = &ErrDownloadBlock{
			BlockNumber: header.Number.Uint64(),
			Err:         err,
		}
		// return
		return
	}
	// check if instance has to stream blocks
	if c.options.StreamBlocks {
		select {
		case <-c.shutdown:
			return
		case c.blocks <- block:
		}
	}
	// check if instance has to stream transactions
	if c.options.StreamTransactions {
		for _, tx := range block.Transactions() {
			select {
			case <-c.shutdown:
				return
			case c.transactions <- &interfaces.TxWithBlock{
				Transaction: tx,
				BlockNumber: block.NumberU64(),
				Timestamp:   block.Time(),
			}:
			}
		}
	}
	return nil
}

// downloadBlock pulls block data from ethereum node including transactions, it retries a total amount of times
// equivalent to Options.MaxRetries before returning an error
func (c *Client) downloadBlock(header *types.Header) (block *types.Block, err error) {
	// sleep before pulling the block, the Ethereum network is BIG, synchronization between thousands of nodes takes time
	// ethereum nodes might notify of a new incoming block but the node we have queried  might still be unaware of its presence.
	time.Sleep(c.options.WaitAfterHeader)
	var lastError error
	for tryN := 0; ; {
		// check if during retries the Streamer was shutdown
		select {
		case <-c.shutdown:
			return nil, status.ErrShutdown // if it was shutdown return the error
		default:
		}
		// wait breaks only if MaxRetries is bigger-equal than 0 and number of tries is bigger than MaxRetries
		if c.options.MaxRetries != -1 && tryN >= c.options.MaxRetries { // max retries reached
			return nil, &status.ErrMaxRetriesReached{
				LastError: lastError,
			}
		}
		ctx, cancel := c.context()
		block, lastError = nodeop.DownloadBlock(ctx, c.client, header.Hash())
		cancel()
		if lastError == nil {
			return
		}
		tryN++
		time.Sleep(c.options.RetryWait)
	}
}

// sendError sends an error only one time and closes the errors channel
func (c *Client) sendError(err error) {
	c.sendErrorOnce.Do(func() {
		c.errs <- err
		close(c.errs)
	})
}

// Close frees the resources held by Client, it should always be called once the instance is not used anymore event after
// an error is forwarded from Err(), the error returned is status.ErrNotConnected in case the c
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// check if closed
	if !c.connected {
		return status.ErrNotConnected
	}
	// if not clean up and free resources

	// first exit the loop
	close(c.shutdown)
	// wait for the main loop to exit
	<-c.loopExit
	// close ethereum client
	c.client.Close()
	// send shutdown error to parent
	c.sendError(status.ErrShutdown)
	// set closed to false
	c.connected = false
	// return
	return nil
}

// Err returns a channel that forwards errors coming from the Streamer; per ethereum.Subscription, only one error is
// forwarded to this channel
func (c *Client) Err() <-chan error {
	return c.errs
}

// Block returns the channel used to forward blocks
func (c *Client) Block() <-chan *types.Block {
	return c.blocks
}

// Header returns a channel that forwards block headers
func (c *Client) Header() <-chan *types.Header {
	return c.headers
}

// Transaction returns a channel that forwards incoming transactions
func (c *Client) Transaction() <-chan *interfaces.TxWithBlock {
	return c.transactions
}

// ErrDownloadBlock defines a block download error
type ErrDownloadBlock struct {
	BlockNumber uint64
	Err         error
}

// Error implements error interface
func (e *ErrDownloadBlock) Error() string {
	return fmt.Sprintf("failure in pulling block %d: %s", e.BlockNumber, e.Err)
}

// Unwrap implements errors.Unwrapper
func (e *ErrDownloadBlock) Unwrap() error {
	return e.Err
}
