package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
)

func CreateStructFromSchema(data []byte) (any, error) {
	e := &Attribute{}
	err := json.Unmarshal(data, e)
	if err != nil {
		return nil, err
	}

	return parseAttribute(e)
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

func parseAttribute(attribute *Attribute) (any, error) {
	fields := []reflect.StructField{}
	for _, a := range attribute.Properties {
		t, err := parseType(a.Type)
		if err != nil {
			return nil, err
		}

		if t.Slice {
			innerType := parseInnerType(t, a)

			sliceType := reflect.SliceOf(innerType)

			if t.OuterPointer {
				sliceType = reflect.PointerTo(sliceType)
			}

			nf := reflect.StructField{
				Name: a.Name,
				Type: sliceType,
				Tag:  reflect.StructTag(a.Tags),
			}

			fields = append(fields, nf)
		} else if t.Map {
			innerType := parseInnerType(t, a)

			keyType := reflect.TypeOf(t.MapKey)
			mapType := reflect.MapOf(keyType, innerType)

			if t.OuterPointer {
				mapType = reflect.PointerTo(mapType)
			}

			fields = append(fields, reflect.StructField{
				Name: a.Name,
				Type: mapType,
				Tag:  reflect.StructTag(a.Tags),
			})
		} else {
			innerType := parseInnerType(t, a)

			fields = append(fields, reflect.StructField{
				Name: a.Name,
				Type: innerType,
				Tag:  reflect.StructTag(a.Tags),
			})
		}
	}

	t := reflect.StructOf(fields)

	return reflect.New(t).Interface(), nil
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

func parseInnerType(t *PropertyType, a *Attribute) reflect.Type {
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
	}

	// if the property is a pointer, we need to wrap the inner type in a pointer
	if innerType != nil && (t.OuterPointer || t.InnerPointer) {
		innerType = reflect.PointerTo(innerType)
	}

	if a.Properties != nil {
		se, err := parseAttribute(a)
		if err != nil {
			return nil
		}

		innerType = reflect.TypeOf(se)
	}

	return innerType
}
