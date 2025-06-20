package main

import (
	"testing"

	"github.com/jumppad-labs/hclconfig/plugins/example/pkg/person"
	plugintesting "github.com/jumppad-labs/hclconfig/plugins/testing"
	"github.com/jumppad-labs/hclconfig/schema"
	"github.com/stretchr/testify/require"
)

// setupInProcessPlugin creates an in-process plugin host for testing
func setupInProcessPlugin(t *testing.T) *plugintesting.TestPluginHost {
	plugin := &PersonPlugin{}
	return plugintesting.InProcessPluginSetup(t, plugin)
}

// setupExternalPlugin creates an external process plugin host for testing
func setupExternalPlugin(t *testing.T) *plugintesting.TestPluginHost {
	return plugintesting.ExternalPluginSetup(t, "./build/example")
}

// TestInProcessPluginSchemaValidation tests that the in-process plugin returns valid schema
func TestInProcessPluginSchemaValidation(t *testing.T) {
	ph := setupInProcessPlugin(t)

	// Test schema validation
	types := ph.GetTypes()
	require.Len(t, types, 1, "Should have 1 registered type")
	require.Equal(t, "resource", types[0].Type, "Type should be 'resource'")
	require.Equal(t, "person", types[0].SubType, "SubType should be 'person'")
	require.NotEmpty(t, types[0].Schema, "Schema should not be empty")

	// Verify schema can create a concrete type
	wireType, err := schema.CreateStructFromSchema(types[0].Schema)
	require.NoError(t, err, "Should be able to create struct from schema")
	require.NotNil(t, wireType, "Wire type should not be nil")
}

// TestInProcessPluginConcreteTypeCreation tests parsing HCL and creating concrete Person types
func TestInProcessPluginConcreteTypeCreation(t *testing.T) {
	ph := setupInProcessPlugin(t)

	// Parse HCL file into Person objects
	people := plugintesting.ParseHCLWithPluginSchema(t, ph, "./examples/person.hcl", person.Person{})

	// Verify results
	require.Len(t, people, 3, "Should have three persons")
	require.Equal(t, "John", people[0].FirstName, "First person should be John")
	require.Equal(t, "Jane", people[1].FirstName, "Second person should be Jane")
	require.Equal(t, "Alice", people[2].FirstName, "Third person should be Alice")
}

// TestInProcessPluginValidate tests the Validate operation
func TestInProcessPluginValidate(t *testing.T) {
	ph := setupInProcessPlugin(t)

	// Parse HCL file and get serialized JSON data for each person
	peopleData := plugintesting.ParseHCLWithPluginSchemaToEntityData(t, ph, "./examples/person.hcl", person.Person{})
	require.NotEmpty(t, peopleData, "Should have parsed people from HCL file")

	// Test each person individually
	for i, personJSON := range peopleData {
		// Call Validate on the plugin
		err := ph.Validate("resource", "person", personJSON)
		require.NoError(t, err, "Should validate person %d", i)
	}
}

// TestInProcessPluginCreate tests the Create operation
func TestInProcessPluginCreate(t *testing.T) {
	ph := setupInProcessPlugin(t)

	// Parse HCL file and get serialized JSON data for each person
	peopleData := plugintesting.ParseHCLWithPluginSchemaToEntityData(t, ph, "./examples/simple_person.hcl", person.Person{})
	require.NotEmpty(t, peopleData, "Should have parsed people from HCL file")

	// Test each person individually
	for i, personJSON := range peopleData {
		// Call Create on the plugin
		err := ph.Create("resource", "person", personJSON)
		require.NoError(t, err, "Should create person %d", i)
	}
}

// TestInProcessPluginChanged tests the Changed operation
func TestInProcessPluginChanged(t *testing.T) {
	ph := setupInProcessPlugin(t)

	// Parse HCL file and get serialized JSON data for each person
	peopleData := plugintesting.ParseHCLWithPluginSchemaToEntityData(t, ph, "./examples/simple_person.hcl", person.Person{})
	require.NotEmpty(t, peopleData, "Should have parsed people from HCL file")

	// Test each person individually
	for i, personJSON := range peopleData {
		// Call Changed on the plugin
		changed, err := ph.Changed("resource", "person", personJSON)
		require.NoError(t, err, "Should check changed status for person %d", i)
		require.False(t, changed, "Person %d should not have changed", i)
	}
}

// TestInProcessPluginDestroy tests the Destroy operation
func TestInProcessPluginDestroy(t *testing.T) {
	ph := setupInProcessPlugin(t)

	// Parse HCL file and get serialized JSON data for each person
	peopleData := plugintesting.ParseHCLWithPluginSchemaToEntityData(t, ph, "./examples/simple_person.hcl", person.Person{})
	require.NotEmpty(t, peopleData, "Should have parsed people from HCL file")

	// Test each person individually
	for i, personJSON := range peopleData {
		// Call Destroy on the plugin
		err := ph.Destroy("resource", "person", personJSON)
		require.NoError(t, err, "Should destroy person %d", i)
	}
}
