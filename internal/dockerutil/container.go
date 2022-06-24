package dockerutil

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"context"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.uber.org/zap"
)

type Container struct {
	log      *zap.Logger
	resource *dockertest.Resource
}

type RunOptions struct {
	Repository    string   // required
	Tag           string   // if not present, defaults to "latest"
	Cmd           []string // required
	ContainerName string   // required
	HostName      string   // required

	// Each element must be in format MY_ENV_VAR=value.
	Env []string

	// Bind mounts: https://docs.docker.com/storage/bind-mounts/, E.g. /my_local_dir:/my_container_dir
	Binds []string
}

func StartContainer(
	ctx context.Context, // TODO: have to roll my own cancellation here
	log *zap.Logger,
	pool *dockertest.Pool,
	networkID string,
	opts RunOptions,
) (*Container, error) {
	logger := log.With(
		zap.String("container", opts.ContainerName),
		zap.String("host", opts.HostName),
		zap.String("image", strings.Join([]string{opts.Repository, opts.Tag}, ":")),
	)
	runOpts := dockertest.RunOptions{
		Hostname:     opts.HostName,
		Name:         opts.ContainerName,
		Repository:   opts.Repository,
		Tag:          opts.Tag,
		Env:          opts.Env,
		Cmd:          opts.Cmd,
		ExposedPorts: nil, // TODO: is this necessary if we publish all ports anyway?
		NetworkID:    networkID,
		Auth:         docker.AuthConfiguration{}, // Only pull public images for now.
		Privileged:   false,
		User:         GetDockerUserString(),
	}
	hostConfig := func(cfg *docker.HostConfig) {
		cfg.PublishAllPorts = true
		cfg.Binds = opts.Binds
	}
	logger.Info("Starting container", zap.String("command", strings.Join(opts.Cmd, " ")))
	res, err := pool.RunWithOptions(&runOpts, hostConfig)
	if err != nil {
		return nil, err
	}

	if !res.Container.State.Running {
		_ = res.Close()
		panic("not running") // TODO: test me
	}

	// In case of failed test cleanup, expire the container from Docker Engine to eventually stop orphaned containers.
	const ninetyMinutes = 90 * 60
	_ = res.Expire(ninetyMinutes) // This argument expects uint seconds, not time.Duration.

	return &Container{
		log:      logger,
		resource: res,
	}, nil
}

type ExecResult struct {
	Stdout []byte
	Stderr []byte
}

// "env" each element must be in format "MY_ENV_VAR=value"
// TODO: roll my own context cancellation? I could copy the exec below to get at the ctx of the underlying lib
func (c *Container) Exec(ctx context.Context, cmd []string, env []string) (ExecResult, error) {
	if len(cmd) == 0 {
		panic(errors.New("empty cmd"))
	}
	var (
		stdout = new(bytes.Buffer)
		stderr = new(bytes.Buffer)
	)
	opts := dockertest.ExecOptions{
		Env:    env,
		StdOut: stdout,
		StdErr: stderr,
	}
	code, err := c.resource.Exec(cmd, opts)
	if err != nil {
		return ExecResult{}, err
	}
	if code != 0 {
		out := strings.Join([]string{stdout.String(), stderr.String()}, " ")
		return ExecResult{}, fmt.Errorf("exit code %d: %s", code, out)
	}
	return ExecResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, nil
}

func (c *Container) HostPort(port string) string {
	return GetHostPort(c.resource.Container, port)
}
