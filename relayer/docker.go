package relayer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
	"go.uber.org/zap"
)

// DockerRelayer provides a common base for relayer implementations
// that run on Docker.
type DockerRelayer struct {
	log *zap.Logger

	// c defines all the commands to run inside the container.
	c RelayerCommander

	networkID  string
	client     *client.Client
	volumeName string

	testName string

	customImage *ibc.DockerImage
	pullImage   bool

	// The ID of the container created by StartRelayer.
	containerID string

	// wallets contains a mapping of chainID to relayer wallet
	wallets map[string]ibc.RelayerWallet
}

var _ ibc.Relayer = (*DockerRelayer)(nil)

// NewDockerRelayer returns a new DockerRelayer.
func NewDockerRelayer(ctx context.Context, log *zap.Logger, testName string, cli *client.Client, networkID string, c RelayerCommander, options ...RelayerOption) (*DockerRelayer, error) {
	r := DockerRelayer{
		log: log,

		c: c,

		networkID: networkID,
		client:    cli,

		// pull true by default, can be overridden with options
		pullImage: true,

		testName: testName,

		wallets: map[string]ibc.RelayerWallet{},
	}

	for _, opt := range options {
		switch o := opt.(type) {
		case RelayerOptionDockerImage:
			r.customImage = &o.DockerImage
		case RelayerOptionImagePull:
			r.pullImage = o.Pull
		}
	}

	containerImage := r.containerImage()
	if err := r.pullContainerImageIfNecessary(containerImage); err != nil {
		return nil, fmt.Errorf("pulling container image %s: %w", containerImage.Ref(), err)
	}

	v, err := cli.VolumeCreate(ctx, volumetypes.VolumeCreateBody{
		// Have to leave Driver unspecified for Docker Desktop compatibility.

		Labels: map[string]string{dockerutil.CleanupLabel: testName},
	})
	if err != nil {
		return nil, fmt.Errorf("creating volume: %w", err)
	}
	r.volumeName = v.Name

	// Untar an empty tar in order to get side effect of correct permissions
	// on node home directory.
	emptyTar, err := archive.Generate()
	if err != nil {
		return nil, fmt.Errorf("generating empty tar for init: %w", err)
	}
	if err := r.untarIntoNodeHome(ctx, emptyTar); err != nil {
		return nil, err // Already wrapped.
	}

	if init := r.c.Init(r.NodeHome()); len(init) > 0 {
		// Initialization should complete immediately,
		// but add a 1-minute timeout in case Docker hangs on a developer workstation.
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		// Using a nop reporter here because it keeps the API simpler,
		// and the init command is typically not of high interest.
		if err := dockerutil.HandleNodeJobError(r.NodeJob(ctx, ibc.NopRelayerExecReporter{}, init)); err != nil {
			return nil, err
		}
	}

	return &r, nil
}

func (r *DockerRelayer) AddChainConfiguration(ctx context.Context, rep ibc.RelayerExecReporter, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
	// For rly this file is json, but the file extension should not matter.
	// Using .config to avoid implying any particular format.
	chainConfigFile := chainConfig.ChainID + ".config"

	chainConfigContainerFilePath := path.Join(r.NodeHome(), chainConfigFile)

	configContent, err := r.c.ConfigContent(ctx, chainConfig, keyName, rpcAddr, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to generate config content: %w", err)
	}

	tar, err := archive.Generate(chainConfigFile, string(configContent))
	if err != nil {
		return fmt.Errorf("generating tar for configuration: %w", err)
	}

	if err := r.untarIntoNodeHome(ctx, tar); err != nil {
		return err // Already wrapped.
	}

	cmd := r.c.AddChainConfiguration(chainConfigContainerFilePath, r.NodeHome())

	// Adding the chain configuration simply reads from a file on disk,
	// so this should also complete immediately.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	return dockerutil.HandleNodeJobError(r.NodeJob(ctx, rep, cmd))
}

func (r *DockerRelayer) AddKey(ctx context.Context, rep ibc.RelayerExecReporter, chainID, keyName string) (ibc.RelayerWallet, error) {
	cmd := r.c.AddKey(chainID, keyName, r.NodeHome())

	// Adding a key should be near-instantaneous, so add a 1-minute timeout
	// to detect if Docker has hung.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	exitCode, stdout, stderr, err := r.NodeJob(ctx, rep, cmd)
	if err != nil {
		return ibc.RelayerWallet{}, dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	wallet, err := r.c.ParseAddKeyOutput(stdout, stderr)
	if err != nil {
		return ibc.RelayerWallet{}, dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	r.wallets[chainID] = wallet
	return wallet, nil
}

func (r *DockerRelayer) GetWallet(chainID string) (ibc.RelayerWallet, bool) {
	wallet, ok := r.wallets[chainID]
	return wallet, ok
}

func (r *DockerRelayer) CreateChannel(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, opts ibc.CreateChannelOptions) error {
	cmd := r.c.CreateChannel(pathName, opts, r.NodeHome())
	return dockerutil.HandleNodeJobError(r.NodeJob(ctx, rep, cmd))
}

func (r *DockerRelayer) CreateClients(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	cmd := r.c.CreateClients(pathName, r.NodeHome())
	return dockerutil.HandleNodeJobError(r.NodeJob(ctx, rep, cmd))
}

func (r *DockerRelayer) CreateConnections(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	cmd := r.c.CreateConnections(pathName, r.NodeHome())
	return dockerutil.HandleNodeJobError(r.NodeJob(ctx, rep, cmd))
}

func (r *DockerRelayer) FlushAcknowledgements(ctx context.Context, rep ibc.RelayerExecReporter, pathName, channelID string) error {
	cmd := r.c.FlushAcknowledgements(pathName, channelID, r.NodeHome())
	return dockerutil.HandleNodeJobError(r.NodeJob(ctx, rep, cmd))
}

func (r *DockerRelayer) FlushPackets(ctx context.Context, rep ibc.RelayerExecReporter, pathName, channelID string) error {
	cmd := r.c.FlushPackets(pathName, channelID, r.NodeHome())
	return dockerutil.HandleNodeJobError(r.NodeJob(ctx, rep, cmd))
}

func (r *DockerRelayer) GeneratePath(ctx context.Context, rep ibc.RelayerExecReporter, srcChainID, dstChainID, pathName string) error {
	cmd := r.c.GeneratePath(srcChainID, dstChainID, pathName, r.NodeHome())
	return dockerutil.HandleNodeJobError(r.NodeJob(ctx, rep, cmd))
}

func (r *DockerRelayer) GetChannels(ctx context.Context, rep ibc.RelayerExecReporter, chainID string) ([]ibc.ChannelOutput, error) {
	cmd := r.c.GetChannels(chainID, r.NodeHome())

	// Getting channels should be very quick, but go up to a 3-minute timeout just in case.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	exitCode, stdout, stderr, err := r.NodeJob(ctx, rep, cmd)
	if err != nil {
		return nil, dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	return r.c.ParseGetChannelsOutput(stdout, stderr)
}

func (r *DockerRelayer) GetConnections(ctx context.Context, rep ibc.RelayerExecReporter, chainID string) (ibc.ConnectionOutputs, error) {
	cmd := r.c.GetConnections(chainID, r.NodeHome())
	exitCode, stdout, stderr, err := r.NodeJob(ctx, rep, cmd)
	if err != nil {
		return nil, dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	return r.c.ParseGetConnectionsOutput(stdout, stderr)
}

func (r *DockerRelayer) LinkPath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, opts ibc.CreateChannelOptions) error {
	cmd := r.c.LinkPath(pathName, r.NodeHome(), opts)
	return dockerutil.HandleNodeJobError(r.NodeJob(ctx, rep, cmd))
}

func (r *DockerRelayer) RestoreKey(ctx context.Context, rep ibc.RelayerExecReporter, chainID, keyName, mnemonic string) error {
	cmd := r.c.RestoreKey(chainID, keyName, mnemonic, r.NodeHome())

	// Restoring a key should be near-instantaneous, so add a 1-minute timeout
	// to detect if Docker has hung.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	exitCode, stdout, stderr, err := r.NodeJob(ctx, rep, cmd)
	if err != nil {
		return dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	r.wallets[chainID] = ibc.RelayerWallet{
		Mnemonic: mnemonic,
		Address:  r.c.ParseRestoreKeyOutput(stdout, stdout),
	}
	return nil
}

func (r *DockerRelayer) UpdateClients(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	cmd := r.c.UpdateClients(pathName, r.NodeHome())
	return dockerutil.HandleNodeJobError(r.NodeJob(ctx, rep, cmd))
}

func (r *DockerRelayer) StartRelayer(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	return r.createNodeContainer(ctx, pathName)
}

func (r *DockerRelayer) StopRelayer(ctx context.Context, rep ibc.RelayerExecReporter) error {
	if err := r.stopContainer(ctx); err != nil {
		return err
	}

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	rc, err := r.client.ContainerLogs(ctx, r.containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "50",
	})
	if err != nil {
		return fmt.Errorf("StopRelayer: retrieving ContainerLogs: %w", err)
	}
	defer func() { _ = rc.Close() }()

	// Logs are multiplexed into one stream; see docs for ContainerLogs.
	_, err = stdcopy.StdCopy(stdoutBuf, stderrBuf, rc)
	if err != nil {
		return fmt.Errorf("StopRelayer: demuxing logs: %w", err)
	}
	_ = rc.Close()

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	c, err := r.client.ContainerInspect(ctx, r.containerID)
	if err != nil {
		return fmt.Errorf("StopRelayer: inspecting container: %w", err)
	}

	startedAt, err := time.Parse(c.State.StartedAt, time.RFC3339Nano)
	if err != nil {
		r.log.Info("Failed to parse container StartedAt", zap.Error(err))
		startedAt = time.Unix(0, 0)
	}

	finishedAt, err := time.Parse(c.State.FinishedAt, time.RFC3339Nano)
	if err != nil {
		r.log.Info("Failed to parse container FinishedAt", zap.Error(err))
		finishedAt = time.Now().UTC()
	}

	rep.TrackRelayerExec(
		c.Name,
		c.Args,
		stdout, stderr,
		c.State.ExitCode,
		startedAt,
		finishedAt,
		nil,
	)

	r.log.Debug(
		fmt.Sprintf("Stopped docker container\nstdout:\n%s\nstderr:\n%s", stdout, stderr),
		zap.String("container_id", r.containerID),
		zap.String("container", c.Name),
	)

	return r.client.ContainerRemove(ctx, r.containerID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		// TODO: should this set Force=true?
	})
}

func (r *DockerRelayer) containerImage() ibc.DockerImage {
	if r.customImage != nil {
		return *r.customImage
	}
	return ibc.DockerImage{
		Repository: r.c.DefaultContainerImage(),
		Version:    r.c.DefaultContainerVersion(),
	}
}

func (r *DockerRelayer) pullContainerImageIfNecessary(containerImage ibc.DockerImage) error {
	if !r.pullImage {
		return nil
	}

	rc, err := r.client.ImagePull(context.TODO(), containerImage.Ref(), types.ImagePullOptions{})
	if err != nil {
		return err
	}

	_, _ = io.Copy(io.Discard, rc)
	_ = rc.Close()
	return nil
}

func (r *DockerRelayer) createNodeContainer(ctx context.Context, pathName string) error {
	containerImage := r.containerImage()
	containerName := fmt.Sprintf("%s-%s", r.c.Name(), pathName)
	cmd := r.c.StartRelayer(pathName, r.NodeHome())
	r.log.Info(
		"Running command",
		zap.String("command", strings.Join(cmd, " ")),
		zap.String("container", containerName),
	)
	cc, err := r.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: containerImage.Ref(),

			Entrypoint: []string{},
			Cmd:        cmd,

			Hostname: r.HostName(pathName),
			User:     dockerutil.GetDockerUserString(),

			Labels: map[string]string{dockerutil.CleanupLabel: r.testName},
		},
		&container.HostConfig{
			Binds:      r.Bind(),
			AutoRemove: false,
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				r.networkID: {},
			},
		},
		nil,
		containerName,
	)
	if err != nil {
		return err
	}

	r.containerID = cc.ID
	return dockerutil.StartContainer(ctx, r.client, r.containerID)
}

func (r *DockerRelayer) NodeJob(ctx context.Context, rep ibc.RelayerExecReporter, cmd []string) (
	exitCode int,
	stdout, stderr string,
	err error,
) {
	startedAt := time.Now()
	var containerName string
	defer func() {
		rep.TrackRelayerExec(
			containerName,
			cmd,
			stdout, stderr,
			exitCode,
			startedAt, time.Now(),
			err,
		)
	}()
	containerImage := r.containerImage()
	counter, _, _, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(counter).Name()
	funcName := strings.Split(caller, ".")
	containerName = fmt.Sprintf("%s-%s-%s", r.Name(), funcName[len(funcName)-1], dockerutil.RandLowerCaseLetterString(3))

	r.log.Info(
		"Running command",
		zap.String("command", strings.Join(cmd, " ")),
		zap.String("container", containerName),
	)

	cc, err := r.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: containerImage.Ref(),

			Entrypoint: []string{},
			Cmd:        cmd,

			// random hostname is fine here, just for setup
			Hostname: dockerutil.CondenseHostName(containerName),
			User:     dockerutil.GetDockerUserString(),

			Labels: map[string]string{dockerutil.CleanupLabel: r.testName},
		},
		&container.HostConfig{
			Binds:      r.Bind(),
			AutoRemove: false,
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				r.networkID: {},
			},
		},
		nil,
		containerName,
	)
	if err != nil {
		return 1, "", "", err
	}

	if err := dockerutil.StartContainer(ctx, r.client, cc.ID); err != nil {
		return 1, "", "", err
	}

	waitCh, errCh := r.client.ContainerWait(ctx, cc.ID, container.WaitConditionNextExit)
	select {
	case <-ctx.Done():
		return 1, "", "", ctx.Err()
	case err := <-errCh:
		return 1, "", "", err
	case res := <-waitCh:
		exitCode = int(res.StatusCode)
		if res.Error != nil {
			return exitCode, "", "", errors.New(res.Error.Message)
		}
	}

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	rc, err := r.client.ContainerLogs(ctx, cc.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "50",
	})
	if err != nil {
		return exitCode, "", "", fmt.Errorf("NodeJob: retrieving ContainerLogs: %w", err)
	}
	defer func() { _ = rc.Close() }()

	// Logs are multiplexed into one stream; see docs for ContainerLogs.
	_, err = stdcopy.StdCopy(stdoutBuf, stderrBuf, rc)
	if err != nil {
		return exitCode, "", "", fmt.Errorf("NodeJob: demuxing logs: %w", err)
	}
	_ = rc.Close()

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()
	if err := r.client.ContainerRemove(ctx, cc.ID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
	}); err != nil {
		return exitCode, "", "", fmt.Errorf("NodeJob: removing container: %w", err)
	}
	r.log.Debug(
		fmt.Sprintf("stdout:\n%s\nstderr:\n%s", stdout, stderr),
		zap.String("container", containerName),
	)
	return exitCode, stdout, stderr, err
}

func (r *DockerRelayer) stopContainer(ctx context.Context) error {
	timeout := 30 * time.Second
	return r.client.ContainerStop(ctx, r.containerID, &timeout)
}

func (r *DockerRelayer) Name() string {
	return r.c.Name() + "-" + dockerutil.SanitizeContainerName(r.testName)
}

// Bind returns the home folder bind point for running the node.
func (r *DockerRelayer) Bind() []string {
	return []string{r.volumeName + ":" + r.NodeHome()}
}

func (r *DockerRelayer) NodeHome() string {
	// Relayer writes to these files, so /var seems like a reasonable root.
	// https://tldp.org/LDP/Linux-Filesystem-Hierarchy/html/var.html
	return "/var/relayer-" + r.c.Name()
}

func (r *DockerRelayer) HostName(pathName string) string {
	return dockerutil.CondenseHostName(fmt.Sprintf("%s-%s", r.c.Name(), pathName))
}

func (r *DockerRelayer) UseDockerNetwork() bool {
	return true
}

// untarIntoNodeHome untars the given io.Reader into r's NodeHome() directory,
// and then sets chmod 777 recursively on NodeHome.
//
// There is likely a more intelligent way to create the tar archive
// that does not require chmod, but given the scope of these containers,
// this approach is simple enough for now.
func (r *DockerRelayer) untarIntoNodeHome(ctx context.Context, tar io.Reader) error {
	containerName := "untar-chmod-" + dockerutil.RandLowerCaseLetterString(5)
	cc, err := r.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: "busybox",

			Cmd: []string{"chmod", "-R", "777", r.NodeHome()},

			Hostname: r.HostName(containerName),
			User:     dockerutil.GetDockerUserString(),

			Labels: map[string]string{dockerutil.CleanupLabel: r.testName},
		},
		&container.HostConfig{
			Binds:      r.Bind(),
			AutoRemove: false,
		},
		nil, // No network config necessary.
		nil,
		containerName,
	)
	if err != nil {
		return fmt.Errorf("creating container for untar/chmod: %w", err)
	}
	defer func() {
		if err := r.client.ContainerRemove(ctx, cc.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
			r.log.Info(
				"Failed to remove tar container",
				zap.String("container_id", cc.ID),
				zap.Error(err),
			)
		}
	}()

	// It is safe to copy into the container before starting it.
	if err := r.client.CopyToContainer(
		ctx,
		cc.ID,
		r.NodeHome(),
		tar,
		types.CopyToContainerOptions{},
	); err != nil {
		return fmt.Errorf("copying tar to container: %w", err)
	}

	// Start the container, which will only run chmod -R 777 against r.NodeHome().
	if err := r.client.ContainerStart(ctx, cc.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	waitCh, errCh := r.client.ContainerWait(ctx, cc.ID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case res := <-waitCh:
		if res.Error != nil {
			return fmt.Errorf("waiting for container: %s", res.Error.Message)
		}
	}

	return nil
}

type RelayerCommander interface {
	// Name is the name of the relayer, e.g. "rly" or "hermes".
	Name() string

	DefaultContainerImage() string
	DefaultContainerVersion() string

	// ConfigContent generates the content of the config file that will be passed to AddChainConfiguration.
	ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error)

	// ParseAddKeyOutput processes the output of AddKey
	// to produce the wallet that was created.
	ParseAddKeyOutput(stdout, stderr string) (ibc.RelayerWallet, error)

	// ParseRestoreKeyOutput extracts the address from the output of RestoreKey.
	ParseRestoreKeyOutput(stdout, stderr string) string

	// ParseGetChannelsOutput processes the output of GetChannels
	// to produce the channel output values.
	ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error)

	// ParseGetConnectionsOutput processes the output of GetConnections
	// to produce the connection output values.
	ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error)

	// Init is the command to run on the first call to AddChainConfiguration.
	// If the returned command is nil or empty, nothing will be executed.
	Init(homeDir string) []string

	// The remaining methods produce the command to run inside the container.

	AddChainConfiguration(containerFilePath, homeDir string) []string
	AddKey(chainID, keyName, homeDir string) []string
	CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string
	CreateClients(pathName, homeDir string) []string
	CreateConnections(pathName, homeDir string) []string
	FlushAcknowledgements(pathName, channelID, homeDir string) []string
	FlushPackets(pathName, channelID, homeDir string) []string
	GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string
	GetChannels(chainID, homeDir string) []string
	GetConnections(chainID, homeDir string) []string
	LinkPath(pathName, homeDir string, opts ibc.CreateChannelOptions) []string
	RestoreKey(chainID, keyName, mnemonic, homeDir string) []string
	StartRelayer(pathName, homeDir string) []string
	UpdateClients(pathName, homeDir string) []string
}
