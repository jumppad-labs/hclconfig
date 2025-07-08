package convert

import (
	"testing"

	"github.com/instruqt/hclconfig/test_fixtures/structs"
	"github.com/stretchr/testify/require"
)

func TestGoStructToCtyValue(t *testing.T) {
	cont := structs.Container{
		Command: []string{"ls", "-las"},
	}

	//val := reflect.ValueOf(cont)
	_, err := GoToCtyValue(&cont)

	require.NoError(t, err)
}
