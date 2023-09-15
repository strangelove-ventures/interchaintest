package cosmwasm

import (
	"path/filepath"
	"fmt"
	"os"
	"strings"
)

type Workspace struct {
	DockerImage string
	Version string
	RelativePath string
}

func NewWorkspace(relativePath string) *Workspace {
	return &Workspace{
		DockerImage: "cosmwasm/workspace-optimizer",
		Version: "0.14.0",
		RelativePath: relativePath,
	}
}

func (w *Workspace) WithDockerImage(image string) *Workspace {
	w.DockerImage = image
	return w
}

func (w *Workspace) WithVersion(version string) *Workspace {
	w.Version = version
	return w
}

// CompileCwContract takes a relative path input for the contract to compile
// CosmWasm's rust-optimizer is used for compilation
// Successful compilation will return the absolute path of the new binary
// - contractPath is the relative path of the contract project on local machine
func (w *Workspace) Compile() (string, error) {
	repoPathFull, err := compile(w.DockerImage, w.Version, w.RelativePath)

	// Form the path to the artifacts directory, used for checksum.txt and package.wasm
	artifactsPath := filepath.Join(repoPathFull, "artifacts")

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