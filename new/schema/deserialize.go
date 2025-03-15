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
	for _, p := range attribute.Properties {
		t, err := parseType(p.Type)
		if err != nil {
			return nil, err
		}

		if t.Slice {
			innerType := reflect.TypeOf(t.Type)

			if p.Properties != nil {
				se, err := parseAttribute(p)
				if err != nil {
					return nil, err
				}

				innerType = reflect.TypeOf(se)
			}

			if t.InnerPointer {
				innerType = reflect.PointerTo(innerType)
			}

			sliceType := reflect.SliceOf(innerType)

			if t.OuterPointer {
				sliceType = reflect.PointerTo(sliceType)
			}

			nf := reflect.StructField{
				Name: p.Name,
				Type: sliceType,
				Tag:  reflect.StructTag(p.Tags),
			}

			fields = append(fields, nf)
		} else if t.Map {
			innerType := reflect.TypeOf(t.Type)

			if p.Properties != nil {
				se, err := parseAttribute(p)
				if err != nil {
					return nil, err
				}

				innerType = reflect.TypeOf(se)
			}

			if t.InnerPointer {
				innerType = reflect.PointerTo(innerType)
			}

			keyType := reflect.TypeOf(t.MapKey)
			mapType := reflect.MapOf(keyType, innerType)

			if t.OuterPointer {
				mapType = reflect.PointerTo(mapType)
			}

			fields = append(fields, reflect.StructField{
				Name: p.Name,
				Type: mapType,
				Tag:  reflect.StructTag(p.Tags),
			})
		} else {
			innerType := reflect.TypeOf(t.Type)

			if p.Properties != nil {
				se, err := parseAttribute(p)
				if err != nil {
					return nil, err
				}

				innerType = reflect.TypeOf(se)
			}

			if t.OuterPointer {
				innerType = reflect.PointerTo(innerType)
			}

			fields = append(fields, reflect.StructField{
				Name: p.Name,
				Type: innerType,
				Tag:  reflect.StructTag(p.Tags),
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
		(
			(?P<package>.+)         package of the attribute
			(?:\.)                  ignore the .
		)?                        optional
		(?P<type>.+)              type of the attribute
		$                         end of type
	*/
	// expr := regexp.MustCompile(`^(?P<outerpointer>\*)?((?P<slice>\[])|((?P<map>map)(?:\[)(?P<mapkey>.+)(?:])))?(?P<innerpointer>\*)?((?P<package>.+)(?:\.))?(?P<type>.+)$`)
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
