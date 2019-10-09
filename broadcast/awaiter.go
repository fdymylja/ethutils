package broadcast

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/errors"
	"github.com/fdymylja/ethutils/interfaces"
	"sync"
)

type waitRequest struct {
	hash common.Hash
	resp chan *interfaces.TxWithBlock
}

type waitResponse struct {
	stopWaiting chan struct{}
	response    chan *interfaces.TxWithBlock
	errs        chan error
}

// Awaiter waits for blocks and/or transactions
type Awaiter struct {
	stream interfaces.Streamer

	txWaiters map[common.Hash][]chan *interfaces.TxWithBlock
	// request channels
	waitRequests chan *waitRequest

	// external
	errs chan error
	// synchronization
	shutdown     chan struct{}
	shutdownOnce *sync.Once
	closed       chan struct{}
}

func NewAwaiter(streamer interfaces.Streamer) *Awaiter {
	a := &Awaiter{
		stream:       streamer,
		txWaiters:    make(map[common.Hash][]chan *interfaces.TxWithBlock),
		waitRequests: make(chan *waitRequest),
		errs:         make(chan error),
		shutdown:     make(chan struct{}),
		shutdownOnce: new(sync.Once),
		closed:       make(chan struct{}),
	}
	go a.loop()
	return a
}

// loop routes events coming from channel and routes them to their respective handlers
func (a *Awaiter) loop() {
	for {
		select {
		case <-a.shutdown:
			a.cleanup()
			return
		case err := <-a.stream.Err():
			a.onError(err)
		case block := <-a.stream.Block():
			a.onBlock(block)
		case tx := <-a.stream.Transaction():
			a.onTransaction(tx)
		case header := <-a.stream.Header():
			a.onHeader(header)
		case req := <-a.waitRequests:
			a.newWaitRequest(req)
		}
	}
}

func (a *Awaiter) onBlock(block *types.Block) {

}

func (a *Awaiter) onHeader(header *types.Header) {

}

// onTransaction is called when a new tx arrives
func (a *Awaiter) onTransaction(tx *interfaces.TxWithBlock) {
	// TODO add block reorg handler
	// get tx hash
	hash := tx.Transaction.Hash()
	// see if tx hash is in the list of transaction listeners
	listeners, ok := a.txWaiters[hash]
	// case no one is listening return
	if !ok {
		return
	}
	// loop through listeners and forward the transactions to their listening channels
	for _, listener := range listeners {
		listener <- tx
		close(listener)
	}
	// delete listeners from waiting list
	delete(a.txWaiters, hash)
}

// newWaitRequests handles new notify for transactions request
func (a *Awaiter) newWaitRequest(req *waitRequest) {
	// append new request to list
	a.txWaiters[req.hash] = append(a.txWaiters[req.hash], req.resp)
}

func (a *Awaiter) Wait(tx common.Hash) (<-chan *interfaces.TxWithBlock, error) {
	// make response channel
	resp := make(chan *interfaces.TxWithBlock, 1)
	// make request
	req := &waitRequest{
		hash: tx,
		resp: resp,
	}
	// try to send request
	select {
	case a.waitRequests <- req:
	case <-a.shutdown:
		err := errors.ErrShutdown
		return nil, err
	}
	// return
	return resp, nil
}

func (a *Awaiter) cleanup() {
	a.stream.Close()
	a.removeWaiters()
	close(a.closed)
}

func (a *Awaiter) removeWaiters() {
	for _, waiters := range a.txWaiters {
		for _, waiter := range waiters {
			close(waiter)
		}
	}
}

func (a *Awaiter) onError(err error) {
	select {
	case <-a.shutdown: // dropped error
	case a.errs <- err: // forward error to parent
	}
}

func (a *Awaiter) Close() error {
	a.shutdownOnce.Do(func() {
		close(a.shutdown)
	})
	<-a.closed
	return nil
}

func (a *Awaiter) Err() <-chan error {
	return a.errs
}
