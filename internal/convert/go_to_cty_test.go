package convert

import (
	"testing"

	"github.com/jumppad-labs/hclconfig/internal/test_fixtures/plugin/structs"
	"github.com/stretchr/testify/require"
)

func TestGoStructToCtyValue(t *testing.T) {
	cont := structs.Container{
		ContainerBase: structs.ContainerBase{
			Command: []string{"ls", "-las"},
		},
	}

	//val := reflect.ValueOf(cont)
	_, err := GoToCtyValue(&cont)

	require.NoError(t, err)
}
