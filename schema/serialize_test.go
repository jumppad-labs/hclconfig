package schema

import (
	"testing"

	fixtures "github.com/jumppad-labs/hclconfig/schema/test_fixtures"
	"github.com/stretchr/testify/require"
)

func TestSerializeInt(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyInt{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntJSON, string(b))
}

func TestSerializeIntSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyIntSlice{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntSliceJSON, string(b))
}

func TestSerializeIntPtr(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyIntPtr{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntPtrJSON, string(b))
}

func TestSerializeIntPtrSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyIntPtrSlice{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntPtrSliceJSON, string(b))
}

func TestSerializeString(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyString{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStringJSON, string(b))
}

func TestSerializeStringSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStringSlice{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStringSliceJSON, string(b))
}

func TestSerializeStringPtr(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStringPtr{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStringPtrJSON, string(b))
}

func TestSerializeStringPtrSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStringPtrSlice{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStringPtrSliceJSON, string(b))
}

func TestSerializeBool(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyBool{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyBoolJSON, string(b))
}

func TestSerializeBoolSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyBoolSlice{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyBoolSliceJSON, string(b))
}

func TestSerializeBoolPtr(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyBoolPtr{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyBoolPtrJSON, string(b))
}

func TestSerializeBoolPtrSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyBoolPtrSlice{})
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyBoolPtrSliceJSON, string(b))
}