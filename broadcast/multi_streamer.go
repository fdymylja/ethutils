package broadcast

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"sync"
)

type StoppableStreamer interface {
	interfaces.Streamer
	Stop()
}

// msChildIF defines the operations MultiStream can do on its children
type msChildIF interface {
	close()                                     // closes the children
	sendBlock(block *types.Block)               // sendBlock forwards a block to the children
	sendHeader(header *types.Header)            // sendHeaders sends a header to the children
	sendError(err error)                        // sendError forwards an error to the children
	sendTransaction(tx *interfaces.TxWithBlock) // sendTransaction forwards a transaction to the children
}

/*
type msChildParent interface {
	Done() <-chan struct{}
	removeChild(child *msChild)
}
// msChild is a MultiStreamer child
type msChild struct {
	sendErrOnce *sync.Once
	sendErrorExternal chan error
	parentError chan error
	tx          chan *interfaces.TxWithBlock
	headers     chan *types.Header
	blocks      chan *types.Block

	parentStreamer msChildParent //

	stopOnce *sync.Once
	stop chan struct{}
	stopped        chan struct{} // stopped is closed if the msChild is being closed
}

func newMsChild(parent msChildParent) *msChild {
	c := &msChild{
		sendErrOnce:    new(sync.Once),
		sendErrorExternal:      make(chan error),
		parentError:    make(chan error, 1),
		tx:             make(chan *interfaces.TxWithBlock),
		headers:        make(chan *types.Header),
		blocks:         make(chan *types.Block),
		stopOnce:       new(sync.Once),
		stopped:        make(chan struct{}),
		parentStreamer: parent,
	}
	return c
}

func (s *msChild) listenEvents() {
	for {
		select {
		case <-s.stopped: // case external shutdown
			s.externalStop()
			return
		case err := <-s.parentError: // in case parent multi streamer sends us an error then externally forward this error and proceed with cleanup
			s.sendErrorExternal <- err
			return

		}
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
		s.parentError <- err
		close(s.parentError)
	})
}
*/

// MultiStreamer takes a interfaces.Streamer and forwards the content tu multiple writers
type MultiStream struct {
	mu *sync.Mutex

	listeners map[msChildIF]struct{}

	shutdownOnce *sync.Once
	shutdown     chan struct{}
	streamer     interfaces.Streamer
}

// NewMultiStream takes a interfaces.Streamer and returns a MultiStream instance
func NewMultiStream(streamer interfaces.Streamer) *MultiStream {
	m := &MultiStream{
		mu:           new(sync.Mutex),
		shutdownOnce: new(sync.Once),
		shutdown:     make(chan struct{}),
		streamer:     streamer,

		listeners: make(map[msChildIF]struct{}),
	}
	go m.loop()
	return m
}

// loop
func (m *MultiStream) loop() {
	defer m.cleanup() // if we exit the cycle end everything
	for {
		select {
		case err := <-m.streamer.Err(): // in case of error send error to every listener, return cause streamer returns one error only
			m.onError(err)
			return
		case <-m.shutdown: // in case of shutdown return
			return
		case block := <-m.streamer.Block(): // forward block
			m.onBlock(block)
		case header := <-m.streamer.Header(): // forward header
			m.onHeader(header)
		case tx := <-m.streamer.Transaction(): // forward tx
			m.onTransaction(tx)
		}
	}
}

// onBlock forwards blocks to every streamer
func (m *MultiStream) onBlock(block *types.Block) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// forward block to each listener
	for listener := range m.listeners {
		listener.sendBlock(block)
	}
}

func (m *MultiStream) onTransaction(tx *interfaces.TxWithBlock) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// forward tx to each listener
	for listener := range m.listeners {
		listener.sendTransaction(tx)
	}
}

func (m *MultiStream) onHeader(header *types.Header) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// forward header to each listener
	for listener := range m.listeners {
		listener.sendHeader(header)
	}
}

func (m *MultiStream) onError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// forward error
	for listener := range m.listeners {
		listener.sendError(err)
	}
}

func (m *MultiStream) removeListener(child msChildIF) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// remove
	delete(m.listeners, child)
}
func (m *MultiStream) Listen() StoppableStreamer {

	return nil
}

func (m *MultiStream) cleanup() {
	m.shutdownOnce.Do(func() {
		close(m.shutdown)
	})
}
