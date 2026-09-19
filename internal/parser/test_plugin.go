package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// ReadCall records the saved (old) and configured (new) copies passed to Read
type ReadCall struct {
	ID  string
	Old []byte
	New []byte
}

// TestPlugin provides test resource types for testing the parser.
// The walker calls providers in parallel, so every field is guarded by mu.
type TestPlugin struct {
	plugins.PluginBase

	mu sync.Mutex

	ReadResources      []string // Track all read resource names
	CreatedResources   []string // Track all created resource names
	DestroyedResources []string // Track all destroyed resource names
	UpdatedResources   []string // Track all updated resource names
	ChangedCalls       []string // Track all changed checks (format: "oldID->newID")

	// Calls records every provider call in order, formatted "<operation> <id>"
	Calls []string

	// ReadCalls records the copies passed to every Read
	ReadCalls []ReadCall

	// Error configuration maps
	CreateErrors  map[string]error // Maps resource ID to error for Create operations
	DestroyErrors map[string]error // Maps resource ID to error for Destroy operations
	UpdateErrors  map[string]error // Maps resource ID to error for Update operations
	ReadErrors    map[string]error // Maps resource ID to error for Read operations
	ChangedErrors map[string]error // Maps resource ID to error for Changed operations

	// Scenario configuration maps
	ReadNotFound map[string]bool   // Maps resource ID to Read returning plugins.ErrNotFound
	ReadObserved map[string]string // Maps resource ID to the observed value Read sets on a network
	// MutateConfigured maps resource ID to a subnet that Create, Read and Update
	// write over the network's configured subnet
	MutateConfigured map[string]string
	ChangedResults   map[string]bool // Maps resource ID to a Changed result that overrides the default

	// CreateSetsID sets the network's ProviderID to "id-<name>" on Create, it is
	// enabled by Init
	CreateSetsID bool
}

// Ensure TestPlugin implements Plugin interface
var _ plugins.Plugin = (*TestPlugin)(nil)

// GetReadResources returns the list of resource names that were read
func (p *TestPlugin) GetReadResources() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.ReadResources...)
}

// GetCreatedResources returns the list of resource names that were created
func (p *TestPlugin) GetCreatedResources() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.CreatedResources...)
}

// GetDestroyedResources returns the list of resource names that were destroyed
func (p *TestPlugin) GetDestroyedResources() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.DestroyedResources...)
}

// GetUpdatedResources returns the list of resource names that were updated
func (p *TestPlugin) GetUpdatedResources() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.UpdatedResources...)
}

// GetChangedCalls returns the list of changed checks that were made
func (p *TestPlugin) GetChangedCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.ChangedCalls...)
}

// GetCalls returns every provider call in order, formatted "<operation> <id>"
func (p *TestPlugin) GetCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.Calls...)
}

// GetReadCalls returns the copies passed to every Read
func (p *TestPlugin) GetReadCalls() []ReadCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]ReadCall{}, p.ReadCalls...)
}

// ResetCalls clears the recorded calls, leaving the configuration in place
func (p *TestPlugin) ResetCalls() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ReadResources = []string{}
	p.CreatedResources = []string{}
	p.DestroyedResources = []string{}
	p.UpdatedResources = []string{}
	p.ChangedCalls = []string{}
	p.Calls = []string{}
	p.ReadCalls = []ReadCall{}
}

// SetCreateError configures an error to be returned when creating a resource with the given ID
func (p *TestPlugin) SetCreateError(resourceID string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.CreateErrors == nil {
		p.CreateErrors = make(map[string]error)
	}
	p.CreateErrors[resourceID] = err
}

// SetDestroyError configures an error to be returned when destroying a resource with the given ID
func (p *TestPlugin) SetDestroyError(resourceID string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.DestroyErrors == nil {
		p.DestroyErrors = make(map[string]error)
	}
	p.DestroyErrors[resourceID] = err
}

// SetUpdateError configures an error to be returned when updating a resource with the given ID
func (p *TestPlugin) SetUpdateError(resourceID string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.UpdateErrors == nil {
		p.UpdateErrors = make(map[string]error)
	}
	p.UpdateErrors[resourceID] = err
}

// SetReadError configures an error to be returned when reading a resource with the given ID
func (p *TestPlugin) SetReadError(resourceID string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ReadErrors == nil {
		p.ReadErrors = make(map[string]error)
	}
	p.ReadErrors[resourceID] = err
}

// SetChangedError configures an error to be returned when checking if a resource with the given ID has changed
func (p *TestPlugin) SetChangedError(resourceID string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ChangedErrors == nil {
		p.ChangedErrors = make(map[string]error)
	}
	p.ChangedErrors[resourceID] = err
}

// SetReadNotFound configures Read to report the resource with the given ID as not found
func (p *TestPlugin) SetReadNotFound(resourceID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ReadNotFound == nil {
		p.ReadNotFound = make(map[string]bool)
	}
	p.ReadNotFound[resourceID] = true
}

// SetReadObserved configures Read to set the observed value of the network
// with the given ID, simulating drift in the real resource
func (p *TestPlugin) SetReadObserved(resourceID string, observed string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ReadObserved == nil {
		p.ReadObserved = make(map[string]string)
	}
	p.ReadObserved[resourceID] = observed
}

// SetChangedResult configures Changed to return the given result for the
// resource with the given ID instead of the default change detection
func (p *TestPlugin) SetChangedResult(resourceID string, changed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ChangedResults == nil {
		p.ChangedResults = make(map[string]bool)
	}
	p.ChangedResults[resourceID] = changed
}

// SetMutateConfigured configures Create, Read and Update to write the given
// subnet over the configured subnet of the network with the given ID
func (p *TestPlugin) SetMutateConfigured(resourceID string, subnet string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.MutateConfigured == nil {
		p.MutateConfigured = make(map[string]string)
	}
	p.MutateConfigured[resourceID] = subnet
}

// mutateConfigured writes the configured mutation over a network's subnet, the
// caller must hold mu
func (p *TestPlugin) mutateConfigured(resourceID string, resource any) {
	subnet, exists := p.MutateConfigured[resourceID]
	if !exists {
		return
	}

	if network, ok := resource.(*structs.Network); ok {
		network.Subnet = subnet
	}
}

// ClearErrors clears all configured errors
func (p *TestPlugin) ClearErrors() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CreateErrors = make(map[string]error)
	p.DestroyErrors = make(map[string]error)
	p.UpdateErrors = make(map[string]error)
	p.ReadErrors = make(map[string]error)
	p.ChangedErrors = make(map[string]error)
}

// Init initializes the test plugin with test resource types
func (p *TestPlugin) Init(logger logger.Logger, state plugins.State) error {
	// Initialize all tracking slices and error maps
	p.ResetCalls()
	p.ClearErrors()

	// Initialize scenario configuration
	p.mu.Lock()
	p.ReadNotFound = make(map[string]bool)
	p.ReadObserved = make(map[string]string)
	p.ChangedResults = make(map[string]bool)
	p.MutateConfigured = make(map[string]string)
	p.CreateSetsID = true
	p.mu.Unlock()

	// Register Container resource
	containerResource := &structs.Container{}
	containerProvider := &TestResourceProvider[*structs.Container]{plugin: p}
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"container",
		containerResource,
		containerProvider,
	)
	if err != nil {
		return err
	}

	sidecarResource := &structs.Sidecar{}
	sidecarProvider := &TestResourceProvider[*structs.Sidecar]{plugin: p}
	err = plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"sidecar",
		sidecarResource,
		sidecarProvider,
	)
	if err != nil {
		return err
	}

	// Register Network resource
	networkResource := &structs.Network{}
	networkProvider := &TestResourceProvider[*structs.Network]{plugin: p}
	err = plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"network",
		networkResource,
		networkProvider,
	)
	if err != nil {
		return err
	}

	// Register Template resource
	templateResource := &structs.Template{}
	templateProvider := &TestResourceProvider[*structs.Template]{plugin: p}
	err = plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"template",
		templateResource,
		templateProvider,
	)
	if err != nil {
		return err
	}

	return nil
}

// TestResourceProvider is a generic test provider for any resource type.
// It embeds DefaultChanged, so change detection is the default unless a
// result is configured on the plugin.
type TestResourceProvider[T any] struct {
	plugins.DefaultChanged[T]

	logger    logger.Logger
	state     plugins.State
	functions plugins.ProviderFunctions
	plugin    *TestPlugin // Reference to parent plugin for tracking
}

// Init initializes the test provider
func (p *TestResourceProvider[T]) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	p.state = state
	p.functions = functions
	p.logger = logger
	return nil
}

// Create tracks the resource name for testing and sets the network's ProviderID
func (p *TestResourceProvider[T]) Create(ctx context.Context, resource T) (T, error) {
	meta, err := types.GetMeta(resource)
	if err != nil {
		return resource, err
	}

	p.plugin.mu.Lock()
	defer p.plugin.mu.Unlock()

	p.plugin.CreatedResources = append(p.plugin.CreatedResources, meta.ID)
	p.plugin.Calls = append(p.plugin.Calls, "create "+meta.ID)

	if err, exists := p.plugin.CreateErrors[meta.ID]; exists && err != nil {
		return resource, err
	}

	if network, ok := any(resource).(*structs.Network); ok && p.plugin.CreateSetsID {
		network.ProviderID = "id-" + meta.Name
	}

	// attach each of the container's networks, assigning an address
	if container, ok := any(resource).(*structs.Container); ok {
		for i := range container.Networks {
			container.Networks[i].AssignedAddress = "assigned-" + container.Networks[i].Name
		}
	}

	p.plugin.mutateConfigured(meta.ID, resource)

	return resource, nil
}

// Destroy tracks the resource name for testing
func (p *TestResourceProvider[T]) Destroy(ctx context.Context, resource T, force bool) error {
	meta, err := types.GetMeta(resource)
	if err != nil {
		return err
	}

	p.plugin.mu.Lock()
	defer p.plugin.mu.Unlock()

	p.plugin.DestroyedResources = append(p.plugin.DestroyedResources, meta.ID)
	p.plugin.Calls = append(p.plugin.Calls, "destroy "+meta.ID)

	if err, exists := p.plugin.DestroyErrors[meta.ID]; exists && err != nil {
		return err
	}

	return nil
}

// Read tracks the copies it receives and returns the configured copy, with the
// network's observed value set when one is configured. It adds nothing else: the
// computed values saved by the last apply are carried over by the parser.
func (p *TestResourceProvider[T]) Read(ctx context.Context, old T, resource T) (T, error) {
	meta, err := types.GetMeta(resource)
	if err != nil {
		return resource, err
	}

	oldData, err := json.Marshal(old)
	if err != nil {
		return resource, err
	}

	newData, err := json.Marshal(resource)
	if err != nil {
		return resource, err
	}

	p.plugin.mu.Lock()
	defer p.plugin.mu.Unlock()

	p.plugin.ReadResources = append(p.plugin.ReadResources, meta.ID)
	p.plugin.Calls = append(p.plugin.Calls, "read "+meta.ID)
	p.plugin.ReadCalls = append(p.plugin.ReadCalls, ReadCall{ID: meta.ID, Old: oldData, New: newData})

	if err, exists := p.plugin.ReadErrors[meta.ID]; exists && err != nil {
		return resource, err
	}

	if p.plugin.ReadNotFound[meta.ID] {
		return resource, plugins.ErrNotFound
	}

	p.plugin.mutateConfigured(meta.ID, resource)

	if observed, exists := p.plugin.ReadObserved[meta.ID]; exists {
		if network, ok := any(resource).(*structs.Network); ok {
			network.Observed = observed
		}
	}

	return resource, nil
}

// Update tracks the resource name for testing
func (p *TestResourceProvider[T]) Update(ctx context.Context, resource T) (T, error) {
	meta, err := types.GetMeta(resource)
	if err != nil {
		return resource, err
	}

	p.plugin.mu.Lock()
	defer p.plugin.mu.Unlock()

	p.plugin.UpdatedResources = append(p.plugin.UpdatedResources, meta.ID)
	p.plugin.Calls = append(p.plugin.Calls, "update "+meta.ID)

	if err, exists := p.plugin.UpdateErrors[meta.ID]; exists && err != nil {
		return resource, err
	}

	p.plugin.mutateConfigured(meta.ID, resource)

	return resource, nil
}

// Changed tracks the comparison, returns a configured error or result, and
// otherwise uses the default change detection
func (p *TestResourceProvider[T]) Changed(ctx context.Context, old T, new T) (bool, error) {
	oldMeta, err := types.GetMeta(old)
	if err != nil {
		return false, err
	}

	newMeta, err := types.GetMeta(new)
	if err != nil {
		return false, err
	}

	p.plugin.mu.Lock()
	p.plugin.ChangedCalls = append(p.plugin.ChangedCalls, fmt.Sprintf("%s->%s", oldMeta.ID, newMeta.ID))
	p.plugin.Calls = append(p.plugin.Calls, "changed "+newMeta.ID)
	changedErr, hasError := p.plugin.ChangedErrors[newMeta.ID]
	changedResult, hasResult := p.plugin.ChangedResults[newMeta.ID]
	p.plugin.mu.Unlock()

	if hasError && changedErr != nil {
		return false, changedErr
	}

	if hasResult {
		return changedResult, nil
	}

	return p.DefaultChanged.Changed(ctx, old, new)
}

// Functions returns no functions
func (p *TestResourceProvider[T]) Functions() plugins.ProviderFunctions {
	return p.functions
}
