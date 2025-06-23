package hclconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPluginDiscovery_DiscoverPlugins(t *testing.T) {
	setup := newTestPluginSetup(t)

	// Create test directory structure
	validDir := setup.createPluginDir("valid")
	invalidDir := setup.createPluginDir("invalid")
	mixedDir := setup.createPluginDir("mixed")
	emptyDir := setup.createPluginDir("empty")
	nonExistentDir := filepath.Join(setup.testDir, "non-existent")

	// Build the example plugin once
	examplePlugin := setup.buildExamplePlugin("test-plugin-base")

	// Test cases
	tests := []struct {
		name        string
		setupFunc   func()
		dirs        []string
		pattern     string
		wantPlugins int
		wantErr     bool
	}{
		{
			name: "single valid plugin",
			setupFunc: func() {
				setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-test")
			},
			dirs:        []string{validDir},
			pattern:     "hclconfig-plugin-*",
			wantPlugins: 1,
			wantErr:     false,
		},
		{
			name: "multiple valid plugins",
			setupFunc: func() {
				setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-one")
				setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-two")
			},
			dirs:        []string{validDir},
			pattern:     "hclconfig-plugin-*",
			wantPlugins: 2,
			wantErr:     false,
		},
		{
			name: "plugin not matching pattern",
			setupFunc: func() {
				setup.copyPlugin(examplePlugin, invalidDir, "not-a-plugin")
			},
			dirs:        []string{invalidDir},
			pattern:     "hclconfig-plugin-*",
			wantPlugins: 0,
			wantErr:     false,
		},
		{
			name: "non-executable file",
			setupFunc: func() {
				setup.createNonExecutable(invalidDir, "hclconfig-plugin-fake")
			},
			dirs:        []string{invalidDir},
			pattern:     "hclconfig-plugin-*",
			wantPlugins: 0,
			wantErr:     false,
		},
		{
			name: "mixed directory",
			setupFunc: func() {
				setup.copyPlugin(examplePlugin, mixedDir, "hclconfig-plugin-good")
				setup.createNonPlugin(mixedDir, "hclconfig-plugin-bad")
				setup.createNonExecutable(mixedDir, "hclconfig-plugin-text.txt")
				setup.copyPlugin(examplePlugin, mixedDir, "wrong-pattern")
			},
			dirs:        []string{mixedDir},
			pattern:     "hclconfig-plugin-*",
			wantPlugins: 2, // plugin-good and plugin-bad (both executables)
			wantErr:     false,
		},
		{
			name:        "empty directory",
			setupFunc:   func() {},
			dirs:        []string{emptyDir},
			pattern:     "hclconfig-plugin-*",
			wantPlugins: 0,
			wantErr:     false,
		},
		{
			name:        "non-existent directory",
			setupFunc:   func() {},
			dirs:        []string{nonExistentDir},
			pattern:     "hclconfig-plugin-*",
			wantPlugins: 0,
			wantErr:     false,
		},
		{
			name: "multiple directories",
			setupFunc: func() {
				setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-dir1")
				setup.copyPlugin(examplePlugin, mixedDir, "hclconfig-plugin-dir2")
			},
			dirs:        []string{validDir, mixedDir, emptyDir},
			pattern:     "hclconfig-plugin-*",
			wantPlugins: 2,
			wantErr:     false,
		},
		{
			name: "custom pattern",
			setupFunc: func() {
				setup.copyPlugin(examplePlugin, validDir, "my-custom-plugin-test")
				setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-ignored")
			},
			dirs:        []string{validDir},
			pattern:     "my-custom-plugin-*",
			wantPlugins: 1,
			wantErr:     false,
		},
		{
			name: "duplicate directories",
			setupFunc: func() {
				setup.copyPlugin(examplePlugin, validDir, "hclconfig-plugin-unique")
			},
			dirs:        []string{validDir, validDir, validDir},
			pattern:     "hclconfig-plugin-*",
			wantPlugins: 1, // Should deduplicate
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up directories before each test
			os.RemoveAll(validDir)
			os.RemoveAll(invalidDir)
			os.RemoveAll(mixedDir)
			os.RemoveAll(emptyDir)
			
			setup.createPluginDir("valid")
			setup.createPluginDir("invalid")
			setup.createPluginDir("mixed")
			setup.createPluginDir("empty")

			// Run setup
			tt.setupFunc()

			// Create discovery instance
			var logs []string
			logger := func(msg string) {
				logs = append(logs, msg)
			}
			
			pd := NewPluginDiscovery(tt.dirs, tt.pattern, logger)
			
			// Discover plugins
			plugins, err := pd.DiscoverPlugins()
			
			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("DiscoverPlugins() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// Check plugin count
			if len(plugins) != tt.wantPlugins {
				t.Errorf("DiscoverPlugins() found %d plugins, want %d", len(plugins), tt.wantPlugins)
				t.Logf("Found plugins: %v", plugins)
				t.Logf("Logs: %v", logs)
			}
			
			// Verify all returned paths exist and are absolute
			for _, plugin := range plugins {
				if !filepath.IsAbs(plugin) {
					t.Errorf("Plugin path is not absolute: %s", plugin)
				}
				if _, err := os.Stat(plugin); err != nil {
					t.Errorf("Plugin file does not exist: %s", plugin)
				}
			}
		})
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
	
	if plugins[0] != exePath {
		t.Errorf("Expected plugin path %s, got %s", exePath, plugins[0])
	}
}

func TestExpandPluginDirectories(t *testing.T) {
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

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "expand home directory",
			input:    []string{"~/plugins", "~/.config/plugins"},
			expected: []string{filepath.Join(homeDir, "plugins"), filepath.Join(homeDir, ".config/plugins")},
		},
		{
			name:     "expand environment variables",
			input:    []string{"$TEST_PLUGIN_DIR", "${TEST_PLUGIN_DIR}/sub"},
			expected: []string{"/test/plugins", "/test/plugins/sub"},
		},
		{
			name:     "no expansion needed",
			input:    []string{"/absolute/path", "./relative/path"},
			expected: []string{"/absolute/path", "./relative/path"},
		},
		{
			name:     "mixed paths",
			input:    []string{"~/plugins", "$TEST_PLUGIN_DIR", "/absolute"},
			expected: []string{filepath.Join(homeDir, "plugins"), "/test/plugins", "/absolute"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandPluginDirectories(tt.input)
			
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d paths, got %d", len(tt.expected), len(result))
			}
			
			for i, path := range result {
				// Normalize paths for comparison
				expected := filepath.Clean(tt.expected[i])
				got := filepath.Clean(path)
				
				if got != expected {
					t.Errorf("Path %d: expected %s, got %s", i, expected, got)
				}
			}
		})
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
		var logs []string
		opts := &ParserOptions{
			PluginDirectories:   []string{pluginDir},
			AutoDiscoverPlugins: true,
			PluginNamePattern:   "hclconfig-plugin-*",
			Logger: func(msg string) {
				logs = append(logs, msg)
			},
		}
		
		p := NewParser(opts)
		
		// Check that plugin was discovered and loaded
		foundDiscoveryLog := false
		foundLoadLog := false
		for _, log := range logs {
			if strings.Contains(log, "Successfully loaded plugin") {
				foundLoadLog = true
			}
			if strings.Contains(log, "Plugin discovery complete") {
				foundDiscoveryLog = true
			}
		}
		
		if !foundDiscoveryLog {
			t.Error("Expected to find plugin discovery log")
		}
		if !foundLoadLog {
			t.Error("Expected to find plugin load log")
		}
		
		// Verify plugin is actually loaded by checking registered types
		if len(p.pluginHosts) == 0 {
			t.Error("Expected at least one plugin host to be registered")
		}
	})
	
	// Test with auto-discovery disabled
	t.Run("auto-discovery disabled", func(t *testing.T) {
		var logs []string
		opts := &ParserOptions{
			PluginDirectories:   []string{pluginDir},
			AutoDiscoverPlugins: false,
			PluginNamePattern:   "hclconfig-plugin-*",
			Logger: func(msg string) {
				logs = append(logs, msg)
			},
		}
		
		p := NewParser(opts)
		
		// Check that no discovery happened
		for _, log := range logs {
			if strings.Contains(log, "Plugin discovery") {
				t.Error("Plugin discovery should not run when disabled")
			}
		}
		
		// Verify no plugins loaded
		if len(p.pluginHosts) != 0 {
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
		
		var logs []string
		opts := DefaultOptions()
		opts.Logger = func(msg string) {
			logs = append(logs, msg)
		}
		
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
		
		// Should find 2 plugins
		successCount := 0
		for _, log := range logs {
			if strings.Contains(log, "Successfully loaded plugin") {
				successCount++
			}
		}
		
		if successCount < 2 {
			t.Errorf("Expected to load 2 plugins, loaded %d", successCount)
			t.Logf("Logs: %v", logs)
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
		
		var logs []string
		opts.Logger = func(msg string) {
			logs = append(logs, msg)
		}
		
		p := NewParser(opts)
		
		// Verify no plugins were loaded
		if len(p.pluginHosts) != 0 {
			t.Error("Expected no plugins to be loaded when discovery is disabled")
		}
	})
}