package stream

import (
	"container/list"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"sync"
)

type ssParent interface {
	removeListener(childIF msChildIF)
}

type stoppableStreamer struct {
	blocksFromStreamer       chan *types.Block // blocksFromStreamer is used to forward blocks coming from streamer to listenAndSendBlocks goroutine
	blocksToListener         chan *types.Block // blocksToListener is used to forward blocks to the listener
	headersFromStreamer      chan *types.Header
	headersToListener        chan *types.Header
	transactionsToListener   chan *interfaces.TxWithBlock
	transactionsFromStreamer chan *interfaces.TxWithBlock

	shutdown    chan struct{}
	errs        chan error
	sendErrOnce sync.Once
	closeOnce   sync.Once
	stopOnce    sync.Once

	parent ssParent

	// todo synchronize goroutines exit with a wait group and a mutex
	mu      sync.Mutex
	sendOps sync.WaitGroup
}

func newStoppableStreamer(parent ssParent) *stoppableStreamer {
	s := &stoppableStreamer{
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

		sendOps: sync.WaitGroup{},
		mu:      sync.Mutex{},
	}
	s.sendOps.Add(3) // add one worker for each goroutine to synchronize close operations
	go s.listenAndSendTransactions()
	go s.listenAndSendBlocks()
	go s.listenAndSendHeaders()
	return s
}

func (s *stoppableStreamer) listenAndSendTransactions() {
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
				// TODO queue limits
				transactionQueue.PushBack(newTx)
			}
		}
	}
}

func (s *stoppableStreamer) sendTransaction(tx *interfaces.TxWithBlock) {
	select {
	case s.transactionsFromStreamer <- tx: // this op should never block, unless listenAndSendTransactions goroutine has quit
	case <-s.shutdown: // in case the instance has shutdown drop the message
	}
}

func (s *stoppableStreamer) listenAndSendBlocks() {
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
			block := blocksQueue.Front()
			if block == nil { // if there are no new blocks on the queue go on first select case and wait for new blocks
				break
			}
			// send block or wait for another block to be received
			select {
			case <-s.shutdown: // if shutdown, exit goroutine
				return
			case newBlock := <-s.blocksFromStreamer: // if a new block was received, insert it in the queue
				// TODO check for queue limits
				blocksQueue.PushBack(newBlock)
			case s.blocksToListener <- block.Value.(*types.Block): // if block was forwarded to listener then remove it from queue
				blocksQueue.Remove(block)
			}
		}
	}
}

func (s *stoppableStreamer) sendBlock(block *types.Block) {
	select {
	case s.blocksFromStreamer <- block: // this op should never block, unless listenAndSendBlocks goroutine has quit
	case <-s.shutdown: // in case the instance has shutdown drop the block send op
	}
}

func (s *stoppableStreamer) listenAndSendHeaders() {
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

func (s *stoppableStreamer) sendHeader(header *types.Header) {
	select {
	case s.headersFromStreamer <- header: // this op should never block, unless listenAndSendHeaders goroutine has quit
	case <-s.shutdown: // in case instance has shutdown drop the header send op
	}
}

func (s *stoppableStreamer) stop() {
	s.stopOnce.Do(func() {
		s.parent.removeListener(s)
		s.close()
	})
}

func (s *stoppableStreamer) close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		close(s.shutdown)
		s.sendOps.Wait()
		s.sendError(status.ErrShutdown) // send shutdown error
	})
}

func (s *stoppableStreamer) sendError(err error) { // send one error only
	s.sendErrOnce.Do(func() {
		s.errs <- err
		close(s.errs)
	})
}

// implement Streamer

func (s *stoppableStreamer) Block() <-chan *types.Block {
	return s.blocksToListener
}

func (s *stoppableStreamer) Header() <-chan *types.Header {
	return s.headersToListener
}

func (s *stoppableStreamer) Transaction() <-chan *interfaces.TxWithBlock {
	return s.transactionsToListener
}

func (s *stoppableStreamer) Err() <-chan error {
	return s.errs
}

func (s *stoppableStreamer) Close() error {
	s.stop()
	return nil
}
