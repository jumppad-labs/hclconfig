package schema

import (
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

func TestSerializeIntMap(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyIntMap{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntMapJSON, string(b))
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

func TestSerializeIntPtrMap(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyIntMapPtr{}, maxDepth)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyIntMapPtrJSON, string(b))
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

	require.JSONEq(t, fixtures.MyStructPtrDepth1JSON, string(b))
}

func TestSerializeStructPtrDepth2(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructPtr{}, 2)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructPtrDepth2JSON, string(b))
}

func TestSerializeStructPtrDepth3(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructPtr{}, 3)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructPtrDepth3JSON, string(b))
}

func TestSerializeStructSliceDepth1(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructSlice{}, 1)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructSliceDepth1JSON, string(b))
}

func TestSerializeStructSliceDepth2(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructSlice{}, 2)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructSliceDepth2JSON, string(b))
}

func TestSerializeStructPtrSliceDepth1(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructPtrSlice{}, 1)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructPtrSliceDepth1JSON, string(b))
}

func TestSerializeStructPtrSliceDepth2(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructPtrSlice{}, 2)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructPtrSliceDepth2JSON, string(b))
}

func TestSerializeStructMapDepth1(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructMap{}, 1)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructMapDepth1JSON, string(b))
}

func TestSerializeStructMapDepth2(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructMap{}, 2)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructMapDepth2JSON, string(b))
}

func TestSerializeStructMapPtrDepth1(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructMapPtr{}, 1)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructMapPtrDepth1JSON, string(b))
}

func TestSerializeStructMapPtrDepth2(t *testing.T) {
	b, err := GenerateFromInstance(fixtures.MyStructMapPtr{}, 2)
	require.NoError(t, err)

	require.JSONEq(t, fixtures.MyStructMapPtrDepth2JSON, string(b))
}
