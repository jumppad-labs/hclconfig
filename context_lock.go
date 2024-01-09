package hclconfig

import (
	"sync"

	"github.com/hashicorp/hcl/v2"
)

var locks = sync.Map{}

// getContextLock ensures that a HCL Context is not written and read
// at the same time
func getContextLock(ctx *hcl.EvalContext) func() {
	lock, _ := locks.LoadOrStore(ctx, &sync.Mutex{})

	// obtain a lock
	lock.(*sync.Mutex).Lock()

	// return a function to allow unlocking
	return func() {
		lock.(*sync.Mutex).Unlock()
	}
}
