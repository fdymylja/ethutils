package mocks

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"math/big"
	"sync"
	"testing"
	"time"
)

type EthereumTester interface {
	Node() interfaces.Node
	Stop()                                                       // Stop stops the ethereum tester
	SetBlockTime(duration time.Duration)                         // SetBlockTime allows to define a custom interval time for block production
	SetBlockLoader(f BlockLoader)                                // SetBlockLoader sets a custom BlockLoader function
	StartHeaders() (<-chan *types.Header, ethereum.Subscription) // StartHeaders starts the production of headers
}

// ethereumNode emulates some functions of a working ethereum node, it loads blocks from the provided blocksDir
type ethereumNode struct {
	blockLoader    BlockLoader                  // BlockLoader loads blocks
	blocks         []*types.Block               // blocks contains the testing blocks
	hashBlockMap   map[common.Hash]*types.Block // hashBlockMap maps block hashes to block contents
	numberBlockMap map[*big.Int]*types.Block    // numberBlockMap maps block numbers to block contents
	blockTime      time.Duration                // blockTime is the time that must pass before pushing another block
	currBlock      uint64                       // currBlock is the current block
	nodeOpTimeout  time.Duration                // nodeOpTimeout is the timeout of nodes operations
	test           testing.TB                   // test represents the testing instance

	headers      chan *types.Header // headers is the channel used to send headers to parent
	sendErrOnce  *sync.Once         // sendErrOnce emulates the behaviour of an ethereum subscription
	subErrs      chan error         // subErrs is the channel used to send error messages
	shutdown     chan struct{}      // shutdown signals finished to goroutines
	shutdownOnce *sync.Once         // shutdownOnce makes sure shutdown is done only once
	sendOps      *sync.WaitGroup    // sendOps synchronizes chan send operations related with shutdown
	closed       chan struct{}      // closed signals closed to Stop caller
}

func NewEthereumNode(tb testing.TB) EthereumTester {
	return &ethereumNode{
		blockLoader:   ropstenLoadBlocks(6532150, 6532170),
		blocks:        nil,
		hashBlockMap:  nil,
		blockTime:     15 * time.Second,
		currBlock:     0,
		nodeOpTimeout: 15 * time.Second,
		test:          tb,
		sendErrOnce:   &sync.Once{},
		subErrs:       make(chan error),
		shutdown:      make(chan struct{}),
		shutdownOnce:  &sync.Once{},
		headers:       make(chan *types.Header),
		sendOps:       &sync.WaitGroup{},
		closed:        make(chan struct{}),
	}
}

// Start starts the production of headers
func (t *ethereumNode) StartHeaders() (<-chan *types.Header, ethereum.Subscription) {
	t.loadBlocks(t.blockLoader)
	go t.produceHeaders()
	return t.headers, t
}

// Node returns a mock instance of a Node interface
func (t *ethereumNode) Node() interfaces.Node {
	return t
}

// Stop stops the production of content
func (t *ethereumNode) Stop() {
	t.shutdownOnce.Do(func() {
		t.test.Log("ethereumNode: Stop: shutdown recv")
		close(t.shutdown)
		t.sendOps.Wait()
		close(t.closed)
	})
	<-t.closed
}

func (t *ethereumNode) SetBlockTime(duration time.Duration) {
	t.blockTime = duration
}

func (t *ethereumNode) SetBlockLoader(f BlockLoader) {
	t.blockLoader = f
}

// TODO make this standard between streamer and ethereumNode
// loadBlocks loads blocks from files saved in the form of blockNumber.block TODO: use channel for dynamic block loading
func (t *ethereumNode) loadBlocks(f BlockLoader) {
	blocks, err := f()
	if err != nil {
		t.test.Fatal(err)
	}
	t.blocks = blocks
	t.hashBlockMap = make(map[common.Hash]*types.Block, len(blocks))
	t.numberBlockMap = make(map[*big.Int]*types.Block, len(blocks))
	for _, block := range blocks {
		t.hashBlockMap[block.Hash()] = block
		t.numberBlockMap[block.Number()] = block
	}
	t.test.Log("ethereumNode: successfully loaded blocks")
}

// produceHeaders produces headers
func (t *ethereumNode) produceHeaders() {
	for {
		t.sendOps.Add(1)
		select {
		case <-t.shutdown:
			t.test.Log("ethereumNode: produceHeaders: shutdown recv")
			t.sendOps.Done()
			return
		case <-time.After(t.blockTime):
			// generate a block
			t.headers <- t.generateHeaders()
			t.sendOps.Done()
		}

	}
}

// generateHeaders generates new headers based on a mocked current block number
// if the max number of blocks is reached it will restart iterating from the first one
func (t *ethereumNode) generateHeaders() *types.Header {
	// if we reach the max number of blocks available then restart iterating from the first one
	if t.currBlock == uint64(len(t.blocks)-1) {
		t.currBlock = 0
	}
	header := t.blocks[t.currBlock].Header()
	// increase current block
	t.currBlock += 1
	return header
}

func (t *ethereumNode) BlockByHash(ctx context.Context, hash common.Hash) (block *types.Block, err error) {
	block, ok := t.hashBlockMap[hash]
	if !ok {
		return nil, ethereum.NotFound
	}
	return
}

func (t *ethereumNode) BlockByNumber(ctx context.Context, n *big.Int) (block *types.Block, err error) {
	block, ok := t.numberBlockMap[n]
	if !ok {
		return nil, ethereum.NotFound
	}
	return
}

func (t *ethereumNode) Close() {
	t.test.Log("ethereumNode: Close")
}

func (t *ethereumNode) SubscribeNewHead(ctx context.Context, headers chan<- *types.Header) (sub ethereum.Subscription, err error) {
	return nil, errors.New("not implemented")
}

func (t *ethereumNode) Unsubscribe() {
	t.sendError(nil)
}

func (t *ethereumNode) Err() <-chan error {
	return t.subErrs
}

func (t *ethereumNode) sendError(err error) {
	t.test.Logf("ethereumNode: sendError: %s", err)
	t.sendErrOnce.Do(func() {
		if err == nil {
			close(t.subErrs)
			return
		}
		select {
		case t.subErrs <- err:
		case <-t.shutdown:
			t.test.Logf("ethereumNode: sendError: error dropped due to shutdown: %s", err)
		}
	})
}
