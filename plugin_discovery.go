package hclconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PluginDiscovery handles the discovery of plugin binaries in configured directories
type PluginDiscovery struct {
	directories []string
	pattern     string
	logger      func(string)
}

// NewPluginDiscovery creates a new PluginDiscovery instance
func NewPluginDiscovery(directories []string, pattern string, logger func(string)) *PluginDiscovery {
	if logger == nil {
		logger = func(string) {} // no-op logger
	}
	
	if pattern == "" {
		pattern = "hclconfig-plugin-*"
	}
	
	return &PluginDiscovery{
		directories: directories,
		pattern:     pattern,
		logger:      logger,
	}
}

// DiscoverPlugins searches for plugin binaries in all configured directories
func (pd *PluginDiscovery) DiscoverPlugins() ([]string, error) {
	var plugins []string
	var errors []error
	
	// Deduplicate directories
	seen := make(map[string]bool)
	uniqueDirs := []string{}
	for _, dir := range pd.directories {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			pd.logger(fmt.Sprintf("Failed to resolve directory %s: %v", dir, err))
			continue
		}
		if !seen[absDir] {
			seen[absDir] = true
			uniqueDirs = append(uniqueDirs, absDir)
		}
	}
	
	for _, dir := range uniqueDirs {
		found, err := pd.discoverInDirectory(dir)
		if err != nil {
			pd.logger(fmt.Sprintf("Failed to discover plugins in %s: %v", dir, err))
			errors = append(errors, err)
			continue
		}
		plugins = append(plugins, found...)
	}
	
	if len(plugins) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("no plugins found, encountered %d errors during discovery", len(errors))
	}
	
	pd.logger(fmt.Sprintf("Discovered %d plugins", len(plugins)))
	return plugins, nil
}

// discoverInDirectory searches for plugins in a single directory
func (pd *PluginDiscovery) discoverInDirectory(dir string) ([]string, error) {
	var plugins []string
	
	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			pd.logger(fmt.Sprintf("Plugin directory does not exist: %s", dir))
			return plugins, nil // Not an error, just no plugins
		}
		return nil, err
	}
	
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	
	// Read directory contents
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}
	
	pd.logger(fmt.Sprintf("Searching for plugins in %s", dir))
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		fullPath := filepath.Join(dir, entry.Name())
		
		if pd.isPluginBinary(fullPath) {
			pd.logger(fmt.Sprintf("Found plugin: %s", fullPath))
			plugins = append(plugins, fullPath)
		}
	}
	
	return plugins, nil
}

// isPluginBinary checks if a file is a valid plugin binary
func (pd *PluginDiscovery) isPluginBinary(path string) bool {
	// Get base name for pattern matching
	baseName := filepath.Base(path)
	
	// On Windows, remove .exe extension for pattern matching
	if runtime.GOOS == "windows" {
		baseName = strings.TrimSuffix(baseName, ".exe")
	}
	
	// Check if name matches pattern
	matched, err := filepath.Match(pd.pattern, baseName)
	if err != nil || !matched {
		return false
	}
	
	// Check if file is executable
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	
	// Check executable permissions
	if runtime.GOOS == "windows" {
		// On Windows, check for .exe extension
		return strings.HasSuffix(strings.ToLower(path), ".exe")
	}
	
	// On Unix-like systems, check execute permission
	mode := info.Mode()
	return mode.IsRegular() && (mode.Perm()&0111 != 0)
}

// ExpandPluginDirectories expands environment variables and home directory in paths
func ExpandPluginDirectories(dirs []string) []string {
	expanded := make([]string, 0, len(dirs))
	
	for _, dir := range dirs {
		// Expand environment variables
		dir = os.ExpandEnv(dir)
		
		// Expand home directory
		if strings.HasPrefix(dir, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				dir = filepath.Join(home, dir[2:])
			}
		}
		
		expanded = append(expanded, dir)
	}
	
	return expanded
}