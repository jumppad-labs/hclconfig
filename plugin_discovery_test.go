package hclconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jumppad-labs/hclconfig/logger"
)


func TestPluginDiscoverySingleValidPlugin(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")
	
	setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-test")

	testLogger := logger.NewTestLogger(t)
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{validDir}, "hclconfig-plugin-*", loggerFunc)
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
		if !filepath.IsAbs(plugin.Path) {
			t.Errorf("Plugin path is not absolute: %s", plugin.Path)
		}
		if _, err := os.Stat(plugin.Path); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin.Path)
		}
	}
}

func TestPluginDiscoveryMultipleValidPlugins(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")
	
	setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-one")
	setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-two")

	testLogger := logger.NewTestLogger(t)
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{validDir}, "hclconfig-plugin-*", loggerFunc)
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
		if !filepath.IsAbs(plugin.Path) {
			t.Errorf("Plugin path is not absolute: %s", plugin.Path)
		}
		if _, err := os.Stat(plugin.Path); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin.Path)
		}
	}
}

func TestPluginDiscoveryPluginNotMatchingPattern(t *testing.T) {
	setup := newTestPluginSetup(t)
	invalidDir := setup.createPluginDir("invalid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")
	
	setup.copyPlugin(examplePlugin, invalidDir, "not-a-plugin")

	testLogger := logger.NewTestLogger(t)
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{invalidDir}, "hclconfig-plugin-*", loggerFunc)
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
	
	setup.createNonExecutable(invalidDir, "hclconfig-plugin-fake")

	testLogger := logger.NewTestLogger(t)
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{invalidDir}, "hclconfig-plugin-*", loggerFunc)
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
	
	setup.copyPlugin(examplePlugin, mixedDir, "hclconfig-plugin-good")
	setup.createNonPlugin(mixedDir, "hclconfig-plugin-bad")
	setup.createNonExecutable(mixedDir, "hclconfig-plugin-text.txt")
	setup.copyPlugin(examplePlugin, mixedDir, "wrong-pattern")

	testLogger := logger.NewTestLogger(t)
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{mixedDir}, "hclconfig-plugin-*", loggerFunc)
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
		if !filepath.IsAbs(plugin.Path) {
			t.Errorf("Plugin path is not absolute: %s", plugin.Path)
		}
		if _, err := os.Stat(plugin.Path); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin.Path)
		}
	}
}

func TestPluginDiscoveryEmptyDirectory(t *testing.T) {
	setup := newTestPluginSetup(t)
	emptyDir := setup.createPluginDir("empty")

	testLogger := logger.NewTestLogger(t)
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{emptyDir}, "hclconfig-plugin-*", loggerFunc)
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
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{nonExistentDir}, "hclconfig-plugin-*", loggerFunc)
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
	
	setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-dir1")
	setup.copyPlugin(examplePlugin, mixedDir, "hclconfig-plugin-dir2")

	testLogger := logger.NewTestLogger(t)
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{validDir, mixedDir, emptyDir}, "hclconfig-plugin-*", loggerFunc)
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
		if !filepath.IsAbs(plugin.Path) {
			t.Errorf("Plugin path is not absolute: %s", plugin.Path)
		}
		if _, err := os.Stat(plugin.Path); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin.Path)
		}
	}
}

func TestPluginDiscoveryCustomPattern(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")
	
	setup.copyPlugin(examplePlugin, validDir, "my-custom-plugin-test")
	setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-ignored")

	testLogger := logger.NewTestLogger(t)
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{validDir}, "my-custom-plugin-*", loggerFunc)
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
		if !filepath.IsAbs(plugin.Path) {
			t.Errorf("Plugin path is not absolute: %s", plugin.Path)
		}
		if _, err := os.Stat(plugin.Path); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin.Path)
		}
	}
}

func TestPluginDiscoveryDuplicateDirectories(t *testing.T) {
	setup := newTestPluginSetup(t)
	validDir := setup.createPluginDir("valid")
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")
	
	setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-unique")

	testLogger := logger.NewTestLogger(t)
	loggerFunc := func(msg string) {
		testLogger.Info(msg)
	}
	
	pd := NewPluginDiscovery([]string{validDir, validDir, validDir}, "hclconfig-plugin-*", loggerFunc)
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
		if !filepath.IsAbs(plugin.Path) {
			t.Errorf("Plugin path is not absolute: %s", plugin.Path)
		}
		if _, err := os.Stat(plugin.Path); err != nil {
			t.Errorf("Plugin file does not exist: %s", plugin.Path)
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
	exePath := setup.copyPlugin(examplePlugin, dir, "hclconfig-plugin-test.exe")
	
	// Create without .exe extension (should not be found on Windows)
	nonExePath := filepath.Join(dir, "hclconfig-plugin-noext")
	if err := os.WriteFile(nonExePath, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	
	pd := NewPluginDiscovery([]string{dir}, "hclconfig-plugin-*", nil)
	plugins, err := pd.DiscoverPlugins()
	
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if len(plugins) != 1 {
		t.Fatalf("Expected 1 plugin, found %d", len(plugins))
	}
	
	if plugins[0].Path != exePath {
		t.Errorf("Expected plugin path %s, got %s", exePath, plugins[0].Path)
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

func TestParserIntegration_AutoDiscovery(t *testing.T) {
	setup := newTestPluginSetup(t)
	
	// Create plugin directory
	pluginDir := setup.createPluginDir("plugins")
	
	// Build and copy example plugin
	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, pluginDir, "hclconfig-plugin-example")
	
	// Test with auto-discovery enabled
	t.Run("auto-discovery enabled", func(t *testing.T) {
		opts := &ParserOptions{
			PluginDirectories:   []string{pluginDir},
			AutoDiscoverPlugins: true,
			PluginNamePattern:   "hclconfig-plugin-*",
			Logger: logger.NewTestLogger(t),
		}
		
		p := NewParser(opts)
		
		// Check that plugin was discovered and loaded by verifying plugin registry
		
		// Verify plugin is actually loaded by checking registered types
		if len(p.pluginRegistry.GetPluginHosts()) == 0 {
			t.Error("Expected at least one plugin host to be registered")
		}
	})
	
	// Test with auto-discovery disabled
	t.Run("auto-discovery disabled", func(t *testing.T) {
		opts := &ParserOptions{
			PluginDirectories:   []string{pluginDir},
			AutoDiscoverPlugins: false,
			PluginNamePattern:   "hclconfig-plugin-*",
			Logger: logger.NewTestLogger(t),
		}
		
		p := NewParser(opts)
		
		// Check that no discovery happened by verifying plugin registry is empty
		
		// Verify no plugins loaded
		if len(p.pluginRegistry.GetPluginHosts()) != 0 {
			t.Error("Expected no plugin hosts when auto-discovery is disabled")
		}
	})
}

func TestParserIntegration_EnvironmentVariables(t *testing.T) {
	setup := newTestPluginSetup(t)
	
	// Save original env
	originalPath := os.Getenv("HCLCONFIG_PLUGIN_PATH")
	originalDisable := os.Getenv("HCLCONFIG_DISABLE_PLUGIN_DISCOVERY")
	defer func() {
		os.Setenv("HCLCONFIG_PLUGIN_PATH", originalPath)
		os.Setenv("HCLCONFIG_DISABLE_PLUGIN_DISCOVERY", originalDisable)
	}()
	
	// Create plugin directories
	envDir1 := setup.createPluginDir("env1")
	envDir2 := setup.createPluginDir("env2")
	
	// Build and copy plugins
	examplePlugin := setup.buildExamplePlugin("test-plugin")
	setup.copyPlugin(examplePlugin, envDir1, "hclconfig-plugin-env1")
	setup.copyPlugin(examplePlugin, envDir2, "hclconfig-plugin-env2")
	
	// Test HCLCONFIG_PLUGIN_PATH
	t.Run("plugin path from environment", func(t *testing.T) {
		separator := ":"
		if runtime.GOOS == "windows" {
			separator = ";"
		}
		os.Setenv("HCLCONFIG_PLUGIN_PATH", envDir1+separator+envDir2)
		os.Setenv("HCLCONFIG_DISABLE_PLUGIN_DISCOVERY", "")
		
		opts := DefaultOptions()
		opts.Logger = logger.NewTestLogger(t)
		
		// Verify directories were added
		foundEnv1 := false
		foundEnv2 := false
		for _, dir := range opts.PluginDirectories {
			if filepath.Clean(dir) == filepath.Clean(envDir1) {
				foundEnv1 = true
			}
			if filepath.Clean(dir) == filepath.Clean(envDir2) {
				foundEnv2 = true
			}
		}
		
		if !foundEnv1 || !foundEnv2 {
			t.Error("Expected environment directories to be included")
			t.Logf("Directories: %v", opts.PluginDirectories)
		}
		
		// Create parser and verify plugins are discovered
		p := NewParser(opts)
		
		// Should find 2 plugins - verify by checking plugin registry
		if len(p.pluginRegistry.GetPluginHosts()) < 2 {
			t.Errorf("Expected to load 2 plugins, loaded %d", len(p.pluginRegistry.GetPluginHosts()))
		}
		
		_ = p // Use p to avoid unused variable warning
	})
	
	// Test HCLCONFIG_DISABLE_PLUGIN_DISCOVERY
	t.Run("disable discovery from environment", func(t *testing.T) {
		os.Setenv("HCLCONFIG_PLUGIN_PATH", envDir1)
		os.Setenv("HCLCONFIG_DISABLE_PLUGIN_DISCOVERY", "true")
		
		opts := DefaultOptions()
		
		if opts.AutoDiscoverPlugins {
			t.Error("Expected AutoDiscoverPlugins to be false when HCLCONFIG_DISABLE_PLUGIN_DISCOVERY=true")
		}
		
		opts.Logger = logger.NewTestLogger(t)
		
		p := NewParser(opts)
		
		// Verify no plugins were loaded
		if len(p.pluginRegistry.GetPluginHosts()) != 0 {
			t.Error("Expected no plugins to be loaded when discovery is disabled")
		}
	})
}