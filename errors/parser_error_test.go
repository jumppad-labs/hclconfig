package errors

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParserErrorOutputsString(t *testing.T) {
	f, pathErr := filepath.Abs("./test_fixtures/simple/container.hcl")
	require.NoError(t, pathErr)

	err := ParserError{}
	err.Line = 80
	err.Column = 18
	err.Filename = f
	err.Message = "something has gone wrong, Erik probably made a typo somewhere, nic will have to fix"

	require.Contains(t, err.Error(), "Error:")
	require.Contains(t, err.Error(), "80")
}
