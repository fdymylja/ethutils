package listeners

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"sync"
)

type Transaction struct {
	streamer interfaces.Streamer

	mu sync.Mutex

	txID               common.Hash             // txID is the transaction ID we're looking for
	txFound            bool                    // txFound marks if the transaction was found or not
	tx                 *interfaces.TxWithBlock // tx is the content of the transaction the instance is looking for
	confirmationBlocks uint64                  // confirmationBlocks defines how many blocks after insertion we need to witness before considering the transaction confirmed

	sendTxFound chan *interfaces.TxWithBlock

	shutdown    chan struct{} // shutdown synchronizes exit operations
	errs        chan error
	sendErrOnce sync.Once
}

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
		txID:               txID,
		confirmationBlocks: confirmBlocks,
		shutdown:           make(chan struct{}),
	}
	go t.loop()
	return t
}

func (t *Transaction) loop() {
	defer t.cleanup()
	for {
		select {
		case <-t.shutdown:
			return
		case err := <-t.streamer.Err():
			t.onError(err)
			return
		case header := <-t.streamer.Header():
			exit := t.onHeader(header)
			if exit {
				return
			}
		case tx := <-t.streamer.Transaction():
			exit := t.onTransaction(tx)
			if exit {
				return
			}
		case block := <-t.streamer.Block():
			exit := t.onBlock(block)
			if exit {
				return
			}
		}
	}
}

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
func (t *Transaction) onError(err error) {
	t.sendErrOnce.Do(func() {
		t.errs <- err
	})
}

func (t *Transaction) cleanup() {

}

func (t *Transaction) onHeader(header *types.Header) bool {
	return t.onBlockNumberUpdate(header.Number.Uint64())
}

func (t *Transaction) onBlock(block *types.Block) bool {
	return t.onBlockNumberUpdate(block.NumberU64())
}
