package schema

import (
	"encoding/json"
	"reflect"
)

func GenerateFromInstance(v any) ([]byte, error) {
	elem := reflect.ValueOf(v)
	e, err := serializeAttribute(elem)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(e, " ", " ")
}

func serializeAttribute(v reflect.Value) (*Attribute, error) {
	switch v.Kind() {
	case reflect.Struct:
		e := &Attribute{
			Type: v.Type().String(),
		}

		for i := 0; i < v.NumField(); i++ {
			fe, err := serializeAttribute(v.Field(i))
			if err != nil {
				return nil, err
			}

			fe.Name = v.Type().Field(i).Name
			fe.Type = v.Field(i).Type().String()
			fe.Tags = string(v.Type().Field(i).Tag)

			e.Properties = append(e.Properties, fe)
		}

		return e, nil

	case reflect.Slice:
		sv := reflect.New(v.Type().Elem()).Elem()
		return serializeAttribute(sv)

	case reflect.Map:
		sv := reflect.New(v.Type().Elem()).Elem()
		return serializeAttribute(sv)

	case reflect.Ptr:
		sv := reflect.New(v.Type().Elem()).Elem()
		return serializeAttribute(sv)

	// handle all other types here e.g. string, bool, int, float64, etc.
	default:
		fe := &Attribute{
			Type: v.Type().String(),
		}

		return fe, nil
	}
}
