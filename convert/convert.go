package convert

import (
	"fmt"

	"github.com/jumppad-labs/hclconfig/types"
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

	if r, ok := val.(types.Resource); ok {
		ctyMap := ctyVal.AsValueMap()

		// add disabled to the parent
		ctyMap["disabled"] = cty.BoolVal(r.GetDisabled())

		// add depends_on to the parent
		depTyp, err := gocty.ImpliedType(r.GetDependencies())
		if err != nil {
			return cty.False, err
		}

		dep, err := gocty.ToCtyValue(r.GetDependencies(), depTyp)
		if err != nil {
			return cty.False, fmt.Errorf("unable to convert depends_on to cty: %s", err)
		}
		ctyMap["depends_on"] = dep

		// add the meta properties to the parent
		typ, err := gocty.ImpliedType(r.Metadata())
		if err != nil {
			return cty.False, err
		}

		metaVal, err := gocty.ToCtyValue(r.Metadata(), typ)
		if err != nil {
			return cty.False, err
		}

		ctyMap["meta"] = metaVal
		ctyVal = cty.ObjectVal(ctyMap)
	}

	return ctyVal, nil
}

func CtyToGo(val cty.Value, target any) error {
	return gocty.FromCtyValue(val, target)
}
