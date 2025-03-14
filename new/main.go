package main

import (
	"encoding/json"
	"reflect"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/kr/pretty"
	"github.com/zclconf/go-cty/cty"
)

type Network struct {
	Name string `hcl:"name"`
}
type MyEntity struct {
	Foo string `hcl:"foo"`

	Networks []Network `hcl:"network,block"`
}

func main() {
	var sfs []reflect.StructField
	sfs = append(sfs, reflect.StructField{
		Name: "Foo",
		Type: reflect.TypeOf("string"),
		Tag:  `hcl:"foo"`,
	})

	sfs = append(sfs, reflect.StructField{
		Name: "Networks",
		Type: reflect.TypeOf([]struct {
			Name string `hcl:"name"`
		}{}),
		Tag: `hcl:"network,block"`,
	})

	st := reflect.StructOf(sfs)
	p := reflect.New(st).Interface()

	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile("./test.hcl")

	if diags.HasErrors() {
		pretty.Println(diags.Error())
	}

	resources := []interface{}{}

	//p := map[string]cty.Value{}

	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{},
	}

	valMap := map[string]cty.Value{}
	valMap["a"] = cty.StringVal("foo")

	ctx.Variables["var"] = cty.ObjectVal(valMap)

	body := f.Body.(*hclsyntax.Body)

	for _, block := range body.Blocks {
		diags := gohcl.DecodeBody(block.Body, ctx, p)
		if diags.HasErrors() {
			pretty.Println(diags.Error())
		}

		resources = append(resources, p)

		d, _ := json.Marshal(p)

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
