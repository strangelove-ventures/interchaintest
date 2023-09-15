package cosmwasm

import (
	"bufio"
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

// NewWorkspace returns a workspace struct, populated with defaults and its relative path
// relativePath is the relative path to the workspace on local machine
func NewWorkspace(relativePath string) *Workspace {
	return &Workspace{
		DockerImage: "cosmwasm/workspace-optimizer",
		Version: "0.14.0",
		RelativePath: relativePath,
	}
}

// WithDockerImage sets a custom docker image to use
func (w *Workspace) WithDockerImage(image string) *Workspace {
	w.DockerImage = image
	return w
}

// WithVersion sets a custom version to use
func (w *Workspace) WithVersion(version string) *Workspace {
	w.Version = version
	return w
}

// Compile will compile the workspace's contracts
//   cosmwasm/workspace-optimizer is the expected docker image
// Successful compilation will return a map of crate names to binary locations
func (w *Workspace) Compile() (map[string]string, error) {
	repoPathFull, err := compile(w.DockerImage, w.Version, w.RelativePath)
	if err != nil {
		return nil, err
	}

	// Form the path to the artifacts directory, used for checksum.txt and package.wasm
	artifactsPath := filepath.Join(repoPathFull, "artifacts")

	// Parse the checksums.txt for the crate/wasm binary names
	wasmBinaries := make(map[string]string)
	checksumsPath := filepath.Join(artifactsPath, "checksums.txt")
	file, err := os.Open(checksumsPath)
	if err != nil {
		return nil, fmt.Errorf("checksums open: %w", err)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		_, wasmBin, found := strings.Cut(line, "  ")
		if !found {
			return nil, fmt.Errorf("wasm binary name not found")
		}
		wasmBin = strings.TrimSpace(wasmBin)
		crateName, _, found := strings.Cut(wasmBin, ".")
		if !found {
			return nil, fmt.Errorf("wasm binary name invalid")
		}
		wasmBinaries[crateName] = filepath.Join(artifactsPath, wasmBin)
	}

	return wasmBinaries, nil
  }