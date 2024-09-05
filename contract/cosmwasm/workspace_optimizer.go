package cosmwasm

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Workspace struct {
	DockerImage      string
	Version          string
	RelativePath     string
	wasmBinariesChan chan map[string]string
	errChan          chan error
}

// NewWorkspace returns a workspace struct, populated with defaults and its relative path
// relativePath is the relative path to the workspace on local machine.
func NewWorkspace(relativePath string) *Workspace {
	return &Workspace{
		DockerImage:  "cosmwasm/workspace-optimizer",
		Version:      "0.14.0",
		RelativePath: relativePath,
	}
}

// WithDockerImage sets a custom docker image to use.
func (w *Workspace) WithDockerImage(image string) *Workspace {
	w.DockerImage = image
	return w
}

// WithVersion sets a custom version to use.
func (w *Workspace) WithVersion(version string) *Workspace {
	w.Version = version
	return w
}

// Compile will compile the workspace's contracts
//
//	cosmwasm/workspace-optimizer is the expected docker image
//
// The workspace object is returned, call WaitForCompile() to get results.
func (w *Workspace) Compile() *Workspace {
	w.wasmBinariesChan = make(chan map[string]string)
	w.errChan = make(chan error, 1)

	go func() {
		repoPathFull, err := compile(w.DockerImage, w.Version, w.RelativePath)
		if err != nil {
			w.errChan <- err
			return
		}

		// Form the path to the artifacts directory, used for checksum.txt and package.wasm
		artifactsPath := filepath.Join(repoPathFull, "artifacts")

		// Parse the checksums.txt for the crate/wasm binary names
		wasmBinaries := make(map[string]string)
		checksumsPath := filepath.Join(artifactsPath, "checksums.txt")
		file, err := os.Open(checksumsPath)
		if err != nil {
			w.errChan <- fmt.Errorf("checksums open: %w", err)
			return
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			_, wasmBin, found := strings.Cut(line, "  ")
			if !found {
				w.errChan <- fmt.Errorf("wasm binary name not found")
				return
			}
			wasmBin = strings.TrimSpace(wasmBin)
			crateName, _, found := strings.Cut(wasmBin, ".")
			if !found {
				w.errChan <- fmt.Errorf("wasm binary name invalid")
				return
			}
			wasmBinaries[crateName] = filepath.Join(artifactsPath, wasmBin)
		}
		w.wasmBinariesChan <- wasmBinaries
	}()

	return w
}

// WaitForCompile will wait until coyympilation is complete, this can be called after chain setup
// Successful compilation will return a map of crate names to binary locations in a channel.
func (w *Workspace) WaitForCompile() (map[string]string, error) {
	contractBinaries := make(map[string]string)
	select {
	case err := <-w.errChan:
		return contractBinaries, err
	case contractBinaries = <-w.wasmBinariesChan:
	}
	return contractBinaries, nil
}
