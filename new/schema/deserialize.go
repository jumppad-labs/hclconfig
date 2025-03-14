package schema

import (
	"encoding/json"
	"reflect"
)

func CreateStructFromSchema(data []byte) (interface{}, error) {
	e := &Entity{}
	err := json.Unmarshal(data, e)
	if err != nil {
		return nil, err
	}

	return parseEntity(e)
}

func parseEntity(e *Entity) (interface{}, error) {
	fields := []reflect.StructField{}
	for _, p := range e.Properties {
		switch p.Type {
		case "string":
			fields = append(fields, reflect.StructField{
				Name: p.Name,
				Type: reflect.TypeOf("string"),
				Tag:  reflect.StructTag(p.Tags),
			})
		// Need to work out how to identify these types
		// can probably just use the start of the type [] to figure
		// out if it's a slice, the type of the object in the slice can be ignored
		// We do need to handle ptrs though so probably need to figure if *type or not
		case "[]main.Network":
			fallthrough
		case "[]schema.Network":
			se, err := parseEntity(p)
			if err != nil {
				return nil, err
			}

			nf := reflect.StructField{
				Name: p.Name,
				Type: reflect.SliceOf(reflect.TypeOf(se).Elem()),
				Tag:  reflect.StructTag(p.Tags),
			}

			fields = append(fields, nf)
		}
	}

	t := reflect.StructOf(fields)

	return reflect.New(t).Interface(), nil
}
