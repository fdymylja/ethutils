package broadcast

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/fdymylja/ethutils/interfaces"
	"sync"
)

// Awaiter allows to wait for transactions on the ethereum network
type Awaiter struct {
	streamer interfaces.Streamer
	mu       *sync.Mutex

	errs            chan error    // parentError forwards errors to parent
	shutdown        chan struct{} // shutdown signals the Awaiter to exit
	shutdownWaiters chan struct{} // shutdownWaiters signals every waiter to exit
	shutdownOnce    *sync.Once    // shutdownOnce makes sure shutdown op can be only performed one time

	txWaiters map[common.Hash]map[*TxWaiter]struct{} // txWaiters maps a transaction hash to a set of TxWaiters

}

// NewAwaiter generates a new Awaiter instance taking an interfaces.Streamer
func NewAwaiter(streamer interfaces.Streamer) *Awaiter {
	a := &Awaiter{
		errs:            make(chan error, 1),
		txWaiters:       make(map[common.Hash]map[*TxWaiter]struct{}),
		streamer:        streamer,
		shutdown:        make(chan struct{}),
		shutdownWaiters: make(chan struct{}),
		mu:              new(sync.Mutex),
		shutdownOnce:    new(sync.Once),
	}
	go a.loop()
	return a
}

func (a *Awaiter) loop() {
	for {
		select {
		case <-a.shutdown:
			a.cleanup()
			return
		case err := <-a.streamer.Err():
			a.onError(err)
		case tx := <-a.streamer.Transaction():
			a.onTransaction(tx)
		case _ = <-a.streamer.Block():
		case _ = <-a.streamer.Header():

		}
	}
}

func (a *Awaiter) onTransaction(tx *interfaces.TxWithBlock) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// extract the list of waiters
	waiters, ok := a.txWaiters[tx.Transaction.Hash()]
	// in case no one is waiting for this transaction exit
	if !ok {
		return
	}
	// iterate through waiters and send the transaction
	for waiter := range waiters {
		waiter.sendResponse(tx)
	}
	// remove the waiters
	delete(a.txWaiters, tx.Transaction.Hash())
}

// removeTxAwaiter attempts at removing a TxAwaiter from the wait list
func (a *Awaiter) removeTxWaiter(waiter *TxWaiter) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// check if hash is present in the waiters list
	waiters, ok := a.txWaiters[waiter.Hash()]
	if !ok {
		return
	}
	// check if waiter is present in the waiters list
	_, ok = waiters[waiter]
	if !ok {
		return
	}
	delete(waiters, waiter)
	waiter.waiterRemoved()
}

// NotifyTransaction takes a transaction hash and waits for it to be broadcast on the ethereum network
func (a *Awaiter) NotifyTransaction(hash common.Hash) *TxWaiter {
	waiter := NewTxWaiter(hash, a)
	a.mu.Lock()
	defer a.mu.Unlock()
	// check if map of waiters related to the hash exists, if it doesn't create it
	if a.txWaiters[hash] == nil {
		a.txWaiters[hash] = make(map[*TxWaiter]struct{})
	}
	// put the waiter in the set
	a.txWaiters[hash][waiter] = struct{}{}
	// return
	return waiter
}

// shuttingDown signals to TxWaiter when the instance closes so it can free resources
func (a *Awaiter) shuttingDown() <-chan struct{} {
	return a.shutdownWaiters
}

// onError sends errors to parent
func (a *Awaiter) onError(err error) {
	select {
	case a.errs <- err:
	case <-a.shutdown:
	}
}

func (a *Awaiter) cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()
	close(a.shutdownWaiters)
}

// Close closes the instance, it can only be waiterRemoved once, other calls will do nothing
func (a *Awaiter) Close() error {
	a.shutdownOnce.Do(func() {
		close(a.shutdown)
	})
	return nil
}

// Err forwards errors from Awaiter
func (a *Awaiter) Err() <-chan error {
	return a.errs
}
