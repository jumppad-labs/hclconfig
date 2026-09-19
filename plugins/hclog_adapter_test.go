package plugins

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

type adapterCall struct {
	level string
	msg   string
	args  []interface{}
}

// adapterRecorder records every call made to it
type adapterRecorder struct {
	calls []adapterCall
}

func (r *adapterRecorder) Info(msg string, args ...interface{}) {
	r.calls = append(r.calls, adapterCall{"info", msg, args})
}

func (r *adapterRecorder) Debug(msg string, args ...interface{}) {
	r.calls = append(r.calls, adapterCall{"debug", msg, args})
}

func (r *adapterRecorder) Warn(msg string, args ...interface{}) {
	r.calls = append(r.calls, adapterCall{"warn", msg, args})
}

func (r *adapterRecorder) Error(msg string, args ...interface{}) {
	r.calls = append(r.calls, adapterCall{"error", msg, args})
}

func TestHCLogAdapterPassesOnDebugAndInfoAtDebugAndWarnAndErrorAtTheirLevel(t *testing.T) {
	r := &adapterRecorder{}
	l := newHCLogAdapter(r)

	l.Debug("starting plugin", "path", "build/external")
	l.Info("plugin process exited")
	l.Warn("plugin slow")
	l.Error("plugin failed", "error", "boom")

	require.Equal(t, []adapterCall{
		{"debug", "starting plugin", []interface{}{"path", "build/external"}},
		{"debug", "plugin process exited", nil},
		{"warn", "plugin slow", nil},
		{"error", "plugin failed", []interface{}{"error", "boom"}},
	}, r.calls)
}

func TestHCLogAdapterDropsTrace(t *testing.T) {
	r := &adapterRecorder{}
	l := newHCLogAdapter(r)

	l.Trace("waiting for stdio data")
	l.Log(hclog.Trace, "waiting for stdio data")

	require.Empty(t, r.calls)
	require.False(t, l.IsTrace())
	require.True(t, l.IsDebug())
}

func TestHCLogAdapterLogPassesOnAtTheGivenLevel(t *testing.T) {
	r := &adapterRecorder{}
	l := newHCLogAdapter(r)

	l.Log(hclog.Debug, "plugin address")
	l.Log(hclog.Info, "using plugin")
	l.Log(hclog.Warn, "plugin slow")
	l.Log(hclog.Error, "plugin failed")

	require.Equal(t, []adapterCall{
		{"debug", "plugin address", nil},
		{"debug", "using plugin", nil},
		{"warn", "plugin slow", nil},
		{"error", "plugin failed", nil},
	}, r.calls)
}

func TestHCLogAdapterWithAddsImpliedArgs(t *testing.T) {
	r := &adapterRecorder{}
	l := newHCLogAdapter(r).With("pid", 123).Named("external")

	l.Info("plugin started", "path", "build/external")

	require.Equal(t, []interface{}{"pid", 123}, l.ImpliedArgs())
	require.Equal(t, []adapterCall{
		{"debug", "plugin started", []interface{}{"pid", 123, "path", "build/external"}},
	}, r.calls)
}

func TestHCLogAdapterNamedAppendsToTheName(t *testing.T) {
	l := newHCLogAdapter(&adapterRecorder{}).Named("plugin").Named("stdio")

	require.Equal(t, "plugin.stdio", l.Name())
	require.Equal(t, "external", l.ResetNamed("external").Name())
}

func TestHCLogAdapterStandardWriterWritesEachLineAtDebug(t *testing.T) {
	r := &adapterRecorder{}
	l := newHCLogAdapter(r)

	_, err := l.StandardWriter(nil).Write([]byte("first line\nsecond line\n"))
	require.NoError(t, err)

	require.Equal(t, []adapterCall{
		{"debug", "first line", nil},
		{"debug", "second line", nil},
	}, r.calls)
}

func TestHCLogAdapterForNilLoggerDiscards(t *testing.T) {
	l := newHCLogAdapter(nil)

	require.NotPanics(t, func() {
		l.Error("plugin failed")
	})
}

func TestHCLogAdapterBuriesEndOfStdioStream(t *testing.T) {
	r := &adapterRecorder{}
	l := newHCLogAdapter(r)

	l.Debug("received EOF, stopping recv loop", "err", "rpc error: code = Unavailable desc = error reading from server: EOF")

	require.Empty(t, r.calls)
}

func TestHCLogAdapterPassesOnEndOfStdioStreamAboveDebug(t *testing.T) {
	r := &adapterRecorder{}
	l := newHCLogAdapter(r)

	l.Error("received EOF, stopping recv loop")

	require.Equal(t, []adapterCall{
		{"error", "received EOF, stopping recv loop", nil},
	}, r.calls)
}
