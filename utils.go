package hclconfig

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"os"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// ParseVars converts a map[string]cty.Value into map[string]interface
// where the interface are generic go types like string, number, bool, slice, map
func ParseVars(value map[string]cty.Value) map[string]any {
	vars := map[string]any{}

	for k, v := range value {
		vars[k] = castVar(v)
	}

	return vars
}

// ReadFileLocation reads a file between the given locations
func ReadFileLocation(filename string, startLine, startCol, endLine, endCol int) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", err)
	}

	fs := bufio.NewScanner(f)

	cl := 0
	output := ""

	for fs.Scan() {
		cl++
		if cl >= startLine && cl <= endLine {
			switch cl {
			case startLine:
				if startLine == endLine {
					output += fs.Text()[startCol-1 : endCol-1]
				} else {
					output += fmt.Sprintf("%s%s", fs.Text()[startCol-1:], LineEnding)
				}
			case endLine:
				output += fs.Text()[:endCol-1]
			default:
				output += fmt.Sprintf("%s%s", fs.Text(), LineEnding)
			}
		}
	}

	return output, nil
}

// HashString creates an MD5 hash of the given string
func HashString(in string) string {
	h := md5.New()
	h.Write([]byte(in))

	return fmt.Sprintf("%x", h.Sum(nil))
}

// Sharif dont like it
// Rock the cast var
// Rock the cast var
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
		v, err := convert.Convert(v, cty.String)
		if err == nil {
			return v
		}
	}

	return nil
}

