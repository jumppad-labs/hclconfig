package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// EnsureAbsolute ensure that the given path is either absolute or
// if relative is converted to absolute based on the path of the config
func EnsureAbsolute(path, file string) string {
	// if the file starts with a / and we are on windows
	// we should treat this as absolute
	if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
		return filepath.Clean(path)
	}

	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	// path is relative so make absolute using the current file path as base
	file, _ = filepath.Abs(file)

	baseDir := file
	// check if the basepath is a file return its directory
	s, _ := os.Stat(file)
	if !s.IsDir() {
		baseDir = filepath.Dir(file)
	}

	fp := filepath.Join(baseDir, path)

	return filepath.Clean(fp)
}

// ParseVars converts a map[string]cty.Value into map[string]interface
// where the interface are generic go types like string, number, bool, slice, map
func ParseVars(value map[string]cty.Value) map[string]any {
	vars := map[string]any{}

	for k, v := range value {
		vars[k] = castVar(v)
	}

	return vars
}

// castVar converts a cty.Value to a Go any type
func castVar(v cty.Value) any {
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
		if v.IsNull() {
			return nil
		}

		return ParseVars(v.AsValueMap())
	} else if v.Type().IsTupleType() || v.Type().IsListType() {
		vars := []any{}

		if v.IsNull() {
			return vars
		}

		i := v.ElementIterator()

		for {
			if !i.Next() {
				// cant iterate
				break
			}

			_, value := i.Element()
			vars = append(vars, castVar(value))
		}

		return vars
	} else if v.Type() == cty.DynamicPseudoType {
		val, err := convert.Convert(v, cty.String)
		if err == nil {
			return val.AsString()
		}
	}

	return nil
}
