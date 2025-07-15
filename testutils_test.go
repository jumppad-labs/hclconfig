package hclconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestProcessesTypes(t *testing.T) {
	vars := map[string]cty.Value{}
	vars["string"] = cty.StringVal("abc")
	vars["number"] = cty.NumberIntVal(23)
	vars["bool"] = cty.BoolVal(true)
	vars["array"] = cty.ListVal(
		[]cty.Value{
			cty.StringVal("abc"),
			cty.StringVal("123"),
		})

	vars["map"] = cty.MapVal(map[string]cty.Value{
		"foo": cty.StringVal("abc"),
	})

	output := ParseVars(vars)

	require.Equal(t, "abc", output["string"])

	num := int64(output["number"].(float64))
	require.Equal(t, int64(23), num)

	require.True(t, output["bool"].(bool))

	require.Equal(t, "abc", output["array"].([]any)[0])
	require.Equal(t, "123", output["array"].([]any)[1])

	require.Equal(t, "abc", output["map"].(map[string]any)["foo"])
}

// CreateConfigFromStrings is a test helper function that
// parses the given contents strings as HCL and returns a Shipyard Config
func CreateConfigFromStrings(t *testing.T, contents ...string) (*Config, string) {
	//dir := CreateTestFiles(t, contents...)

	//c := resources.NewConfig()
	//err := ParseFolder(dir, c, false, "", false, []string{}, nil, "")
	//require.NoError(t, err)

	//return c, dir

	return nil, ""
}

// createsTestFiles creates a temporary directory and
// stores temp files into it
// returns directory containing files
// cleanup function
// usage:
// d, cleanup := createTestFiles(t, `cluster "abc" {}`, `docs "bcdf" {}`)
// defer cleanup()
func CreateTestFiles(t *testing.T, contents ...string) string {
	dir := createTempDirectory(t)

	for _, x := range contents {
		createNamedFile(t, dir, "*.hcl", x)
	}

	t.Cleanup(func() {
		removeTestFiles(t, dir)
	})

	return dir
}

// createTestFile creates a hcl file from the given contents
func CreateTestFile(t *testing.T, contents string) string {
	dir := createTempDirectory(t)

	t.Cleanup(func() {
		removeTestFiles(t, dir)
	})

	return createNamedFile(t, dir, "*.hcl", contents)
}

// create a temporary directory
func createTempDirectory(t *testing.T) string {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("Unable to create temporary directory: %s", err)
	}

	return dir
}

func createNamedFile(t *testing.T, dir, name, contents string) string {
	f, err := os.CreateTemp(dir, name)
	if err != nil {
		t.Fatalf("Error creating temp file %s", err)
	}
	defer f.Close()

	if _, err := f.WriteString(contents); err != nil {
		t.Fatalf("Error writing temp file contents: %s", err)
	}

	return f.Name()
}

// remove test files cleans up any temporary files created
// with createTestFile
func removeTestFiles(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("Unable to remove temporary files %s", err)
	}
}

// Plugin discovery test utilities

// testPluginSetup contains helper functions for plugin discovery tests
type testPluginSetup struct {
	t       *testing.T
	testDir string
}

// newTestPluginSetup creates a new test setup
func newTestPluginSetup(t *testing.T) *testPluginSetup {
	// Create a temporary directory for test files
	testDir, err := os.MkdirTemp("", "plugin-discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Ensure cleanup
	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})

	return &testPluginSetup{
		t:       t,
		testDir: testDir,
	}
}

// createPluginDir creates a plugin directory and returns its path
func (s *testPluginSetup) createPluginDir(name string) string {
	dir := filepath.Join(s.testDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.t.Fatalf("Failed to create plugin directory %s: %v", dir, err)
	}
	return dir
}

// buildExamplePlugin builds the example plugin binary and returns its path
func (s *testPluginSetup) buildExamplePlugin(outputName string) string {
	outputPath := filepath.Join(s.testDir, outputName)
	if runtime.GOOS == "windows" {
		outputPath += ".exe"
	}

	// Build the example plugin
	cmd := exec.Command("go", "build", "-o", outputPath, "./plugins/example")
	cmd.Dir = getRootDir()

	output, err := cmd.CombinedOutput()
	if err != nil {
		s.t.Fatalf("Failed to build example plugin: %v\nOutput: %s", err, output)
	}

	return outputPath
}

// getRootDir finds the project root directory
func getRootDir() string {
	// Start from current directory and walk up until we find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			panic("Could not find project root")
		}
		dir = parent
	}
}

// copyPlugin copies a plugin binary to the proper namespace/type/platform/binary structure
func (s *testPluginSetup) copyPlugin(src, baseDir, namespace, pluginType, platform, binaryName string) string {
	// Create directory structure: baseDir/namespace/type/platform/
	pluginDir := filepath.Join(baseDir, namespace, pluginType, platform)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		s.t.Fatalf("Failed to create plugin directory %s: %v", pluginDir, err)
	}

	// Copy the plugin binary
	dst := filepath.Join(pluginDir, binaryName)
	if runtime.GOOS == "windows" && !hasExeExtension(dst) {
		dst += ".exe"
	}

	srcData, err := os.ReadFile(src)
	if err != nil {
		s.t.Fatalf("Failed to read source plugin %s: %v", src, err)
	}

	if err := os.WriteFile(dst, srcData, 0755); err != nil {
		s.t.Fatalf("Failed to write plugin to %s: %v", dst, err)
	}

	return dst
}

// createNonPlugin creates a non-plugin executable file
func (s *testPluginSetup) createNonPlugin(dir, name string) string {
	path := filepath.Join(dir, name)

	var content string
	if runtime.GOOS == "windows" {
		path += ".bat"
		content = "@echo off\necho This is not a plugin\n"
	} else {
		content = "#!/bin/sh\necho 'This is not a plugin'\n"
	}

	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		s.t.Fatalf("Failed to create non-plugin %s: %v", path, err)
	}

	return path
}

// createNonExecutable creates a non-executable file
func (s *testPluginSetup) createNonExecutable(dir, name string) string {
	path := filepath.Join(dir, name)
	content := "This is not an executable file"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		s.t.Fatalf("Failed to create non-executable %s: %v", path, err)
	}

	return path
}

// hasExeExtension checks if a filename has .exe extension
func hasExeExtension(filename string) bool {
	return len(filename) > 4 && filename[len(filename)-4:] == ".exe"
}
