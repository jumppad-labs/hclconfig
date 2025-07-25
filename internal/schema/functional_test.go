package schema

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

type Network struct {
	Name    string `hcl:"name"`
	Enabled bool   `hcl:"enabled"`
}

type Nested struct {
	Name  string  `hcl:"name"`
	Inner *Nested `hcl:"inner,block"`
}

type MyEntity struct {
	Foo      string            `hcl:"foo"`
	Count    int               `hcl:"count"`
	Float    float64           `hcl:"float"`
	FooRef   *string           `hcl:"foo_ref"`
	CountRef *int              `hcl:"count_ref"`
	FloatRef *float64          `hcl:"float_ref"`
	Map      map[string]string `hcl:"map"`
	Slice    []string          `hcl:"slice"`

	NetworkMap    map[string]Network `hcl:"network_map"`
	Networks      []*Network         `hcl:"network,block"`
	NetworkStruct Network            `hcl:"network_struct,block"`
	NetworkRef    *Network           `hcl:"network_ref,block"`

	Nested1 *Nested `hcl:"nested_1,block"`
	Nested2 *Nested `hcl:"nested_2,block"`
}

func TestEnd2EndTestConfigToStruct(t *testing.T) {
	// create the scehma from the struct
	jsonSchema, err := GenerateSchemaFromInstance(MyEntity{}, 10)
	require.NoError(t, err)

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
		wireType, err := CreateInstanceFromSchema(jsonSchema, nil)
		require.NoError(t, err)

		// Decode the block into the struct
		diags := gohcl.DecodeBody(block.Body, ctx, wireType)
		require.Empty(t, diags.Errs())

		// Marshal the struct to concrete type
		my := &MyEntity{}
		err = UnmarshalUntyped(wireType, my)
		require.NoError(t, err)

		// Check the Vaules
		require.Equal(t, "foo", my.Foo)
		require.Equal(t, 2, my.Count)
		require.Equal(t, 3.33, my.Float)

		fooRef := "foo"
		countRef := 3
		floatRef := 4.33
		require.Equal(t, &fooRef, my.FooRef)
		require.Equal(t, &countRef, my.CountRef)
		require.Equal(t, &floatRef, my.FloatRef)

		require.Equal(t, map[string]string{"a": "b"}, my.Map)
		require.Equal(t, []string{"a", "b", "c"}, my.Slice)

		require.Equal(t, "default", my.NetworkMap["default"].Name)
		require.True(t, my.NetworkMap["default"].Enabled)
		require.Equal(t, "other", my.NetworkMap["other"].Name)
		require.False(t, my.NetworkMap["other"].Enabled)

		require.Equal(t, "one", my.Networks[0].Name)
		require.True(t, my.Networks[0].Enabled)

		require.Equal(t, "two", my.Networks[1].Name)
		require.False(t, my.Networks[1].Enabled)

		require.Equal(t, "struct", my.NetworkStruct.Name)
		require.True(t, my.NetworkStruct.Enabled)

		require.Equal(t, "ref", my.NetworkRef.Name)
		require.False(t, my.NetworkRef.Enabled)

		require.Equal(t, "foo", my.Nested1.Name)

		require.Equal(t, "bar", my.Nested2.Name)
		require.Equal(t, "baz", my.Nested2.Inner.Name)
	}
}
