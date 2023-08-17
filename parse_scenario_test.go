package hclconfig

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockTestFunction struct {
	name        string
	description string
	funcType    string
	function    interface{}
	mock.Mock
}

func (mf *mockTestFunction) CallFunction(args ...interface{}) {

}

func newMockTestFunction(t *testing.T, name, description, funcType string) *mockTestFunction {
	mtf := &mockTestFunction{name: name, description: description, funcType: funcType}
	mtf.On("CallFunction", mock.Anything)

	mtf.function = func(ctx context.Context, l TestLogger, args ...interface{}) {
		mtf.CallFunction(args)
	}

	return mtf
}

func setupParserTests(t *testing.T) *Parser {
	p := setupParser(t)

	// register the test functions
	tf := newMockTestFunction(t, "navigate", "the browser navigates to", TestFuncCommand)
	p.RegisterTestFunction(tf.name, tf.description, tf.funcType, tf.function)

	return p
}

func TestParseTestReturnsWithoutError(t *testing.T) {
	p := setupParserTests(t)

	out := bytes.NewBufferString("")

	c, err := p.TestConfig("./test_fixtures/tests/test", out)
	require.NoError(t, err)
	require.NotNil(t, c)
}
