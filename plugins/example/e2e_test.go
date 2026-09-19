package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	plugintesting "github.com/jumppad-labs/xcl/plugins/testing"
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
	wireType, err := schema.CreateInstanceFromSchema(types[0].Schema, nil)
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
		_, err := ph.Create("resource", "person", personJSON)
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
		changed, err := ph.Changed("resource", "person", personJSON, personJSON)
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

// TestExternalPluginSchemaValidation tests that the external plugin returns valid schema
func TestExternalPluginSchemaValidation(t *testing.T) {
	// Build the plugin first
	buildCmd := plugintesting.BuildPlugin(t, ".")
	require.NoError(t, buildCmd, "Plugin should build successfully")

	ph := setupExternalPlugin(t)

	// Test schema validation
	types := ph.GetTypes()
	require.Len(t, types, 1, "Should have 1 registered type")
	require.Equal(t, "resource", types[0].Type, "Type should be 'resource'")
	require.Equal(t, "person", types[0].SubType, "SubType should be 'person'")
	require.NotEmpty(t, types[0].Schema, "Schema should not be empty")

	// Verify schema can create a concrete type
	wireType, err := schema.CreateInstanceFromSchema(types[0].Schema, nil)
	require.NoError(t, err, "Should be able to create struct from schema")
	require.NotNil(t, wireType, "Wire type should not be nil")
}

// TestExternalPluginCRUDOperations tests CRUD operations with external plugin
func TestExternalPluginCRUDOperations(t *testing.T) {
	// Build the plugin first
	buildCmd := plugintesting.BuildPlugin(t, ".")
	require.NoError(t, buildCmd, "Plugin should build successfully")

	ph := setupExternalPlugin(t)

	// Parse HCL file and get serialized JSON data for each person
	peopleData := plugintesting.ParseHCLWithPluginSchemaToEntityData(t, ph, "./examples/simple_person.hcl", person.Person{})
	require.NotEmpty(t, peopleData, "Should have parsed people from HCL file")

	// Test each person individually
	for i, personJSON := range peopleData {
		// Test Validate
		err := ph.Validate("resource", "person", personJSON)
		require.NoError(t, err, "Should validate person %d", i)

		// Test Create
		_, err = ph.Create("resource", "person", personJSON)
		require.NoError(t, err, "Should create person %d", i)

		// Test Changed
		changed, err := ph.Changed("resource", "person", personJSON, personJSON)
		require.NoError(t, err, "Should check changed status for person %d", i)
		require.False(t, changed, "Person %d should not have changed", i)

		// Test Destroy
		err = ph.Destroy("resource", "person", personJSON)
		require.NoError(t, err, "Should destroy person %d", i)
	}
}

// TestInProcessPluginRead tests that the in-process plugin returns the new copy
// of a person when the saved copy can still be located
func TestInProcessPluginRead(t *testing.T) {
	ph := setupInProcessPlugin(t)

	ctx := context.Background()
	oldData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com","address":"1 Old Street"}`)
	newData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":31,"email":"test@example.com","address":"2 New Street"}`)

	result, err := ph.Read(ctx, "resource", "person", oldData, newData)
	require.NoError(t, err, "Should read successfully")

	readPerson := person.Person{}
	err = json.Unmarshal(result, &readPerson)
	require.NoError(t, err, "Should unmarshal the read result")

	require.Equal(t, "Test", readPerson.FirstName, "First name should come from the new copy")
	require.Equal(t, "User", readPerson.LastName, "Last name should come from the new copy")
	require.Equal(t, 31, readPerson.Age, "Age should come from the new copy")
	require.Equal(t, "2 New Street", readPerson.Address, "Address should come from the new copy")
	require.Equal(t, "test@example.com", readPerson.Email, "Email should come from the new copy")
}

// TestInProcessPluginReadNotFoundIsErrNotFound tests that the in-process plugin
// reports a person whose saved copy can not be located as not found
func TestInProcessPluginReadNotFoundIsErrNotFound(t *testing.T) {
	ph := setupInProcessPlugin(t)

	ctx := context.Background()
	oldData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":30,"email":"missing@example.com"}`)
	newData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":30,"email":"missing@example.com"}`)

	result, err := ph.Read(ctx, "resource", "person", oldData, newData)
	require.Error(t, err, "Should fail to read a missing person")
	require.ErrorIs(t, err, plugins.ErrNotFound, "Error should be recognised as not found")
	require.Nil(t, result, "Result should be nil when the person is not found")
}

// TestExternalPluginRead tests that the external plugin receives the new copy
// of a person on Read and returns its values
func TestExternalPluginRead(t *testing.T) {
	// Build the plugin first
	buildCmd := plugintesting.BuildPlugin(t, ".")
	require.NoError(t, buildCmd, "Plugin should build successfully")

	ph := setupExternalPlugin(t)

	// The old and new copies differ in age and address so that the result
	// shows which copy the plugin returned
	ctx := context.Background()
	oldData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com","address":"1 Old Street"}`)
	newData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":31,"email":"test@example.com","address":"2 New Street"}`)

	result, err := ph.Read(ctx, "resource", "person", oldData, newData)
	require.NoError(t, err, "Should read successfully")

	readPerson := person.Person{}
	err = json.Unmarshal(result, &readPerson)
	require.NoError(t, err, "Should unmarshal the read result")

	require.Equal(t, "Test", readPerson.FirstName, "First name should come from the new copy")
	require.Equal(t, "User", readPerson.LastName, "Last name should come from the new copy")
	require.Equal(t, 31, readPerson.Age, "Age should come from the new copy")
	require.Equal(t, "2 New Street", readPerson.Address, "Address should come from the new copy")
	require.Equal(t, "test@example.com", readPerson.Email, "Email should come from the new copy")
}

// TestExternalPluginReadUsesOldCopyToLocateResource tests that the external
// plugin receives the old copy on Read, the example provider locates the person
// using the old copy's email so a missing old email must produce not found
// even when the new copy's email is ordinary
func TestExternalPluginReadUsesOldCopyToLocateResource(t *testing.T) {
	// Build the plugin first
	buildCmd := plugintesting.BuildPlugin(t, ".")
	require.NoError(t, buildCmd, "Plugin should build successfully")

	ph := setupExternalPlugin(t)

	ctx := context.Background()
	oldData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":30,"email":"missing@example.com"}`)
	newData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com"}`)

	result, err := ph.Read(ctx, "resource", "person", oldData, newData)
	require.Error(t, err, "Should fail to read when the old copy can not be located")
	require.ErrorIs(t, err, plugins.ErrNotFound, "Error should be recognised as not found")
	require.Nil(t, result, "Result should be nil when the person is not found")
}

// TestExternalPluginReadNotFoundIsErrNotFound tests that ErrNotFound returned by
// the external plugin is recognised as not found on the host side
func TestExternalPluginReadNotFoundIsErrNotFound(t *testing.T) {
	// Build the plugin first
	buildCmd := plugintesting.BuildPlugin(t, ".")
	require.NoError(t, buildCmd, "Plugin should build successfully")

	ph := setupExternalPlugin(t)

	ctx := context.Background()
	oldData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":30,"email":"missing@example.com"}`)
	newData := []byte(`{"meta":{"id":"test","type":"resource","sub_type":"person"},"first_name":"Test","last_name":"User","age":30,"email":"missing@example.com"}`)

	result, err := ph.Read(ctx, "resource", "person", oldData, newData)
	require.Error(t, err, "Should fail to read a missing person")
	require.ErrorIs(t, err, plugins.ErrNotFound, "Error should be recognised as not found")
	require.Nil(t, result, "Result should be nil when the person is not found")
}

// fullPersonJSON is a person with every configured field set, used to check
// that the example provider leaves configured fields alone
var fullPersonJSON = []byte(`{"meta":{"id":"resource.person.full","type":"resource","name":"full","file":"main.hcl","line":1},"first_name":"Ada","last_name":"Lovelace","age":36,"email":"ada@example.com","address":"12 St James's Square","description":"First programmer"}`)

// TestInProcessPluginCreateLeavesConfiguredFieldsAlone tests that Create returns
// every configured field exactly as it was sent
func TestInProcessPluginCreateLeavesConfiguredFieldsAlone(t *testing.T) {
	ph := setupInProcessPlugin(t)

	result, err := ph.Create("resource", "person", fullPersonJSON)
	require.NoError(t, err, "Should create successfully")

	createdPerson := person.Person{}
	err = json.Unmarshal(result, &createdPerson)
	require.NoError(t, err, "Should unmarshal the create result")

	require.Equal(t, "Ada", createdPerson.FirstName, "First name should be unchanged")
	require.Equal(t, "Lovelace", createdPerson.LastName, "Last name should be unchanged")
	require.Equal(t, 36, createdPerson.Age, "Age should be unchanged")
	require.Equal(t, "ada@example.com", createdPerson.Email, "Email should be unchanged")
	require.Equal(t, "12 St James's Square", createdPerson.Address, "Address should be unchanged")
	require.Equal(t, "First programmer", createdPerson.Description, "Description should be unchanged")
}

// TestInProcessPluginReadLeavesConfiguredFieldsAlone tests that Read returns
// every configured field of the new copy exactly as it was sent
func TestInProcessPluginReadLeavesConfiguredFieldsAlone(t *testing.T) {
	ph := setupInProcessPlugin(t)

	ctx := context.Background()
	result, err := ph.Read(ctx, "resource", "person", fullPersonJSON, fullPersonJSON)
	require.NoError(t, err, "Should read successfully")

	readPerson := person.Person{}
	err = json.Unmarshal(result, &readPerson)
	require.NoError(t, err, "Should unmarshal the read result")

	require.Equal(t, "Ada", readPerson.FirstName, "First name should be unchanged")
	require.Equal(t, "Lovelace", readPerson.LastName, "Last name should be unchanged")
	require.Equal(t, 36, readPerson.Age, "Age should be unchanged")
	require.Equal(t, "ada@example.com", readPerson.Email, "Email should be unchanged")
	require.Equal(t, "12 St James's Square", readPerson.Address, "Address should be unchanged")
	require.Equal(t, "First programmer", readPerson.Description, "Description should be unchanged")
}

// TestInProcessPluginUpdateLeavesConfiguredFieldsAlone tests that Update returns
// every configured field exactly as it was sent
func TestInProcessPluginUpdateLeavesConfiguredFieldsAlone(t *testing.T) {
	ph := setupInProcessPlugin(t)

	result, err := ph.Update("resource", "person", fullPersonJSON)
	require.NoError(t, err, "Should update successfully")

	updatedPerson := person.Person{}
	err = json.Unmarshal(result, &updatedPerson)
	require.NoError(t, err, "Should unmarshal the update result")

	require.Equal(t, "Ada", updatedPerson.FirstName, "First name should be unchanged")
	require.Equal(t, "Lovelace", updatedPerson.LastName, "Last name should be unchanged")
	require.Equal(t, 36, updatedPerson.Age, "Age should be unchanged")
	require.Equal(t, "ada@example.com", updatedPerson.Email, "Email should be unchanged")
	require.Equal(t, "12 St James's Square", updatedPerson.Address, "Address should be unchanged")
	require.Equal(t, "First programmer", updatedPerson.Description, "Description should be unchanged")
}

// TestInProcessPluginChangedReportsNoChangeForIdenticalData tests that the
// default change detection reports no change when both copies are identical
func TestInProcessPluginChangedReportsNoChangeForIdenticalData(t *testing.T) {
	ph := setupInProcessPlugin(t)

	oldData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com"}`)
	newData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com"}`)

	changed, err := ph.Changed("resource", "person", oldData, newData)
	require.NoError(t, err, "Should check changed status")
	require.False(t, changed, "Identical data should not be reported as changed")
}

// TestInProcessPluginChangedReportsChangeWhenFieldDiffers tests that the
// default change detection reports a change when a configured field differs
func TestInProcessPluginChangedReportsChangeWhenFieldDiffers(t *testing.T) {
	ph := setupInProcessPlugin(t)

	oldData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com"}`)
	newData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":31,"email":"test@example.com"}`)

	changed, err := ph.Changed("resource", "person", oldData, newData)
	require.NoError(t, err, "Should check changed status")
	require.True(t, changed, "A different age should be reported as changed")
}

// TestInProcessPluginChangedIgnoresMetadata tests that the default change
// detection ignores differences in xcl's resource metadata
func TestInProcessPluginChangedIgnoresMetadata(t *testing.T) {
	ph := setupInProcessPlugin(t)

	oldData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test","file":"old.hcl","line":1},"first_name":"Test","last_name":"User","age":30}`)
	newData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test","file":"new.hcl","line":42},"first_name":"Test","last_name":"User","age":30}`)

	changed, err := ph.Changed("resource", "person", oldData, newData)
	require.NoError(t, err, "Should check changed status")
	require.False(t, changed, "Differences only in meta should not be reported as changed")
}

// TestInProcessPluginReadHelperReturnsConfiguredCopy tests the TestRead helper
// returns the new copy with its configured fields unchanged
func TestInProcessPluginReadHelperReturnsConfiguredCopy(t *testing.T) {
	ph := setupInProcessPlugin(t)
	ops := plugintesting.NewTestPluginOperations(ph)

	oldData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com"}`)
	newData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":31,"email":"test@example.com","description":"Updated"}`)

	result := ops.TestRead("resource", "person", oldData, newData)

	readPerson := person.Person{}
	err := json.Unmarshal(result, &readPerson)
	require.NoError(t, err, "Should unmarshal the read result")

	require.Equal(t, "Test", readPerson.FirstName, "First name should come from the new copy")
	require.Equal(t, "User", readPerson.LastName, "Last name should come from the new copy")
	require.Equal(t, 31, readPerson.Age, "Age should come from the new copy")
	require.Equal(t, "test@example.com", readPerson.Email, "Email should come from the new copy")
	require.Equal(t, "Updated", readPerson.Description, "Description should come from the new copy")
}

// TestExternalPluginChangedReportsChangeWhenFieldDiffers tests that the default
// change detection reports a change through the external plugin
func TestExternalPluginChangedReportsChangeWhenFieldDiffers(t *testing.T) {
	// Build the plugin first
	buildCmd := plugintesting.BuildPlugin(t, ".")
	require.NoError(t, buildCmd, "Plugin should build successfully")

	ph := setupExternalPlugin(t)

	oldData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com"}`)
	newData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":31,"email":"test@example.com"}`)

	changed, err := ph.Changed("resource", "person", oldData, newData)
	require.NoError(t, err, "Should check changed status")
	require.True(t, changed, "A different age should be reported as changed")
}

// TestExternalPluginChangedReportsNoChangeForIdenticalData tests that the
// default change detection reports no change through the external plugin
func TestExternalPluginChangedReportsNoChangeForIdenticalData(t *testing.T) {
	// Build the plugin first
	buildCmd := plugintesting.BuildPlugin(t, ".")
	require.NoError(t, buildCmd, "Plugin should build successfully")

	ph := setupExternalPlugin(t)

	oldData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com"}`)
	newData := []byte(`{"meta":{"id":"resource.person.test","type":"resource","name":"test"},"first_name":"Test","last_name":"User","age":30,"email":"test@example.com"}`)

	changed, err := ph.Changed("resource", "person", oldData, newData)
	require.NoError(t, err, "Should check changed status")
	require.False(t, changed, "Identical data should not be reported as changed")
}
