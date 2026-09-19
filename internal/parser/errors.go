package parser

import "errors"

// ErrEmptyConfiguration is returned by Apply when the configuration declares no
// blocks. Applying it would remove everything, which is what Destroy is for, so
// nothing is destroyed, created, changed or saved.
var ErrEmptyConfiguration = errors.New("the configuration declares no blocks, use Destroy to remove everything")
