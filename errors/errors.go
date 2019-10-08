package errors

import "errors"

var ErrMaxRetriesReached = errors.New("maximum number of retries reached")
var ErrShutdown = errors.New("operation stopped due to shutdown")
