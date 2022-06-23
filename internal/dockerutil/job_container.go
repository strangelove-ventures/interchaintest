package dockerutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// JobContainer loosely mimics os/exec package for running one-off docker containers.
// Job containers are expected to invoke commands and exit. Therefore, they are not suitable for servers or daemons.
type JobContainer struct {
	pool            *dockertest.Pool
	repository, tag string
	networkID       string
}

// NewJobContainer returns a valid JobContainer.
// "networkID" is from dockertest.CreateNetwork or similar.
// All arguments must be non-zero values or this function panics.
func NewJobContainer(pool *dockertest.Pool, networkID string, repository, tag string) *JobContainer {
	if pool == nil {
		panic(errors.New("pool cannot be nil"))
	}
	if networkID == "" {
		panic(errors.New("networkID cannot be empty"))
	}
	if repository == "" {
		panic(errors.New("repository cannot be empty"))
	}
	if tag == "" {
		panic(errors.New("tag cannot be empty"))
	}
	return &JobContainer{
		pool:       pool,
		networkID:  networkID,
		repository: repository,
		tag:        tag,
	}
}

// JobOptions optionally configure a JobContainer.
type JobOptions struct {
	// bind mounts: https://docs.docker.com/storage/bind-mounts/
	Binds []string
}

// Run runs the docker image and invokes "cmd". "cmd" is the command and any arguments.
// A non-zero status code returns a non-nil error.
func (job *JobContainer) Run(ctx context.Context, jobName string, cmd []string, opts JobOptions) (stdout []byte, stderr []byte, err error) {
	if len(cmd) == 0 {
		panic(errors.New("cmd cannot be empty"))
	}

	fullName := fmt.Sprintf("%s-%s", jobName, RandLowerCaseLetterString(3))
	fullName = SanitizeContainerName(fullName)

	// dockertest offers a higher level api via the direct "dockertest" package. However, the package does not
	// allow for one-off job containers in this manner. You can use a *dockertest.Resource to exec into a running
	// container. However, this requires the container is running a long-lived process like a daemon. While it's
	// reasonable to assume a program like "sleep" is present in the container, it is not guaranteed.
	cont, err := job.pool.Client.CreateContainer(docker.CreateContainerOptions{
		Context: ctx,
		Name:    fullName,
		Config: &docker.Config{
			User:     GetDockerUserString(),
			Hostname: CondenseHostName(fullName),
			Image:    fmt.Sprintf("%s:%s", job.repository, job.tag),
			Cmd:      cmd,
			Labels:   map[string]string{labelKey: jobName},
		},
		HostConfig: &docker.HostConfig{
			Binds:           opts.Binds,
			PublishAllPorts: true,
			AutoRemove:      false,
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				job.networkID: {},
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("Client.CreateContainer: %w", err)
	}
	err = job.pool.Client.StartContainerWithContext(cont.ID, nil, ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Client.StartContainerWithContext for container %s: %w", cont.ID, err)
	}

	exitCode, err := job.pool.Client.WaitContainerWithContext(cont.ID, ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Client.WaitContainerWithContext: %w", err)
	}
	var (
		stdoutBuf = new(bytes.Buffer)
		stderrBuf = new(bytes.Buffer)
	)
	_ = job.pool.Client.Logs(docker.LogsOptions{Context: ctx, Container: cont.ID, OutputStream: stdoutBuf, ErrorStream: stderrBuf, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
	_ = job.pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID, Context: ctx})

	if exitCode != 0 {
		out := strings.Join([]string{stdoutBuf.String(), stderrBuf.String()}, " ")
		return nil, nil, fmt.Errorf("status code %d: %s", exitCode, out)
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
}
