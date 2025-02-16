package stream

import (
	"container/list"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"sync"
)

type multiStreamParent interface {
	removeListener(childIF msChildIF)
}

type multiStreamChildren struct {
	blocksFromStreamer       chan *types.Block            // blocksFromStreamer is used to forward blocks coming from streamer to listenAndSendBlocks goroutine
	blocksToListener         chan *types.Block            // blocksToListener is used to forward blocks to the listener
	headersFromStreamer      chan *types.Header           // headersFromStreamer is used to forward blocks coming from streamer to listenAndServeHeaders goroutine
	headersToListener        chan *types.Header           // headersToListener is used to forward blocks to the listener
	transactionsFromStreamer chan *interfaces.TxWithBlock // transactionsFromStreamer is used to forward transactions coming from streamer to listenAndServeTransactions goroutine
	transactionsToListener   chan *interfaces.TxWithBlock // transactionsToListener is used to forward transactions to the listener

	options *MultiStreamOptions

	shutdown    chan struct{}
	errs        chan error
	sendErrOnce sync.Once
	closeOnce   sync.Once
	stopOnce    sync.Once

	parent multiStreamParent

	sendOps sync.WaitGroup
}

func newStoppableStreamer(parent multiStreamParent, options ...*MultiStreamOptions) *multiStreamChildren {
	option := DefaultMultiStreamOptions
	if len(options) > 0 && options[0] != nil {
		option = options[0]
	}
	s := &multiStreamChildren{
		blocksFromStreamer:       make(chan *types.Block),
		blocksToListener:         make(chan *types.Block),
		headersFromStreamer:      make(chan *types.Header),
		headersToListener:        make(chan *types.Header),
		transactionsToListener:   make(chan *interfaces.TxWithBlock),
		transactionsFromStreamer: make(chan *interfaces.TxWithBlock),
		errs:                     make(chan error, 1),
		shutdown:                 make(chan struct{}),
		sendErrOnce:              sync.Once{},
		closeOnce:                sync.Once{},
		stopOnce:                 sync.Once{},
		parent:                   parent,

		options: option,

		sendOps: sync.WaitGroup{},
	}
	s.sendOps.Add(3) // add one worker for each goroutine to synchronize close operations
	go s.listenShutdown()
	go s.listenAndSendTransactions()
	go s.listenAndSendBlocks()
	go s.listenAndSendHeaders()
	return s
}

func (s *multiStreamChildren) listenShutdown() {
	<-s.shutdown
	s.cleanup()
}

func (s *multiStreamChildren) listenAndSendTransactions() {
	defer s.sendOps.Done()
	transactionQueue := list.New()
	defer transactionQueue.Init() // clean the list once the function exits
	for {
		// wait for new incoming transactions, this is only if the queue is empty
		select {
		case <-s.shutdown:
			return
		case tx := <-s.transactionsFromStreamer: // in case a new transaction is forwarded
			// push tx to queue
			transactionQueue.PushBack(tx)
		}
		// now that we have at least one transaction in the queue forward the tx to the listener, while waiting
		// for new transactions to be put on the queue
		for {
			// check if the instance has reached the maximum allowed
			if transactionQueue.Len() >= s.options.MaxTransactionsQueueSize {
				s.sendError(ErrMaximumTransactionQueueSizeReached)
				s.close()
				return
			}
			tx := transactionQueue.Front()
			if tx == nil { // in case the queue is empty return to first select case
				break
			}
			// try to forward transaction
			select {
			case <-s.shutdown:
				return
			case s.transactionsToListener <- tx.Value.(*interfaces.TxWithBlock): // case send to listener
				transactionQueue.Remove(tx) // remove element from transactions list
			case newTx := <-s.transactionsFromStreamer: // case a new transaction has arrived push it to the transaction queue
				transactionQueue.PushBack(newTx)
			}
		}
	}
}

func (s *multiStreamChildren) sendTransaction(tx *interfaces.TxWithBlock) {
	select {
	case s.transactionsFromStreamer <- tx: // this op should never block, unless listenAndSendTransactions goroutine has quit
	case <-s.shutdown: // in case the instance has shutdown drop the message
	}
}

func (s *multiStreamChildren) listenAndSendBlocks() {
	defer s.sendOps.Done()    // signal the goroutine has exited on function return
	blocksQueue := list.New() // create a new queue
	defer blocksQueue.Init()  // clean list when the goroutine exits
	for {
		// wait for a block in case the queue is empty
		select {
		case <-s.shutdown: // case instance shutdown, exit
			return
		case block := <-s.blocksFromStreamer: // case a new block put it inside the queue
			blocksQueue.PushBack(block)
		}
		// once we have some elements in the queue start sending them to the listener, while waiting for new blocks from streamer
		for {
			// if queue max length was reached then forward error and quit
			if blocksQueue.Len() >= s.options.MaxBlocksQueueSize {
				s.sendError(ErrMaximumBlocksQueueSizeReached)
				s.close()
				return
			}
			block := blocksQueue.Front()
			if block == nil { // if there are no new blocks on the queue go on first select case and wait for new blocks
				break
			}
			// send block or wait for another block to be received
			select {
			case <-s.shutdown: // if shutdown, exit goroutine
				return
			case newBlock := <-s.blocksFromStreamer: // if a new block was received, insert it in the queue
				// push new block into queue
				blocksQueue.PushBack(newBlock)
			case s.blocksToListener <- block.Value.(*types.Block): // if block was forwarded to listener then remove it from queue
				blocksQueue.Remove(block)
			}
		}
	}
}

func (s *multiStreamChildren) sendBlock(block *types.Block) {
	select {
	case s.blocksFromStreamer <- block: // this op should never block, unless listenAndSendBlocks goroutine has quit
	case <-s.shutdown: // in case the instance has shutdown drop the block send op
	}
}

func (s *multiStreamChildren) listenAndSendHeaders() {
	defer s.sendOps.Done()
	headersQueue := list.New()
	defer headersQueue.Init()
	for {
		select {
		case <-s.shutdown:
			return
		case header := <-s.headersFromStreamer:
			headersQueue.PushBack(header)
		}
		// once we have at least one elem in the queue
		for {
			// check if maximum queue size was reached
			if headersQueue.Len() >= s.options.MaxHeadersQueueSize {
				s.sendError(ErrMaximumHeadersQueueSizeReached)
				s.close()
				return
			}
			header := headersQueue.Front()
			if header == nil { // case queue empty go to first select and wait for new headers to fill the queue
				break
			}
			// try to send header or in case a new header is received put it in the queue
			select {
			case <-s.shutdown:
				return
			case newHeader := <-s.headersFromStreamer:
				headersQueue.PushBack(newHeader)
			case s.headersToListener <- header.Value.(*types.Header):
				headersQueue.Remove(header)
			}
		}
	}
}

func (s *multiStreamChildren) sendHeader(header *types.Header) {
	select {
	case s.headersFromStreamer <- header: // this op should never block, unless listenAndSendHeaders goroutine has quit
	case <-s.shutdown: // in case instance has shutdown drop the header send op
	}
}

func (s *multiStreamChildren) stop() {
	s.stopOnce.Do(func() {
		s.parent.removeListener(s)
		s.close()
	})
}

// cleanup is called after close is called
func (s *multiStreamChildren) cleanup() {
	s.sendOps.Wait()
	s.sendError(status.ErrShutdown) // send shutdown error
}

func (s *multiStreamChildren) close() {
	s.closeOnce.Do(func() {
		close(s.shutdown)
	})
}

func (s *multiStreamChildren) sendError(err error) { // send one error only
	s.sendErrOnce.Do(func() {
		s.errs <- err
		close(s.errs)
	})
}

// implement interfaces.Streamer

// Block implements interfaces.Streamer
func (s *multiStreamChildren) Block() <-chan *types.Block {
	return s.blocksToListener
}

// Header implements interfaces.Streamer
func (s *multiStreamChildren) Header() <-chan *types.Header {
	return s.headersToListener
}

// Transaction implements interfaces.Streamer
func (s *multiStreamChildren) Transaction() <-chan *interfaces.TxWithBlock {
	return s.transactionsToListener
}

// Err implements interfaces.Streamer
func (s *multiStreamChildren) Err() <-chan error {
	return s.errs
}

// Close implements interfaces.Streamer
func (s *multiStreamChildren) Close() error {
	s.stop()
	return nil
}
