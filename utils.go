package hclconfig

import (
	"github.com/zclconf/go-cty/cty"
)

// ParseVars converts a map[string]cty.Value into map[string]interface
// where the interface are generic go types like string, number, bool, slice, map
func ParseVars(value map[string]cty.Value) map[string]interface{} {
	vars := map[string]interface{}{}

	for k, v := range value {
		vars[k] = castVar(v)
	}

	return vars
}

func castVar(v cty.Value) interface{} {

	if v.Type() == cty.String {
		return v.AsString()
	} else if v.Type() == cty.Bool {
		return v.True()
	} else if v.Type() == cty.Number {
		// If something blows up here, remember that conversation we had when
		// we said that nobody will ever use a number bigger than float64 ... yeah
		// Handlebars does not understand BigFloat.
		val, _ := v.AsBigFloat().Float64()
		return val
	} else if v.Type().IsObjectType() || v.Type().IsMapType() {
		return ParseVars(v.AsValueMap())
	} else if v.Type().IsTupleType() || v.Type().IsListType() {
		i := v.ElementIterator()
		vars := []interface{}{}
		for {
			if !i.Next() {
				// cant iterate
				break
			}

			_, value := i.Element()
			vars = append(vars, castVar(value))
		}

		return vars
	}

	return nil
}
