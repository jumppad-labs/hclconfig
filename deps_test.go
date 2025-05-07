package hclconfig

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDependenciesValidNoError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/deps/valid.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	require.NoError(t, err)
}

func TestDependenciesInvalidError(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/deps/invalid.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	fmt.Println(err)
	require.Error(t, err)
}
