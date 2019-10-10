package broadcast

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"sync"
)

type txProducer interface {
	shuttingDown() <-chan struct{} // shuttingDown signals when the tx producer is shuttingDown
	removeTxAwaiter(w *TxWaiter)   // removeTxAwaiter removes TxWaiter from the waiters list

}

// TxWaiter is a type that waits for transactions to be included in an ethereum block, it uses a txProducer to receive
type TxWaiter struct {
	hash       common.Hash                  // hash is the has of the transaction
	waiterDone chan struct{}                // waiterDone signals the waiter is shuttingDown
	stop       chan struct{}                // stop is used to signal that we should stop waiting for the transaction
	stopOnce   *sync.Once                   // stopOnce makes sure the TxWaiter is stopped only once
	errs       chan error                   // errs is used to signal the caller that there was some error
	resp       chan *interfaces.TxWithBlock // resp is used to forward transaction information
	listener   chan *interfaces.TxWithBlock // listener is the channel used to forward information to the caller

	txProducer txProducer
}

// NewTxWaiter creates a TxWaiter instance and starts the loop that waits for the transaction to be included
func NewTxWaiter(hash common.Hash, producer txProducer) *TxWaiter {
	waiter := &TxWaiter{
		txProducer: producer,
		hash:       hash,
		stop:       make(chan struct{}),
		stopOnce:   new(sync.Once),
		waiterDone: make(chan struct{}),
		errs:       make(chan error, 1),
		resp:       make(chan *interfaces.TxWithBlock, 1),
		listener:   make(chan *interfaces.TxWithBlock, 1),
	}
	go waiter.wait()
	return waiter
}

// wait waits for events regarding TxWaiter
func (w *TxWaiter) wait() {
	select {
	// case the transaction producer is shuttingDown exit sending a shutdown error
	case <-w.txProducer.shuttingDown():
		w.errs <- status.ErrShutdown
	// case the instance is stopped by the instance waiting for a transaction
	case <-w.stop:
		w.txProducer.removeTxAwaiter(w) // remove the txWaiter instance from the list of waiters
		w.done()
	// case transaction is found
	case tx := <-w.resp:
		w.listener <- tx
	}
}

// shuttingDown is called when Stop is called and signals the parent user that the TxWaiter has exited
func (w *TxWaiter) done() {
	close(w.waiterDone)
}
