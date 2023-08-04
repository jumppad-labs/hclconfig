package convert

import (
	"testing"

	"github.com/jumppad-labs/hclconfig/test_fixtures/structs"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

func TestCtyValue(t *testing.T) {
	cont := structs.Container{
		Command: []string{"ls", "-las"},
	}

	//val := reflect.ValueOf(cont)
	cty, err := GoToCtyValue(&cont)

	pretty.Println(cty)
	require.NoError(t, err)
}
