package schema

import (
	"encoding/json"
	"reflect"
)

type Entity struct {
	Name       string    `json:"name,omitempty"`
	Type       string    `json:"type,omitempty"`
	Tags       string    `json:"tags,omitempty"`
	Properties []*Entity `json:"properties,omitempty"`
}

func GenerateFromInstance(v any) ([]byte, error) {
	elem := reflect.ValueOf(v)
	e, err := serializeEntity(elem)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(e, " ", " ")
}

func serializeEntity(v reflect.Value) (*Entity, error) {
	switch v.Kind() {
	case reflect.Struct:
		e := &Entity{
			Type: v.Type().String(),
		}

		for i := 0; i < v.NumField(); i++ {
			fe, err := serializeEntity(v.Field(i))
			if err != nil {
				return nil, err
			}

			fe.Name = v.Type().Field(i).Name
			fe.Type = v.Field(i).Type().String()
			fe.Tags = string(v.Type().Field(i).Tag)

			e.Properties = append(e.Properties, fe)
		}

		return e, nil

	case reflect.String:
		fe := &Entity{
			Type: "string",
		}

		return fe, nil

	case reflect.Slice:
		sv := reflect.New(v.Type().Elem()).Elem()
		return serializeEntity(sv)

	}

	return nil, nil
}
