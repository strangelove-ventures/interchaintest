package dockerutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestStartContainer(t *testing.T) {
	ctx := context.Background()

	pool, network := DockerSetup(t)

	t.Run("happy path", func(t *testing.T) {
		opts := StartOptions{
			Repository:    "busybox",
			Cmd:           []string{"sleep", "100"},
			ContainerName: t.Name(),
			HostName:      t.Name(),
		}
		c, err := StartContainer(ctx, zap.NewNop(), pool, network, opts)
		require.NoError(t, err)

		require.NoError(t, c.resource.Close()) // Expose
	})

}
