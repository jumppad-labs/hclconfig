package plugins

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/logger"
)

// readRecordingProvider is a fake provider that records the resources passed
// to Read and returns the configured result and error.
type readRecordingProvider struct {
	DefaultChanged[*testResource]

	readOld    *testResource
	readNew    *testResource
	readCalled bool

	readResult *testResource
	readError  error
}

func (p *readRecordingProvider) Init(state State, functions ProviderFunctions, logger Logger) error {
	return nil
}

func (p *readRecordingProvider) Create(ctx context.Context, resource *testResource) (*testResource, error) {
	return resource, nil
}

func (p *readRecordingProvider) Destroy(ctx context.Context, resource *testResource, force bool) error {
	return nil
}

func (p *readRecordingProvider) Read(ctx context.Context, old *testResource, new *testResource) (*testResource, error) {
	p.readCalled = true
	p.readOld = old
	p.readNew = new

	return p.readResult, p.readError
}

func (p *readRecordingProvider) Update(ctx context.Context, resource *testResource) (*testResource, error) {
	return resource, nil
}

func (p *readRecordingProvider) Functions() ProviderFunctions {
	return nil
}

// emptyState is a State that holds no resources.
type emptyState struct{}

func (emptyState) Get(key string) (any, error) {
	return nil, fmt.Errorf("resource %s not found", key)
}

func (emptyState) Find(pattern string) ([]any, error) {
	return nil, fmt.Errorf("no resources match %s", pattern)
}

// readTestPlugin is an in-process plugin that registers a single provider
// for the type resource.test.
type readTestPlugin struct {
	PluginBase

	provider *readRecordingProvider
}

func (p *readTestPlugin) Init(logger Logger, state State) error {
	p.SetLogger(logger)
	p.SetState(state)

	return RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "test", &testResource{}, p.provider)
}

func TestTypedProviderAdapterReadPassesOldAndNewToProvider(t *testing.T) {
	provider := &readRecordingProvider{
		readResult: &testResource{Name: "web", Count: 5},
	}
	adapter := NewTypedProviderAdapter[*testResource](provider, &testResource{})

	oldData := []byte(`{"name":"web","count":1}`)
	newData := []byte(`{"name":"web","count":2}`)

	_, err := adapter.Read(context.Background(), oldData, newData)
	require.NoError(t, err)

	require.True(t, provider.readCalled)
	require.NotNil(t, provider.readOld)
	require.Equal(t, "web", provider.readOld.Name)
	require.Equal(t, 1, provider.readOld.Count)
	require.NotNil(t, provider.readNew)
	require.Equal(t, "web", provider.readNew.Name)
	require.Equal(t, 2, provider.readNew.Count)
}

func TestTypedProviderAdapterReadReturnsProviderResultAsJSON(t *testing.T) {
	provider := &readRecordingProvider{
		readResult: &testResource{Name: "web", Count: 5},
	}
	adapter := NewTypedProviderAdapter[*testResource](provider, &testResource{})

	oldData := []byte(`{"name":"web","count":1}`)
	newData := []byte(`{"name":"web","count":2}`)

	data, err := adapter.Read(context.Background(), oldData, newData)
	require.NoError(t, err)

	expected := `{
		"meta": {"id": "", "name": "", "type": "", "file": "", "line": 0, "column": 0},
		"name": "web",
		"count": 5
	}`
	require.JSONEq(t, expected, string(data))
}

func TestTypedProviderAdapterReadPassesZeroValueForNilData(t *testing.T) {
	provider := &readRecordingProvider{
		readResult: &testResource{Name: "web"},
	}
	adapter := NewTypedProviderAdapter[*testResource](provider, &testResource{})

	_, err := adapter.Read(context.Background(), nil, nil)
	require.NoError(t, err)

	require.True(t, provider.readCalled)
	require.Nil(t, provider.readOld)
	require.Nil(t, provider.readNew)
}

func TestTypedProviderAdapterReadReturnsErrNotFoundFromProvider(t *testing.T) {
	provider := &readRecordingProvider{
		readError: ErrNotFound,
	}
	adapter := NewTypedProviderAdapter[*testResource](provider, &testResource{})

	oldData := []byte(`{"name":"web","count":1}`)
	newData := []byte(`{"name":"web","count":1}`)

	data, err := adapter.Read(context.Background(), oldData, newData)
	require.ErrorIs(t, err, ErrNotFound)
	require.Nil(t, data)
}

func TestTypedProviderAdapterReadReturnsWrappedErrNotFoundFromProvider(t *testing.T) {
	provider := &readRecordingProvider{
		readError: fmt.Errorf("container web is gone: %w", ErrNotFound),
	}
	adapter := NewTypedProviderAdapter[*testResource](provider, &testResource{})

	oldData := []byte(`{"name":"web","count":1}`)
	newData := []byte(`{"name":"web","count":1}`)

	_, err := adapter.Read(context.Background(), oldData, newData)
	require.ErrorIs(t, err, ErrNotFound)
	require.EqualError(t, err, "container web is gone: resource not found")
}

func TestDirectPluginHostReadPassesOldAndNewToProvider(t *testing.T) {
	provider := &readRecordingProvider{
		readResult: &testResource{Name: "web", Count: 5},
	}
	plugin := &readTestPlugin{provider: provider}

	host, err := NewDirectPluginHost(logger.NewTestLogger(t), emptyState{}, plugin)
	require.NoError(t, err)

	oldData := []byte(`{"name":"web","count":1}`)
	newData := []byte(`{"name":"web","count":2}`)

	data, err := host.Read(context.Background(), "resource", "test", oldData, newData)
	require.NoError(t, err)

	require.Equal(t, 1, provider.readOld.Count)
	require.Equal(t, 2, provider.readNew.Count)

	expected := `{
		"meta": {"id": "", "name": "", "type": "", "file": "", "line": 0, "column": 0},
		"name": "web",
		"count": 5
	}`
	require.JSONEq(t, expected, string(data))
}

func TestDirectPluginHostReadReturnsErrNotFoundFromProvider(t *testing.T) {
	provider := &readRecordingProvider{
		readError: fmt.Errorf("container web is gone: %w", ErrNotFound),
	}
	plugin := &readTestPlugin{provider: provider}

	host, err := NewDirectPluginHost(logger.NewTestLogger(t), emptyState{}, plugin)
	require.NoError(t, err)

	oldData := []byte(`{"name":"web","count":1}`)
	newData := []byte(`{"name":"web","count":1}`)

	_, err = host.Read(context.Background(), "resource", "test", oldData, newData)
	require.ErrorIs(t, err, ErrNotFound)
}
