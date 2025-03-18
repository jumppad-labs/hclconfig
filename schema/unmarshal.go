package schema

import "encoding/json"

// UnmarshalUntyped unmarshals an untyped struct created by reflection
// into a concrete struct type.
func UnmarshalUntyped(from any, into any) error {
	// convert the untyped struct to json
	d, err := json.Marshal(from)
	if err != nil {
		return err
	}

	// unmarshal the json into the concrete struct
	err = json.Unmarshal(d, into)
	if err != nil {
		return err
	}

	return nil
}
