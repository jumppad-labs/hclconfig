package errors

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendErrorAddsError(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(&ParserError{})

	require.Len(t, ce.Errors, 1)
}

func TestContainsWarningsReturnsTrue(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(&ParserError{Level: ParserErrorLevelWarning})

	require.True(t, ce.ContainsWarnings())
	require.False(t, ce.ContainsErrors())
}

func TestContainsErrorsReturnsTrue(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(&ParserError{Level: ParserErrorLevelError})

	require.False(t, ce.ContainsWarnings())
	require.True(t, ce.ContainsErrors())
}

func TestErrorReturnsConcatonatedString(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(&ParserError{Message: "boom"})
	ce.AppendError(&ParserError{Message: "bang"})

	require.Equal(t, "Error:\n  boom\n\n  :0,0\n\nError:\n  bang\n\n  :0,0\n", ce.Error())
}
