package dockerutil

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ory/dockertest/v3/docker"
)

func NewDockerKeyring(ctx context.Context, dc *docker.Client, localDirectory, containerKeyringDir, containerId string) (keyring.Keyring, error) {
	var buf bytes.Buffer
	err := dc.DownloadFromContainer(containerId, docker.DownloadFromContainerOptions{
		OutputStream: &buf,
		Path:         containerKeyringDir,
		Context:      ctx,
	})
	if err != nil {
		return nil, err
	}

	tarFiles := map[string][]byte{}

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

		if hdr.Name != "keyring-test/" {
			tarFiles[hdr.Name] = fileBuff.Bytes()
		}
	}

	if err := os.Mkdir(fmt.Sprintf("%s/keyring-test", localDirectory), os.ModePerm); err != nil {
		return nil, err
	}

	for k, v := range tarFiles {
		splitName := strings.Split(k, "/")
		name := splitName[len(splitName)-1]
		filePath := fmt.Sprintf("%s/keyring-test/%s", localDirectory, name)
		err := ioutil.WriteFile(filePath, v, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	return keyring.New("", keyring.BackendTest, localDirectory, os.Stdin)
}
