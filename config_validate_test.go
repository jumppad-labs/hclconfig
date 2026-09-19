package xcl

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	statemocks "github.com/jumppad-labs/xcl/state/mocks"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupConfig builds a Config wired to a plugin registry that has the test
// resource types (container, network, template, sidecar) registered, and a mock
// state store that reports no previous state.
//
// It returns the Config, the TestPlugin that records every create, update and
// destroy the walk performs, and the mock state store so a test can assert what
// was persisted.
func setupConfig(t *testing.T) (*Config, *parser.TestPlugin, *statemocks.MockStateStore) {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	log := logger.NewTestLogger(t)

	pr := registry.NewPluginRegistry(log)

	testPlugin := &parser.TestPlugin{}
	err := pr.RegisterPlugin(testPlugin)
	require.NoError(t, err)

	ss := &statemocks.MockStateStore{}
	ss.On("Exists").Return(false)
	ss.On("Load").Return(nil, nil)
	ss.On("Save", mock.Anything).Return(nil)

	c := NewConfig(
		WithPluginRegistry(pr),
		WithStateStore(ss),
	)

	return c, testPlugin, ss
}

// TestValidateAcceptsValidConfiguration asserts that asking whether a valid
// configuration is valid reports success.
func TestValidateAcceptsValidConfiguration(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/simple/container.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Validate(path)
	require.NoError(t, err)
}

// TestValidateCreatesChangesAndRemovesNothing asserts that validating a valid
// configuration creates, changes or removes nothing and persists no state.
func TestValidateCreatesChangesAndRemovesNothing(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/simple/container.xcl")
	require.NoError(t, err)

	c, testPlugin, ss := setupConfig(t)

	err = c.Validate(path)
	require.NoError(t, err)

	require.Empty(t, testPlugin.GetCreatedResources())
	require.Empty(t, testPlugin.GetUpdatedResources())
	require.Empty(t, testPlugin.GetDestroyedResources())

	ss.AssertNotCalled(t, "Save", mock.Anything)
	require.Equal(t, 0, c.ResourceCount())
}

// TestValidateAcceptsModulesConfiguration asserts that a configuration that was
// valid before the validation gate was introduced is still accepted.
func TestValidateAcceptsModulesConfiguration(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/modules/modules.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Validate(path)
	require.NoError(t, err)
}

// TestValidateReturnsConfigErrorWhenBlockHasNoName asserts that validating an
// invalid configuration reports failure as a *errors.ConfigError.
func TestValidateReturnsConfigErrorWhenBlockHasNoName(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/invalid/no_name.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Validate(path)
	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Validate should report failure as a *errors.ConfigError")
	require.NotEmpty(t, ce.Errors)
}

// TestValidateReturnsConfigErrorWhenFileIsMalformed asserts that a file the HCL
// parser cannot read at all is reported as a *errors.ConfigError.
func TestValidateReturnsConfigErrorWhenFileIsMalformed(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/process_error/bad_format.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Validate(path)
	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Validate should report failure as a *errors.ConfigError")
	require.NotEmpty(t, ce.Errors)
}

// TestValidateCreatesChangesAndRemovesNothingWhenConfigurationIsInvalid asserts
// that a rejected configuration reaches neither a provider nor the state store.
func TestValidateCreatesChangesAndRemovesNothingWhenConfigurationIsInvalid(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/invalid/no_name.xcl")
	require.NoError(t, err)

	c, testPlugin, ss := setupConfig(t)

	err = c.Validate(path)
	require.Error(t, err)

	require.Empty(t, testPlugin.GetCreatedResources())
	require.Empty(t, testPlugin.GetUpdatedResources())
	require.Empty(t, testPlugin.GetDestroyedResources())

	ss.AssertNotCalled(t, "Save", mock.Anything)
	require.Equal(t, 0, c.ResourceCount())
}

// TestValidateReturnsErrorAlone asserts at compile time that Validate answers
// with a single error and no longer offers a change-preview alongside it.
func TestValidateReturnsErrorAlone(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/single/container.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	// A single-value assignment only compiles while Validate returns exactly one
	// value. It would not compile against the previous (*Diff, error) contract.
	var result error = c.Validate(path)
	require.NoError(t, result)
}

// TestValidateReturnsErrorWhenNoPathsGiven asserts that Validate rejects a call
// that names nothing to validate.
func TestValidateReturnsErrorWhenNoPathsGiven(t *testing.T) {
	c, _, _ := setupConfig(t)

	err := c.Validate()
	require.Error(t, err)
	require.Equal(t, "at least one path is required", err.Error())
}

// TestValidateJudgesDirectoryAsOneUnit asserts that a configuration spread over
// several files in one directory is validated together, so a reference that is
// only resolvable across files is accepted.
func TestValidateJudgesDirectoryAsOneUnit(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "network.xcl"), []byte(`
resource "network" "onprem" {
  subnet = "10.6.0.0/16"
}
`), 0644)
	require.NoError(t, err)

	// This file is not valid on its own, it refers to a network declared in the
	// other file. It is only valid when the directory is judged as one unit.
	err = os.WriteFile(filepath.Join(dir, "container.xcl"), []byte(`
resource "container" "consul" {
  command = ["consul", "agent", "-dev"]

  network {
    name       = resource.network.onprem.meta.name
    ip_address = "10.6.0.200"
  }
}
`), 0644)
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Validate(dir)
	require.NoError(t, err)
}

// TestValidateReturnsConfigErrorWhenOneFileInDirectoryIsInvalid asserts that a
// directory is judged as one unit when reporting failure too, so a problem in
// any file fails the whole configuration.
func TestValidateReturnsConfigErrorWhenOneFileInDirectoryIsInvalid(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "network.xcl"), []byte(`
resource "network" "onprem" {
  subnet = "10.6.0.0/16"
}
`), 0644)
	require.NoError(t, err)

	// Missing its name label.
	err = os.WriteFile(filepath.Join(dir, "container.xcl"), []byte(`
resource "container" {
  command = ["consul", "agent", "-dev"]
}
`), 0644)
	require.NoError(t, err)

	c, testPlugin, _ := setupConfig(t)

	err = c.Validate(dir)
	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Validate should report failure as a *errors.ConfigError")
	require.NotEmpty(t, ce.Errors)

	// The valid network in the sibling file must not have been acted upon.
	require.Empty(t, testPlugin.GetCreatedResources())
}

// TestApplyCreatesResourcesFromValidConfiguration asserts that applying a valid
// configuration creates its resources and persists the resulting state.
func TestApplyCreatesResourcesFromValidConfiguration(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/single/container.xcl")
	require.NoError(t, err)

	c, testPlugin, ss := setupConfig(t)

	err = c.Apply(path)
	require.NoError(t, err)

	require.Contains(t, testPlugin.GetCreatedResources(), "resource.network.onprem")
	require.Contains(t, testPlugin.GetCreatedResources(), "resource.container.consul")

	ss.AssertCalled(t, "Save", mock.Anything)
}

// TestApplySavesStateWhenProviderFails asserts that when a provider call fails
// the apply reports the failure and still persists the progress it made, with
// the failed resource recorded as failed.
func TestApplySavesStateWhenProviderFails(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/single/container.xcl")
	require.NoError(t, err)

	c, testPlugin, ss := setupConfig(t)

	// the plugin is initialised when it is registered, so errors are set after
	testPlugin.SetCreateError("resource.container.consul", fmt.Errorf("container runtime unavailable"))

	err = c.Apply(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create failed for resource.container.consul")

	ss.AssertCalled(t, "Save", mock.Anything)

	var saved *state.State
	for _, call := range ss.Calls {
		if call.Method == "Save" {
			saved = call.Arguments.Get(0).(*state.State)
		}
	}
	require.NotNil(t, saved)

	consul, err := saved.FindResource("resource.container.consul")
	require.NoError(t, err)

	consulMeta, err := types.GetMeta(consul)
	require.NoError(t, err)
	require.Equal(t, types.StatusFailed, consulMeta.Status)

	onprem, err := saved.FindResource("resource.network.onprem")
	require.NoError(t, err)

	onpremMeta, err := types.GetMeta(onprem)
	require.NoError(t, err)
	require.Equal(t, types.StatusCreated, onpremMeta.Status)
}

// TestApplyCreatesChangesAndRemovesNothingWhenConfigurationIsInvalid asserts
// that applying an invalid configuration reports failure and that the gate
// stops it before anything is created, changed, removed or persisted.
func TestApplyCreatesChangesAndRemovesNothingWhenConfigurationIsInvalid(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/invalid/no_name.xcl")
	require.NoError(t, err)

	c, testPlugin, ss := setupConfig(t)

	err = c.Apply(path)
	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Apply should report failure as a *errors.ConfigError")
	require.NotEmpty(t, ce.Errors)

	require.Empty(t, testPlugin.GetCreatedResources())
	require.Empty(t, testPlugin.GetUpdatedResources())
	require.Empty(t, testPlugin.GetDestroyedResources())

	ss.AssertNotCalled(t, "Save", mock.Anything)
	require.Equal(t, 0, c.ResourceCount())
}

// TestApplyCreatesNothingWhenFileIsMalformed asserts that a file the HCL parser
// cannot read at all stops the apply before any resource is created.
func TestApplyCreatesNothingWhenFileIsMalformed(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/process_error/bad_format.xcl")
	require.NoError(t, err)

	c, testPlugin, ss := setupConfig(t)

	err = c.Apply(path)
	require.Error(t, err)

	require.Empty(t, testPlugin.GetCreatedResources())
	require.Empty(t, testPlugin.GetUpdatedResources())
	require.Empty(t, testPlugin.GetDestroyedResources())

	ss.AssertNotCalled(t, "Save", mock.Anything)
}

// TestValidateStillProducesTheSameResourcesAsApply asserts that a configuration
// that was valid before the validation gate was introduced still parses to the
// same set of resources it always did.
func TestValidateStillProducesTheSameResourcesAsApply(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/simple/container.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Validate(path)
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	require.Equal(t, 9, c.ResourceCount())

	r, err := c.FindResource("resource.container.consul")
	require.NoError(t, err)
	require.NotNil(t, r)

	r, err = c.FindResource("resource.network.onprem")
	require.NoError(t, err)
	require.NotNil(t, r)
}

// TestValidateRejectsConfiguredComputedField asserts that validating a
// configuration that sets a computed field reports failure naming the resource
// and the field.
func TestValidateRejectsConfiguredComputedField(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/computed_set/top.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Validate(path)
	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Validate should report failure as a *errors.ConfigError")
	require.Len(t, ce.Errors, 1)

	pe, ok := ce.Errors[0].(*errors.ParserError)
	require.True(t, ok, "the problem should be a *errors.ParserError")
	require.Equal(t, path, pe.Filename)
	require.Equal(t, 3, pe.Line)
	require.Equal(t, "resource 'resource.network.main' sets computed field 'provider_id', computed fields are set by the provider and cannot be configured", pe.Message)
}

// TestApplyCreatesNothingWhenComputedFieldIsConfigured asserts that applying a
// configuration that sets a computed field is stopped before any provider is
// called and before any state is persisted.
func TestApplyCreatesNothingWhenComputedFieldIsConfigured(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/computed_set/top.xcl")
	require.NoError(t, err)

	c, testPlugin, ss := setupConfig(t)

	err = c.Apply(path)
	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Apply should report failure as a *errors.ConfigError")
	require.Len(t, ce.Errors, 1)
	require.Contains(t, ce.Errors[0].Error(), "provider_id")

	require.Empty(t, testPlugin.GetCalls())
	require.Empty(t, testPlugin.GetCreatedResources())

	ss.AssertNotCalled(t, "Save", mock.Anything)
	require.Equal(t, 0, c.ResourceCount())
}

// TestValidateAcceptsUnsetComputedField asserts that a configuration that
// leaves every computed field to the provider is valid.
func TestValidateAcceptsUnsetComputedField(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/computed_set/unset.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Validate(path)
	require.NoError(t, err)
}
