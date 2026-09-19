package ctydebug_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jumppad-labs/xcl/internal/cty/ctydebug"
	"github.com/jumppad-labs/xcl/internal/cty"
)

func ExampleCmpOptions_diffObjects() {
	a := cty.ObjectVal(map[string]cty.Value{
		"foo": cty.StringVal("a"),
		"bar": cty.StringVal("b"),
	})
	b := cty.ObjectVal(map[string]cty.Value{
		"bar": cty.StringVal("c"),
	})

	fmt.Println("value diff:")
	fmt.Print(cmp.Diff(b, a, ctydebug.CmpOptions))
	fmt.Println("type diff:")
	fmt.Print(cmp.Diff(b.Type(), a.Type(), ctydebug.CmpOptions))

	// FIXME: It's unfortunate that cmp just uses is own printer for the
	// internals of cty.Value for the compact version inside diffs as
	// we can see below, but the main point here is to be able to see
	// the names of the attributes that are different, which this
	// achieves. Perhaps we can improve on this in future to somehow
	// make it use the GoString of these values.
}

func TestCmpPaths(t *testing.T) {
	pathA := cty.GetAttrPath("a")
	pathB := cty.GetAttrPath("b").Index(cty.Zero)

	if diff := cmp.Diff(pathA, pathB, ctydebug.CmpOptions); diff == "" {
		t.Errorf("did not detect diff between pathA and pathB")
	}
	if diff := cmp.Diff(pathB, pathB, ctydebug.CmpOptions); diff != "" {
		t.Errorf("detected unexpected diff between pathB and itself\n%s", diff)
	}
}
