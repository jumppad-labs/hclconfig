package convert

import (
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

func GoToCtyValue(val interface{}) (t cty.Value, err error) {
	typ, err := gocty.ImpliedType(val)
	if err != nil {
		return cty.False, err
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				panic(r)
			}
		}
	}()

	t, err = gocty.ToCtyValue(val, typ)
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

		objMap := t.AsValueMap()
		metaMap := metaVal.AsValueMap()

		for k, v := range metaMap {
			objMap[k] = v
		}

		t = cty.ObjectVal(objMap)
	}

	return t, err
}

func CtyToGo(val cty.Value, target interface{}) error {
	return gocty.FromCtyValue(val, target)
}
