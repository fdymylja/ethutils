package mocks

import (
	"container/ring"
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/mocks/testblocks"
	"github.com/fdymylja/ethutils/status"
	"math/big"
	"sync"
)

// Node mocks a interfaces.Node
type Node struct {
	mu sync.Mutex

	closed   bool
	shutdown chan struct{}
	blocks   []*types.Block

	blockRing *ring.Ring

	activeRoutines sync.WaitGroup
}

// NewNode generates a new instance of Node
func NewNode() *Node {
	blocks := testblocks.BlocksSlice.MustDecode()
	rng := ring.New(len(blocks))
	for _, block := range blocks {
		rng.Value = block
		rng = rng.Next()
	}
	return &Node{
		mu:             sync.Mutex{},
		closed:         false,
		shutdown:       make(chan struct{}),
		blocks:         blocks,
		blockRing:      rng,
		activeRoutines: sync.WaitGroup{},
	}
}

// Subscription mocks an ethereum.Subscription
type Subscription struct {
	err          chan error
	closeErrOnce sync.Once
	shutdownOnce sync.Once
	shutdown     chan struct{}
}

// Unsubscribe closes the subscription
func (s *Subscription) Unsubscribe() {
	s.shutdownOnce.Do(func() {
		close(s.shutdown)
		s.closeErrOnce.Do(func() {
			close(s.err)
		})
	})
}

// Err returns a channel that forwards a subscription error
func (s *Subscription) Err() <-chan error {
	return s.err
}

// BlockByHash implements interfaces.Node
func (n *Node) BlockByHash(ctx context.Context, hash common.Hash) (block *types.Block, err error) {
	for _, b := range n.blocks {
		if b.Hash() == hash {
			return b, nil
		}
	}
	return nil, ethereum.NotFound
}

// BlockByNumber implements interfaces.Node
func (n *Node) BlockByNumber(ctx context.Context, blockN *big.Int) (block *types.Block, err error) {
	for _, b := range n.blocks {
		if blockN.Cmp(b.Number()) == 0 {
			return b, nil
		}
	}
	return nil, ethereum.NotFound
}

// Close closes the node
func (n *Node) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return
	}
	n.closed = true
	close(n.shutdown)
	// wait for all the active goroutines to exit
	n.activeRoutines.Wait()
}

// SubscribeNewHead implements interfaces.Node
func (n *Node) SubscribeNewHead(ctx context.Context, headers chan<- *types.Header) (sub ethereum.Subscription, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	// check if dead
	if n.closed {
		return nil, status.ErrClosed
	}
	// if not dead
	mockSub := &Subscription{
		err:          make(chan error, 1),
		closeErrOnce: sync.Once{},
		shutdownOnce: sync.Once{},
		shutdown:     make(chan struct{}),
	}
	// add one active routine
	n.activeRoutines.Add(1)
	go func() {
		defer n.activeRoutines.Done() // when the goroutine exits signal it to the wait group
		// if there are no blocks, PANEEC
		if n.blockRing.Len() == 0 {
			panic("no block is loaded")
		}
		for {
			n.blockRing.Do(func(b interface{}) {
				select {
				case <-mockSub.shutdown:
					return
				case <-n.shutdown:
					return
				case headers <- b.(*types.Block).Header():
				}
			})
		}
	}()
	return mockSub, nil
}
