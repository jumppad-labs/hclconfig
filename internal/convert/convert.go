package convert

import (
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

func GoToCtyValue(val any) (cty.Value, error) {
	typ, err := gocty.ImpliedType(val)
	if err != nil {
		return cty.False, err
	}

	ctyVal, err := gocty.ToCtyValue(val, typ)
	if err != nil {
		return cty.False, err
	}

	return ctyVal, nil
}

func CtyToGo(val cty.Value, target any) error {
	return gocty.FromCtyValue(val, target)
}
