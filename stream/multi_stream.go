package stream

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"sync"
)

type StoppableStreamer interface {
	interfaces.Streamer
	Stop()
}

type msChildIF interface {
	sendBlock(block *types.Block)
	sendError(err error)
	sendHeader(header *types.Header)
	sendTransaction(tx *interfaces.TxWithBlock)
	close()
}

// MultiStream allows to stream data to more listeners
type MultiStream struct {
	streamer interfaces.Streamer

	mu        *sync.Mutex
	loopExit  chan struct{}
	shutdown  chan struct{}
	listeners map[msChildIF]struct{}

	sendErrorOnce *sync.Once
}

func NewMultiStream(streamer interfaces.Streamer) *MultiStream {
	m := &MultiStream{streamer: streamer}
	go m.loop()
	return m
}

func (s *MultiStream) loop() {
	defer close(s.loopExit)
	for {
		select {
		case <-s.shutdown:
			return
		case err := <-s.streamer.Err():
			s.onError(err)
			return
		case block := <-s.streamer.Block():
			s.onBlock(block)
		case header := <-s.streamer.Header():
			s.onHeader(header)
		case tx := <-s.streamer.Transaction():
			s.onTransaction(tx)
		}
	}
}

func (s *MultiStream) onError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendErrorOnce.Do(func() {
		for listener := range s.listeners {
			listener.sendError(err)
		}
	})
}

func (s *MultiStream) onHeader(header *types.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for listener := range s.listeners {
		listener.sendHeader(header)
	}
}

func (s *MultiStream) onBlock(block *types.Block) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for listener := range s.listeners {
		listener.sendBlock(block)
	}
}

func (s *MultiStream) onTransaction(tx *interfaces.TxWithBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for listener := range s.listeners {
		listener.sendTransaction(tx)
	}
}

func (s *MultiStream) Listener() StoppableStreamer {

	return nil
}
