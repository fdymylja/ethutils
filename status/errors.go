package status

import (
	"errors"
	"fmt"
)

// ErrMaxRetriesReached defines an error given when the max number of retries has been reached
// it also returns the last error
type ErrMaxRetriesReached struct {
	LastError error
}

func (err *ErrMaxRetriesReached) Error() string {
	if err.LastError != nil {
		return fmt.Sprintf("max number of retries reached: %s", err.LastError)
	}
	return fmt.Sprintf("max number of retries reached")
}

// ErrShutdown is returned when an ongoing operation is stopped by an instance shutdown
var ErrShutdown = errors.New("operation stopped due to shutdown")

// ErrClosed is returned when the instance we are using has been already closed, hence the call is a no-op
var ErrClosed = errors.New("unable to make operation: instance is shutdown")
