package functionaltest

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/plugins/example"
	"github.com/jumppad-labs/hclconfig/schema"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestStuff(t *testing.T) {
	l := &plugins.TestLogger{}
	ph := plugins.NewPluginHost(l, nil, "")

	err := ph.Start("../example/bin/example")
	require.NoError(t, err, "Plugin should start without error")

	types := ph.GetTypes()
	require.Len(t, types, 1, "Should have one registered type")

	person := types[0]
	require.Equal(t, "resource", person.Type, "Type should be 'resource'")
	require.Equal(t, "person", person.SubType, "SubType should be 'person'")
	require.NotEmpty(t, person.Schema, "Schema should not be empty")

	// read the example file into a struct created from the plugin schema
	wireType, err := schema.CreateStructFromSchema(person.Schema)
	if err != nil {
		pretty.Println(err)
		return
	}

	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile("../example/test_fixtures/person.hcl")

	if diags.HasErrors() {
		pretty.Println(diags.Error())
	}

	// Create a new eval context
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{},
	}

	valMap := map[string]cty.Value{}
	valMap["a"] = cty.StringVal("foo")
	ctx.Variables["var"] = cty.ObjectVal(valMap)

	people := make([]example.Person, 0)

	// Loop through the blocks in the HCL file
	body := f.Body.(*hclsyntax.Body)

	for _, block := range body.Blocks {
		// Decode the block into the struct
		diags := gohcl.DecodeBody(block.Body, ctx, wireType)
		if diags.HasErrors() {
			pretty.Println(diags.Error())
		}

		person := example.Person{}
		schema.UnmarshalUntyped(wireType, &person)
		people = append(people, person)
	}

	require.Len(t, people, 3, "Should have three person")

	// Check the first person
	require.Equal(t, "John", people[0].FirstName, "First name should be 'John'")
	require.Equal(t, "Jane", people[1].FirstName, "First name should be 'Jane'")
	require.Equal(t, "Alice", people[2].FirstName, "First name should be 'Alice'")
}
