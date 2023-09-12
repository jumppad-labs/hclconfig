package errors

import "strings"

// ConfigError defines an error that was encountered while parsing the config
type ConfigError struct {
	// ParseErrors is a list of errors that were encountered while reading the config
	// from the text file
	ParseErrors []error

	// ProcessErrors is a list of errors that were encountered while processing the
	// config which includes calling the process function on the resource or any
	// default callbacks
	ProcessErrors []error
}

func NewConfigError() *ConfigError {
	return &ConfigError{
		ParseErrors:   []error{},
		ProcessErrors: []error{},
	}
}

// AppendParseError adds a new parse error to the list of errors
func (p *ConfigError) AppendParseError(err error) {
	p.ParseErrors = append(p.ParseErrors, err)
}

// AppendProcessError adds a new process error to the list of errors
func (p *ConfigError) AppendProcessError(err error) {
	p.ProcessErrors = append(p.ProcessErrors, err)
}

// Error pretty prints the error message as a string
func (p *ConfigError) Error() string {
	err := strings.Builder{}

	for _, e := range p.ParseErrors {
		err.WriteString(e.Error() + "\n")
	}

	for _, e := range p.ProcessErrors {
		err.WriteString(e.Error() + "\n")
	}

	return strings.TrimSuffix(err.String(), "\n")
}
