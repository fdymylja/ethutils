package listeners

import (
	"context"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"sync"
)

// BlockWaiter takes an interfaces.Streamer and a block number and forwards the block to the blockReceived channel, there are different
// ways to wait for the block, one is to listen for it using Block(), otherwise it is possible to Wait() for the block
// or WaitContext(context.Context), BlockWaiter will forward one error only, the error received will be forward in case you're listening
// for errors using Err(), or to WaitContext() and Wait(). It is better to wait for a block using only one path.
type BlockWaiter struct {
	streamer  interfaces.Streamer
	waitBlock uint64

	mu            *sync.Mutex
	terminated    bool
	blockReceived chan *types.Block
	errs          chan error
	sendErrOnce   *sync.Once
	shutdown      chan struct{}
	closeOnce     *sync.Once
	cleanupDone   chan struct{}
}

// NewBlock generates a new BlockWaiter instance, taking a streamer and the blockNumber we're looking for.
func NewBlock(streamer interfaces.Streamer, blockNumber uint64) *BlockWaiter {
	b := &BlockWaiter{
		streamer:      streamer,
		waitBlock:     blockNumber,
		mu:            new(sync.Mutex),
		terminated:    false,
		blockReceived: make(chan *types.Block), // make the send op blocking so we're sure that the block is forwarded before exit
		errs:          make(chan error, 1),
		sendErrOnce:   new(sync.Once),
		shutdown:      make(chan struct{}),
		closeOnce:     new(sync.Once),
		cleanupDone:   make(chan struct{}),
	}
	go b.loop()
	return b
}

func (b *BlockWaiter) loop() {
	defer b.cleanup()
	for {
		select {
		case <-b.shutdown:
			return
		case err := <-b.streamer.Err(): // handle error
			b.onError(err)
			return
		case block := <-b.streamer.Block(): // handle block
			exit := b.onBlock(block)
			if exit {
				return
			}
		case <-b.streamer.Transaction(): // ignore tx
		case <-b.streamer.Header(): // ignore headers
		}
	}
}

func (b *BlockWaiter) onBlock(block *types.Block) bool {
	// check if the block we received is not the one we're looking for
	if b.waitBlock != block.NumberU64() {
		return false
	}
	// if it is the one we're looking for forward it
	select {
	case b.blockReceived <- block: // forward block
	case <-b.shutdown: // in case of shutdown drop it
	}
	return true
}

func (b *BlockWaiter) onError(err error) {
	b.sendError(err)
}

func (b *BlockWaiter) sendError(err error) {
	b.sendErrOnce.Do(func() {
		b.errs <- err
		close(b.errs)
	})
}

func (b *BlockWaiter) cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()
	// send shutdown error in case the cleanup is coming from Close()
	b.sendError(status.ErrShutdown)
	// close streamer
	b.streamer.Close()
	// set instance as terminated
	b.terminated = true
	// signal cleanup done
	close(b.cleanupDone)
}

// Close closes the BlockWaiter, this operation should be done once only. The instance will close if there are errors,
// so calling Close after an error is not required, trying to Close twice will return a status.ErrClosed error.
func (b *BlockWaiter) Close() error {
	b.mu.Lock()
	if b.terminated {
		b.mu.Unlock()
		return status.ErrClosed
	}
	b.closeOnce.Do(func() {
		b.terminated = true
		close(b.shutdown)
	})
	// free lock for cleanup
	b.mu.Unlock()
	// wait for cleanup to finish
	<-b.cleanupDone
	return nil
}

// Block returns the channel that will forward the block the instance is looking for
func (b *BlockWaiter) Block() <-chan *types.Block {
	return b.blockReceived
}

// Err will return the channel that forwards errors coming from the underlying streamer or from the instance itself
func (b *BlockWaiter) Err() <-chan error {
	return b.errs
}

// WaitContext waits for a block taking a context for cancellation, if context is cancelled or expires, Close() should be
// called afterwards to free the resources, WaitContext will return only if there are errors from the underlying streamer
// or Close is called, if Close is called before block is found the function will return a status.ErrShutdown error.
// Calls after Close to WaitContext will return a status.ErrShutdown error
func (b *BlockWaiter) WaitContext(ctx context.Context) (block *types.Block, err error) {
	select {
	case block = <-b.Block():
	case err2, ok := <-b.Err():
		if !ok {
			err = status.ErrShutdown
			return
		}
		err = err2
	case <-ctx.Done():
		err = ctx.Err()
	}
	return
}

// Wait will wait indefinitely for a block, same rules as WaitContext apply except that there cannot be a context expiration.
func (b *BlockWaiter) Wait() (block *types.Block, err error) {
	return b.WaitContext(context.Background())
}
