package listeners

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"sync"
)

// Transaction awaits a transaction, it can also take a number of confirmation blocks required to be witnessed to
// consider the transaction confirmed.
type Transaction struct {
	streamer           interfaces.Streamer          // streamer forwards events from the ethereum network
	mu                 sync.Mutex                   // mu is used for sync purposes
	alive              bool                         // alive signals if the instance is running or not
	txID               common.Hash                  // txID is the transaction ID we're looking for
	txFound            bool                         // txFound marks if the transaction was found or not
	tx                 *interfaces.TxWithBlock      // tx is the content of the transaction the instance is looking for
	confirmationBlocks uint64                       // confirmationBlocks defines how many blocks after insertion we need to witness before considering the transaction confirmed
	sendTxFound        chan *interfaces.TxWithBlock // sendTxFound forwards the incoming transaction the instance is looking for
	shutdown           chan struct{}                // shutdown synchronizes exit operations
	shutdownOnce       sync.Once                    // shutdownOnce makes sure shutdown is performed only once
	signalClosed       chan struct{}                // signalClosed signals to Close function that the instance was shutdown correctly
	errs               chan error                   // errs forwards one error to the parent
	sendErrOnce        sync.Once                    // sendErrOnce makes sure only one error is sent
}

// NewTransaction builds a new transaction listener, it takes a streamer, a transaction ID and (optional) the number of
// confirmation blocks required to deem the transaction finalized
func NewTransaction(streamer interfaces.Streamer, txID common.Hash, confirmationBlocks ...uint64) *Transaction {
	var confirmBlocks uint64 = 0
	// check for confirmation blocks
	if len(confirmationBlocks) != 0 && confirmationBlocks[0] != 0 {
		confirmBlocks = confirmationBlocks[0]
	}
	// return instance
	t := &Transaction{
		streamer:           streamer,
		mu:                 sync.Mutex{},
		alive:              false,
		txID:               txID,
		txFound:            false,
		tx:                 nil,
		confirmationBlocks: confirmBlocks,
		sendTxFound:        make(chan *interfaces.TxWithBlock),
		shutdown:           make(chan struct{}),
		shutdownOnce:       sync.Once{},
		signalClosed:       make(chan struct{}),
		errs:               make(chan error),
		sendErrOnce:        sync.Once{},
	}
	go t.loop()
	return t
}

// loop
func (t *Transaction) loop() {
	// when this function exits cleanup
	defer t.cleanup()
	for {
		select {
		case <-t.shutdown: // case Close is called
			return
		case err := <-t.streamer.Err(): // case an error is received from streamer
			t.sendError(err)
			return
		case header := <-t.streamer.Header(): // case a header is received, exit signals that what the instance had to done was done and we can stop working
			exit := t.onHeader(header)
			if exit {
				return
			}
		case tx := <-t.streamer.Transaction(): // case a transaction is received, exit signals to quit
			exit := t.onTransaction(tx)
			if exit {
				return
			}
		case block := <-t.streamer.Block(): // case a block is received, exit signals to quit
			exit := t.onBlock(block)
			if exit {
				return
			}
		}
	}
}

// onBlockNumberUpdate handles operations when a block update is received, it checks if the blocks witnessed are enough
// to deem a transaction finalized
func (t *Transaction) onBlockNumberUpdate(blockNumber uint64) (exit bool) {
	// check if we need confirmation blocks, otherwise return, with exit state = false
	if t.confirmationBlocks == 0 {
		return false
	}
	// check if the transaction was found, if not return with exit state = false
	if !t.txFound {
		return false
	}
	// if the tx was found, check how many blocks we've witnessed as of now
	witnessedBlocks := int64(blockNumber - t.tx.BlockNumber) // use an int64 to support the case of big reorg events in which we can have a negative witnessedBlock value
	// check if the number of witnessed blocks since tx insert is lower than the number of confirmation blocks
	if witnessedBlocks < int64(t.confirmationBlocks) {
		return false
	}
	// if the number of witnessed blocks is higher or equal then forward transaction
	t.forwardTransaction()
	// return with exit status = true
	return true
}

func (t *Transaction) onTransaction(tx *interfaces.TxWithBlock) (exit bool) {
	// check if transaction hash matches the one we're looking for, if not ignore and return
	if tx.Transaction.Hash() != t.txID {
		return false
	}
	// case the transaction id matches
	t.tx = tx
	t.txFound = true
	// check if the instance has to wait for blocks confirmation
	if t.confirmationBlocks != 0 {
		return false
	}
	// if it doesn't have to listen for blocks confirmation then forward transaction
	t.forwardTransaction()
	// since we forwarded the information we were looking for, return a positive quit signal
	return true
}

// forwardTransaction takes care of forwarding the transaction the listener is looking for
func (t *Transaction) forwardTransaction() {
	if t.tx == nil {
		panic("tx is nil while forward was being called")
	}
	select {
	case t.sendTxFound <- t.tx:
	case <-t.shutdown:
		return
	}
}

// sendError takes care of forwarding an error once
func (t *Transaction) sendError(err error) {
	t.sendErrOnce.Do(func() {
		t.errs <- err
		close(t.errs)
	})
}

// cleanup can only be called by loop, and if it is called it means that loop has returned
func (t *Transaction) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.alive = false
	t.sendError(status.ErrShutdown)
	t.streamer.Close()
	// signal closed to Close() caller
	close(t.signalClosed)
}

// onHeader handles operations after a header is received
func (t *Transaction) onHeader(header *types.Header) bool {
	return t.onBlockNumberUpdate(header.Number.Uint64())
}

// onBlock handles operations after a block is received
func (t *Transaction) onBlock(block *types.Block) bool {
	return t.onBlockNumberUpdate(block.NumberU64())
}

// Transaction return the channel that has to forward the transaction the instance is waiting for
func (t *Transaction) Transaction() <-chan *interfaces.TxWithBlock {
	return t.sendTxFound
}

// Err returns an error channel
func (t *Transaction) Err() <-chan error {
	return t.errs
}

// WaitContext allows to wait for a transaction until all requirements are met and returns an error in case of context
// cancellation, errors from the streamer or shutdown being sent
func (t *Transaction) WaitContext(ctx context.Context) (tx *interfaces.TxWithBlock, err error) {
	select {
	case tx = <-t.Transaction(): // wait for the tx
	case err2, ok := <-t.Err(): // wait for an error
		if !ok { // in case the channel is closed, then return  status.ErrShutdown
			err = status.ErrShutdown
			return
		}
		err = err2
	case <-ctx.Done(): // wait for context cancellation
		err = ctx.Err()
	}
	return
}

// Wait waits for a transaction until all the requirements are met (tx ID matches and block confirmations), returns an error
// only if the streamer has reported an error or a shutdown was sent
func (t *Transaction) Wait() (tx *interfaces.TxWithBlock, err error) {
	return t.WaitContext(context.Background())
}

// Close closes the transaction listener, returns an error only if the instance has already been shutdown due to error or
// Close() being already called
func (t *Transaction) Close() error {
	// lock
	t.mu.Lock()
	// check if already closed
	if !t.alive {
		t.mu.Unlock()
		return status.ErrClosed
	}
	t.alive = false
	t.shutdownOnce.Do(func() {
		close(t.shutdown)
	})
	t.mu.Unlock()
	<-t.signalClosed
	return nil
}
