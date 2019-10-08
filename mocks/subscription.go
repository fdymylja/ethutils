package mocks

import (
	"errors"
	"testing"
)

type ErrorSubscription struct {
	test testing.TB
	errs chan error
}

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

func (sub *ErrorSubscription) Err() <-chan error {
	return sub.errs
}

func (sub *ErrorSubscription) Unsubscribe() {
	sub.test.Log("ErrorSubscription: unsubscribe recv")
}
