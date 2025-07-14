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

// DiscoverPlugins searches for plugin binaries in all configured directories using namespace/type/platform structure
func (pd *PluginDiscovery) DiscoverPlugins() ([]PluginInfo, error) {
	var plugins []PluginInfo
	var errors []error
	
	// Get current platform
	currentPlatform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	
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
		found, err := pd.discoverInDirectory(dir, currentPlatform)
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
	
	pd.logger(fmt.Sprintf("Discovered %d plugins for platform %s", len(plugins), currentPlatform))
	return plugins, nil
}

// DiscoverPlugin finds a specific plugin by namespace and type
func (pd *PluginDiscovery) DiscoverPlugin(namespace, pluginType string) (*PluginInfo, error) {
	currentPlatform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	
	for _, dir := range pd.directories {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		
		plugin, err := pd.discoverSpecificPlugin(absDir, namespace, pluginType, currentPlatform)
		if err != nil {
			continue
		}
		if plugin != nil {
			return plugin, nil
		}
	}
	
	return nil, fmt.Errorf("plugin %s/%s not found for platform %s", namespace, pluginType, currentPlatform)
}

// discoverInDirectory searches for plugins in a single directory using namespace/type/platform structure
func (pd *PluginDiscovery) discoverInDirectory(dir string, platform string) ([]PluginInfo, error) {
	var plugins []PluginInfo
	
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
	
	// Read namespace directories
	namespaces, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}
	
	pd.logger(fmt.Sprintf("Searching for plugins in %s for platform %s", dir, platform))
	
	for _, namespaceEntry := range namespaces {
		if !namespaceEntry.IsDir() {
			continue
		}
		
		namespace := namespaceEntry.Name()
		namespacePath := filepath.Join(dir, namespace)
		
		// Read plugin type directories within namespace
		types, err := os.ReadDir(namespacePath)
		if err != nil {
			pd.logger(fmt.Sprintf("Failed to read namespace directory %s: %v", namespacePath, err))
			continue
		}
		
		for _, typeEntry := range types {
			if !typeEntry.IsDir() {
				continue
			}
			
			pluginType := typeEntry.Name()
			typePath := filepath.Join(namespacePath, pluginType)
			
			// Look for platform-specific directory
			platformPath := filepath.Join(typePath, platform)
			if platformInfo, err := os.Stat(platformPath); err == nil && platformInfo.IsDir() {
				// Found platform directory, look for plugin binary
				if plugin := pd.findPluginInPlatformDir(platformPath, namespace, pluginType, platform); plugin != nil {
					plugins = append(plugins, *plugin)
				}
			}
		}
	}
	
	return plugins, nil
}

// discoverSpecificPlugin finds a specific plugin by namespace and type
func (pd *PluginDiscovery) discoverSpecificPlugin(dir, namespace, pluginType, platform string) (*PluginInfo, error) {
	// Construct path: dir/namespace/type/platform/
	pluginPath := filepath.Join(dir, namespace, pluginType, platform)
	
	if info, err := os.Stat(pluginPath); err != nil || !info.IsDir() {
		return nil, nil // Plugin not found in this directory
	}
	
	return pd.findPluginInPlatformDir(pluginPath, namespace, pluginType, platform), nil
}

// findPluginInPlatformDir looks for a plugin binary in a platform directory
func (pd *PluginDiscovery) findPluginInPlatformDir(platformPath, namespace, pluginType, platform string) *PluginInfo {
	entries, err := os.ReadDir(platformPath)
	if err != nil {
		pd.logger(fmt.Sprintf("Failed to read platform directory %s: %v", platformPath, err))
		return nil
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		fullPath := filepath.Join(platformPath, entry.Name())
		
		if pd.isPluginBinary(fullPath) {
			pd.logger(fmt.Sprintf("Found plugin: %s/%s at %s", namespace, pluginType, fullPath))
			return &PluginInfo{
				Path:      fullPath,
				Namespace: namespace,
				Type:      pluginType,
				Platform:  platform,
			}
		}
	}
	
	return nil
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