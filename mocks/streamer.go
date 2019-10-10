package mocks

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"sync"
	"testing"
	"time"
)

// TODO make instance concurrency safe and don't allow anything to start if blocks are not loaded

// Streamer implements a mock ethutils.Streamer used for testing
type Streamer struct {
	test        testing.TB
	blockLoader BlockLoader
	blocks      []*types.Block // blocks loaded with BlockPuller func
	blockTime   time.Duration
	currBlock   int
	maxBlocks   int

	chanBlocks  chan *types.Block
	chanTx      chan *interfaces.TxWithBlock
	chanHeaders chan *types.Header
	errs        chan error

	shutdown     chan struct{}
	shutdownOnce *sync.Once
}

// NewStreamer builds a Streamer instance
func NewStreamer(t testing.TB) *Streamer {
	return &Streamer{
		test:         t,
		blockLoader:  ropstenLoadBlocks(6532150, 6532170),
		blocks:       nil,
		blockTime:    15 * time.Second,
		currBlock:    0,
		maxBlocks:    0,
		chanBlocks:   make(chan *types.Block),
		chanTx:       make(chan *interfaces.TxWithBlock),
		chanHeaders:  make(chan *types.Header),
		errs:         make(chan error),
		shutdown:     make(chan struct{}),
		shutdownOnce: new(sync.Once),
	}
}

// SetBlockLoader sets the function that loads blocks into Streamer
func (s *Streamer) SetBlockLoader(f BlockLoader) {
	s.blockLoader = f
	return
}

// SetBlockTime sets the block time
func (s *Streamer) SetBlockTime(t time.Duration) {
	s.blockTime = t
}

// Start starts the production of blocks and transactions
func (s *Streamer) Start() {
	var err error
	s.blocks, err = s.blockLoader()
	if err != nil {
		s.test.Fatal(err)
	}
	if len(s.blocks) == 0 {
		s.test.Fatal("blocks not loaded")
	}
	go func() {
		for {
			// case the current block is the same as last block then restart from the first block
			if s.currBlock == len(s.blocks) {
				s.currBlock = 0
			}
			// emulate a block time
			time.Sleep(s.blockTime)
			// get block
			block := s.blocks[s.currBlock]
			header, txs := block.Header(), block.Transactions()
			// send block
			select {
			case <-s.shutdown:
				// s.test.Log("Streamer: loop stopped")
				return
			case s.chanBlocks <- block:
			}
			// send header
			select {
			case <-s.shutdown:
				// s.test.Log("Streamer: loop stopped")
				return
			case s.chanHeaders <- header:
			}
			// send txs
			for _, tx := range txs {
				select {
				case <-s.shutdown:
					s.test.Log("Stream: loop stopped")
					return
				case s.chanTx <- &interfaces.TxWithBlock{
					Transaction: tx,
					BlockNumber: header.Number.Uint64(),
					Timestamp:   header.Time,
				}:
				}
			}
			s.currBlock++
		}
	}()
}

// Block is a function to implement interfaces.Streamer
func (s *Streamer) Block() <-chan *types.Block {
	return s.chanBlocks
}

// Header is a function to implement interfaces.Streamer
func (s *Streamer) Header() <-chan *types.Header {
	return s.chanHeaders
}

// TransactionIncluded is a function to implement interfaces.Streamer
func (s *Streamer) Transaction() <-chan *interfaces.TxWithBlock {
	return s.chanTx
}

// Err is a function to implement interfaces.Streamer
func (s *Streamer) Err() <-chan error {
	return s.errs
}

// Close is a function to implement interfaces.Streamer
func (s *Streamer) Close() error {
	s.test.Log("Streamer: closed")
	return nil
}

// InjectError injects an error to the streamer
func (s *Streamer) InjectError(err error) {
	go func() {
		select {
		case <-s.shutdown:
			// s.test.Log("exit before error recv") // race detector
		case s.errs <- err:
		}
	}()
}

// InjectTransaction injects a transaction in the streamer
func (s *Streamer) InjectTransaction(tx *interfaces.TxWithBlock) {
	go func() {
		select {
		case <-s.shutdown:
			// s.test.Log("exit before error recv") // race detector hates me
		case s.chanTx <- tx:
		}
	}()
}

// InjectHeader injects a header in the streamer
func (s *Streamer) InjectHeader(header *types.Header) {
	go func() {
		select {
		case <-s.shutdown:
			// s.test.Log("exit before error recv") // race detector hates me
		case s.chanHeaders <- header:
		}
	}()
}

// Stop stops the streamer, the operation will be execute only once, subsequent calls will not do anything
func (s *Streamer) Stop() {
	s.shutdownOnce.Do(func() {
		close(s.shutdown)
	})
}

func (s *Streamer) randomTx() *interfaces.TxWithBlock {
	return nil
}
