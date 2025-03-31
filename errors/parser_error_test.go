package errors

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParserErrorOutputsString(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/simple/container.hcl")
	require.NoError(t, pathErr)

	err := ParserError{}
	err.Line = 80
	err.Column = 18
	err.Filename = f
	err.Message = "something has gone wrong, Erik probably made a typo somewhere, nic will have to fix"

	require.Contains(t, err.Error(), "error:")
	require.Contains(t, err.Error(), "80")
}

func TestParserErrorHighlightsLine(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/simple/container.hcl")
	require.NoError(t, pathErr)

	err := ParserError{}
	err.Line = 1
	err.Column = 18
	err.Filename = f
	err.Message = "something has gone wrong, Erik probably made a typo somewhere, nic will have to fix"

	errStr := err.Error()

	fmt.Println(errStr)

	require.Contains(t, err.Error(), "\033[1m      1 | variable")
}

func TestParserErrorNonErrorLineGrey(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/simple/container.hcl")
	require.NoError(t, pathErr)

	err := ParserError{}
	err.Line = 2
	err.Column = 18
	err.Filename = f
	err.Message = "something has gone wrong, Erik probably made a typo somewhere, nic will have to fix"

	errStr := err.Error()

	fmt.Println(errStr)

	require.Contains(t, err.Error(), "\033[2m      1 | variable")
}

func TestParserErrorFromMessage(t *testing.T) {
	f, pathErr := filepath.Abs("../test_fixtures/simple/container.hcl")
	require.NoError(t, pathErr)

	err := ParserError{}
	err.Line = 9
	err.Column = 3
	err.Filename = f
	err.Message = `unable to decode body: ../test_fixtures/simple/container.hcl:9,3-8: Unsupported argument; An argument named "image" is not expected here.`

	errStr := err.Error()
	fmt.Println(errStr)

	require.Contains(t, err.Error(), "\033[1m      9 | resource \"template\" \"consul_config\" {")
}
