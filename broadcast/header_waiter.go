package broadcast

import (
	"github.com/ethereum/go-ethereum/core/types"
	"sync"
)

type headerProducer interface {
	shuttingDown() <-chan struct{}      // shuttingDown signals when the headerProducer is shutting down
	removeHeaderWaiter(w *HeaderWaiter) // removeHeaderWaiter is the function that removes HeaderWaiter from the headers waiter list
}

// HeaderWaiter waits for new headers
type HeaderWaiter struct {
	//
	blockNumber uint64 // blockNumber is the block number we are looking for

	waiterDone    chan struct{}      // waiterDone signals when the waiter is shutting down
	stop          chan struct{}      // stop is called externally to signal that HeaderWaiter has to stop waiting
	stopOnce      *sync.Once         // stopOnce makes sure Stop() is called only once
	errs          chan error         // parentError is used to signal the caller that there was some error
	resp          chan *types.Header // resp is used by the headerProducer to forward the new block
	blockInserted chan *types.Header // blockInserted signals the user that the block was included

	headerProducer headerProducer // headerProducer is the parent instance that forwards the headers we're looking for
}
