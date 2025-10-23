package state

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

type testExampleResource struct {
	types.ResourceBase
}

func testCreateState(t *testing.T) (StateStore, string) {
	p := path.Join(t.TempDir(), "state.json")

	ss, err := NewFileStateStore(p)

	require.NoError(t, err)
	require.NotNil(t, ss)
	require.FileExists(t, p)

	return ss, p
}

func testSaveState(t *testing.T) (StateStore, string) {
	ss, p := testCreateState(t)
	s := NewState()
	err := s.AppendResource(&testExampleResource{types.ResourceBase{Meta: types.Meta{Type: "test", Name: "example"}}})
	require.NoError(t, err)

	err = ss.Save(s)
	require.NoError(t, err)
	require.FileExists(t, p)

	return ss, p
}

func testNewStateAtExistingPath(t *testing.T) (StateStore, string) {
	_, p := testSaveState(t)

	// create the file first
	err := os.WriteFile(p, []byte("{}"), 0644)
	require.NoError(t, err)

	ss, err := NewFileStateStore(p)

	require.NoError(t, err)
	require.NotNil(t, ss)

	return ss, p
}

func TestCreatesStateAtEmptyPath(t *testing.T) {
	testCreateState(t)
}

func TestSaveSavesStateToFile(t *testing.T) {
	_, p := testSaveState(t)

	// load the file and check contents
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	res := []*testExampleResource{}
	err = json.Unmarshal(data, &res)
	require.NoError(t, err)
	require.Equal(t, "resource.test.example", res[0].Meta.ID)
}

func TestNewStateAtExistingPath(t *testing.T) {
	testNewStateAtExistingPath(t)
}

func TestLoadStateContainsResources(t *testing.T) {
	ss, _ := testSaveState(t)

	s, err := ss.Load()
	require.NoError(t, err)
	require.NotNil(t, s)
}
