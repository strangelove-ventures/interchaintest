package dockerutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// Image is a docker image.
type Image struct {
	log             *zap.Logger
	pool            *dockertest.Pool
	repository, tag string
	networkID       string
	testName        string
}

// NewImage returns a valid Image.
//
// "pool" and "networkID" are likely from DockerSetup.
// "testName" is from a (*testing.T).Name() and should match the t.Name() from DockerSetup to ensure proper cleanup.
//
// Most arguments (except tag) must be non-zero values or this function panics.
// If tag is absent, defaults to "latest".
// Currently, only public docker images are supported.
func NewImage(logger *zap.Logger, pool *dockertest.Pool, networkID string, testName string, repository, tag string) *Image {
	if logger == nil {
		panic(errors.New("nil logger"))
	}
	if pool == nil {
		panic(errors.New("pool cannot be nil"))
	}
	if networkID == "" {
		panic(errors.New("networkID cannot be empty"))
	}
	if testName == "" {
		panic("testName cannot be empty")
	}
	if repository == "" {
		panic(errors.New("repository cannot be empty"))
	}
	if tag == "" {
		tag = "latest"
	}
	return &Image{
		log: logger.With(
			zap.String("image", fmt.Sprintf("%s:%s", repository, tag)),
			zap.String("test_name", testName),
		),
		pool:       pool,
		networkID:  networkID,
		repository: repository,
		tag:        tag,
		testName:   testName,
	}
}

// ContainerOptions optionally configures starting a Container.
type ContainerOptions struct {
	// bind mounts: https://docs.docker.com/storage/bind-mounts/
	Binds []string

	// Environment variables
	Env []string

	// If blank, defaults to a reasonable non-root user.
	User string
}

// Run creates and runs a container invoking "cmd". The container resources are removed after exit.
//
// Run blocks until the command completes. Thus, Run is not suitable for daemons or servers. Use Start instead.
// A non-zero status code returns an error.
func (image *Image) Run(ctx context.Context, cmd []string, opts ContainerOptions) (stdout, stderr []byte, err error) {
	c, err := image.Start(ctx, cmd, opts)
	if err != nil {
		return nil, nil, err
	}
	return c.Wait(ctx)
}

// ensurePulled can only pull public images.
func (image *Image) ensurePulled() error {
	client := image.pool.Client
	_, err := client.InspectImage(fmt.Sprintf("%s:%s", image.repository, image.tag))
	if err != nil {
		if err := client.PullImage(docker.PullImageOptions{
			Repository: image.repository,
			Tag:        image.tag,
		}, docker.AuthConfiguration{}); err != nil {
			return fmt.Errorf("pull image %s:%s: %w", image.repository, image.tag, err)
		}
	}
	return nil
}

func (image *Image) createContainer(ctx context.Context, containerName, hostName string, cmd []string, opts ContainerOptions) (*docker.Container, error) {
	// Although this shouldn't happen because the name includes randomness, in reality there seems to intermittent
	// chances of collisions.
	if resource, ok := image.pool.ContainerByName(containerName); ok {
		if err := image.pool.Purge(resource); err != nil {
			return nil, fmt.Errorf("unable to purge container %s: %w", containerName, err)
		}
	}

	// Ensure reasonable defaults.
	if opts.User == "" {
		opts.User = GetDockerUserString()
	}

	return image.pool.Client.CreateContainer(docker.CreateContainerOptions{
		Context: ctx,
		Name:    containerName,
		Config: &docker.Config{
			User:     opts.User,
			Hostname: hostName,
			Image:    fmt.Sprintf("%s:%s", image.repository, image.tag),
			Cmd:      cmd,
			Env:      opts.Env,
			Labels:   map[string]string{CleanupLabel: image.testName},
		},
		HostConfig: &docker.HostConfig{
			Binds:           opts.Binds,
			PublishAllPorts: true, // Because we publish all ports, no need to expose specific ports.
			AutoRemove:      false,
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				image.networkID: {},
			},
		},
	})
}

// Start pulls the image if not present, creates a container, and runs it.
func (image *Image) Start(ctx context.Context, cmd []string, opts ContainerOptions) (*Container, error) {
	if len(cmd) == 0 {
		panic(errors.New("cmd cannot be empty"))
	}

	if err := image.ensurePulled(); err != nil {
		return nil, image.wrapErr(err)
	}

	var (
		containerName = SanitizeContainerName(image.testName + "-" + RandLowerCaseLetterString(6))
		hostName      = CondenseHostName(containerName)
		logger        = image.log.With(
			zap.String("command", strings.Join(cmd, " ")),
			zap.String("hostname", hostName),
			zap.String("container", containerName),
		)
	)

	c, err := image.createContainer(ctx, containerName, hostName, cmd, opts)
	if err != nil {
		return nil, image.wrapErr(fmt.Errorf("create container %s: %w", containerName, err))
	}

	logger.Info("Running container")

	err = image.pool.Client.StartContainerWithContext(c.ID, nil, ctx)
	if err != nil {
		return nil, image.wrapErr(fmt.Errorf("start container %s: %w", containerName, err))
	}

	// Copying (*dockertest.Pool).Run logic which inspects after starting.
	c, err = image.pool.Client.InspectContainerWithContext(c.ID, ctx)
	if err != nil {
		return nil, image.wrapErr(fmt.Errorf("inspect started container %s: %w", containerName, err))
	}

	return &Container{
		Name:      containerName,
		Hostname:  hostName,
		log:       logger,
		image:     image,
		container: c,
	}, nil
}

func (image *Image) wrapErr(err error) error {
	return fmt.Errorf("image %s:%s: %w", image.repository, image.tag, err)
}

// Container is a docker container. Use (*Image).Start to create a new container.
type Container struct {
	Name     string
	Hostname string

	log       *zap.Logger
	image     *Image
	container *docker.Container
}

// Wait blocks until the container exits. Calling wait is not suitable for daemons and servers.
// A non-zero status code returns an error.
//
// Wait implicitly calls Stop.
func (c *Container) Wait(ctx context.Context) (stdout, stderr []byte, err error) {
	var (
		image = c.image
		cont  = c.container
	)

	exitCode, err := image.pool.Client.WaitContainerWithContext(cont.ID, ctx)
	if err != nil {
		return nil, nil, c.image.wrapErr(fmt.Errorf("wait for container %s: %w", c.Name, err))
	}

	var (
		stdoutBuf = new(bytes.Buffer)
		stderrBuf = new(bytes.Buffer)
	)
	err = image.pool.Client.Logs(docker.LogsOptions{Context: ctx, Container: cont.ID, OutputStream: stdoutBuf, ErrorStream: stderrBuf, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
	if err != nil {
		c.log.Info("Failed to get container logs", zap.Error(err), zap.String("container_id", cont.ID))
	}
	err = c.Stop(10 * time.Second)
	if err != nil {
		c.log.Error("Failed to stop and remove container", zap.Error(err), zap.String("container_id", cont.ID))
	}

	if exitCode != 0 {
		out := strings.Join([]string{stdoutBuf.String(), stderrBuf.String()}, " ")
		return nil, nil, fmt.Errorf("exit code %d: %s", exitCode, out)
	}

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
}

// Stop gives the container up to timeout to stop and remove itself from the network.
func (c *Container) Stop(timeout time.Duration) error {
	// Use timeout*2 to give both stop and remove container operations a chance to complete.
	ctx, cancel := context.WithTimeout(context.Background(), timeout*2)
	defer cancel()

	var (
		client     = c.image.pool.Client
		notFound   *docker.NoSuchContainer
		notRunning *docker.ContainerNotRunning

		merr error
	)

	err := client.StopContainerWithContext(c.container.ID, uint(timeout.Seconds()), ctx)
	switch {
	case errors.As(err, &notFound) || errors.As(err, &notRunning):
	// ignore
	case err != nil:
		multierr.AppendInto(&merr, fmt.Errorf("stop container %s: %w", c.Name, err))
		// Proceed to remove container.
	}

	// RemoveContainerOptions duplicates (*dockertest.Resource).Prune.
	err = client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            c.container.ID,
		Context:       ctx,
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !errors.As(err, &notFound) {
		multierr.AppendInto(&merr, fmt.Errorf("remove container %s: %w", c.Name, err))
	}

	if merr != nil {
		return c.image.wrapErr(merr)
	}

	return nil
}
