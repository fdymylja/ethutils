package mocks

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
)

type Streamer struct {
	closed bool
	tx     chan *interfaces.TxWithBlock
	blocks chan *types.Block
	header chan *types.Header
	errs   chan error
}

func (s *Streamer) Block() <-chan *types.Block {
	return s.blocks
}

func (s *Streamer) Header() <-chan *types.Header {
	return s.header
}

func (s *Streamer) Transaction() <-chan *interfaces.TxWithBlock {
	return s.tx
}

func (s *Streamer) Err() <-chan error {
	return s.errs
}

func (s *Streamer) Close() error {
	s.closed = true
	return nil
}

func (s *Streamer) SendError(err error) {
	s.errs <- err
}

func (s *Streamer) SendTransaction(tx *interfaces.TxWithBlock) {
	s.tx <- tx
}

func (s *Streamer) SendBlock(block *types.Block) {
	s.blocks <- block
}

func (s *Streamer) SendHeader(header *types.Header) {
	s.header <- header
}

func (s *Streamer) Terminated() bool {
	return s.closed
}

func NewStreamer() *Streamer {
	return &Streamer{
		closed: false,
		tx:     make(chan *interfaces.TxWithBlock),
		blocks: make(chan *types.Block),
		header: make(chan *types.Header),
		errs:   make(chan error, 1),
	}
}
