package main

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jumppad-labs/hclconfig/new/schema"
	"github.com/kr/pretty"
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

func main() {
	// Create a schema from the struct
	fmt.Println("Creating schema from struct")
	jsonSchema, err := schema.GenerateFromInstance(MyEntity{})
	if err != nil {
		pretty.Println(err)
		return
	}

	pretty.Println(string(jsonSchema))
	fmt.Println()

	// Parse the HCL file
	fmt.Println("Parsing HCL file")
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile("./test.hcl")

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

	// Loop through the blocks in the HCL file
	body := f.Body.(*hclsyntax.Body)

	for _, block := range body.Blocks {
		fmt.Println("Parsing block")

		// Create a new struct from the schema
		fmt.Println("Creating struct from schema")
		wireType, err := schema.CreateStructFromSchema(jsonSchema)
		if err != nil {
			pretty.Println(err)
			return
		}

		// Decode the block into the struct
		diags := gohcl.DecodeBody(block.Body, ctx, wireType)
		if diags.HasErrors() {
			pretty.Println(diags.Error())
		}

		pretty.Println(wireType)
		fmt.Println()

		// Marshal the struct to JSON
		fmt.Println("Marshalling struct to JSON")
		d, _ := json.MarshalIndent(wireType, " ", " ")

		pretty.Println(string(d))
		fmt.Println()

		// Unmarshal the JSON back into the struct
		fmt.Println("Unmarshalling JSON to struct")
		my := &MyEntity{}
		json.Unmarshal(d, my)

		pretty.Println(my)
	}

	//jsons := [][]byte{}
	//for _, r := range resources {
	//	d, _ := json.Marshal(cty.ObjectVal(r), cty.DynamicPseudoType)
	//	jsons = append(jsons, d)
	//}

	//for _, j := range jsons {
	//	v, _ := json.Unmarshal(j, cty.DynamicPseudoType)
	//	pretty.Println(v)

	//	myEntity := &MyEntity{}
	//	err := gocty.FromCtyValue(v, myEntity)
	//	if err != nil {
	//		pretty.Println(err)
	//	}

	//	pretty.Println(myEntity)
	//}
}
