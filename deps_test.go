package hclconfig

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDepsValidated(t *testing.T) {
	absoluteFolderPath, err := filepath.Abs("./test_fixtures/deps/main.hcl")
	if err != nil {
		t.Fatal(err)
	}

	p := setupParser(t)

	_, err = p.ParseFile(absoluteFolderPath)
	fmt.Println(err)
	require.Error(t, err)
}
