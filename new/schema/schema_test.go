package schema

import (
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

type Network struct {
	Name string `hcl:"name"`
}

type MyEntity struct {
	Foo string `hcl:"foo"`

	Networks []Network `hcl:"network,block"`
}

var myEntityJson = `{
  "type": "schema.MyEntity",
  "properties": [
   {
    "name": "Foo",
    "type": "string",
    "tags": "hcl:\"foo\""
   },
   {
    "name": "Networks",
    "type": "[]schema.Network",
    "tags": "hcl:\"network,block\"",
    "properties": [
     {
      "name": "Name",
      "type": "string",
      "tags": "hcl:\"name\""
     }
    ]
   }
  ]
 }`

func TestSerialize(t *testing.T) {
	b, err := GenerateFromInstance(MyEntity{})
	require.NoError(t, err)

	pretty.Println(string(b))

	require.JSONEq(t, myEntityJson, string(b))
}
