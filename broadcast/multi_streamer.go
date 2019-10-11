package broadcast

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"sync"
)

type msChildParent interface {
	Done() <-chan struct{}
	removeChild(child *msChild)
}

type StoppableStreamer interface {
	interfaces.Streamer
	Stop()
}

// msChild is a MultiStreamer child
type msChild struct {
	sendErrOnce *sync.Once
	errs        chan error
	tx          chan *interfaces.TxWithBlock
	headers     chan *types.Header
	blocks      chan *types.Block

	parentStreamer msChildParent //
	stopped        chan struct{} // stopped is closed if the msChild is being closed
}

func newMsChild(parent msChildParent) *msChild {
	return &msChild{
		sendErrOnce:    new(sync.Once),
		errs:           make(chan error, 1),
		tx:             make(chan *interfaces.TxWithBlock),
		headers:        make(chan *types.Header),
		blocks:         make(chan *types.Block),
		stopped:        make(chan struct{}),
		parentStreamer: parent,
	}
}

// sendTx forwards the transaction to the listener, listens for tx send event, msChild stop event and parent shutdown o exit the goroutine
func (s *msChild) sendTx(tx *interfaces.TxWithBlock) {
	go func() {
		select {
		case s.tx <- tx: // tx forwarded
		case <-s.stopped: // msChild stopped
		case <-s.parentStreamer.Done(): // parent done
		}
	}()
}

// sendBlock forwards a block to the listener, listens for block send event, msChild stop event and parent shutdown to exit the goroutine
func (s *msChild) sendBlock(block *types.Block) {
	go func() {
		select {
		case s.blocks <- block: // block forwarded
		case <-s.stopped: // msChild stopped
		case <-s.parentStreamer.Done(): // parent done
		}
	}()
}

// sendHeader forwards a header to the listener, listens for header send event, msChild stop event and parent shutdown to exit the goroutine
func (s *msChild) sendHeader(header *types.Header) {
	go func() {
		select {
		case s.headers <- header: // header forwarded
		case <-s.stopped: // msChild stopped
		case <-s.parentStreamer.Done(): // parent done
		}
	}()
}

// sendError forwards exactly one error
func (s *msChild) sendError(err error) {
	s.sendErrOnce.Do(func() {
		s.errs <- err
		close(s.errs)
	})
}

// MultiStreamer takes a interfaces.Streamer and forwards the content tu multiple writers
type MultiStream struct {
	mu *sync.Mutex

	streamer interfaces.Streamer
}

func NewMultiStream(streamer interfaces.Streamer) *MultiStream {
	m := &MultiStream{
		mu:       new(sync.Mutex),
		streamer: streamer,
	}

	return m
}

func (m *MultiStream) Listen() StoppableStreamer {

	return nil
}

func (m *MultiStream) Done() <-chan struct{} {
	return m.streamer.Done()
}
