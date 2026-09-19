package plugins

import (
	"io"
	"log"
	"strings"

	"github.com/hashicorp/go-hclog"
)

// hclogAdapter lets go-plugin, which starts external plugins and connects to
// them, log through xcl's Logger instead of its own default logger, which
// writes straight to stderr. go-plugin logs how it starts and talks to the
// plugin process, and passes on what the plugin process writes to stderr.
//
// Trace messages are dropped, they only describe go-plugin's stdio plumbing.
// Debug and Info are passed on at Debug: they describe how the plugin process
// is run, which in-process plugins have no equivalent of, so like the rest of
// xcl's plugin plumbing they are debug output. Warn and Error are passed on at
// the same level, they report a problem.
//
// go-plugin's debug message for the end of the plugin's stdio stream is
// dropped too, see buriedMessages.
type hclogAdapter struct {
	logger Logger
	name   string
	args   []interface{}
	level  hclog.Level
}

// buriedMessages are go-plugin debug messages that describe normal behaviour
// but read like a failure, so they are not passed on.
var buriedMessages = map[string]bool{
	// Logged when the plugin's stdio stream ends, which it always does when the
	// plugin process stops. go-plugin logs it with the reason the stream ended,
	// i.e. err="rpc error: code = Unavailable desc = error reading from
	// server: EOF", which is not an error. go-plugin treats EOF, Unavailable
	// and Canceled all as the stream ending normally.
	"received EOF, stopping recv loop": true,
}

// Ensure hclogAdapter implements the hclog.Logger interface
var _ hclog.Logger = (*hclogAdapter)(nil)

// newHCLogAdapter returns an hclog.Logger that writes to l. A nil l returns a
// logger that discards everything.
func newHCLogAdapter(l Logger) hclog.Logger {
	if l == nil {
		return hclog.NewNullLogger()
	}

	return &hclogAdapter{logger: l, level: hclog.Debug}
}

// Log writes msg at level, adding any implied args from With
func (a *hclogAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	if level < a.level {
		return
	}

	if level <= hclog.Debug && buriedMessages[msg] {
		return
	}

	if len(a.args) > 0 {
		args = append(append([]interface{}{}, a.args...), args...)
	}

	// every line leads with an event, go-plugin's own logs are one kind
	args = append([]interface{}{"event", "go-plugin"}, args...)

	switch {
	case level >= hclog.Error:
		a.logger.Error(msg, args...)
	case level == hclog.Warn:
		a.logger.Warn(msg, args...)
	default:
		a.logger.Debug(msg, args...)
	}
}

func (a *hclogAdapter) Trace(msg string, args ...interface{}) { a.Log(hclog.Trace, msg, args...) }
func (a *hclogAdapter) Debug(msg string, args ...interface{}) { a.Log(hclog.Debug, msg, args...) }
func (a *hclogAdapter) Info(msg string, args ...interface{})  { a.Log(hclog.Info, msg, args...) }
func (a *hclogAdapter) Warn(msg string, args ...interface{})  { a.Log(hclog.Warn, msg, args...) }
func (a *hclogAdapter) Error(msg string, args ...interface{}) { a.Log(hclog.Error, msg, args...) }

func (a *hclogAdapter) IsTrace() bool { return a.level <= hclog.Trace }
func (a *hclogAdapter) IsDebug() bool { return a.level <= hclog.Debug }
func (a *hclogAdapter) IsInfo() bool  { return a.level <= hclog.Info }
func (a *hclogAdapter) IsWarn() bool  { return a.level <= hclog.Warn }
func (a *hclogAdapter) IsError() bool { return a.level <= hclog.Error }

// ImpliedArgs returns the args added with With
func (a *hclogAdapter) ImpliedArgs() []interface{} {
	return a.args
}

// With returns a logger that adds args to every message
func (a *hclogAdapter) With(args ...interface{}) hclog.Logger {
	c := *a
	c.args = append(append([]interface{}{}, a.args...), args...)
	return &c
}

// Name returns the logger's name. go-plugin names its loggers after what
// they log for, the messages are already tagged with the plugin by xcl so the
// name is not written.
func (a *hclogAdapter) Name() string {
	return a.name
}

// Named returns a logger whose name is name appended to this logger's name
func (a *hclogAdapter) Named(name string) hclog.Logger {
	c := *a
	if c.name == "" {
		c.name = name
	} else {
		c.name = c.name + "." + name
	}

	return &c
}

// ResetNamed returns a logger named name
func (a *hclogAdapter) ResetNamed(name string) hclog.Logger {
	c := *a
	c.name = name
	return &c
}

// SetLevel sets the lowest level that is passed on
func (a *hclogAdapter) SetLevel(level hclog.Level) {
	a.level = level
}

// StandardLogger returns a standard library logger that writes each line to
// this logger at Info, which is passed on at Debug
func (a *hclogAdapter) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(a.StandardWriter(opts), "", 0)
}

// StandardWriter returns a writer that writes each line to this logger at
// Info, which is passed on at Debug
func (a *hclogAdapter) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return &hclogLineWriter{adapter: a}
}

// hclogLineWriter writes each line written to it as an Info message
type hclogLineWriter struct {
	adapter *hclogAdapter
}

func (w *hclogLineWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line != "" {
			w.adapter.Info(line)
		}
	}

	return len(p), nil
}
