package test

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
)

type Logger struct {
	options LoggerOptions
}

type LoggerOptions struct {
	Width  int
	Writer io.Writer
}

func DefaultOptions() LoggerOptions {
	return LoggerOptions{
		Width:  100,
		Writer: os.Stdout,
	}
}

func NewLogger(options LoggerOptions) *Logger {
	return &Logger{options}
}

func (l *Logger) LogScenario(d string) {
	var style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		PaddingTop(2).
		PaddingLeft(0).
		Width(l.options.Width)

	l.Log(fmt.Sprintf("SCENARIO: '%s'", d), style)
}

func (l *Logger) LogIt(d string) {
	var style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#B9E3CE")).
		PaddingTop(1).
		PaddingLeft(2).
		Width(l.options.Width)

	l.Log(fmt.Sprintf("IT: '%s'", d), style)
}

func (l *Logger) LogFunctionDescription(d string) {
	var style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		PaddingTop(0).
		PaddingLeft(8).
		Width(l.options.Width)

	l.Log(d, style)
}

func (l *Logger) LogFunctionLine(d string) {
	var style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3F48CC")).
		PaddingTop(0).
		PaddingLeft(10).
		Width(l.options.Width)

	l.Log(d, style)
}

func (l *Logger) LogError(d string) {
	var style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EC1C24")).
		PaddingTop(1).
		PaddingLeft(0).
		Width(l.options.Width)

	l.Log(d, style)
}

func (l *Logger) Log(s string, style lipgloss.Style) {
	fmt.Fprintf(l.options.Writer, "%s\n", style.Render(s))
}
