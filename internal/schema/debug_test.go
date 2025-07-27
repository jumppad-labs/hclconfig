package schema

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jumppad-labs/hclconfig/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestTemplateSchemaHCLTagPreservation(t *testing.T) {
	// Create a Template instance to generate schema from
	template := &structs.Template{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				Name: "test",
				Type: "template",
			},
		},
		Source:      "test source",
		Destination: "test destination",
		Vars:        cty.StringVal("test"),
		AppendFile:  true,
	}

	// Step 1: Generate schema from Template instance
	t.Log("=== Step 1: Generate schema from Template instance ===")
	schemaJSON, schemaErr := GenerateSchemaFromInstance(template, 5)
	require.NoError(t, schemaErr)

	// Print the schema to see if HCL tags are preserved
	t.Logf("Generated schema JSON:\n%s", string(schemaJSON))

	// Parse the schema to verify HCL tags
	var schemaAttr Attribute
	parseErr := json.Unmarshal(schemaJSON, &schemaAttr)
	require.NoError(t, parseErr)

	// Step 2: Verify HCL tags are preserved in schema
	t.Log("=== Step 2: Verify HCL tags are preserved in schema ===")
	for _, prop := range schemaAttr.Properties {
		t.Logf("Property: %s, Type: %s, Tags: %s", prop.Name, prop.Type, prop.Tags)
		
		// Check specific fields we expect
		switch prop.Name {
		case "Source":
			require.Contains(t, prop.Tags, `hcl:"source"`, "Source field should have hcl:\"source\" tag")
		case "Destination":
			require.Contains(t, prop.Tags, `hcl:"destination"`, "Destination field should have hcl:\"destination\" tag")
		case "Vars":
			require.Contains(t, prop.Tags, `hcl:"vars,optional"`, "Vars field should have hcl:\"vars,optional\" tag")
		case "AppendFile":
			require.Contains(t, prop.Tags, `hcl:"append_file,optional"`, "AppendFile field should have hcl:\"append_file,optional\" tag")
		}
	}

	// Step 3: Deserialize schema back to struct
	t.Log("=== Step 3: Deserialize schema back to struct ===")
	typeMapping := map[string]reflect.Type{
		"types.Meta":               reflect.TypeOf(types.Meta{}),
		"types.ResourceBase":       reflect.TypeOf(types.ResourceBase{}),
		"map[string]interface {}":  reflect.TypeOf(map[string]interface{}{}),
		"map[string]any":           reflect.TypeOf(map[string]interface{}{}),
		"cty.Value":                reflect.TypeOf((*interface{})(nil)).Elem(),
	}

	dynamicTemplate, deserializeErr := CreateInstanceFromSchema(schemaJSON, typeMapping)
	require.NoError(t, deserializeErr)
	require.NotNil(t, dynamicTemplate)

	// Step 4: Inspect the dynamic struct tags
	t.Log("=== Step 4: Inspect the dynamic struct tags ===")
	dynamicType := reflect.TypeOf(dynamicTemplate)
	if dynamicType.Kind() == reflect.Ptr {
		dynamicType = dynamicType.Elem()
	}

	t.Logf("Dynamic struct type: %s", dynamicType.String())
	for i := 0; i < dynamicType.NumField(); i++ {
		field := dynamicType.Field(i)
		t.Logf("Field: %s, Type: %s, Tag: %s", field.Name, field.Type, field.Tag)
		
		// Check that HCL tags are preserved
		switch field.Name {
		case "Source":
			require.Contains(t, string(field.Tag), `hcl:"source"`, "Dynamic Source field should have hcl:\"source\" tag")
		case "Destination":
			require.Contains(t, string(field.Tag), `hcl:"destination"`, "Dynamic Destination field should have hcl:\"destination\" tag")
		case "Vars":
			require.Contains(t, string(field.Tag), `hcl:"vars,optional"`, "Dynamic Vars field should have hcl:\"vars,optional\" tag")
		case "AppendFile":
			require.Contains(t, string(field.Tag), `hcl:"append_file,optional"`, "Dynamic AppendFile field should have hcl:\"append_file,optional\" tag")
		}
	}

	// Step 5: Test actual HCL parsing with the dynamic struct
	t.Log("=== Step 5: Test actual HCL parsing with the dynamic struct ===")
	hclSource := `
destination = "test.txt"
source = "hello world"
append_file = true
vars = {
  key = "value"
}
`

	hclFile, hclDiags := hclsyntax.ParseConfig([]byte(hclSource), "test.hcl", hcl.Pos{Line: 1, Column: 1})
	require.False(t, hclDiags.HasErrors(), "HCL parse should succeed: %v", hclDiags)

	// Try to decode into the dynamic struct
	dynamicValue := reflect.New(dynamicType)
	decodeErr := gohcl.DecodeBody(hclFile.Body, nil, dynamicValue.Interface())
	if decodeErr != nil {
		t.Logf("HCL decoding error: %v", decodeErr)
		t.Logf("This confirms the issue - HCL cannot find the attributes in the dynamic struct")
	} else {
		t.Log("HCL decoding succeeded - tags are working correctly")
		decodedTemplate := dynamicValue.Interface()
		t.Logf("Decoded template: %+v", decodedTemplate)
	}
}