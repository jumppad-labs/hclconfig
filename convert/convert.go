package convert

import (
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

func GoToCtyValue(val interface{}) (cty.Value, error) {
	typ, err := gocty.ImpliedType(val)
	if err != nil {
		return cty.False, err
	}

	ctyVal, err := gocty.ToCtyValue(val, typ)
	if err != nil {
		return cty.False, err
	}

	if r, ok := val.(types.Resource); ok {
		typ, err := gocty.ImpliedType(r.Metadata())
		if err != nil {
			return cty.False, err
		}

		metaVal, err := gocty.ToCtyValue(r.Metadata(), typ)
		if err != nil {
			return cty.False, err
		}

		objMap := ctyVal.AsValueMap()
		metaMap := metaVal.AsValueMap()

		for k, v := range metaMap {
			objMap[k] = v
		}

		ctyVal = cty.ObjectVal(objMap)
	}

	return ctyVal, nil
}

func CtyToGo(val cty.Value, target interface{}) error {
	return gocty.FromCtyValue(val, target)
}
