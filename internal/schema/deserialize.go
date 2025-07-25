package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// CreateInstanceFromSchema takes a JSON schema generally created using GenerateFromInstance
// and dynamically creates a struct.
// This is a generic function that can create any struct type.
// typeMapping allows mapping type names to actual reflect.Types for proper type creation
func CreateInstanceFromSchema(data []byte, typeMapping map[string]reflect.Type) (any, error) {
	e := &Attribute{}
	err := json.Unmarshal(data, e)
	if err != nil {
		return nil, err
	}

	result, err := parseAttribute(e, typeMapping)
	if err != nil {
		return nil, err
	}

	// Return the dynamically created struct directly
	// It has embedded ResourceBase and schema fields at root level
	return result, nil
}

type PropertyType struct {
	OuterPointer bool
	Struct       bool
	Slice        bool
	Map          bool
	MapKey       string
	InnerPointer bool
	Type         string
}

// parseAttribute always returns a pointer to the struct
func parseAttribute(attribute *Attribute, typeMapping map[string]reflect.Type) (any, error) {
	// Start with embedded ResourceBase
	fields := []reflect.StructField{}

	for _, a := range attribute.Properties {
		// Handle anonymous embedded fields first
		if a.Anonymous {
			t, err := parseType(a.Type)
			if err != nil {
				return nil, err
			}

			embeddedType := parseInnerType(t, a, typeMapping)

			// Extract the field name from the type (e.g., "types.ResourceBase" -> "ResourceBase")
			fieldName := extractTypeName(a.Type)

			field := reflect.StructField{
				Name:      fieldName, // Anonymous fields need the type name
				Type:      embeddedType,
				Tag:       reflect.StructTag(a.Tags),
				Anonymous: true,
			}

			fields = append(fields, field)
			continue
		}

		t, err := parseType(a.Type)
		if err != nil {
			return nil, err
		}

		if t.Slice {
			innerType := parseInnerType(t, a, typeMapping)
			sliceType := reflect.SliceOf(innerType)

			if t.OuterPointer {
				sliceType = reflect.PointerTo(sliceType)
			}

			nf := reflect.StructField{
				Name: a.Name,
				Type: sliceType,
				Tag:  reflect.StructTag(a.Tags),
			}

			// Set PkgPath for unexported fields to avoid reflection errors
			if len(a.Name) > 0 && a.Name[0] >= 'a' && a.Name[0] <= 'z' {
				nf.PkgPath = "github.com/jumppad-labs/hclconfig/internal/schema"
			}

			fields = append(fields, nf)

		} else if t.Map {
			innerType := parseInnerType(t, a, typeMapping)

			keyType := reflect.TypeOf(t.MapKey)
			mapType := reflect.MapOf(keyType, innerType)

			if t.OuterPointer {
				mapType = reflect.PointerTo(mapType)
			}

			field := reflect.StructField{
				Name: a.Name,
				Type: mapType,
				Tag:  reflect.StructTag(a.Tags),
			}

			// Set PkgPath for unexported fields to avoid reflection errors
			if len(a.Name) > 0 && a.Name[0] >= 'a' && a.Name[0] <= 'z' {
				field.PkgPath = "github.com/jumppad-labs/hclconfig/internal/schema"
			}

			fields = append(fields, field)
		} else {
			innerType := parseInnerType(t, a, typeMapping)

			field := reflect.StructField{
				Name: a.Name,
				Type: innerType,
				Tag:  reflect.StructTag(a.Tags),
			}

			// Set PkgPath for unexported fields to avoid reflection errors
			if len(a.Name) > 0 && a.Name[0] >= 'a' && a.Name[0] <= 'z' {
				field.PkgPath = "github.com/jumppad-labs/hclconfig/internal/schema"
			}

			fields = append(fields, field)
		}
	}

	structType := reflect.StructOf(fields)
	instance := reflect.New(structType)

	// Return the struct directly - it implements Resource through embedded ResourceBase
	return instance.Interface(), nil
}

func parseType(t string) (*PropertyType, error) {
	/*
		^                         start of type
		(?P<outerpointer>\*)?     is the outer type a pointer?
		(
			(?P<slice>\[])          is the outer type a slice?
			|                       or
			(
				(?P<map>map)          is the outer type a map?
				(?:\[)                ignore the [
				(?P<mapkey>.+)        key type of the map
				(?:])                 ignore the ]
			)
		)?                        optional
		(?P<innerpointer>\*)?     is the inner type a pointer?
		(?P<type>.+)              type of the attribute
		$                         end of type
	*/

	expr := regexp.MustCompile(`^(?P<outerpointer>\*)?((?P<slice>\[])|((?P<map>map)(?:\[)(?P<mapkey>.+)(?:])))?(?P<innerpointer>\*)?(?P<type>.+)$`)
	matches := expr.FindAllStringSubmatch(t, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("unable to parse type: %s", t)
	}

	parts := make(map[string]string)
	for i, name := range expr.SubexpNames() {
		if i != 0 && name != "" {
			parts[name] = matches[0][i]
		}
	}

	tp := PropertyType{
		OuterPointer: parts["outerpointer"] != "",
		Slice:        parts["slice"] != "",
		Map:          parts["map"] != "",
		InnerPointer: parts["innerpointer"] != "",
		Type:         parts["type"],
	}

	if parts["mapkey"] != "" {
		tp.MapKey = parts["mapkey"]
	}

	return &tp, nil
}

func parseInnerType(t *PropertyType, a *Attribute, typeMapping map[string]reflect.Type) reflect.Type {
	var innerType reflect.Type

	switch t.Type {
	case "string":
		innerType = reflect.TypeOf("")
	case "bool":
		innerType = reflect.TypeOf(true)
	case "byte":
		innerType = reflect.TypeOf(byte(0))
	case "rune":
		innerType = reflect.TypeOf(rune(0))
	case "int":
		innerType = reflect.TypeOf(0)
	case "int64":
		innerType = reflect.TypeOf(int64(0))
	case "int32":
		innerType = reflect.TypeOf(int32(0))
	case "uint":
		innerType = reflect.TypeOf(uint(0))
	case "uint64":
		innerType = reflect.TypeOf(uint64(0))
	case "uint32":
		innerType = reflect.TypeOf(uint32(0))
	case "uintptr":
		innerType = reflect.TypeOf(uintptr(0))
	case "float":
		innerType = reflect.TypeOf(0.0)
	case "float64":
		innerType = reflect.TypeOf(float64(0.0))
	case "float32":
		innerType = reflect.TypeOf(float32(0.0))
	case "complex64":
		innerType = reflect.TypeOf(complex64(0))
	case "complex128":
		innerType = reflect.TypeOf(complex128(0))
	default:
		// Check type mapping first
		if typeMapping != nil {
			if mappedType, exists := typeMapping[t.Type]; exists {
				innerType = mappedType
				break
			}
		}
		
		// Handle interface{} and other unrecognized types
		if strings.Contains(t.Type, "interface") {
			innerType = reflect.TypeOf((*interface{})(nil)).Elem()
		} else {
			// For any other unrecognized type, use interface{} as fallback
			innerType = reflect.TypeOf((*interface{})(nil)).Elem()
		}
	}

	// if the property is a pointer, we need to wrap the inner type in a pointer
	if innerType != nil && (t.OuterPointer || t.InnerPointer) {
		innerType = reflect.PointerTo(innerType)
	}

	// if there are properties, we need to create a struct
	if a.Properties != nil {
		// Check if we found a real type mapping (not interface{})
		// We need to handle the case where innerType might be wrapped in a pointer
		hasRealTypeMapping := false
		if innerType != nil {
			checkType := innerType
			// Unwrap pointer if needed
			if checkType.Kind() == reflect.Ptr {
				checkType = checkType.Elem()
			}
			// If it's not interface{}, we have a real type mapping
			if checkType.Kind() != reflect.Interface {
				hasRealTypeMapping = true
			}
		}
		
		// Only create anonymous struct if we don't have a real type mapping
		if !hasRealTypeMapping {
			se, err := parseAttribute(a, typeMapping)
			if err != nil {
				return nil
			}

			innerType = reflect.TypeOf(se)

			// parse attribute always returns a pointer to the struct
			// if the inner type is not a pointer, we need to dereference it
			if !t.InnerPointer && !t.OuterPointer {
				innerType = reflect.TypeOf(se).Elem()
			}
		}
	}

	return innerType
}

// extractTypeName extracts the type name from a full type string
// e.g., "types.ResourceBase" -> "ResourceBase", "MyStruct" -> "MyStruct"
func extractTypeName(typeStr string) string {
	// Handle qualified types like "package.Type"
	parts := strings.Split(typeStr, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1] // Return the last part (type name)
	}
	return typeStr // Return as-is if no package qualifier
}
