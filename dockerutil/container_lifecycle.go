package dockerutil

import (
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

// Example Go/Cosmos-SDK panic format is `panic: bad Duration: time: invalid duration "bad"\n`
var panicRe = regexp.MustCompile(`panic:.*\n`)

type ContainerLifecycle struct {
	log               *zap.Logger
	client            *dockerclient.Client
	containerName     string
	id                string
	preStartListeners Listeners
}

func NewContainerLifecycle(log *zap.Logger, client *dockerclient.Client, containerName string) *ContainerLifecycle {
	return &ContainerLifecycle{
		log:           log,
		client:        client,
		containerName: containerName,
	}
}

func (c *ContainerLifecycle) CreateContainer(
	ctx context.Context,
	testName string,
	networkID string,
	image ibc.DockerImage,
	ports nat.PortMap,
	volumeBinds []string,
	mounts []mount.Mount,
	hostName string,
	cmd []string,
	env []string,
	entrypoint []string,
) error {
	imageRef := image.Ref()
	c.log.Info(
		"Will run command",
		zap.String("image", imageRef),
		zap.String("container", c.containerName),
		zap.String("command", strings.Join(cmd, " ")),
	)

	if err := image.PullImage(ctx, c.client); err != nil {
		return err
	}

	pS := nat.PortSet{}
	for k := range ports {
		pS[k] = struct{}{}
	}

	pb, listeners, err := GeneratePortBindings(ports)
	if err != nil {
		return fmt.Errorf("failed to generate port bindings: %w", err)
	}

	c.preStartListeners = listeners

	cc, err := c.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: imageRef,

			Entrypoint: entrypoint,
			Cmd:        cmd,
			Env:        env,

			Hostname: hostName,

			Labels: map[string]string{CleanupLabel: testName},

			ExposedPorts: pS,
		},
		&container.HostConfig{
			Binds:           volumeBinds,
			PortBindings:    pb,
			PublishAllPorts: true,
			AutoRemove:      false,
			DNS:             []string{},
			Mounts:          mounts,
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkID: {},
			},
		},
		nil,
		c.containerName,
	)
	if err != nil {
		listeners.CloseAll()
		c.preStartListeners = []net.Listener{}
		return err
	}
	c.id = cc.ID
	return nil
}

func (c *ContainerLifecycle) StartContainer(ctx context.Context) error {
	// lock port allocation for the time between freeing the ports from the
	// temporary listeners to the consumption of the ports by the container
	mu.RLock()
	defer mu.RUnlock()

	c.preStartListeners.CloseAll()
	c.preStartListeners = []net.Listener{}

	if err := StartContainer(ctx, c.client, c.id); err != nil {
		return err
	}

	if err := c.CheckForFailedStart(ctx, time.Second*1); err != nil {
		return err
	}

	c.log.Info("Container started", zap.String("container", c.containerName))
	return nil
}

// CheckForFailedStart checks the logs of the container for a
// panic message after a wait period to allow the container to start.
func (c *ContainerLifecycle) CheckForFailedStart(ctx context.Context, wait time.Duration) error {
	time.Sleep(wait)
	containerLogs, err := c.client.ContainerLogs(ctx, c.id, dockertypes.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return fmt.Errorf("failed to read logs from container %s: %w", c.containerName, err)
	}
	defer containerLogs.Close()

	logs := new(strings.Builder)
	_, err = io.Copy(logs, containerLogs)
	if err != nil {
		return fmt.Errorf("failed to read logs from container %s: %w", c.containerName, err)
	}

	if err := ParseSDKPanicFromText(logs.String()); err != nil {
		// Must use Println and not the logger as there are ascii escape codes in the logs.
		fmt.Printf("\nContainer name: %s.\nerror: %s.\nlogs\n%s\n", c.containerName, err.Error(), logs.String())
		return fmt.Errorf("container %s failed to start: %w", c.containerName, err)
	}

	return nil
}

// ParsePanicFromText returns a panic line if it exists in the logs so
// that it can be returned to the user in a proper error message instead of
// hanging.
func ParseSDKPanicFromText(text string) error {
	if !strings.Contains(text, "panic: ") {
		return nil
	}

	match := panicRe.FindString(text)
	if match != "" {
		panicMessage := strings.TrimSpace(match)
		return fmt.Errorf("%s", panicMessage)
	}

	return nil
}

func (c *ContainerLifecycle) PauseContainer(ctx context.Context) error {
	return c.client.ContainerPause(ctx, c.id)
}

func (c *ContainerLifecycle) UnpauseContainer(ctx context.Context) error {
	return c.client.ContainerUnpause(ctx, c.id)
}

func (c *ContainerLifecycle) StopContainer(ctx context.Context) error {
	var timeout container.StopOptions
	timeoutSec := 30
	timeout.Timeout = &timeoutSec

	return c.client.ContainerStop(ctx, c.id, timeout)
}

func (c *ContainerLifecycle) RemoveContainer(ctx context.Context) error {
	err := c.client.ContainerRemove(ctx, c.id, dockertypes.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("remove container %s: %w", c.containerName, err)
	}
	return nil
}

func (c *ContainerLifecycle) ContainerID() string {
	return c.id
}

func (c *ContainerLifecycle) GetHostPorts(ctx context.Context, portIDs ...string) ([]string, error) {
	cjson, err := c.client.ContainerInspect(ctx, c.id)
	if err != nil {
		return nil, err
	}
	ports := make([]string, len(portIDs))
	for i, p := range portIDs {
		ports[i] = GetHostPort(cjson, p)
	}
	return ports, nil
}

// Running will inspect the container and check its state to determine if it is currently running.
// If the container is running nil will be returned, otherwise an error is returned.
func (c *ContainerLifecycle) Running(ctx context.Context) error {
	cjson, err := c.client.ContainerInspect(ctx, c.id)
	if err != nil {
		return err
	}
	if cjson.State.Running {
		return nil
	}
	return fmt.Errorf("container with name %s and id %s is not running", c.containerName, c.id)
}
