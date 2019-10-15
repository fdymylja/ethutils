package stream

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"sync"
)

// msChildIF defines the unexported behaviour of a listener used internally by MultiStream
// all operations should be non-blocking to avoid stalling the loop function forever, which is
// the point of exit of MultiStream
type msChildIF interface {
	sendBlock(block *types.Block)               // sendBlock forwards a block received
	sendError(err error)                        // sendError forwards an error, this is used only once
	sendHeader(header *types.Header)            // sendHeader forwards a header
	sendTransaction(tx *interfaces.TxWithBlock) // sendTransaction forwards a transaction
	close()                                     // close signals termination of MultiStream to children
}

// MultiStream fans out data coming from a type that implements interfaces.Streamer to more listeners
// listeners can be generated using NewListener() function, the children listeners implement interfaces.Streamer
// in case an error is received from the internal streamer this error is forwarded to all children
//
type MultiStream struct {
	streamer    interfaces.Streamer    // streamer is the client that forwards new information to MultiStream
	mu          *sync.Mutex            // mu is used for sync purposes
	closed      bool                   // closed is to stop operations in case MultiStream is not active
	shutdown    chan struct{}          // shutdown signals the loop goroutine to exit
	cleanupDone chan struct{}          // cleanUpDone signals to Close() users that the operation is finished
	listeners   map[msChildIF]struct{} // listeners is the list of children that want to receive updates

	lastError     error      // lastError keeps track of the last error of the listener, which is the same among all children
	sendErrorOnce *sync.Once // sendErrorOnce makes sure that only one error is sent to children
}

// removeListener removes a listener from the list of listening children
func (s *MultiStream) removeListener(childIF msChildIF) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.listeners, childIF)
}

// NewMultiStream generates a new MultiStream instance based on a Streamer
func NewMultiStream(streamer interfaces.Streamer) *MultiStream {
	m := &MultiStream{
		streamer:      streamer,
		mu:            new(sync.Mutex),
		closed:        false,
		shutdown:      make(chan struct{}),
		listeners:     make(map[msChildIF]struct{}),
		sendErrorOnce: new(sync.Once),
	}
	go m.loop()
	return m
}

// loop is the main goroutine that takes care of forwarding new information to children, it's also the exit
// point of the instance, an error coming from Streamer or a shutdown signal trigger the cleanup operations
// which consist of sending an error to all children
func (s *MultiStream) loop() {
	defer s.cleanup()
	for {
		select {
		case <-s.shutdown: // in case of external shutdown
			return
		case err := <-s.streamer.Err(): // case of error coming from the streamer
			s.onError(err)
			return
		case block := <-s.streamer.Block(): // case a new block was forwarded
			s.onBlock(block)
		case header := <-s.streamer.Header(): // case a new header was forwarded
			s.onHeader(header)
		case tx := <-s.streamer.Transaction(): // case a new transaction is forwarded
			s.onTransaction(tx)
		}
	}
}

// onError forwards an error to all listeners, this is done only once
func (s *MultiStream) onError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendErrorOnce.Do(func() {
		for listener := range s.listeners {
			listener.sendError(err)
			s.lastError = err
		}
	})
}

// onHeader forwards a header to all listeners
func (s *MultiStream) onHeader(header *types.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for listener := range s.listeners {
		listener.sendHeader(header)
	}
}

// onBlock forwards a block to all listeners
func (s *MultiStream) onBlock(block *types.Block) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for listener := range s.listeners {
		listener.sendBlock(block)
	}
}

// onTransaction takes care of forwarding a transaction to all children
func (s *MultiStream) onTransaction(tx *interfaces.TxWithBlock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for listener := range s.listeners {
		listener.sendTransaction(tx)
	}
}

// cleanup takes care of doing clean up operations, in case the cleanup was not preceded by an error
// coming from the internal streamer interface then a status.ErrShutdown is forwarded to all the children
// listeners
func (s *MultiStream) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// send error to listeners, in case an error was already sent this part will do nothing
	// in case no error was sent and the shutdown was external then all listeners will be notified
	// of an external shutdown, in this case Close() being called
	s.sendErrorOnce.Do(func() {
		for listener := range s.listeners {
			listener.sendError(status.ErrShutdown)
			s.lastError = status.ErrShutdown
		}
	})
	// close all listeners
	for listener := range s.listeners {
		listener.close()
	}
	// set listeners to nil
	s.listeners = nil
	// close streamer
	s.streamer.Close()
	// set closed to true, set it again to false in case the shutdown is not external, Close() being called,
	// but internal, due to error.
	s.closed = true
}

func (s *MultiStream) NewListener() (interfaces.Streamer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// check if closed
	if s.closed {
		return nil, status.ErrClosed
	}
	// generate a new listener
	listener := newStoppableStreamer(s)
	// add listener to listeners set
	s.listeners[listener] = struct{}{}
	// return
	return listener, nil
}

// Close terminates the MultiStream instance and sends status.ErrShutdown to all listeners children
func (s *MultiStream) Close() error {
	// lock USE A SIGNAL THAT SIGNALS A SUCCESSFUL TERMINATION SO U USE MU LOCK HERE AND IN RETURN
	s.mu.Lock()
	defer s.mu.Unlock()
	// check if already closed
	if s.closed {
		return status.ErrClosed
	}
	// set closed to false
	s.closed = true
	// signal exit to loop function
	close(s.shutdown)
	return nil
}

// Err returns the last error of the listener, the error is shared among all children listeners
// this serves as a common place to gather the last, and only, error coming from MultiStream
func (s *MultiStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}
