package cosmwasm

import (
	"path/filepath"
	"fmt"
	"os"
	"strings"
)

type Contract struct {
	DockerImage string
	Version string
	RelativePath string
}

// NewContract return a contract struct, populated with defaults and its relative path
// relativePath is the relative path to the contract on local machine
func NewContract(relativePath string) *Contract {
	return &Contract{
		DockerImage: "cosmwasm/rust-optimizer",
		Version: "0.14.0",
		RelativePath: relativePath,
	}
}

// WithDockerImage sets a custom docker image to use
func (c *Contract) WithDockerImage(image string) *Contract {
	c.DockerImage = image
	return c
}

// WithVersion sets a custom version to use
func (c *Contract) WithVersion(version string) *Contract {
	c.Version = version
	return c
}

// Compile will compile the contract
//   cosmwasm/rust-optimizer is the expected docker image
// Successful compilation will return the binary location in a channel
func (c *Contract) Compile() (<-chan string, <-chan error) {
	wasmBinPathChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		repoPathFull, err := compile(c.DockerImage, c.Version, c.RelativePath)
		if err != nil {
			errChan <- err
			return
		}

		// Form the path to the artifacts directory, used for checksum.txt and package.wasm
		artifactsPath := filepath.Join(repoPathFull, "artifacts")

		// Parse the checksums.txt for the crate/wasm binary name
		checksumsPath := filepath.Join(artifactsPath, "checksums.txt")
		checksumsBz, err := os.ReadFile(checksumsPath)
		if err != nil {
			errChan <- fmt.Errorf("checksums read: %w", err)
			return
		}
		_, wasmBin, found := strings.Cut(string(checksumsBz), "  ")
		if !found {
			errChan <- fmt.Errorf("wasm binary name not found")
			return
		}

		// Form the path to the wasm binary
		wasmBinPathChan <- filepath.Join(artifactsPath, strings.TrimSpace(wasmBin))
	}()

	return wasmBinPathChan, errChan
}

