package stream

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"sync"
)

type ssParent interface {
	removeListener(childIF msChildIF)
}

type stoppableStreamer struct {
	blocks       chan *types.Block
	errs         chan error
	headers      chan *types.Header
	shutdown     chan struct{}
	transactions chan *interfaces.TxWithBlock

	sendErrOnce *sync.Once
	closeOnce   *sync.Once
	stopOnce    *sync.Once

	parent ssParent

	// todo synchronize goroutines exit with a wait group and a mutex
	mu      *sync.Mutex
	alive   bool
	sendOps *sync.WaitGroup
}

func newStoppableStreamer(parent ssParent) *stoppableStreamer {
	return &stoppableStreamer{
		blocks:       make(chan *types.Block),
		errs:         make(chan error, 1),
		headers:      make(chan *types.Header),
		shutdown:     make(chan struct{}),
		transactions: make(chan *interfaces.TxWithBlock),
		sendErrOnce:  new(sync.Once),
		closeOnce:    new(sync.Once),
		stopOnce:     new(sync.Once),
		parent:       parent,

		alive:   true,
		sendOps: new(sync.WaitGroup),
		mu:      new(sync.Mutex),
	}
}

func (s *stoppableStreamer) sendError(err error) { // send one error only
	s.sendErrOnce.Do(func() {
		s.errs <- err
		close(s.errs)
	})
}

func (s *stoppableStreamer) sendTransaction(tx *interfaces.TxWithBlock) {
	// check if alive
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.alive {
		return
	}
	// if alive add one send op
	s.sendOps.Add(1)
	go func() {
		// after we're done sending signal that goroutine is done
		defer s.sendOps.Done()
		select {
		case <-s.shutdown:
		case s.transactions <- tx:
		}
	}()
}

func (s *stoppableStreamer) sendBlock(block *types.Block) {
	// check if alive
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.alive {
		return
	}
	// if alive add one send op
	s.sendOps.Add(1)
	go func() {
		// after we're done sending signal that goroutine is done
		defer s.sendOps.Done()
		select {
		case <-s.shutdown:
		case s.blocks <- block:
		}
	}()
}

func (s *stoppableStreamer) sendHeader(header *types.Header) {
	// check if alive
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.alive {
		return
	}
	// if alive add one send op
	s.sendOps.Add(1)
	go func() {
		// after we're done sending signal that goroutine is done
		defer s.sendOps.Done()
		select {
		case <-s.shutdown:
		case s.headers <- header:
		}
	}()
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
		s.alive = false
		close(s.shutdown)
		s.sendOps.Wait()
		s.sendError(status.ErrShutdown) // send shutdown error
	})
}

// implement Streamer

func (s *stoppableStreamer) Block() <-chan *types.Block {
	return s.blocks
}

func (s *stoppableStreamer) Header() <-chan *types.Header {
	return s.headers
}

func (s *stoppableStreamer) Transaction() <-chan *interfaces.TxWithBlock {
	return s.transactions
}

func (s *stoppableStreamer) Err() <-chan error {
	return s.errs
}

func (s *stoppableStreamer) Close() error {
	s.stop()
	return nil
}
