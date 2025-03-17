package schema

import (
	"fmt"
	"testing"

	fixtures "github.com/jumppad-labs/hclconfig/schema/test_fixtures"
	"github.com/stretchr/testify/require"
)

const maxDepth = 10

func TestSerializeInt(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyInt{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntJSON, string(b))
}

func TestSerializeIntSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyIntSlice{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntSliceJSON, string(b))
}

func TestSerializeIntPtr(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyIntPtr{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntPtrJSON, string(b))
}

func TestSerializeIntPtrSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyIntPtrSlice{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntPtrSliceJSON, string(b))
}

func TestSerializeString(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyString{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStringJSON, string(b))
}

func TestSerializeStringSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStringSlice{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStringSliceJSON, string(b))
}

func TestSerializeStringPtr(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStringPtr{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStringPtrJSON, string(b))
}

func TestSerializeStringPtrSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStringPtrSlice{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStringPtrSliceJSON, string(b))
}

func TestSerializeBool(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyBool{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyBoolJSON, string(b))
}

func TestSerializeBoolSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyBoolSlice{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyBoolSliceJSON, string(b))
}

func TestSerializeBoolPtr(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyBoolPtr{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyBoolPtrJSON, string(b))
}

func TestSerializeBoolPtrSlice(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyBoolPtrSlice{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyBoolPtrSliceJSON, string(b))
}

func TestSerializeStructPtrDepth1(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructPtr{}, 1)
	require.NoError(t, err)
	fmt.Println(string(b))

	require.JSONEq(t, fixtures.MyStructPtrDepth1JSON, string(b))
}

func TestSerializeStructPtrDepth2(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructPtr{}, 2)
	require.NoError(t, err)
	fmt.Println(string(b))

	require.JSONEq(t, fixtures.MyStructPtrDepth2JSON, string(b))
}

func TestSerializeStructPtrDepth3(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructPtr{}, 3)
	require.NoError(t, err)
	fmt.Println(string(b))

	require.JSONEq(t, fixtures.MyStructPtrDepth3JSON, string(b))
}

func TestSerializeStructPtrSliceDepth2(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructPtrSlice{}, 2)
	require.NoError(t, err)
	fmt.Println(string(b))

	require.JSONEq(t, fixtures.MyStructPtrSliceDepth2JSON, string(b))
}