package convert

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestGoStructToCtyValue(t *testing.T) {
	cont1 := structs.ContainerBase{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID: "test_container",
			},
		},
		Command: []string{"ls", "-las"},
	}

	//val := reflect.ValueOf(cont)
	ct, err := GoToCtyValue(&cont1)
	require.NoError(t, err)
	require.NotNil(t, ct.AsValueMap()["meta"])

	val := ct.AsValueMap()["meta"].AsValueMap()["id"].AsString() // should be "test"
	require.Equal(t, "test_container", val)
}

func TestGoStructToCtyValueWithEmbeddedStruct(t *testing.T) {
	cont2 := structs.Container{
		ContainerBase: structs.ContainerBase{
			ResourceBase: types.ResourceBase{
				Meta: types.Meta{
					ID: "test_container",
				},
			},
			Command: []string{"ls", "-las"},
		},
	}

	//val := reflect.ValueOf(cont)
	ct, err := GoToCtyValue(&cont2)
	require.NoError(t, err)
	require.NotNil(t, ct.AsValueMap()["meta"])
	require.False(t, ct.AsValueMap()["meta"].IsNull()) // should not be null

	val := ct.AsValueMap()["meta"].AsValueMap()["id"].AsString() // should be "test"
	require.Equal(t, "test_container", val)
}

func TestGoToCtyWorksWithDeserializedTypes(t *testing.T) {
	con1 := structs.Template{}

	s, err := schema.GenerateSchemaFromInstance(con1, 10)
	require.NoError(t, err)

	typeMapping := map[string]reflect.Type{
		"types.Meta":         reflect.TypeOf(types.Meta{}),
		"types.ResourceBase": reflect.TypeOf(types.ResourceBase{}),
		"cty.Value":          reflect.TypeOf(cty.Value{}), // Treat cty.Value as interface{}
	}

	con2, err := schema.CreateInstanceFromSchema(s, typeMapping)
	require.NoError(t, err)

	setField(con2, "Meta", types.Meta{
		ID: "test",
	})
	setField(con2, "Destination", "ls")

	ct, err := GoToCtyValue(con2)
	require.NoError(t, err)

	val := ct.AsValueMap()["destination"].AsString() // should be "ls"
	require.Equal(t, "ls", val)

	val = ct.AsValueMap()["meta"].AsValueMap()["id"].AsString() // should be "test"
	require.Equal(t, "test", val)
}

func setField(obj interface{}, fieldName string, value interface{}) error {
	v := reflect.ValueOf(obj)

	// Must pass a pointer to modify the original
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("object must be a pointer")
	}

	// Get the underlying element
	v = v.Elem()

	// Get the field
	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return fmt.Errorf("field %s not found", fieldName)
	}

	if !field.CanSet() {
		return fmt.Errorf("field %s cannot be set", fieldName)
	}

	// Set the value
	field.Set(reflect.ValueOf(value))
	return nil
}
