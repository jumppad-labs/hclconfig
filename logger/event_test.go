package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLeadWithEventMovesTheEventArgumentToLeadTheMessage(t *testing.T) {
	msg, args := leadWithEvent("calling provider", []interface{}{"event", "create", "resource", "resource.postgres.main"})

	require.Equal(t, "event=create calling provider", msg)
	require.Equal(t, []interface{}{"resource", "resource.postgres.main"}, args)
}

func TestLeadWithEventLiftsAnEventArgumentThatIsNotTheFirstArgument(t *testing.T) {
	msg, args := leadWithEvent("changed", []interface{}{"resource", "resource.postgres.main", "event", "update", "field", "port"})

	require.Equal(t, "event=update changed", msg)
	require.Equal(t, []interface{}{"resource", "resource.postgres.main", "field", "port"}, args)
}

func TestLeadWithEventWritesOnlyTheEventForAnEmptyMessage(t *testing.T) {
	msg, args := leadWithEvent("", []interface{}{"event", "init"})

	require.Equal(t, "event=init", msg)
	require.Empty(t, args)
}

func TestLeadWithEventWritesTheDefaultEventForAMessageWithoutOne(t *testing.T) {
	msg, args := leadWithEvent("starting", []interface{}{"resource", "resource.postgres.main"})

	require.Equal(t, "event=log starting", msg)
	require.Equal(t, []interface{}{"resource", "resource.postgres.main"}, args)
}

func TestLeadWithEventWritesTheDefaultEventForAnEmptyMessageWithoutOne(t *testing.T) {
	msg, args := leadWithEvent("", nil)

	require.Equal(t, "event=log", msg)
	require.Empty(t, args)
}

func TestLeadWithEventDoesNotAddASecondEventToATaggedMessage(t *testing.T) {
	msg, args := leadWithEvent("event=create provider=postgres calling provider", []interface{}{"resource", "resource.postgres.main"})

	require.Equal(t, "event=create provider=postgres calling provider", msg)
	require.Equal(t, []interface{}{"resource", "resource.postgres.main"}, args)
}

func TestLeadWithEventKeepsATaggedEventThatIsTheWholeMessage(t *testing.T) {
	msg, args := leadWithEvent("event=init", nil)

	require.Equal(t, "event=init", msg)
	require.Empty(t, args)
}

func TestSplitEventReturnsTheEventArgumentSeparately(t *testing.T) {
	event, msg, args := splitEvent("calling provider", []interface{}{"event", "destroy", "resource", "resource.postgres.main"})

	require.Equal(t, "event=destroy", event)
	require.Equal(t, "calling provider", msg)
	require.Equal(t, []interface{}{"resource", "resource.postgres.main"}, args)
}

func TestSplitEventTakesTheEventThatLeadsATaggedMessage(t *testing.T) {
	event, msg, args := splitEvent("event=destroy provider=postgres calling provider", []interface{}{"resource", "resource.postgres.main"})

	require.Equal(t, "event=destroy", event)
	require.Equal(t, "provider=postgres calling provider", msg)
	require.Equal(t, []interface{}{"resource", "resource.postgres.main"}, args)
}

func TestSplitEventReturnsTheDefaultEventForAMessageWithoutOne(t *testing.T) {
	event, msg, args := splitEvent("starting", nil)

	require.Equal(t, "event=log", event)
	require.Equal(t, "starting", msg)
	require.Nil(t, args)
}

func TestJoinMessageSkipsEmptyParts(t *testing.T) {
	require.Equal(t, "event=log provider=postgres", joinMessage("event=log", "", "provider=postgres", ""))
}
