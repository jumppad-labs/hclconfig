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

func TestWithTagStartsEveryMessageWithTheDefaultEventThenTheTag(t *testing.T) {
	r := &callRecorder{}
	l := WithTag(r, "provider", "postgres")

	l.Info("create", "id", "resource.postgres.main")
	l.Debug("read")
	l.Warn("update")
	l.Error("destroy")

	require.Equal(t, []loggedCall{
		{"info", "event=log provider=postgres create", []interface{}{"id", "resource.postgres.main"}},
		{"debug", "event=log provider=postgres read", nil},
		{"warn", "event=log provider=postgres update", nil},
		{"error", "event=log provider=postgres destroy", nil},
	}, r.calls)
}

func TestWithTagWritesOnlyTheEventAndTagForAnEmptyMessage(t *testing.T) {
	r := &callRecorder{}
	l := WithTag(r, "plugin", "example")

	l.Info("", "id", "x")

	require.Equal(t, []loggedCall{
		{"info", "event=log plugin=example", []interface{}{"id", "x"}},
	}, r.calls)
}

func TestWithTagNestedWritesOutermostTagFirst(t *testing.T) {
	r := &callRecorder{}
	pluginLogger := WithTag(r, "plugin", "example")
	providerLogger := WithTag(pluginLogger, "provider", "postgres")

	providerLogger.Info("create")

	require.Equal(t, []loggedCall{
		{"info", "event=log plugin=example provider=postgres create", nil},
	}, r.calls)
}

func TestWithTagMovesTheEventArgumentToLeadTheMessage(t *testing.T) {
	r := &callRecorder{}
	l := WithTag(r, "provider", "postgres")

	l.Info("calling provider", "event", "create", "resource", "resource.postgres.main")

	require.Equal(t, []loggedCall{
		{"info", "event=create provider=postgres calling provider", []interface{}{"resource", "resource.postgres.main"}},
	}, r.calls)
}

func TestWithTagRemovesTheEventArgumentWhenItIsTheOnlyArgument(t *testing.T) {
	r := &callRecorder{}
	l := WithTag(r, "provider", "postgres")

	l.Debug("", "event", "init")

	require.Len(t, r.calls, 1)
	require.Equal(t, "debug", r.calls[0].level)
	require.Equal(t, "event=init provider=postgres", r.calls[0].msg)
	require.Empty(t, r.calls[0].args)
}

func TestWithTagLiftsAnEventArgumentThatIsNotTheFirstArgument(t *testing.T) {
	r := &callRecorder{}
	l := WithTag(r, "provider", "postgres")

	l.Warn("changed", "resource", "resource.postgres.main", "event", "update", "field", "port")

	require.Equal(t, []loggedCall{
		{"warn", "event=update provider=postgres changed", []interface{}{"resource", "resource.postgres.main", "field", "port"}},
	}, r.calls)
}

func TestWithTagWritesTheDefaultEventForAMessageWithoutOne(t *testing.T) {
	r := &callRecorder{}
	l := WithTag(r, "provider", "postgres")

	l.Error("destroy failed", "resource", "resource.postgres.main")

	require.Equal(t, []loggedCall{
		{"error", "event=log provider=postgres destroy failed", []interface{}{"resource", "resource.postgres.main"}},
	}, r.calls)
}

func TestWithTagNestedKeepsTheEventBeforeEveryTag(t *testing.T) {
	r := &callRecorder{}
	pluginLogger := WithTag(r, "plugin", "example")
	providerLogger := WithTag(pluginLogger, "provider", "postgres")

	providerLogger.Info("calling provider", "event", "create", "resource", "resource.postgres.main")

	require.Equal(t, []loggedCall{
		{"info", "event=create plugin=example provider=postgres calling provider", []interface{}{"resource", "resource.postgres.main"}},
	}, r.calls)
}

func TestWithTagNestedWritesOnlyOneEventForAMessageWithoutOne(t *testing.T) {
	r := &callRecorder{}
	pluginLogger := WithTag(r, "plugin", "example")
	providerLogger := WithTag(pluginLogger, "provider", "postgres")

	providerLogger.Info("create")

	require.Len(t, r.calls, 1)
	require.Equal(t, "event=log plugin=example provider=postgres create", r.calls[0].msg)
}

func TestWithTagReturnsNilForNilLogger(t *testing.T) {
	require.Nil(t, WithTag(nil, "plugin", "example"))
}
