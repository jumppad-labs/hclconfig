package xcl

import (
	"testing"

	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/stretchr/testify/require"
)

func TestNewConfigWithNoOptions(t *testing.T) {
	cfg := NewConfig()
	require.NotNil(t, cfg)
	require.Nil(t, cfg.pluginRegistry)
	require.Nil(t, cfg.stateStore)
	require.NotNil(t, cfg.variables)
	require.Equal(t, 0, len(cfg.variables))
}

func TestNewConfigWithPluginRegistry(t *testing.T) {
	pr := registry.NewPluginRegistry(nil)
	cfg := NewConfig(WithPluginRegistry(pr))
	require.NotNil(t, cfg)
	require.Equal(t, pr, cfg.pluginRegistry)
}

func TestNewConfigWithVariables(t *testing.T) {
	vars := map[string]any{"foo": "bar", "count": 42}
	cfg := NewConfig(WithVariables(vars))
	require.NotNil(t, cfg)
	require.Equal(t, vars, cfg.variables)
	require.Equal(t, "bar", cfg.variables["foo"])
	require.Equal(t, 42, cfg.variables["count"])
}

func TestNewConfigWithMultipleOptions(t *testing.T) {
	pr := registry.NewPluginRegistry(nil)
	vars := map[string]any{"env": "test"}

	cfg := NewConfig(
		WithPluginRegistry(pr),
		WithVariables(vars),
	)

	require.NotNil(t, cfg)
	require.Equal(t, pr, cfg.pluginRegistry)
	require.Equal(t, vars, cfg.variables)
}

func TestNewConfigOptionsAreComposable(t *testing.T) {
	pr := registry.NewPluginRegistry(nil)
	vars := map[string]any{"env": "test"}

	opts := []ConfigOption{
		WithPluginRegistry(pr),
		WithVariables(vars),
	}

	cfg := NewConfig(opts...)

	require.NotNil(t, cfg)
	require.Equal(t, pr, cfg.pluginRegistry)
	require.Equal(t, vars, cfg.variables)
}
