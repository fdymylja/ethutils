package errors

import (
	"errors"
	"fmt"
)

type ErrMaxRetriesReached struct {
	LastError error
}

func (err *ErrMaxRetriesReached) Error() string {
	if err.LastError != nil {
		return fmt.Sprintf("max number of retries reached: %s", err.LastError)
	}
	return fmt.Sprintf("max number of retries reached")
}

var ErrShutdown = errors.New("operation stopped due to shutdown")
