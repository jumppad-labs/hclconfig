package registry

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jumppad-labs/xcl/logger"
	"github.com/stretchr/testify/require"
)

func TestPluginDiscoverySingleValidPlugin(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-test")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{validDir}, "xcl-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 1 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 1", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryMultipleValidPlugins(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-one")
	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-two")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{validDir}, "xcl-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 2 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 2", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryPluginNotMatchingPattern(t *testing.T) {
	setup := newTestPluginSetup(t)
	invalidDir := setup.createPluginDir("invalid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, invalidDir, "not-a-plugin")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{invalidDir}, "xcl-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 0 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 0", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}
}

func TestPluginDiscoveryNonExecutableFile(t *testing.T) {
	setup := newTestPluginSetup(t)
	invalidDir := setup.createPluginDir("invalid")

	setup.createNonExecutable(invalidDir, "xcl-plugin-fake")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{invalidDir}, "xcl-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 0 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 0", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}
}

func TestPluginDiscoveryMixedDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	mixedDir := setup.createPluginDir("mixed")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, mixedDir, "xcl-plugin-good")
	setup.createNonPlugin(mixedDir, "xcl-plugin-bad")
	setup.createNonExecutable(mixedDir, "xcl-plugin-text.txt")
	setup.copyPlugin(examplePlugin, mixedDir, "wrong-pattern")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{mixedDir}, "xcl-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	// plugin-good and plugin-bad (both executables)
	if len(plugins) != 2 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 2", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryEmptyDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	emptyDir := setup.createPluginDir("empty")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{emptyDir}, "xcl-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 0 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 0", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}
}

func TestPluginDiscoveryNonExistentDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	nonExistentDir := filepath.Join(setup.testDir, "non-existent")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{nonExistentDir}, "xcl-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 0 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 0", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}
}

func TestPluginDiscoveryMultipleDirectories(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	mixedDir := setup.createPluginDir("mixed")
	emptyDir := setup.createPluginDir("empty")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-dir1")
	setup.copyPlugin(examplePlugin, mixedDir, "xcl-plugin-dir2")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{validDir, mixedDir, emptyDir}, "xcl-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 2 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 2", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryCustomPattern(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "my-custom-plugin-test")
	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-ignored")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{validDir}, "my-custom-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	if len(plugins) != 1 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 1", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscoveryDuplicateDirectories(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	setup.copyPlugin(examplePlugin, validDir, "xcl-plugin-unique")

	testLogger := logger.NewTestLogger(t)

	pd := NewPluginDiscovery([]string{validDir, validDir, validDir}, "xcl-plugin-*", testLogger)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Errorf("DiscoverPlugins() error = %v, wantErr false", err)
		return
	}

	// Should deduplicate
	if len(plugins) != 1 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 1", len(plugins))
		t.Logf("Found plugins: %v", plugins)
	}

	for _, plugin := range plugins {
		if !filepath.IsAbs(plugin) {
			t.Errorf("Plugin path is not absolute: %s", plugin)
		}
		if _, err := os.Stat(plugin); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin)
		}
	}
}

func TestPluginDiscovery_WindowsExecutables(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	setup := newTestPluginSetup(t)
	dir := setup.createPluginDir("windows")

	// Build example plugin
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	// Copy with .exe extension (should be found)
	exePath := setup.copyPlugin(examplePlugin, dir, "xcl-plugin-test.exe")

	// Create without .exe extension (should not be found on Windows)
	nonExePath := filepath.Join(dir, "xcl-plugin-noext")
	if err := os.WriteFile(nonExePath, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	pd := NewPluginDiscovery([]string{dir}, "xcl-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("Expected 1 plugin, found %d", len(plugins))
	}

	if plugins[0] != exePath {
		t.Errorf("Expected plugin path %s, got %s", exePath, plugins[0])
	}
}

func TestExpandPluginDirectoriesExpandHomeDirectory(t *testing.T) {
	// Save original env
	originalHome := os.Getenv("HOME")
	originalTestVar := os.Getenv("TEST_PLUGIN_DIR")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("TEST_PLUGIN_DIR", originalTestVar)
	}()

	// Set test environment
	os.Setenv("TEST_PLUGIN_DIR", "/test/plugins")
	homeDir, _ := os.UserHomeDir()

	input := []string{"~/plugins", "~/.config/plugins"}
	expected := []string{filepath.Join(homeDir, "plugins"), filepath.Join(homeDir, ".config/plugins")}

	result := ExpandPluginDirectories(input)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d paths, got %d", len(expected), len(result))
	}

	for i, path := range result {
		// Normalize paths for comparison
		expectedPath := filepath.Clean(expected[i])
		got := filepath.Clean(path)

		if got != expectedPath {
			t.Errorf("Path %d: expected %s, got %s", i, expectedPath, got)
		}
	}
}

func TestExpandPluginDirectoriesExpandEnvironmentVariables(t *testing.T) {
	// Save original env
	originalHome := os.Getenv("HOME")
	originalTestVar := os.Getenv("TEST_PLUGIN_DIR")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("TEST_PLUGIN_DIR", originalTestVar)
	}()

	// Set test environment
	os.Setenv("TEST_PLUGIN_DIR", "/test/plugins")

	input := []string{"$TEST_PLUGIN_DIR", "${TEST_PLUGIN_DIR}/sub"}
	expected := []string{"/test/plugins", "/test/plugins/sub"}

	result := ExpandPluginDirectories(input)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d paths, got %d", len(expected), len(result))
	}

	for i, path := range result {
		// Normalize paths for comparison
		expectedPath := filepath.Clean(expected[i])
		got := filepath.Clean(path)

		if got != expectedPath {
			t.Errorf("Path %d: expected %s, got %s", i, expectedPath, got)
		}
	}
}

func TestExpandPluginDirectoriesNoExpansionNeeded(t *testing.T) {
	// Save original env
	originalHome := os.Getenv("HOME")
	originalTestVar := os.Getenv("TEST_PLUGIN_DIR")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("TEST_PLUGIN_DIR", originalTestVar)
	}()

	// Set test environment
	os.Setenv("TEST_PLUGIN_DIR", "/test/plugins")

	input := []string{"/absolute/path", "./relative/path"}
	expected := []string{"/absolute/path", "./relative/path"}

	result := ExpandPluginDirectories(input)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d paths, got %d", len(expected), len(result))
	}

	for i, path := range result {
		// Normalize paths for comparison
		expectedPath := filepath.Clean(expected[i])
		got := filepath.Clean(path)

		if got != expectedPath {
			t.Errorf("Path %d: expected %s, got %s", i, expectedPath, got)
		}
	}
}

func TestExpandPluginDirectoriesMixedPaths(t *testing.T) {
	// Save original env
	originalHome := os.Getenv("HOME")
	originalTestVar := os.Getenv("TEST_PLUGIN_DIR")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("TEST_PLUGIN_DIR", originalTestVar)
	}()

	// Set test environment
	os.Setenv("TEST_PLUGIN_DIR", "/test/plugins")
	homeDir, _ := os.UserHomeDir()

	input := []string{"~/plugins", "$TEST_PLUGIN_DIR", "/absolute"}
	expected := []string{filepath.Join(homeDir, "plugins"), "/test/plugins", "/absolute"}

	result := ExpandPluginDirectories(input)

	if len(result) != len(expected) {
		t.Fatalf("Expected %d paths, got %d", len(expected), len(result))
	}

	for i, path := range result {
		// Normalize paths for comparison
		expectedPath := filepath.Clean(expected[i])
		got := filepath.Clean(path)

		if got != expectedPath {
			t.Errorf("Path %d: expected %s, got %s", i, expectedPath, got)
		}
	}
}

func TestDiscoverAndLoadPluginsLoadsDiscoveredPlugins(t *testing.T) {
	setup := newTestPluginSetup(t)
	pluginDir := setup.createPluginDir("plugins")

	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, pluginDir, "xcl-plugin-example")

	testLogger := logger.NewTestLogger(t)
	r := NewPluginRegistry(testLogger)

	err := r.DiscoverAndLoadPlugins(testLogger, []string{pluginDir}, "xcl-plugin-*")
	require.NoError(t, err)

	require.Len(t, r.GetPluginHosts(), 1)
}

func TestDiscoverAndLoadPluginsLoadsNothingWhenNoPluginMatches(t *testing.T) {
	setup := newTestPluginSetup(t)
	pluginDir := setup.createPluginDir("plugins")

	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, pluginDir, "not-a-matching-name")

	testLogger := logger.NewTestLogger(t)
	r := NewPluginRegistry(testLogger)

	err := r.DiscoverAndLoadPlugins(testLogger, []string{pluginDir}, "xcl-plugin-*")
	require.NoError(t, err)

	require.Empty(t, r.GetPluginHosts())
}
