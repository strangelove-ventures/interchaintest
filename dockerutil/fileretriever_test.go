package dockerutil_test

import (
	"context"
	dockerutil2 "github.com/strangelove-ventures/ibctest/v6/dockerutil"
	"testing"

	volumetypes "github.com/docker/docker/api/types/volume"
	ibctest "github.com/strangelove-ventures/ibctest/v6"
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
		Labels: map[string]string{dockerutil2.CleanupLabel: t.Name()},
	})
	require.NoError(t, err)

	img := dockerutil2.NewImage(
		zaptest.NewLogger(t),
		cli,
		network,
		t.Name(),
		"busybox", "stable",
	)

	res := img.Run(
		ctx,
		[]string{"sh", "-c", "chmod 0700 /mnt/test && printf 'hello world' > /mnt/test/hello.txt"},
		dockerutil2.ContainerOptions{
			Binds: []string{v.Name + ":/mnt/test"},
			User:  dockerutil2.GetRootUserString(),
		},
	)
	require.NoError(t, res.Err)
	res = img.Run(
		ctx,
		[]string{"sh", "-c", "mkdir -p /mnt/test/foo/bar/ && printf 'test' > /mnt/test/foo/bar/baz.txt"},
		dockerutil2.ContainerOptions{
			Binds: []string{v.Name + ":/mnt/test"},
			User:  dockerutil2.GetRootUserString(),
		},
	)
	require.NoError(t, res.Err)

	fr := dockerutil2.NewFileRetriever(zaptest.NewLogger(t), cli, t.Name())

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
