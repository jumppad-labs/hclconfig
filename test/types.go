package test

import (
	"encoding/json"

	"github.com/jumppad-labs/hclconfig/types"
	"github.com/zclconf/go-cty/cty"
)


type SerializableParams string

func (s *SerializableParams) Serialize(p interface{}) {
	d, _ := json.Marshal(p)
	*s = SerializableParams(d)
}

func (s *SerializableParams) Deserialize(i interface{}) {
	json.Unmarshal([]byte(*s), &i)
}
