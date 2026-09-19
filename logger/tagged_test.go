package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type loggedCall struct {
	level string
	msg   string
	args  []interface{}
}

// callRecorder records every call made to it
type callRecorder struct {
	calls []loggedCall
}

func (r *callRecorder) Info(msg string, args ...interface{}) {
	r.calls = append(r.calls, loggedCall{"info", msg, args})
}

func (r *callRecorder) Debug(msg string, args ...interface{}) {
	r.calls = append(r.calls, loggedCall{"debug", msg, args})
}

func (r *callRecorder) Warn(msg string, args ...interface{}) {
	r.calls = append(r.calls, loggedCall{"warn", msg, args})
}

func (r *callRecorder) Error(msg string, args ...interface{}) {
	r.calls = append(r.calls, loggedCall{"error", msg, args})
}

func TestWithTagStartsEveryMessageWithTheTag(t *testing.T) {
	r := &callRecorder{}
	l := WithTag(r, "provider", "postgres")

	l.Info("create", "id", "resource.postgres.main")
	l.Debug("read")
	l.Warn("update")
	l.Error("destroy")

	require.Equal(t, []loggedCall{
		{"info", "provider=postgres create", []interface{}{"id", "resource.postgres.main"}},
		{"debug", "provider=postgres read", nil},
		{"warn", "provider=postgres update", nil},
		{"error", "provider=postgres destroy", nil},
	}, r.calls)
}

func TestWithTagWritesOnlyTheTagForAnEmptyMessage(t *testing.T) {
	r := &callRecorder{}
	l := WithTag(r, "plugin", "example")

	l.Info("", "id", "x")

	require.Equal(t, []loggedCall{
		{"info", "plugin=example", []interface{}{"id", "x"}},
	}, r.calls)
}

func TestWithTagNestedWritesOutermostTagFirst(t *testing.T) {
	r := &callRecorder{}
	pluginLogger := WithTag(r, "plugin", "example")
	providerLogger := WithTag(pluginLogger, "provider", "postgres")

	providerLogger.Info("create")

	require.Equal(t, []loggedCall{
		{"info", "plugin=example provider=postgres create", nil},
	}, r.calls)
}

func TestWithTagReturnsNilForNilLogger(t *testing.T) {
	require.Nil(t, WithTag(nil, "plugin", "example"))
}
