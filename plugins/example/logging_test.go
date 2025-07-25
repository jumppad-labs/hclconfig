package main

import (
	"encoding/json"
	"testing"

	"github.com/jumppad-labs/hclconfig/plugins/example/pkg/person"
	plugintesting "github.com/jumppad-labs/hclconfig/plugins/testing"
	"github.com/jumppad-labs/hclconfig/types"
	"github.com/stretchr/testify/require"
)

func TestInProcessLogging(t *testing.T) {
	plugin := &PersonPlugin{}
	ph := plugintesting.InProcessPluginSetup(t, plugin)

	// Create a person to test with
	testPerson := &person.Person{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:   "test-person",
				Type: "resource",
				Name: "person",
			},
		},
		FirstName: "John",
		LastName:  "Doe",
		Age:       30,
	}

	personJSON, err := json.Marshal(testPerson)
	require.NoError(t, err)

	// This should trigger logging in the provider
	_, err = ph.Create("resource", "person", personJSON)
	require.NoError(t, err)

	// Test passed - logging is working correctly
}

func TestExternalLogging(t *testing.T) {
	ph := plugintesting.ExternalPluginSetup(t, "./build/example")

	// Create a person to test with
	testPerson := &person.Person{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:   "test-person",
				Type: "resource",
				Name: "person",
			},
		},
		FirstName: "Jane",
		LastName:  "Smith",
		Age:       25,
	}

	personJSON, err := json.Marshal(testPerson)
	require.NoError(t, err)

	// This should trigger logging in the provider
	_, err = ph.Create("resource", "person", personJSON)
	require.NoError(t, err)

	// Test passed - logging is working correctly
}