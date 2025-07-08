package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigResource(t *testing.T) {
	c := &Config{
		Version: "1.0.0",
	}

	require.Equal(t, "1.0.0", c.Version)
}
