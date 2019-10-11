package broadcast

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"github.com/status-im/keycard-go/hexutils"
	"sync"
)

type txProducer interface {
	shuttingDown() <-chan struct{} // shuttingDown signals when the tx producer is shuttingDown
	removeTxWaiter(w *TxWaiter)    // removeTxWaiter removes TxWaiter from the waiters list

}

// TxWaiter is a type that waits for transactions to be included in an ethereum block, it uses a txProducer to receive
// data regarding the transaction
type TxWaiter struct {
	hash       common.Hash                  // hash is the has of the transaction
	waiterDone chan struct{}                // waiterDone signals the waiter is shuttingDown
	stop       chan struct{}                // stop is used to signal that we should stop waiting for the transaction
	stopOnce   *sync.Once                   // stopOnce makes sure the TxWaiter is stopped only once
	errs       chan error                   // errs is used to signal the caller that there was some error
	resp       chan *interfaces.TxWithBlock // resp is used to forward transaction information
	txIncluded chan *interfaces.TxWithBlock // txIncluded is the channel used to forward information to the caller

	txProducer txProducer // txProducer is the parent instance that forwards
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
		txIncluded: make(chan *interfaces.TxWithBlock, 1),
	}
	go waiter.wait()
	return waiter
}

// wait waits for events regarding TxWaiter, only one event will pass through
func (w *TxWaiter) wait() {
	select {
	// case the transaction producer is shuttingDown exit sending a shutdown error
	case <-w.txProducer.shuttingDown():
		w.errs <- status.ErrShutdown
	// case the instance is stopped by the instance waiting for a transaction
	case <-w.stop:
		w.txProducer.removeTxWaiter(w) // remove the txWaiter instance from the list of waiters
	// case transaction is found, forward it to caller
	case tx := <-w.resp:
		w.txIncluded <- tx
	}
}

// waiterRemoved is called by Awaiter to signal that the waiter has been successfully removed
func (w *TxWaiter) waiterRemoved() {
	close(w.waiterDone)
}

// sendResponse should only be used by the txProducer to signal that the transaction has been included
func (w *TxWaiter) sendResponse(tx *interfaces.TxWithBlock) {
	// check if the hash we got matches the hash, TODO: think if maybe here we can avoid a panic and just forward an error
	if !bytes.Equal(w.hash[:], tx.Transaction.Hash().Bytes()) {
		panic(fmt.Sprintf("Tx hash expected: %s, got: %s", hexutils.BytesToHex(w.hash[:]), hexutils.BytesToHex(tx.Transaction.Hash().Bytes())))
	}
	w.resp <- tx
}

// TransactionIncluded forwards the transaction to the listener once it is included in a block
func (w *TxWaiter) TransactionIncluded() <-chan *interfaces.TxWithBlock {
	return w.txIncluded
}

// Stop stops the waiter, it should be always called when there is not anymore interest in waiting a transaction
func (w *TxWaiter) Stop() {
	w.stopOnce.Do(func() {
		close(w.stop)
	})
}

// Done sends a signal once the TxWaiter has exited, this is sent only after Stop is called to communicate that the
// txProducer has removed the waiter
func (w *TxWaiter) Done() <-chan struct{} {
	return w.waiterDone
}

// Err is used to communicate errors to the listener of TxWaiter, generally the errors are status.ErrShutdown
// when the txProducer is closed, only one error is forwarded
func (w *TxWaiter) Err() <-chan error {
	return w.errs
}

// Hash returns the hash of the transaction TxWaiter is waiting
func (w *TxWaiter) Hash() common.Hash {
	return w.hash
}
