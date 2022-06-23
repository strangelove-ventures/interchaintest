package dockerutil

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// ContainerJob loosely mimics os/exec package for running one-off docker containers.
// Job containers are expected to invoke commands and exit. Therefore, they are not suitable for servers or daemons.
type ContainerJob struct {
	pool            *dockertest.Pool
	repository, tag string
	networkID       string
}

// NewContainerJob returns a valid ContainerJob.
// "networkID" is from dockertest.CreateNetwork or similar.
func NewContainerJob(pool *dockertest.Pool, networkID string, repository, tag string) *ContainerJob {
	return &ContainerJob{
		pool:       pool,
		networkID:  networkID,
		repository: repository,
		tag:        tag,
	}
}

// JobOptions optionally configure a ContainerJob.
type JobOptions struct {
	// bind mounts: https://docs.docker.com/storage/bind-mounts/
	Binds []string
}

// Run runs the docker image and invokes "cmd". "cmd" is the command and any arguments.
// A non-zero status code returns a non-nil error.
func (job *ContainerJob) Run(ctx context.Context, jobName string, cmd []string, opts JobOptions) (stdout []byte, stderr []byte, err error) {
	fullName := fmt.Sprintf("%s-%s", jobName, RandLowerCaseLetterString(3))
	fullName = SanitizeContainerName(fullName)

	cont, err := job.pool.Client.CreateContainer(docker.CreateContainerOptions{
		Context: ctx,
		Name:    fullName,
		Config: &docker.Config{
			User:       GetDockerUserString(),
			Hostname:   CondenseHostName(fullName),
			Image:      fmt.Sprintf("%s:%s", job.repository, job.tag),
			Cmd:        cmd,
			Labels:     map[string]string{labelKey: jobName},
			Entrypoint: []string{},
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
	var (
		stdoutBuf = new(bytes.Buffer)
		stderrBuf = new(bytes.Buffer)
	)
	_ = job.pool.Client.Logs(docker.LogsOptions{Context: ctx, Container: cont.ID, OutputStream: stdoutBuf, ErrorStream: stderrBuf, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
	_ = job.pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID, Context: ctx})

	if exitCode != 0 {
		panic(stderrBuf.String())
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
}
