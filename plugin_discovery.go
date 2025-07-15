package hclconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PluginInfo represents a discovered plugin with its metadata
type PluginInfo struct {
	Path      string // Full path to plugin binary
	Namespace string // Plugin namespace (e.g., "jumppad", "community")
	Type      string // Plugin type (e.g., "docker", "kubernetes")
	Platform  string // Platform (e.g., "linux-amd64", "darwin-arm64")
}

// PluginDiscovery handles the discovery of plugin binaries using namespace/type/platform structure
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

// DiscoverPlugins searches for plugin binaries and returns detailed info
func (pd *PluginDiscovery) DiscoverPlugins() ([]PluginInfo, error) {
	var plugins []PluginInfo
	var errors []error
	
	// Get current platform
	currentPlatform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	pd.logger(fmt.Sprintf("Searching for plugins for platform %s", currentPlatform))
	
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
	
	// Walk through each directory to find plugins
	for _, dir := range uniqueDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			pd.logger(fmt.Sprintf("Plugin directory does not exist: %s", dir))
			continue
		}
		
		pd.logger(fmt.Sprintf("Searching for plugins in %s", dir))
		
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip files we can't access
			}
			
			if info.IsDir() {
				return nil // Continue walking into subdirectories
			}
			
			if !pd.isPluginBinary(path) {
				return nil // Not a plugin binary
			}
			
			// Determine plugin metadata from path structure
			pluginInfo := pd.analyzePluginPath(path, dir, currentPlatform)
			if pluginInfo != nil {
				pd.logger(fmt.Sprintf("Found plugin: %s/%s at %s", pluginInfo.Namespace, pluginInfo.Type, path))
				plugins = append(plugins, *pluginInfo)
			}
			
			return nil
		})
		
		if err != nil {
			pd.logger(fmt.Sprintf("Failed to walk directory %s: %v", dir, err))
			errors = append(errors, err)
		}
	}
	
	if len(plugins) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("no plugins found, encountered %d errors during discovery", len(errors))
	}
	
	pd.logger(fmt.Sprintf("Discovered %d plugins for platform %s", len(plugins), currentPlatform))
	return plugins, nil
}

// DiscoverPlugin finds a specific plugin by namespace and type
func (pd *PluginDiscovery) DiscoverPlugin(namespace, pluginType string) (*PluginInfo, error) {
	plugins, err := pd.DiscoverPlugins()
	if err != nil {
		return nil, err
	}
	
	currentPlatform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	
	for _, plugin := range plugins {
		if plugin.Namespace == namespace && plugin.Type == pluginType {
			return &plugin, nil
		}
	}
	
	return nil, fmt.Errorf("plugin %s/%s not found for platform %s", namespace, pluginType, currentPlatform)
}

// analyzePluginPath determines plugin metadata from the file path
// Only supports namespaced structure: namespace/type/platform/binary
func (pd *PluginDiscovery) analyzePluginPath(pluginPath, baseDir, currentPlatform string) *PluginInfo {
	// Get relative path from base directory
	relPath, err := filepath.Rel(baseDir, pluginPath)
	if err != nil {
		return nil // Invalid path
	}
	
	// Split path into components
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	
	// Require exactly namespace/type/platform/binary structure
	if len(parts) != 4 {
		return nil // Not the expected structure
	}
	
	namespace := parts[0]
	pluginType := parts[1]
	platform := parts[2]
	
	// Only include plugins that match current platform
	if platform != currentPlatform {
		return nil // Wrong platform
	}
	
	return &PluginInfo{
		Path:      pluginPath,
		Namespace: namespace,
		Type:      pluginType,
		Platform:  platform,
	}
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