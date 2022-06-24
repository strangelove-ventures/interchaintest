package dockerutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerSetup(t *testing.T) {
	pool := DockerSetup(t)

	require.NotNil(t, pool.Pool())
	require.NotEmpty(t, pool.NetworkID())
}
