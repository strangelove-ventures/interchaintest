package dockerutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ory/dockertest/v3/docker"
	"go.uber.org/zap"
)

// ContainerFactory manages, builds and runs docker containers.
//
// Implementation uses the lower level API of dockertest because more fine-grained control is needed, namely
// to listen to contexts and prevent errors like "container already exists".
type ContainerFactory struct {
	log             *zap.Logger
	pool            Pool
	repository, tag string

	baseName string
}

type CreateOptions struct {
	Repository string
	Tag        string
	Name       string
}

// NewFactory returns a valid ContainerFactory.
// containerName and repository are required or this function panics.
// Tag defaults to "latest" if empty.
func NewFactory(logger *zap.Logger, pool Pool, containerName string, repository, tag string) *ContainerFactory {
	if logger == nil {
		logger = zap.NewNop()
	}

	if containerName == "" {
		panic(errors.New("containerName cannot be empty"))
	}

	if repository == "" {
		panic(errors.New("repository cannot be empty"))
	}
	if tag == "" {
		tag = "latest"
	}

	logger = logger.With(
		zap.String("image", strings.Join([]string{repository, tag}, ":")),
	)

	return &ContainerFactory{
		log:        logger,
		pool:       pool,
		repository: repository,
		tag:        tag,
		baseName:   containerName,
	}
}

// RunOptions optionally configure a Container.
type RunOptions struct {
	// bind mounts: https://docs.docker.com/storage/bind-mounts/
	Binds []string
}

type RunResult struct {
	Stdout []byte
	Stderr []byte
}

// RunJob runs the docker image and invokes "cmd". RunJob blocks until the container exits.
//
// The container is removed after it exits.
// A non-zero status code returns a non-nil error.
func (c *ContainerFactory) RunJob(ctx context.Context, cmd []string, opts RunOptions) (RunResult, error) {
	if len(cmd) == 0 {
		panic(errors.New("cmd cannot be empty"))
	}

	var (
		client = c.pool.Pool().Client
		zero   RunResult
	)

	var (
		name     = SanitizeContainerName("job-" + c.baseName + "-" + RandLowerCaseLetterString(6))
		hostname = CondenseHostName(name)
		logger   = c.log.With(
			zap.String("container", name),
			zap.String("hostname", hostname),
			zap.String("command", strings.Join(cmd, " ")),
		)
	)

	cont, err := c.createContainer(ctx, name, hostname, cmd, opts)
	if err != nil {
		return zero, err // err already wrapped
	}

	logger.Info("Running container job")

	err = client.StartContainerWithContext(cont.ID, nil, ctx)
	if err != nil {
		return zero, fmt.Errorf("start container: %w", err)
	}

	exitCode, err := client.WaitContainerWithContext(cont.ID, ctx)
	if err != nil {
		return zero, fmt.Errorf("wait for container: %w", err)
	}
	var (
		stdoutBuf = new(bytes.Buffer)
		stderrBuf = new(bytes.Buffer)
	)
	err = client.Logs(docker.LogsOptions{Context: ctx, Container: cont.ID, OutputStream: stdoutBuf, ErrorStream: stderrBuf, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
	if err != nil {
		logger.Info("Failed to get container logs", zap.Error(err))
	}

	err = client.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID, Force: true, RemoveVolumes: true})
	if err != nil {
		logger.Info("Failed to remove container", zap.Error(err))
	}

	if exitCode != 0 {
		out := strings.Join([]string{stdoutBuf.String(), stderrBuf.String()}, " ")
		err = fmt.Errorf("exit code %d: %s", exitCode, out)
		logger.Error("Command failed",
			zap.Int("exit_code", exitCode),
			zap.Error(err),
		)
		return zero, err
	}

	logger.Debug("Container job succeeded",
		zap.String("stdout", stdoutBuf.String()),
		zap.String("stderr", stderrBuf.String()),
	)
	return RunResult{
		Stdout: stdoutBuf.Bytes(),
		Stderr: stderrBuf.Bytes(),
	}, nil
}

// Ensure name and hostname are unique or "container already exist" errors happen.
// We cannot reuse containers because we must create them with the cmd. We cannot change the cmd later.
func (c *ContainerFactory) createContainer(ctx context.Context, name, hostname string, cmd []string, opts RunOptions) (*docker.Container, error) {
	if err := c.ensurePull(ctx); err != nil {
		// Error already wrapped.
		return nil, err
	}

	client := c.pool.Pool().Client

	container, err := client.CreateContainer(docker.CreateContainerOptions{
		Context: ctx,
		Name:    name,
		Config: &docker.Config{
			User:       GetDockerUserString(),
			Hostname:   hostname,
			Cmd:        cmd,
			Image:      fmt.Sprintf("%s:%s", c.repository, c.tag),
			Labels:     map[string]string{CleanupLabel: c.baseName},
			StopSignal: "SIGWINCH", // to support timeouts
		},
		HostConfig: &docker.HostConfig{
			Binds:           opts.Binds,
			PublishAllPorts: true,
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				c.pool.NetworkID(): {},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	// Copies dockertest.RunJob logic by inspecting again after creation.
	_, err = client.InspectContainer(container.ID)
	if err != nil {
		return nil, fmt.Errorf("inspect container: %w", err)
	}

	return container, nil
}

// ensurePull only works for public images.
func (c *ContainerFactory) ensurePull(ctx context.Context) error {
	client := c.pool.Pool().Client

	_, err := client.InspectImage(fmt.Sprintf("%s:%s", c.repository, c.tag))
	if err != nil {
		if err := client.PullImage(docker.PullImageOptions{
			Repository: c.repository,
			Tag:        c.tag,
			Context:    ctx,
		}, docker.AuthConfiguration{}); err != nil {
			return fmt.Errorf("pull image: %w", err)
		}
	}

	return nil
}
