package errors

import "strings"

// ConfigError defines an error that was encountered while parsing the config
type ConfigError struct {
	// Errors is a list of errors that were encountered while processing the
	// config which includes calling the process function on the resource or any
	// default callbacks
	// Errors may be marked as warnings or errors
	Errors []error
}

func NewConfigError() *ConfigError {
	return &ConfigError{
		Errors: []error{},
	}
}

// AppendError adds a new parse error to the list of errors
func (p *ConfigError) AppendError(err error) {
	p.Errors = append(p.Errors, err)
}

// ContainsWarnings returns true if any of the errors are warnings
func (p *ConfigError) ContainsWarnings() bool {
	for _, e := range p.Errors {
		if pe, ok := e.(*ParserError); ok && pe.Level == ParserErrorLevelWarning {
			return true
		}
	}

	return false
}

// ContainsErrors returns true if any of the errors are errors
func (p *ConfigError) ContainsErrors() bool {
	for _, e := range p.Errors {
		pe, ok := e.(*ParserError)
		// if not a parser error is a standard error
		if !ok {
			return true
		}

		if pe.Level == ParserErrorLevelError {
			return true
		}

	}

	return false
}

// Error pretty prints the error message as a string
func (p *ConfigError) Error() string {
	err := strings.Builder{}

	for _, e := range p.Errors {
		err.WriteString(e.Error() + "\n")
	}

	return strings.TrimSuffix(err.String(), "\n")
}
