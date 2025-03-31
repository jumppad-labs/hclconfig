package errors

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mitchellh/go-wordwrap"
)

const ParserErrorLevelError = "error"
const ParserErrorLevelWarning = "warning"

// ParserError is a detailed error that is returned from the parser
type ParserError struct {
	Filename string
	Line     int
	Column   int
	Details  string
	Message  string
	Level    string
}

// Error pretty prints the error message as a string
func (p *ParserError) Error() string {
	if p.Level == "" {
		p.Level = ParserErrorLevelError
	}

	err := strings.Builder{}
	err.WriteString(p.Level + ":\n")

	errLines := strings.Split(wordwrap.WrapString(p.Message, 80), "\n")
	for _, l := range errLines {
		err.WriteString("  " + l + "\n")
	}

	if p.Filename != "" {
		err.WriteString("  " + p.Filename + "\n")
	}

	// <error>: <filename>:<line>,<from-column>-<to-column>: <error details>
	// e.g. `unable to decode body: /exec_module/module/module.hcl:9,3-8: Unsupported argument;
	// An argument named "image" is not expected here., and 1 other diagnostic(s)`
	msgRegex, _ := regexp.Compile(`^(?P<error>.*): (?P<file>.*):(?P<line>\d+),(?P<start>\d+)-(?P<end>\d+): (?P<details>.*)$`)
	matches := msgRegex.FindStringSubmatch(p.Message)

	errFile := p.Filename
	errLine := p.Line
	errStart := p.Column
	errEnd := p.Column

	if len(matches) > 0 {
		parts := make(map[string]string)
		for i, name := range msgRegex.SubexpNames() {
			if i != 0 && name != "" {
				parts[name] = matches[i]
			}
		}

		// We can use this later to display the error differently if we want to.
		// errMsg := parts["error"]
		errFile = parts["file"]
		errLine, _ = strconv.Atoi(parts["line"])
		errStart, _ = strconv.Atoi(parts["start"])
		errEnd, _ = strconv.Atoi(parts["end"])
	}

	err.WriteString("\n")
	err.WriteString("  " + fmt.Sprintf("%s:%d,%d-%d\n", errFile, errLine, errStart, errEnd))

	// process the file
	file, _ := os.ReadFile(wordwrap.WrapString(errFile, 80))

	lines := strings.Split(string(file), "\n")

	startLine := errLine - 3
	if startLine < 0 {
		startLine = 0
	}

	endLine := errLine + 2
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	for i := startLine; i < endLine; i++ {
		codeline := wordwrap.WrapString(lines[i], 70)
		codelines := strings.Split(codeline, "\n")

		if i == errLine-1 {
			err.WriteString(fmt.Sprintf("\033[1m  %5d | %s\033[0m\n", i+1, codelines[0]))
		} else {
			err.WriteString(fmt.Sprintf("\033[2m  %5d | %s\033[0m\n", i+1, codelines[0]))
		}

		for _, l := range codelines[1:] {
			if i == errLine-1 {
				err.WriteString(fmt.Sprintf("\033[1m        : %s\033[0m\n", l))
			} else {
				err.WriteString(fmt.Sprintf("\033[2m        : %s\033[0m\n", l))
			}
		}
	}

	return err.String()
}
