package cosmwasm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Contract struct {
	DockerImage     string
	Version         string
	RelativePath    string
	wasmBinPathChan chan string
	errChan         chan error
}

// NewContract return a contract struct, populated with defaults and its relative path
// relativePath is the relative path to the contract on local machine
func NewContract(relativePath string) *Contract {
	return &Contract{
		DockerImage:  "cosmwasm/rust-optimizer",
		Version:      "0.14.0",
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
//
//	cosmwasm/rust-optimizer is the expected docker image
func (c *Contract) Compile() *Contract {
	c.wasmBinPathChan = make(chan string)
	c.errChan = make(chan error, 1)

	go func() {
		repoPathFull, err := compile(c.DockerImage, c.Version, c.RelativePath)
		if err != nil {
			c.errChan <- err
			return
		}

		// Form the path to the artifacts directory, used for checksum.txt and package.wasm
		artifactsPath := filepath.Join(repoPathFull, "artifacts")

		// Parse the checksums.txt for the create/wasm binary name
		checksumsPath := filepath.Join(artifactsPath, "checksums.txt")
		checksumsBz, err := os.ReadFile(checksumsPath)
		if err != nil {
			c.errChan <- fmt.Errorf("checksums read: %w", err)
			return
		}
		_, wasmBin, found := strings.Cut(string(checksumsBz), "  ")
		if !found {
			c.errChan <- fmt.Errorf("wasm binary name not found")
			return
		}

		// Form the path to the wasm binary
		c.wasmBinPathChan <- filepath.Join(artifactsPath, strings.TrimSpace(wasmBin))
	}()

	return c
}

// WaitForCompile will wait until compilation is complete, this can be called after chain setup
// Successful compilation will return the binary location in a channel
func (c *Contract) WaitForCompile() (string, error) {
	contractBinary := ""
	select {
	case err := <-c.errChan:
		return "", err
	case contractBinary = <-c.wasmBinPathChan:
	}
	return contractBinary, nil
}
