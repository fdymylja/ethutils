package broadcast

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"sync"
)

type txWaitRequest struct {
	hash common.Hash
	resp chan *interfaces.TxWithBlock
}

// TransactionWaiter defines a type that allows to perform operations while waiting recv
type TransactionWaiter struct {
	once        *sync.Once
	request     *txWaitRequest
	stopWaiting chan struct{}
	send        chan *interfaces.TxWithBlock
	errs        chan error
}

// Wait blocks until the recv has arrived
func (t *TransactionWaiter) Wait() *interfaces.TxWithBlock {
	return <-t.send
}

func (t *TransactionWaiter) Transaction() <-chan *interfaces.TxWithBlock {
	return t.send
}

func (t *TransactionWaiter) Err() <-chan error {
	return t.errs
}

// Stop stops the waiter, once stopping
func (t *TransactionWaiter) Stop() {
	t.once.Do(func() {
		close(t.stopWaiting)
	})
}

// Awaiter waits for blocks and/or transactions
type Awaiter struct {
	stream interfaces.Streamer

	txWaiters map[common.Hash]map[chan *interfaces.TxWithBlock]struct{}
	// request channels
	waitRequests chan *txWaitRequest
	//
	txStopWaiting chan *txWaitRequest
	// external
	errs chan error
	// synchronization
	shutdown     chan struct{}
	shutdownOnce *sync.Once
	closed       chan struct{}
}

// NewAwaiter builds an Awaiter instance given a streamer
func NewAwaiter(streamer interfaces.Streamer) *Awaiter {
	a := &Awaiter{
		stream:       streamer,
		txWaiters:    make(map[common.Hash]map[chan *interfaces.TxWithBlock]struct{}),
		waitRequests: make(chan *txWaitRequest),
		errs:         make(chan error),
		shutdown:     make(chan struct{}),
		shutdownOnce: new(sync.Once),
		closed:       make(chan struct{}),
	}
	go a.loop()
	return a
}

// wait routes events coming from channel and routes them to their respective handlers
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
		case c := <-a.txStopWaiting:
			a.stopWaitingTransaction(c)

		}
	}
}

func (a *Awaiter) onBlock(block *types.Block) {

}

func (a *Awaiter) onHeader(header *types.Header) {

}

//
func (a *Awaiter) stopWaitingTransaction(remove *txWaitRequest) {
	// check if notifier is inside otherwise panic
	_, ok := a.txWaiters[remove.hash][remove.resp]
	if !ok {
		panic("request to remove was not found")
	}
	//
	delete(a.txWaiters[remove.hash], remove.resp)
}

// onTransaction is called when a new tx arrives
func (a *Awaiter) onTransaction(tx *interfaces.TxWithBlock) {
	// TODO add block reorg handler
	// get tx hash
	hash := tx.Transaction.Hash()
	// see if tx hash is in the list of recv listeners
	listeners, ok := a.txWaiters[hash]
	// case no one is listening return
	if !ok {
		return
	}
	// wait through listeners and forward the transactions to their listening channels
	for listener := range listeners {
		listener <- tx
		close(listener)
	}
	// delete listeners from waiting list
	delete(a.txWaiters, hash)
}

// newWaitRequests handles new notify for transactions request
func (a *Awaiter) newWaitRequest(req *txWaitRequest) {
	// check if map is nil
	if a.txWaiters[req.hash] == nil {
		a.txWaiters[req.hash] = make(map[chan *interfaces.TxWithBlock]struct{})
	}
	// append new request to list
	a.txWaiters[req.hash][req.resp] = struct{}{}
}

// Wait takes a hash and waits for it to be broadcast on the ethereum network
func (a *Awaiter) Wait(tx common.Hash) *TransactionWaiter {

	// create request
	req := &txWaitRequest{
		hash: tx,
		resp: make(chan *interfaces.TxWithBlock, 1),
	}
	// create response and add request
	waitResp := &TransactionWaiter{
		request:     req,
		stopWaiting: make(chan struct{}),
		send:        make(chan *interfaces.TxWithBlock, 1),
		errs:        make(chan error, 1),
		once:        new(sync.Once),
	}
	// send request
	go a.waitResponse(waitResp)
	// return
	return waitResp
}

// waitResponse sends a wait recv request
func (a *Awaiter) waitResponse(waiter *TransactionWaiter) {
	// send request
	select {
	case <-a.shutdown: // case shutdown send error and return
		waiter.errs <- status.ErrClosed
		return
	case a.waitRequests <- waiter.request:
	}
	// wait response
	select {
	case <-a.shutdown:
		// instance was shutdown
		waiter.errs <- status.ErrShutdown
	case <-waiter.stopWaiting:
		// case the user wants to stop waiting TODO
		a.txStopWaiting <- waiter.request
	case tx, ok := <-waiter.request.resp:
		if !ok { // case channel was closed it means we're in for a shutdown
			waiter.errs <- status.ErrShutdown
			return
		}
		// send response to user
		waiter.send <- tx
	}
}

func (a *Awaiter) cleanup() {
	a.stream.Close()
	a.removeWaiters()
	close(a.closed)
}

// removeWaiters removes notification channels
func (a *Awaiter) removeWaiters() {
	for _, waiters := range a.txWaiters {
		for waiter := range waiters {
			close(waiter)
		}
	}
}

// onError forwards errors to parent and drops them in case the instance is shutdown
func (a *Awaiter) onError(err error) {
	select {
	case <-a.shutdown: // dropped error
	case a.errs <- err: // forward error to parent
	}
}

// Close shuts down the Awaiter instance, the operation will be performed only once, subsequent calls will not make changes
func (a *Awaiter) Close() error {
	a.shutdownOnce.Do(func() {
		close(a.shutdown)
	})
	<-a.closed
	return nil
}

// Err returns a channel that forwards errors coming from the Awaiter instance, there should always be a goroutine listening
// for those errors as those error forwards are blocking
func (a *Awaiter) Err() <-chan error {
	return a.errs
}
