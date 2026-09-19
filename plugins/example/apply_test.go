package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

const (
	peopleConfig  = "./testdata/people.xcl"
	testPersonID  = "resource.person.test_person"
	otherPersonID = "resource.person.other_person"
)

// applyEventCollector gathers parser events; the walker fires them in parallel
type applyEventCollector struct {
	mu     sync.Mutex
	events []parser.ParserEvent
}

func (c *applyEventCollector) collect(event parser.ParserEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

// successfulResources returns the IDs of the resources for which the given
// operation succeeded, leaving out builtin types which are not resources
func (c *applyEventCollector) successfulResources(operation string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	ids := []string{}
	for _, event := range c.events {
		if event.Operation == operation && event.Phase == "success" && strings.HasPrefix(event.ResourceID, "resource.") {
			ids = append(ids, event.ResourceID)
		}
	}

	return ids
}

// applyPeople applies the people config with a fresh parser that shares the
// registry and state store, saves the returned state and returns the events
// the apply fired
func applyPeople(t *testing.T, reg *registry.PluginRegistry, store *state.FileStateStore) (*state.State, *applyEventCollector) {
	t.Helper()

	collector := &applyEventCollector{}

	options := parser.DefaultOptions()
	options.Logger = logger.NewTestLogger(t)
	options.ModuleCache = filepath.Join(t.TempDir(), parser.ConfigDirectory, "cache")
	options.PluginRegistry = reg
	options.StateStore = store
	options.OnParserEvent = collector.collect

	p := parser.NewParser(options)

	st, err := p.Apply(peopleConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	err = store.Save(st)
	require.NoError(t, err)

	return st, collector
}

// findPerson returns the person with the given ID from the state
func findPerson(t *testing.T, st *state.State, id string) *person.Person {
	t.Helper()

	resource, err := st.FindResource(id)
	require.NoError(t, err)

	found := &person.Person{}
	err = schema.UnmarshalUntyped(resource, found)
	require.NoError(t, err)

	return found
}

func TestExampleProviderSecondApplyMakesNoCreateOrUpdate(t *testing.T) {
	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())
	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	reg := registry.NewPluginRegistry(logger.NewTestLogger(t))
	err := reg.RegisterPlugin(&PersonPlugin{})
	require.NoError(t, err)

	store, err := state.NewFileStateStore(filepath.Join(t.TempDir(), "state.json"), reg)
	require.NoError(t, err)

	// apply 1: both people are created and given a person id
	_, first := applyPeople(t, reg, store)
	require.ElementsMatch(t, []string{testPersonID, otherPersonID}, first.successfulResources("create"))

	// apply 2: nothing changed, so both people are only read
	st, second := applyPeople(t, reg, store)

	require.Empty(t, second.successfulResources("create"))
	require.Empty(t, second.successfulResources("update"))
	require.ElementsMatch(t, []string{testPersonID, otherPersonID}, second.successfulResources("read"))

	// the example Read adds nothing, the person id was carried over by xcl
	require.Equal(t, "person-test-user", findPerson(t, st, testPersonID).PersonID)
	require.Equal(t, "person-other-person", findPerson(t, st, otherPersonID).PersonID)

	saved, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "person-test-user", findPerson(t, saved, testPersonID).PersonID)
	require.Equal(t, "person-other-person", findPerson(t, saved, otherPersonID).PersonID)
}
