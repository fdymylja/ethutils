package mocks

import (
	"errors"
	"testing"
)

// ErrorSubscription mocks an ethereum.Subscription that returns an error
type ErrorSubscription struct {
	test testing.TB
	errs chan error
}

// NewErrorSubscription creates an ErrorSubscription instance
func NewErrorSubscription(t testing.TB, errs ...error) *ErrorSubscription {
	var err = errors.New("test error")
	sub := &ErrorSubscription{
		test: t,
		errs: make(chan error),
	}
	if len(errs) > 0 && errs[0] != nil {
		err = errs[0]
	}
	go func() {
		sub.errs <- err
	}()
	return sub
}

// Err forwards errors coming from the subscription
func (sub *ErrorSubscription) Err() <-chan error {
	return sub.errs
}

// Unsubscribe mocks the unsubscribe operation of the ethereum.Subscription
func (sub *ErrorSubscription) Unsubscribe() {
	sub.test.Log("ErrorSubscription: unsubscribe recv")
}
