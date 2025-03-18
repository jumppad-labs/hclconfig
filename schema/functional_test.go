package schema

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

type Network struct {
	Name    string `hcl:"name"`
	Enabled bool   `hcl:"enabled"`
}

type MyEntity struct {
	Foo   string            `hcl:"foo"`
	Count int               `hcl:"count"`
	Float float64           `hcl:"float"`
	Map   map[string]string `hcl:"map"`
	Slice []string          `hcl:"slice"`

	NetworkMap    map[string]Network `hcl:"network_map"`
	Networks      []*Network         `hcl:"network,block"`
	NetworkStruct *Network           `hcl:"network_struct,block"`
}

func TestEnd2EndTestConfigToStruct(t *testing.T) {
	// create the scehma from the struct
	jsonSchema, err := GenerateFromInstance(MyEntity{}, 10)
	require.NoError(t, err)
	pretty.Println(string(jsonSchema))

	// parse the HCL file
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile("./test_fixtures/test.hcl")
	require.Empty(t, diags.Errs())

	// Create a new eval context
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{},
	}

	// add interpolation context to check this works
	valMap := map[string]cty.Value{}
	valMap["a"] = cty.StringVal("foo")
	ctx.Variables["var"] = cty.ObjectVal(valMap)

	// Loop through the blocks in the HCL file
	body := f.Body.(*hclsyntax.Body)

	for _, block := range body.Blocks {
		// Create a new struct from the schema
		wireType, err := CreateStructFromSchema(jsonSchema)
		require.NoError(t, err)
		pretty.Println(wireType)

		// Decode the block into the struct
		diags := gohcl.DecodeBody(block.Body, ctx, wireType)
		require.Empty(t, diags.Errs())

		// Marshal the struct to concrete type
		my := &MyEntity{}
		err = UnmarshalUntyped(wireType, my)
		require.NoError(t, err)
	}
}
