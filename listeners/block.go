package listeners

import (
	"context"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"sync"
)

type Block struct {
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

func NewBlock(streamer interfaces.Streamer, blockNumber uint64) *Block {
	b := &Block{
		streamer:      streamer,
		waitBlock:     blockNumber,
		mu:            new(sync.Mutex),
		terminated:    false,
		blockReceived: make(chan *types.Block),
		errs:          make(chan error, 1),
		sendErrOnce:   new(sync.Once),
		shutdown:      make(chan struct{}),
		closeOnce:     new(sync.Once),
		cleanupDone:   make(chan struct{}),
	}
	go b.loop()
	return b
}

func (b *Block) loop() {
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

func (b *Block) onBlock(block *types.Block) bool {
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

func (b *Block) onError(err error) {
	b.sendError(err)
}

func (b *Block) sendError(err error) {
	b.sendErrOnce.Do(func() {
		b.errs <- err
		close(b.errs)
	})
}

func (b *Block) cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()
	// send shutdown error in case the cleanup is coming from Close()
	b.sendError(status.ErrShutdown)
	// close block forward channel
	close(b.blockReceived)
	// close streamer
	b.streamer.Close()
	// set instance as terminated
	b.terminated = true
	// signal cleanup done
	close(b.cleanupDone)
}

func (b *Block) Close() error {
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

func (b *Block) Block() <-chan *types.Block {
	return b.blockReceived
}

func (b *Block) Err() <-chan error {
	return b.errs
}

func (b *Block) WaitContext(ctx context.Context) (block *types.Block, err error) {
	select {
	case block = <-b.Block():
	case err = <-b.Err():
	case <-ctx.Done():
		err = ctx.Err()
	}
	return
}

func (b *Block) Wait() (block *types.Block, err error) {
	return b.WaitContext(context.Background())
}
