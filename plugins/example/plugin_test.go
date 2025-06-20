package main

import (
	"encoding/json"
	"testing"

	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/plugins/example/pkg/person"
	"github.com/jumppad-labs/hclconfig/plugins/mocks"
	"github.com/stretchr/testify/require"
)

func testSetup(t *testing.T) (plugins.Logger, plugins.State) {
	// Create a test logger and state
	logger := &plugins.TestLogger{}
	state := mocks.NewMockState(t)
	return logger, state
}

func testInitPlugin(t *testing.T) *PersonPlugin {
	// Create an instance of the plugin
	plugin := &PersonPlugin{}
	logger, state := testSetup(t)

	err := plugin.Init(logger, state)
	require.NoError(t, err, "Plugin should initialize without error")

	return plugin
}

func TestPluginInitalizesCorrectly(t *testing.T) {
	testInitPlugin(t)
}

func TestPluginCreateCallsTheProviderWithAConcreteType(t *testing.T) {
	p := testInitPlugin(t)

	// Create test person instance
	testPerson := &person.Person{
		FirstName: "John",
		LastName:  "Doe",
		Age:       30,
		Email:     "john.doe@example.com",
	}

	// Marshal to JSON bytes
	personJSON, err := json.Marshal(testPerson)
	require.NoError(t, err)

	// Call the plugin's Create method
	err = p.Create("resource", "person", personJSON)
	require.NoError(t, err)
}
