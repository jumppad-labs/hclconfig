// Example showing how it is possible to generate dynamic structs from a
// schema
// This could be used to move HCL config away from concrete to dynamic types
package main

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Schema struct {
	Type   string   `hcl:",label"`
	Fields []*Field `hcl:"field,block"`
}

type Field struct {
	Name     string   `hcl:",label"`
	Type     string   `hcl:"type"`
	Required bool     `hcl:"required,optional"`
	Fields   []*Field `hcl:"field,block"`
}

type Person struct {
	Name string `hcl:"name,optional"`
	Age  int    `hcl:"age,optional"`

	// Pet []Pet
}

type Pet struct {
	Name string
	Age  int
}

func main() {
	parser := hclparse.NewParser()

	f, diag := parser.ParseHCLFile("./example_schema.hcl")
	if diag.HasErrors() {
		panic(diag.Error())
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		panic("boom")
	}

	ctx := &hcl.EvalContext{}

	for _, b := range body.Blocks {
		val := &Schema{}
		gohcl.DecodeBody(b.Body, ctx, val)

		// generate the dynamic type
		dt := createDynamicType(val)

		parser2 := hclparse.NewParser()

		f, diag := parser2.ParseHCLFile("./example_data.hcl")
		if diag.HasErrors() {
			panic(diag.Error())
		}

		body, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			panic("boom")
		}

		ctx := &hcl.EvalContext{}

		for _, b := range body.Blocks {
			p := reflect.New(dt)
			v := p.Interface()
			gohcl.DecodeBody(b.Body, ctx, v)

			d, _ := json.Marshal(v)
			pp := &Person{}

			json.Unmarshal(d, pp)
		}
	}
}

func createDynamicType(s *Schema) reflect.Type {
	fields := []reflect.StructField{}

	for _, f := range s.Fields {
		t := getType(f.Type)
		if t != nil {
			field := reflect.StructField{
				Name: cases.Title(language.English).String(f.Name),
				Type: getType(f.Type),
				Tag:  reflect.StructTag(fmt.Sprintf(`hcl:"%s,optional"`, f.Name)),
			}

			fields = append(fields, field)
		}
	}

	return reflect.StructOf(fields)
}

func getType(t string) reflect.Type {
	switch t {
	case "string":
		return reflect.TypeOf("")
	case "int":
		return reflect.TypeOf(0)
	default:
		return nil
	}
}
