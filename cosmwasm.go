package interchaintest

import (
	"context"
	"path/filepath"
	"fmt"
	"runtime"
	"io"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
)

type (
	Contract struct {
		DockerImage string
		Version string
		RelativePath string
	}
	cargoToml struct {
		Package packageBlock `toml:"package"`
	}
	packageBlock struct {
		Name string `toml:"name"`
	}
)

func NewContract(relativePath string) *Contract {
	return &Contract{
		DockerImage: "cosmwasm/rust-optimizer",
		Version: "0.14.0",
		RelativePath: relativePath,
	}
}

func (c *Contract) WithDockerImage(image string) *Contract {
	c.DockerImage = image
	return c
}

func (c *Contract) WithVersion(version string) *Contract {
	c.Version = version
	return c
}

// CompileCwContract takes a relative path input for the contract to compile
// CosmWasm's rust-optimizer is used for compilation
// Successful compilation will return the absolute path of the new binary
// - contractPath is the relative path of the contract project on local machine
func (c *Contract) Compile() (string, error) {
	// Set the image to pull/use
	arch := ""
	if runtime.GOARCH == "arm64" {
		arch = "-arm64"
	}
	image := c.DockerImage + arch + ":" + c.Version

	// Get absolute path of contract project
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	contractPath := filepath.Join(pwd, c.RelativePath)

	// Check that Cargo.toml is found
	cargoTomlPath := filepath.Join(contractPath, "Cargo.toml")
	if _, err := os.Stat(cargoTomlPath); err != nil {
		return "", fmt.Errorf("cargo toml not found: %w", err)
	}

	// Get the contract package name
	var cargoToml cargoToml
	_, err = toml.DecodeFile(cargoTomlPath, &cargoToml)
	contractPackageName := cargoToml.Package.Name

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("new client with opts: %w", err)
	}
	defer cli.Close()

	reader, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return "", fmt.Errorf("pull image %s: %w", image, err)
	}

	defer reader.Close()
	io.Copy(os.Stdout, reader)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Tty:   false,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type: mount.TypeBind,
				Source: contractPath,
				Target: "/code",
			},
			{
				Type: mount.TypeVolume,
				Source: contractPackageName+"_cache",
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
		return "", fmt.Errorf("create container %s: %w", image, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("start container %s: %w", image, err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", fmt.Errorf("wait container %s: %w", image, err)
		}
	case <-statusCh:
	}

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return "", fmt.Errorf("logs container %s: %w", image, err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)

	err = cli.ContainerStop(ctx, resp.ID, container.StopOptions{})
	if err != nil {
		// Only return the error if it didn't match an already stopped, or a missing container.
		if !(errdefs.IsNotModified(err) || errdefs.IsNotFound(err)) {
			return "", fmt.Errorf("stop container %s: %w", image, err)
		}
	}

	err = cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !errdefs.IsNotFound(err) {
		return "", fmt.Errorf("remove container %s: %w", image, err)
	}

	// Form the path to the artifacts directory, used for checksum.txt and package.wasm
	artifactsPath := filepath.Join(contractPath, "artifacts")

	// Parse the checksums.txt for the wasm binary name
	checksumsPath := filepath.Join(artifactsPath, "checksums.txt")
	checksumsBz, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", fmt.Errorf("checksums read: %w", err)
	}
	_, wasmBin, found := strings.Cut(string(checksumsBz), "  ")
	if !found {
		return "", fmt.Errorf("wasm binary name not found")
	}

	// Form the path to the wasm binary
	wasmBinPath := filepath.Join(artifactsPath, strings.TrimSpace(wasmBin))
	return wasmBinPath, nil
  }