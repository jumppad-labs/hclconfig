package schema

import (
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

func TestCreatesStructFromSchema(t *testing.T) {
	s, err := CreateStructFromSchema([]byte(myEntityJson))
	require.NoError(t, err)

	pretty.Println(s)
	require.False(t, true)
}
