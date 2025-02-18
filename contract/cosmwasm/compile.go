package cosmwasm

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/api/types/container"
	dockerimagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/hashicorp/go-version"
	"github.com/moby/moby/client"
	"github.com/moby/moby/errdefs"
	"github.com/moby/moby/pkg/stdcopy"
)

// compile will compile the specified repo using the specified docker image and version.
func compile(image string, optVersion string, repoPath string) (string, error) {
	// Set the image to pull/use
	arch := ""
	if runtime.GOARCH == "arm64" {
		arch = "-arm64"
	}
	imageFull := image + arch + ":" + optVersion

	// Check if version is less than 0.13.0, if so, use old cache directory
	cacheDir := "/target"
	versionThresh, err := version.NewVersion("0.13.0")
	if err != nil {
		return "", fmt.Errorf("version threshold 0.13.0: %w", err)
	}
	myVersion, err := version.NewVersion(optVersion)
	if err != nil {
		return "", fmt.Errorf("version %s: %w", optVersion, err)
	}
	if myVersion.LessThan(versionThresh) {
		cacheDir = "/code/target"
	}

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

	reader, err := cli.ImagePull(ctx, imageFull, dockerimagetypes.PullOptions{})
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
				Type:   mount.TypeBind,
				Source: repoPathFull,
				Target: "/code",
			},
			{
				Type:   mount.TypeVolume,
				Source: filepath.Base(repoPathFull) + "_cache",
				Target: cacheDir,
			},
			{
				Type:   mount.TypeVolume,
				Source: "registry_cache",
				Target: "/usr/local/cargo/registry",
			},
		},
	}, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create container %s: %w", imageFull, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
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

	out, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true})
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

	err = cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !errdefs.IsNotFound(err) {
		return "", fmt.Errorf("remove container %s: %w", imageFull, err)
	}

	return repoPathFull, nil
}
