package errors

import (
	"fmt"
	"io/ioutil"
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
func (p ParserError) Error() string {
	err := strings.Builder{}
	err.WriteString("Error:\n")

	errLines := strings.Split(wordwrap.WrapString(p.Message, 80), "\n")
	for _, l := range errLines {
		err.WriteString("  " + l + "\n")
	}

	err.WriteString("\n")

	err.WriteString("  " + fmt.Sprintf("%s:%d,%d\n", p.Filename, p.Line, p.Column))
	// process the file
	file, _ := ioutil.ReadFile(wordwrap.WrapString(p.Filename, 80))

	lines := strings.Split(string(file), "\n")

	startLine := p.Line - 3
	if startLine < 0 {
		startLine = 0
	}

	endLine := p.Line + 2
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	for i := startLine; i < endLine; i++ {
		codeline := wordwrap.WrapString(lines[i], 70)
		codelines := strings.Split(codeline, "\n")

		if i == p.Line-1 {
			err.WriteString(fmt.Sprintf("\033[1m  %5d | %s\033[0m\n", i+1, codelines[0]))
		} else {
			err.WriteString(fmt.Sprintf("\033[2m  %5d | %s\033[0m\n", i+1, codelines[0]))
		}

		for _, l := range codelines[1:] {
			if i == p.Line-1 {
				err.WriteString(fmt.Sprintf("\033[1m        : %s\033[0m\n", l))
			} else {
				err.WriteString(fmt.Sprintf("\033[2m        : %s\033[0m\n", l))
			}
		}
	}

	return err.String()
}
