package schema

import (
	"encoding/json"
	"reflect"
)

/*
* GenerateFromInstance creates a json structure based on the
* fields in the given instance. If depth is specified then
* recursion does not go deeper than depth levels.
 */
func GenerateFromInstance(v any, depth int) ([]byte, error) {
	elem := reflect.ValueOf(v)
	e, err := serializeAttribute(elem, 0, depth)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(e, " ", " ")
}

func serializeAttribute(v reflect.Value, currentDepth, maxDepth int) (*Attribute, error) {

	switch v.Kind() {
	case reflect.Struct:
		if currentDepth == maxDepth {
			return nil, nil
		}

		e := &Attribute{
			Type: v.Type().String(),
		}

		for i := 0; i < v.NumField(); i++ {
			fe, err := serializeAttribute(v.Field(i), currentDepth+1, maxDepth)
			if err != nil {
				return nil, err
			}

			if fe != nil {
				fe.Name = v.Type().Field(i).Name
				fe.Type = v.Field(i).Type().String()
				fe.Tags = string(v.Type().Field(i).Tag)

				e.Properties = append(e.Properties, fe)
			}
		}

		return e, nil

	case reflect.Slice:
		sv := reflect.New(v.Type().Elem()).Elem()
		return serializeAttribute(sv, currentDepth, maxDepth)

	case reflect.Map:
		sv := reflect.New(v.Type().Elem()).Elem()
		return serializeAttribute(sv, currentDepth, maxDepth)

	case reflect.Ptr:
		sv := reflect.New(v.Type().Elem()).Elem()
		return serializeAttribute(sv, currentDepth, maxDepth)

	// handle all other types here e.g. string, bool, int, float64, etc.
	default:
		fe := &Attribute{
			Type: v.Type().String(),
		}

		return fe, nil
	}
}
