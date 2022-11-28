package hclconfig

import (
	"sync"

	"github.com/hashicorp/hcl2/hcl"
)

var locks = sync.Map{}

// getContextLock ensures that a HCL Context is not written and read
// at the same time
func getContextLock(ctx *hcl.EvalContext) func() {
	var lock any
	var ok bool

	lock, ok = locks.Load(ctx)

	// lazy instantiate the lock
	if !ok {
		lock = &sync.Mutex{}

		locks.Store(ctx, lock)
	}

	// obtain a lock
	lock.(*sync.Mutex).Lock()

	// return a function to allow unlocking
	return func() {
		lock.(*sync.Mutex).Unlock()
	}
}
