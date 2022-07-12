package dockerutil_test

import (
	"context"
	"testing"

	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestFileRetriever(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to short mode")
	}

	t.Parallel()

	cli, network := ibctest.DockerSetup(t)

	ctx := context.Background()
	v, err := cli.VolumeCreate(ctx, volumetypes.VolumeCreateBody{
		Labels: map[string]string{dockerutil.CleanupLabel: t.Name()},
	})
	require.NoError(t, err)

	img := dockerutil.NewImage(
		zaptest.NewLogger(t),
		cli,
		network,
		t.Name(),
		"busybox", "stable",
	)

	_, _, err = img.Run(
		ctx,
		[]string{"sh", "-c", "chmod 0700 /mnt/test && printf 'hello world' > /mnt/test/hello.txt"},
		dockerutil.ContainerOptions{
			Binds: []string{v.Name + ":/mnt/test"},
		},
	)
	require.NoError(t, err)
	_, _, err = img.Run(
		ctx,
		[]string{"sh", "-c", "mkdir -p /mnt/test/foo/bar/ && printf 'test' > /mnt/test/foo/bar/baz.txt"},
		dockerutil.ContainerOptions{
			Binds: []string{v.Name + ":/mnt/test"},
		},
	)
	require.NoError(t, err)

	fr := dockerutil.NewFileRetriever(zaptest.NewLogger(t), cli, t.Name())

	t.Run("top-level file", func(t *testing.T) {
		b, err := fr.SingleFileContent(ctx, v.Name, "hello.txt")
		require.NoError(t, err)
		require.Equal(t, string(b), "hello world")
	})

	t.Run("nested file", func(t *testing.T) {
		b, err := fr.SingleFileContent(ctx, v.Name, "foo/bar/baz.txt")
		require.NoError(t, err)
		require.Equal(t, string(b), "test")
	})
}
