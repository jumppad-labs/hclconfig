package parser

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jumppad-labs/xcl/types"
	"github.com/zclconf/go-cty/cty"
)

// Resource types mark fields with options in an `xcl` struct tag, separate from
// the `hcl` tag because gohcl rejects options it does not know:
//
//   - computed: the field is owned by the provider. Users can not set it, and
//     the value saved by the last apply is carried onto the configured resource
//     before it is read. A computed field must also be `hcl:",optional"`.
//   - key: the field identifies an element in a list of blocks. Elements of the
//     saved and configured copies are paired by their key fields, or by position
//     when the element type has none.
const (
	xclTagComputed = "computed"
	xclTagKey      = "key"
)

var (
	resourceBaseType = reflect.TypeOf(types.ResourceBase{})
	ctyValueType     = reflect.TypeOf(cty.Value{})
)

// structField is a field of a struct type that configuration can name
type structField struct {
	// name is the name configuration uses for the field
	name string

	// index is the field's index sequence, for reflect.Value.FieldByIndex
	index []int

	field reflect.StructField
}

// hasXclOption returns true when the field's `xcl` tag has the given option
func hasXclOption(field reflect.StructField, option string) bool {
	tag, ok := field.Tag.Lookup("xcl")
	if !ok {
		return false
	}

	for _, o := range strings.Split(tag, ",") {
		if strings.TrimSpace(o) == option {
			return true
		}
	}

	return false
}

// isComputed returns true when the field is owned by the provider
func isComputed(field reflect.StructField) bool {
	return hasXclOption(field, xclTagComputed)
}

// isKey returns true when the field identifies an element in a list of blocks
func isKey(field reflect.StructField) bool {
	return hasXclOption(field, xclTagKey)
}

// isOptional returns true when the field's `hcl` tag kind is optional
func isOptional(field reflect.StructField) bool {
	tag, ok := field.Tag.Lookup("hcl")
	if !ok {
		return false
	}

	_, kind, _ := strings.Cut(tag, ",")
	return kind == "optional"
}

// structFields returns the fields of a struct type that configuration can name.
// Anonymous embedded structs are flattened into the parent, as they are for
// property names, and xcl's own ResourceBase is skipped: its fields are
// metadata, never provider state.
func structFields(t reflect.Type) []structField {
	fields := []structField{}
	collectStructFields(t, nil, &fields, map[reflect.Type]bool{})

	return fields
}

func collectStructFields(t reflect.Type, prefix []int, fields *[]structField, visited map[reflect.Type]bool) {
	if visited[t] {
		return
	}
	visited[t] = true

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		index := make([]int, len(prefix), len(prefix)+1)
		copy(index, prefix)
		index = append(index, i)

		if field.Anonymous {
			// pointer embeds can be nil, their fields can not be addressed safely
			if field.Type.Kind() == reflect.Struct && field.Type != resourceBaseType {
				collectStructFields(field.Type, index, fields, visited)
			}

			continue
		}

		if !field.IsExported() {
			continue
		}

		name := hclTagName(field)
		if name == "" {
			continue
		}

		*fields = append(*fields, structField{name: name, index: index, field: field})
	}
}

// blockElement returns the struct type held by a block or object field: a
// struct, a pointer to a struct, or a list or map of either. It returns nil for
// any other type.
func blockElement(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		t = t.Elem()
	}

	t = dereference(t)
	if t.Kind() != reflect.Struct || t == ctyValueType || t == resourceBaseType {
		return nil
	}

	return t
}

// computedField is a computed field found on a resource type, at any depth
type computedField struct {
	// path is the field's hcl path, with nested blocks separated by dots,
	// e.g. "network.assigned_address"
	path string

	field reflect.StructField
}

// computedFields returns every computed field of a resource type, including
// those inside nested blocks
func computedFields(t reflect.Type) []computedField {
	found := []computedField{}

	t = dereference(t)
	if t == nil || t.Kind() != reflect.Struct {
		return found
	}

	collectComputedFields(t, "", &found, map[reflect.Type]bool{})

	return found
}

func collectComputedFields(t reflect.Type, path string, found *[]computedField, inPath map[reflect.Type]bool) {
	// guard against a type that contains itself
	if inPath[t] {
		return
	}
	inPath[t] = true
	defer delete(inPath, t)

	for _, f := range structFields(t) {
		if isComputed(f.field) {
			*found = append(*found, computedField{path: path + f.name, field: f.field})
			continue
		}

		if elem := blockElement(f.field.Type); elem != nil {
			collectComputedFields(elem, path+f.name+".", found, inPath)
		}
	}
}

// copyComputed copies the computed values of src onto dst, at any depth. dst
// must be addressable and both must have the same type. Elements of a list of
// blocks are paired by their key fields, or by position when the element type
// has none; elements of a map are paired by map key. Elements present on only
// one side are left as they are.
func copyComputed(dst, src reflect.Value) {
	switch dst.Kind() {
	case reflect.Ptr:
		if dst.IsNil() || src.IsNil() {
			return
		}

		copyComputed(dst.Elem(), src.Elem())

	case reflect.Struct:
		if blockElement(dst.Type()) == nil {
			return
		}

		for _, f := range structFields(dst.Type()) {
			dstField := dst.FieldByIndex(f.index)
			srcField := src.FieldByIndex(f.index)

			if isComputed(f.field) {
				dstField.Set(srcField)
				continue
			}

			if blockElement(f.field.Type) != nil {
				copyComputed(dstField, srcField)
			}
		}

	case reflect.Slice, reflect.Array:
		pairs := pairElements(dst, src)
		for dstIndex, srcIndex := range pairs {
			copyComputed(dst.Index(dstIndex), src.Index(srcIndex))
		}

	case reflect.Map:
		if dst.IsNil() || src.IsNil() {
			return
		}

		for _, key := range dst.MapKeys() {
			srcElement := src.MapIndex(key)
			if !srcElement.IsValid() {
				continue
			}

			// map elements are not addressable, copy, update and store back
			element := reflect.New(dst.Type().Elem()).Elem()
			element.Set(dst.MapIndex(key))
			copyComputed(element, srcElement)
			dst.SetMapIndex(key, element)
		}
	}
}

// pairElements pairs the elements of two lists of blocks, returning a map of
// index in a to index in b. Elements are paired by the element type's key
// fields when it has any, otherwise by position.
func pairElements(a, b reflect.Value) map[int]int {
	pairs := map[int]int{}

	keyFields := elementKeyFields(a.Type().Elem())
	if len(keyFields) == 0 {
		for i := 0; i < a.Len() && i < b.Len(); i++ {
			pairs[i] = i
		}

		return pairs
	}

	indexByKey := map[string]int{}
	for i := 0; i < b.Len(); i++ {
		if key, ok := elementKey(b.Index(i), keyFields); ok {
			indexByKey[key] = i
		}
	}

	for i := 0; i < a.Len(); i++ {
		key, ok := elementKey(a.Index(i), keyFields)
		if !ok {
			continue
		}

		if j, found := indexByKey[key]; found {
			pairs[i] = j
		}
	}

	return pairs
}

// elementKeyFields returns the key fields of a list element type
func elementKeyFields(t reflect.Type) []structField {
	keys := []structField{}

	t = dereference(t)
	if t.Kind() != reflect.Struct {
		return keys
	}

	for _, f := range structFields(t) {
		if isKey(f.field) {
			keys = append(keys, f)
		}
	}

	return keys
}

// elementKey returns the key of a list element built from its key fields. It
// returns false for a nil element.
func elementKey(element reflect.Value, keyFields []structField) (string, bool) {
	for element.Kind() == reflect.Ptr {
		if element.IsNil() {
			return "", false
		}

		element = element.Elem()
	}

	parts := make([]string, 0, len(keyFields))
	for _, f := range keyFields {
		parts = append(parts, fmt.Sprintf("%s=%v", f.name, element.FieldByIndex(f.index).Interface()))
	}

	return strings.Join(parts, ","), true
}
