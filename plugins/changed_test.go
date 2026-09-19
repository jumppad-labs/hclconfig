package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/types"
)

// testResource is a resource shaped like a real provider resource, it embeds
// types.ResourceBase with the remain tag and has a couple of plain fields.
type testResource struct {
	types.ResourceBase `xcl:",remain"`

	Name  string `xcl:"name" json:"name"`
	Count int    `xcl:"count,optional" json:"count,omitempty"`
}

// providerWithDefaultChanged embeds DefaultChanged and does not define its own
// Changed method.
type providerWithDefaultChanged struct {
	DefaultChanged[*testResource]
}

func (p *providerWithDefaultChanged) Init(state State, functions ProviderFunctions, logger Logger) error {
	return nil
}

func (p *providerWithDefaultChanged) Create(ctx context.Context, resource *testResource) (*testResource, error) {
	return resource, nil
}

func (p *providerWithDefaultChanged) Destroy(ctx context.Context, resource *testResource, force bool) error {
	return nil
}

func (p *providerWithDefaultChanged) Read(ctx context.Context, old *testResource, new *testResource) (*testResource, error) {
	return new, nil
}

func (p *providerWithDefaultChanged) Update(ctx context.Context, resource *testResource) (*testResource, error) {
	return resource, nil
}

func (p *providerWithDefaultChanged) Functions() ProviderFunctions {
	return nil
}

// A provider that embeds DefaultChanged and defines no Changed satisfies the
// provider interface.
var _ ResourceProvider[*testResource] = (*providerWithDefaultChanged)(nil)

func TestDefaultChangedReportsNoChangeForIdenticalResources(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "web", Count: 2}

	changed, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new)
	require.NoError(t, err)
	require.False(t, changed)
}

func TestDefaultChangedReportsChangeWhenFieldDiffers(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	new := &testResource{Name: "web", Count: 3}

	changed, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new)
	require.NoError(t, err)
	require.True(t, changed)
}

func TestDefaultChangedIgnoresMetaDifferences(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	old.Meta.File = "/old/main.hcl"
	old.Meta.Line = 10
	old.Meta.Status = types.StatusCreated

	new := &testResource{Name: "web", Count: 2}
	new.Meta.File = "/new/main.hcl"
	new.Meta.Line = 42
	new.Meta.Status = types.StatusFailed

	changed, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new)
	require.NoError(t, err)
	require.False(t, changed)
}

func TestDefaultChangedIgnoresDependsOnAndDisabled(t *testing.T) {
	old := &testResource{Name: "web", Count: 2}
	old.DependsOn = []string{"resource.network.main"}
	old.Disabled = false

	new := &testResource{Name: "web", Count: 2}
	new.DependsOn = []string{"resource.network.other", "resource.volume.data"}
	new.Disabled = true

	changed, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new)
	require.NoError(t, err)
	require.False(t, changed)
}

func TestDefaultChangedReportsNoChangeForTwoNilResources(t *testing.T) {
	var old *testResource
	var new *testResource

	changed, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new)
	require.NoError(t, err)
	require.False(t, changed)
}

func TestDefaultChangedReportsChangeWhenOneResourceIsNil(t *testing.T) {
	var old *testResource
	new := &testResource{Name: "web", Count: 2}

	changed, err := DefaultChanged[*testResource]{}.Changed(context.Background(), old, new)
	require.NoError(t, err)
	require.True(t, changed)
}
