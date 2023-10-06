package cosmwasm

import (
	"context"
	"path/filepath"
	"fmt"
	"runtime"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
)

// compile will compile the specified repo using the specified docker image and version
func compile(image string, version string, repoPath string) (string, error) {
	// Set the image to pull/use
	arch := ""
	if runtime.GOARCH == "arm64" {
		arch = "-arm64"
	}
	imageFull := image + arch + ":" + version

	// Get absolute path of contract project
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	repoPathFull := filepath.Join(pwd, repoPath)

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("new client with opts: %w", err)
	}
	defer cli.Close()

	reader, err := cli.ImagePull(ctx, imageFull, types.ImagePullOptions{})
	if err != nil {
		return "", fmt.Errorf("pull image %s: %w", imageFull, err)
	}

	defer reader.Close()
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return "", fmt.Errorf("io copy %s: %w", imageFull, err)
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageFull,
		Tty:   false,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type: mount.TypeBind,
				Source: repoPathFull,
				Target: "/code",
			},
			{
				Type: mount.TypeVolume,
				Source: filepath.Base(repoPathFull)+"_cache",
				Target: "/target",
			},
			{
				Type: mount.TypeVolume,
				Source: "registry_cache",
				Target: "/usr/local/cargo/registry",
			},
		},
	}, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create container %s: %w", imageFull, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("start container %s: %w", imageFull, err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", fmt.Errorf("wait container %s: %w", imageFull, err)
		}
	case <-statusCh:
	}

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return "", fmt.Errorf("logs container %s: %w", imageFull, err)
	}

	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	if err != nil {
		return "", fmt.Errorf("std copy %s: %w", imageFull, err)
	}

	err = cli.ContainerStop(ctx, resp.ID, container.StopOptions{})
	if err != nil {
		// Only return the error if it didn't match an already stopped, or a missing container.
		if !(errdefs.IsNotModified(err) || errdefs.IsNotFound(err)) {
			return "", fmt.Errorf("stop container %s: %w", imageFull, err)
		}
	}

	err = cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !errdefs.IsNotFound(err) {
		return "", fmt.Errorf("remove container %s: %w", imageFull, err)
	}

	return repoPathFull, nil
}