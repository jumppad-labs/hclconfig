package hclconfig

import (
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/jumppad-labs/hclconfig/types"
)

var locks = sync.Map{}

// withContextLock ensures that a HCL Context is not written and read
// at the same time
func withContextLock(ctx *hcl.EvalContext, call func()) {
	lock, _ := locks.LoadOrStore(ctx, &sync.Mutex{})

	// obtain a lock
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	call()
}

func getResourceLock(r types.Resource) func() {
	lock, _ := locks.LoadOrStore(r, &sync.Mutex{})

	// obtain a lock
	lock.(*sync.Mutex).Lock()

	// return a function to allow unlocking
	return func() {
		lock.(*sync.Mutex).Unlock()
	}
}
