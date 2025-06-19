package functionaltest

import (
	"testing"

	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/stretchr/testify/require"
)

func TestStuff(t *testing.T) {
	l := &plugins.TestLogger{}
	ph := plugins.NewPluginHost(l, nil, "")

	err := ph.Start("../example/bin/example")
	require.NoError(t, err, "Plugin should start without error")

	err = ph.Ping()
	require.Error(t, err, "Ping should succeed")
}
