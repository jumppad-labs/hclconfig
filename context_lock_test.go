package hclconfig

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func TestContextLockDoesNotAllowConcurrentAccesstoContext(t *testing.T) {
	a := &hcl.EvalContext{Variables: map[string]cty.Value{}}

	w := sync.WaitGroup{}
	w.Add(2)

	go func() {
		// get a lock but never unlock it
		getContextLock(a)
		for i := 0; i < 100; i++ {
			a.Variables[fmt.Sprintf("%d", i)] = cty.StringVal("bar")
		}

		w.Done()
	}()

	go func() {
		unlock := getContextLock(a)
		defer unlock()
		for i := 0; i < 100; i++ {
			a.Variables[fmt.Sprintf("%d", i)] = cty.StringVal("bar")
		}

		w.Done()
	}()

	done := make(chan struct{})

	go func() {
		w.Wait()
		<-done
	}()

	to := time.NewTimer(100 * time.Millisecond)
	select {
	case <-to.C:
		t.Log("timed out waiting for wait group, test passed")
	case <-done:
		t.Fatal("should not have completed")
	}
}

func TestContextLockAllowsConcurrentAccesstoDifferentContexts(t *testing.T) {
	a := &hcl.EvalContext{Variables: map[string]cty.Value{}}
	b := &hcl.EvalContext{Variables: map[string]cty.Value{}}

	w := sync.WaitGroup{}
	w.Add(2)

	go func() {
		unlock := getContextLock(a)
		defer unlock()
		for i := 0; i < 100; i++ {
			a.Variables[fmt.Sprintf("%d", i)] = cty.StringVal("bar")
		}

		w.Done()
	}()

	go func() {
		unlock := getContextLock(b)
		defer unlock()
		for i := 0; i < 100; i++ {
			b.Variables[fmt.Sprintf("%d", i)] = cty.StringVal("bar")
		}

		w.Done()
	}()

	done := make(chan struct{})

	go func() {
		w.Wait()
		done <- struct{}{}
	}()

	to := time.NewTimer(100 * time.Millisecond)
	select {
	case <-to.C:
		t.Fatal("timed out waiting for wait group")
	case <-done:
		t.Log("wait group completed, test passed")
	}
}
