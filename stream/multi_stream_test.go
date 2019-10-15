package stream

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
)

type mockStreamer struct {
	closed bool
	tx     chan *interfaces.TxWithBlock
	blocks chan *types.Block
	header chan *types.Header
	errs   chan error
}

func (s *mockStreamer) Block() <-chan *types.Block {
	return s.blocks
}

func (s *mockStreamer) Header() <-chan *types.Header {
	return s.header
}

func (s *mockStreamer) Transaction() <-chan *interfaces.TxWithBlock {
	return s.tx
}

func (s *mockStreamer) Err() <-chan error {
	return s.errs
}

func (s *mockStreamer) Close() error {
	s.closed = true
	return nil
}

func (s *mockStreamer) sendError(err error) {
	s.errs <- err
}

func (s *mockStreamer) sendTx(tx *interfaces.TxWithBlock) {
	s.tx <- tx
}

func (s *mockStreamer) sendBlock(block *types.Block) {
	s.blocks <- block
}

func (s *mockStreamer) sendHeader(header *types.Header) {
	s.header <- header
}

func newMockStreamer() *mockStreamer {
	return &mockStreamer{
		closed: false,
		tx:     make(chan *interfaces.TxWithBlock),
		blocks: make(chan *types.Block),
		header: make(chan *types.Header),
		errs:   make(chan error, 1),
	}
}
