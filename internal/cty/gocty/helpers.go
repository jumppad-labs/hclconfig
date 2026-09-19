package gocty

import (
	"math/big"
	"reflect"

	"github.com/jumppad-labs/xcl/internal/xcl/tags"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/cty/set"
)

var valueType = reflect.TypeOf(cty.Value{})
var typeType = reflect.TypeOf(cty.Type{})

var setType = reflect.TypeOf(set.Set[interface{}]{})

var bigFloatType = reflect.TypeOf(big.Float{})
var bigIntType = reflect.TypeOf(big.Int{})

var emptyInterfaceType = reflect.TypeOf(interface{}(nil))

var stringType = reflect.TypeOf("")

// Modified from original gocty to read attribute names from XCL's `xcl`
// struct tags, and to flatten the attributes of embedded structs.
//
// structTagIndices interrogates the fields of the given type (which must
// be a struct type, or we'll panic) and returns a map from the cty
// attribute names declared via struct tags to the indices of the
// fields holding those tags.
//
// This function will panic if two fields within the struct are tagged with
// the same cty attribute name.
func structTagIndices(st reflect.Type) map[string]int {
	ct := st.NumField()
	ret := make(map[string]int, ct)

	for i := 0; i < ct; i++ {
		field := st.Field(i)
		if name := attributeName(field); name != "" {
			ret[name] = i
		}
	}

	return ret
}

func structTagIndicesWithAnon(st reflect.Type) map[string]int {
	ct := st.NumField()
	ret := make(map[string]int, ct)

	for i := 0; i < ct; i++ {
		field := st.Field(i)
		if field.Anonymous {
			// Recursively process anonymous fields to handle nested embedding
			nested := structTagIndicesWithAnon(field.Type)

			for k, _ := range nested {
				ret[k] = -1
			}
		}

		if name := attributeName(field); name != "" {
			ret[name] = i
		}
	}

	return ret
}

// attributeName returns the cty attribute name for a struct field, which is the
// name in its `xcl` tag. It returns an empty string when the field has no tag,
// the tag has no name, or the tag is not valid.
func attributeName(field reflect.StructField) string {
	tag, ok := field.Tag.Lookup(tags.Name)
	if !ok {
		return ""
	}

	ft, err := tags.Parse(tag)
	if err != nil {
		return ""
	}

	return ft.Name
}
