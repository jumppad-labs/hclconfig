package plugins

import "errors"

// ErrNotFound is returned by a provider's Read when the real resource no longer
// exists. xcl creates the resource again when it sees this error.
//
// Check for it with errors.Is. It keeps its identity when the provider runs in
// a separate plugin process.
var ErrNotFound = errors.New("resource not found")
