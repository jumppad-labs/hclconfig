package parser

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jumppad-labs/xcl/logger"
)

// warnChangedConfiguredValues logs a warning for every configured value a
// provider changed. before is the resource as sent to the provider, after is
// the resource it returned. Computed fields belong to the provider and are never
// reported, nor is a field whose configuration references another resource or a
// module, since its value comes from elsewhere. The check never fails the apply.
func warnChangedConfiguredValues(log logger.Logger, id string, body *hclsyntax.Body, resourceType reflect.Type, before []byte, after []byte) {
	if log == nil {
		return
	}

	for _, path := range changedConfiguredValues(body, resourceType, before, after) {
		log.Warn("provider changed a configured value", "resource", id, "field", path)
	}
}

// changedConfiguredValues returns the path of every non-computed field whose
// value differs between before and after, leaving out fields set by reference
func changedConfiguredValues(body *hclsyntax.Body, resourceType reflect.Type, before []byte, after []byte) []string {
	resourceType = dereference(resourceType)

	beforeValue := reflect.New(resourceType)
	if err := json.Unmarshal(before, beforeValue.Interface()); err != nil {
		return nil
	}

	afterValue := reflect.New(resourceType)
	if err := json.Unmarshal(after, afterValue.Interface()); err != nil {
		return nil
	}

	changed := []string{}
	compareConfigured(body, beforeValue.Elem(), afterValue.Elem(), "", &changed)

	return changed
}

// compareConfigured walks two copies of a struct together, appending the path of
// every non-computed field that differs. body is the configuration of the
// struct, it is nil when the struct was not configured as a block.
func compareConfigured(body *hclsyntax.Body, before, after reflect.Value, path string, changed *[]string) {
	for _, f := range structFields(before.Type()) {
		if isComputed(f.field) {
			continue
		}

		fieldPath := path + f.name
		beforeField := before.FieldByIndex(f.index)
		afterField := after.FieldByIndex(f.index)

		if blockElement(f.field.Type) == nil {
			if reflect.DeepEqual(beforeField.Interface(), afterField.Interface()) {
				continue
			}

			if isReferenceSet(body, f.name) {
				continue
			}

			*changed = append(*changed, fieldPath)
			continue
		}

		compareConfiguredBlocks(body, f.name, beforeField, afterField, fieldPath, changed)
	}
}

// compareConfiguredBlocks compares a block, pointer to a block, or list or map
// of blocks. An element added or removed by the provider is reported once, at
// the path of the field. A block or object set by reference as a whole is not
// compared.
func compareConfiguredBlocks(body *hclsyntax.Body, name string, before, after reflect.Value, path string, changed *[]string) {
	if isReferenceSet(body, name) {
		return
	}

	switch before.Kind() {
	case reflect.Ptr:
		if before.IsNil() && after.IsNil() {
			return
		}

		if before.IsNil() != after.IsNil() {
			*changed = append(*changed, path)

			return
		}

		compareConfiguredBlocks(body, name, before.Elem(), after.Elem(), path, changed)

	case reflect.Struct:
		compareConfigured(nestedBlockBody(body, name, 0), before, after, path+".", changed)

	case reflect.Slice, reflect.Array:
		if before.Len() != after.Len() {
			*changed = append(*changed, path)

			return
		}

		pairs := pairElements(before, after)
		if len(pairs) != before.Len() {
			*changed = append(*changed, path)

			return
		}

		for i := 0; i < before.Len(); i++ {
			elementPath := fmt.Sprintf("%s[%d]", path, i)
			compareConfiguredElement(nestedBlockBody(body, name, i), before.Index(i), after.Index(pairs[i]), elementPath, changed)
		}

	case reflect.Map:
		if before.Len() != after.Len() {
			*changed = append(*changed, path)

			return
		}

		for _, key := range before.MapKeys() {
			afterElement := after.MapIndex(key)
			if !afterElement.IsValid() {
				*changed = append(*changed, path)

				return
			}

			elementPath := fmt.Sprintf("%s[%v]", path, key.Interface())
			compareConfiguredElement(nil, before.MapIndex(key), afterElement, elementPath, changed)
		}
	}
}

// compareConfiguredElement compares one element of a list or map of blocks
func compareConfiguredElement(body *hclsyntax.Body, before, after reflect.Value, path string, changed *[]string) {
	for before.Kind() == reflect.Ptr {
		if before.IsNil() || after.IsNil() {
			if before.IsNil() != after.IsNil() {
				*changed = append(*changed, path)
			}

			return
		}

		before = before.Elem()
		after = after.Elem()
	}

	compareConfigured(body, before, after, path+".", changed)
}

// nestedBlockBody returns the body of the index-th block of the given type, or
// nil when there is none
func nestedBlockBody(body *hclsyntax.Body, blockType string, index int) *hclsyntax.Body {
	if body == nil {
		return nil
	}

	count := 0
	for _, block := range body.Blocks {
		if block.Type != blockType {
			continue
		}

		if count == index {
			return block.Body
		}

		count++
	}

	return nil
}

// isReferenceSet returns true when the named attribute is configured with an
// expression that references another resource or a module
func isReferenceSet(body *hclsyntax.Body, name string) bool {
	if body == nil {
		return false
	}

	attribute, ok := body.Attributes[name]
	if !ok {
		return false
	}

	for _, traversal := range attribute.Expr.Variables() {
		switch traversal.RootName() {
		case "resource", "module":
			return true
		}
	}

	return false
}
