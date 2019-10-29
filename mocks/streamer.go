package mocks

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"sync"
)

// Streamer mocks an interfaces.Streamer
type Streamer struct {
	mu     sync.Mutex
	closed bool
	tx     chan *interfaces.TxWithBlock
	blocks chan *types.Block
	header chan *types.Header
	errs   chan error

	shutdown chan struct{}
}

// Block implements interfaces.Streamer
func (s *Streamer) Block() <-chan *types.Block {
	return s.blocks
}

// Header implements interfaces.Streamer
func (s *Streamer) Header() <-chan *types.Header {
	return s.header
}

// Transaction implements interfaces.Streamer
func (s *Streamer) Transaction() <-chan *interfaces.TxWithBlock {
	return s.tx
}

// Err implements interfaces.Streamer
func (s *Streamer) Err() <-chan error {
	return s.errs
}

// Close implements interfaces.Streamer
func (s *Streamer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// SendError injects an error in the mock streamer
func (s *Streamer) SendError(err error) {
	s.errs <- err
}

// SendTransaction injects a transaction in the mock streamer
func (s *Streamer) SendTransaction(tx *interfaces.TxWithBlock) {
	s.tx <- tx
}

// SendBlock injects a block in the mock streamer
func (s *Streamer) SendBlock(block *types.Block) {
	s.blocks <- block
}

// SendHeader injects an header in the mock streamer
func (s *Streamer) SendHeader(header *types.Header) {
	s.header <- header
}

// Terminated checks if the mock streamer was closed
func (s *Streamer) Terminated() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// NewStreamer builds a mock streamer
func NewStreamer(blocks ...*types.Block) *Streamer {
	if len(blocks) != 0 {

	}
	s := &Streamer{
		closed: false,
		tx:     make(chan *interfaces.TxWithBlock),
		blocks: make(chan *types.Block),
		header: make(chan *types.Header),
		errs:   make(chan error, 1000),
	}
	return s
}
