package mocks

import "sync"

// Subscription mocks an ethereum.Subscription
type Subscription struct {
	errs         chan error
	closeErrOnce sync.Once
	sendErrOnce  sync.Once
	shutdownOnce sync.Once
	shutdown     chan struct{}
}

// NewSubscription is Subscription constructor
func NewSubscription() *Subscription {
	return &Subscription{
		errs:         make(chan error, 1),
		closeErrOnce: sync.Once{},
		shutdownOnce: sync.Once{},
		shutdown:     make(chan struct{}),
	}
}

// Unsubscribe closes the subscription
func (s *Subscription) Unsubscribe() {
	s.shutdownOnce.Do(func() {
		close(s.shutdown)
		s.closeErrOnce.Do(func() {
			close(s.errs)
		})
	})
}

// Err returns a channel that forwards a subscription error
func (s *Subscription) Err() <-chan error {
	return s.errs
}

// SendError forwards an error to the subscription listener
func (s *Subscription) SendError(err error) {
	s.sendErrOnce.Do(func() {
		s.errs <- err
	})
}
