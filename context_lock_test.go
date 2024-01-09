package hclconfig

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestContextLockDoesNotAllowConcurrentAccesstoContext(t *testing.T) {
	a := &hcl.EvalContext{Variables: map[string]cty.Value{}}

	go func() {
		// get a lock but never unlock it
		getContextLock(a)
		for i := 0; i < 100; i++ {
			a.Variables[fmt.Sprintf("%d", i)] = cty.StringVal("bar")
		}
	}()

	go func() {
		unlock := getContextLock(a)
		defer unlock()
		for i := 0; i < 100; i++ {
			a.Variables[fmt.Sprintf("%d", i)] = cty.StringVal("bar")
		}
	}()

	// should never have 200 elements as the second go routine should be blocked
	require.Neverf(t, func() bool {
		return len(a.Variables) == 200
	}, 100*time.Millisecond, 1*time.Millisecond, "a.Varibles should have 200 elements")
}

func TestContextLockAllowsConcurrentAccesstoDifferentContexts(t *testing.T) {
	a := &hcl.EvalContext{Variables: map[string]cty.Value{}}
	b := &hcl.EvalContext{Variables: map[string]cty.Value{}}

	go func() {
		unlock := getContextLock(a)
		defer unlock()
		for i := 0; i < 100; i++ {
			a.Variables[fmt.Sprintf("%d", i)] = cty.StringVal("bar")
		}
	}()

	go func() {
		unlock := getContextLock(b)
		defer unlock()
		for i := 0; i < 100; i++ {
			b.Variables[fmt.Sprintf("%d", i)] = cty.StringVal("bar")
		}
	}()

	require.Eventuallyf(t, func() bool {
		return len(a.Variables) == 100 && len(b.Variables) == 100
	}, 100*time.Millisecond, 1*time.Millisecond, "a.Variables and b.Varibles should have 100 elements")
}
