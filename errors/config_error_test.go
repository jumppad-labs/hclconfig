package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendParseErrorAddsError(t *testing.T) {
	ce := NewConfigError()
	ce.AppendParseError(fmt.Errorf("boom"))

	require.Len(t, ce.ParseErrors, 1)
}

func TestAppendProcessErrorAddsError(t *testing.T) {
	ce := NewConfigError()
	ce.AppendProcessError(fmt.Errorf("boom"))

	require.Len(t, ce.ProcessErrors, 1)
}

func TestErrorReturnsConcatonatedString(t *testing.T) {
	ce := NewConfigError()
	ce.AppendParseError(fmt.Errorf("boom"))
	ce.AppendProcessError(fmt.Errorf("bang"))

	require.Equal(t, "boom\nbang", ce.Error())
}
