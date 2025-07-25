package errors

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
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
	err := strings.Builder{}
	err.WriteString("Error:\n")

	errLines := strings.Split(wordwrap.WrapString(p.Message, 80), "\n")
	for _, l := range errLines {
		err.WriteString("  " + l + "\n")
	}

	err.WriteString("\n")

	err.WriteString("  " + fmt.Sprintf("%s:%d,%d\n", p.Filename, p.Line, p.Column))
	// process the file
	file, _ := os.ReadFile(wordwrap.WrapString(p.Filename, 80))

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

// NewParserError creates a new ParserError with basic parameters
func NewParserError(filename string, line, column int, level, message string) *ParserError {
	return &ParserError{
		Filename: filename,
		Line:     line,
		Column:   column,
		Level:    level,
		Message:  message,
	}
}

// NewParserErrorFromResource creates a ParserError using metadata from a resource
func NewParserErrorFromResource(resource any, level, message string) *ParserError {
	pe := &ParserError{
		Level:   level,
		Message: message,
	}

	// Extract metadata using reflection (avoiding circular dependency with types package)
	if resource != nil {
		if meta := extractMetaFromResource(resource); meta != nil {
			pe.Filename = meta.File
			pe.Line = meta.Line
			pe.Column = meta.Column
		}
	}

	return pe
}

// Meta represents the metadata we need to extract from resources
// This mirrors the types.Meta struct but is defined here to avoid circular imports
type ResourceMeta struct {
	File   string
	Line   int
	Column int
}

// extractMetaFromResource uses reflection to extract metadata from a resource
// This replicates the logic from types.GetMeta but without the circular dependency
func extractMetaFromResource(resource any) *ResourceMeta {
	if resource == nil {
		return nil
	}

	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	if v.Kind() != reflect.Struct {
		return nil
	}

	// Look for ResourceBase field (embedded in all resources)
	resourceBaseField := v.FieldByName("ResourceBase")
	if !resourceBaseField.IsValid() {
		return nil
	}

	// Get the Meta field from ResourceBase
	metaField := resourceBaseField.FieldByName("Meta")
	if !metaField.IsValid() {
		return nil
	}

	// Extract File, Line, Column fields
	fileField := metaField.FieldByName("File")
	lineField := metaField.FieldByName("Line")
	columnField := metaField.FieldByName("Column")

	meta := &ResourceMeta{}
	if fileField.IsValid() && fileField.Kind() == reflect.String {
		meta.File = fileField.String()
	}
	if lineField.IsValid() && lineField.Kind() == reflect.Int {
		meta.Line = int(lineField.Int())
	}
	if columnField.IsValid() && columnField.Kind() == reflect.Int {
		meta.Column = int(columnField.Int())
	}

	return meta
}

// NewParserErrorFromHCLDiag creates a ParserError from HCL diagnostics
func NewParserErrorFromHCLDiag(diag *hcl.Diagnostic, filename string) *ParserError {
	line := 0
	column := 0
	if diag.Subject != nil {
		line = diag.Subject.Start.Line
		column = diag.Subject.Start.Column
	}

	return &ParserError{
		Filename: filename,
		Line:     line,
		Column:   column,
		Level:    ParserErrorLevelError,
		Message:  fmt.Sprintf("unable to parse file: %s", diag.Detail),
	}
}
