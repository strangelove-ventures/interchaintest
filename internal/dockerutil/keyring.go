package dockerutil

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ory/dockertest/v3/docker"
)

// NewLocalKeyringFromDockerContainer copies the contents of the given container directory into a specified local directory.
// This allows test hosts to sign transactions on behalf of test users.
func NewLocalKeyringFromDockerContainer(ctx context.Context, dc *docker.Client, localDirectory, containerKeyringDir, containerId string) (keyring.Keyring, error) {
	var buf bytes.Buffer
	err := dc.DownloadFromContainer(containerId, docker.DownloadFromContainerOptions{
		OutputStream: &buf,
		Path:         containerKeyringDir,
		Context:      ctx,
	})
	if err != nil {
		return nil, err
	}

	if err := os.Mkdir(fmt.Sprintf("%s/keyring-test", localDirectory), os.ModePerm); err != nil {
		return nil, err
	}

	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return nil, err
		}

		var fileBuff bytes.Buffer
		if _, err := io.Copy(&fileBuff, tr); err != nil {
			return nil, err
		}

		name := hdr.Name
		splitName := strings.Split(name, "/")

		extractedFileName := splitName[len(splitName)-1]
		isDirectory := extractedFileName == ""
		if isDirectory {
			continue
		}

		filePath := fmt.Sprintf("%s/keyring-test/%s", localDirectory, extractedFileName)
		if err := os.WriteFile(filePath, fileBuff.Bytes(), os.ModePerm); err != nil {
			return nil, err
		}
	}
	return keyring.New("", keyring.BackendTest, localDirectory, os.Stdin)
}
