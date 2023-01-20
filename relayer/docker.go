package relayer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/strangelove-ventures/ibctest/v7/ibc"
	"github.com/strangelove-ventures/ibctest/v7/internal/dockerutil"
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
	wallets map[string]ibc.Wallet
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

		wallets: map[string]ibc.Wallet{},
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

	// The volume is created owned by root,
	// but we configure the relayer to run as a non-root user,
	// so set the node home (where the volume is mounted) to be owned
	// by the relayer user.
	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: r.log,

		Client: r.client,

		VolumeName: r.volumeName,
		ImageRef:   containerImage.Ref(),
		TestName:   testName,
		UidGid:     containerImage.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set volume owner: %w", err)
	}

	if init := r.c.Init(r.HomeDir()); len(init) > 0 {
		// Initialization should complete immediately,
		// but add a 1-minute timeout in case Docker hangs on a developer workstation.
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		// Using a nop reporter here because it keeps the API simpler,
		// and the init command is typically not of high interest.
		res := r.Exec(ctx, ibc.NopRelayerExecReporter{}, init, nil)
		if res.Err != nil {
			return nil, res.Err
		}
	}

	return &r, nil
}

func (r *DockerRelayer) AddChainConfiguration(ctx context.Context, rep ibc.RelayerExecReporter, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
	// For rly this file is json, but the file extension should not matter.
	// Using .config to avoid implying any particular format.
	chainConfigFile := chainConfig.ChainID + ".config"

	chainConfigContainerFilePath := path.Join(r.HomeDir(), chainConfigFile)

	configContent, err := r.c.ConfigContent(ctx, chainConfig, keyName, rpcAddr, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to generate config content: %w", err)
	}

	fw := dockerutil.NewFileWriter(r.log, r.client, r.testName)
	if err := fw.WriteFile(ctx, r.volumeName, chainConfigFile, configContent); err != nil {
		return fmt.Errorf("failed to rly config: %w", err)
	}

	cmd := r.c.AddChainConfiguration(chainConfigContainerFilePath, r.HomeDir())

	// Adding the chain configuration simply reads from a file on disk,
	// so this should also complete immediately.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) AddKey(ctx context.Context, rep ibc.RelayerExecReporter, chainID, keyName, coinType string) (ibc.Wallet, error) {
	cmd := r.c.AddKey(chainID, keyName, coinType, r.HomeDir())

	// Adding a key should be near-instantaneous, so add a 1-minute timeout
	// to detect if Docker has hung.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return nil, res.Err
	}

	wallet, err := r.c.ParseAddKeyOutput(string(res.Stdout), string(res.Stderr))
	if err != nil {
		return nil, err
	}
	r.wallets[chainID] = wallet
	return wallet, nil
}

func (r *DockerRelayer) GetWallet(chainID string) (ibc.Wallet, bool) {
	wallet, ok := r.wallets[chainID]
	return wallet, ok
}

func (r *DockerRelayer) CreateChannel(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, opts ibc.CreateChannelOptions) error {
	cmd := r.c.CreateChannel(pathName, opts, r.HomeDir())
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) CreateClients(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, opts ibc.CreateClientOptions) error {
	cmd := r.c.CreateClients(pathName, opts, r.HomeDir())
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) CreateConnections(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	cmd := r.c.CreateConnections(pathName, r.HomeDir())
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) FlushAcknowledgements(ctx context.Context, rep ibc.RelayerExecReporter, pathName, channelID string) error {
	cmd := r.c.FlushAcknowledgements(pathName, channelID, r.HomeDir())
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) FlushPackets(ctx context.Context, rep ibc.RelayerExecReporter, pathName, channelID string) error {
	cmd := r.c.FlushPackets(pathName, channelID, r.HomeDir())
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) GeneratePath(ctx context.Context, rep ibc.RelayerExecReporter, srcChainID, dstChainID, pathName string) error {
	cmd := r.c.GeneratePath(srcChainID, dstChainID, pathName, r.HomeDir())
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) UpdatePath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, filter ibc.ChannelFilter) error {
	cmd := r.c.UpdatePath(pathName, r.HomeDir(), filter)
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) GetChannels(ctx context.Context, rep ibc.RelayerExecReporter, chainID string) ([]ibc.ChannelOutput, error) {
	cmd := r.c.GetChannels(chainID, r.HomeDir())

	// Getting channels should be very quick, but go up to a 3-minute timeout just in case.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return nil, res.Err
	}

	return r.c.ParseGetChannelsOutput(string(res.Stdout), string(res.Stderr))
}

func (r *DockerRelayer) GetConnections(ctx context.Context, rep ibc.RelayerExecReporter, chainID string) (ibc.ConnectionOutputs, error) {
	cmd := r.c.GetConnections(chainID, r.HomeDir())
	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return nil, res.Err
	}

	return r.c.ParseGetConnectionsOutput(string(res.Stdout), string(res.Stderr))
}

func (r *DockerRelayer) GetClients(ctx context.Context, rep ibc.RelayerExecReporter, chainID string) (ibc.ClientOutputs, error) {
	cmd := r.c.GetClients(chainID, r.HomeDir())
	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return nil, res.Err
	}

	return r.c.ParseGetClientsOutput(string(res.Stdout), string(res.Stderr))
}

func (r *DockerRelayer) LinkPath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) error {
	cmd := r.c.LinkPath(pathName, r.HomeDir(), channelOpts, clientOpts)
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) Exec(ctx context.Context, rep ibc.RelayerExecReporter, cmd []string, env []string) ibc.RelayerExecResult {
	job := dockerutil.NewImage(r.log, r.client, r.networkID, r.testName, r.containerImage().Repository, r.containerImage().Version)
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: r.Bind(),
	}

	startedAt := time.Now()
	res := job.Run(ctx, cmd, opts)

	defer func() {
		rep.TrackRelayerExec(
			r.Name(),
			cmd,
			string(res.Stdout), string(res.Stderr),
			res.ExitCode,
			startedAt, time.Now(),
			res.Err,
		)
	}()

	return ibc.RelayerExecResult{
		Err:      res.Err,
		ExitCode: res.ExitCode,
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
	}
}

func (r *DockerRelayer) RestoreKey(ctx context.Context, rep ibc.RelayerExecReporter, chainID, keyName, coinType, mnemonic string) error {
	cmd := r.c.RestoreKey(chainID, keyName, coinType, mnemonic, r.HomeDir())

	// Restoring a key should be near-instantaneous, so add a 1-minute timeout
	// to detect if Docker has hung.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}

	addrBytes := r.c.ParseRestoreKeyOutput(string(res.Stdout), string(res.Stderr))

	r.wallets[chainID] = r.c.CreateWallet("", addrBytes, mnemonic)

	return nil
}

func (r *DockerRelayer) UpdateClients(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	cmd := r.c.UpdateClients(pathName, r.HomeDir())
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *DockerRelayer) StartRelayer(ctx context.Context, rep ibc.RelayerExecReporter, pathNames ...string) error {
	return r.createNodeContainer(ctx, pathNames...)
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
		UidGid:     r.c.DockerUser(),
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

func (r *DockerRelayer) createNodeContainer(ctx context.Context, pathNames ...string) error {
	containerImage := r.containerImage()
	joinedPaths := strings.Join(pathNames, ".")
	containerName := fmt.Sprintf("%s-%s", r.c.Name(), joinedPaths)
	cmd := r.c.StartRelayer(r.HomeDir(), pathNames...)
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

			Hostname: r.HostName(joinedPaths),
			User:     r.c.DockerUser(),

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

func (r *DockerRelayer) stopContainer(ctx context.Context) error {
	timeout := 30 * time.Second
	return r.client.ContainerStop(ctx, r.containerID, &timeout)
}

func (r *DockerRelayer) Name() string {
	return r.c.Name() + "-" + dockerutil.SanitizeContainerName(r.testName)
}

// Bind returns the home folder bind point for running the node.
func (r *DockerRelayer) Bind() []string {
	return []string{r.volumeName + ":" + r.HomeDir()}
}

// HomeDir returns the home directory of the relayer on the underlying Docker container's filesystem.
func (r *DockerRelayer) HomeDir() string {
	return "/home/relayer"
}

func (r *DockerRelayer) HostName(pathName string) string {
	return dockerutil.CondenseHostName(fmt.Sprintf("%s-%s", r.c.Name(), pathName))
}

func (r *DockerRelayer) UseDockerNetwork() bool {
	return true
}

type RelayerCommander interface {
	// Name is the name of the relayer, e.g. "rly" or "hermes".
	Name() string

	DefaultContainerImage() string
	DefaultContainerVersion() string

	// The Docker user to use in created container.
	// For ibctest, must be of the format: uid:gid.
	DockerUser() string

	// ConfigContent generates the content of the config file that will be passed to AddChainConfiguration.
	ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error)

	// ParseAddKeyOutput processes the output of AddKey
	// to produce the wallet that was created.
	ParseAddKeyOutput(stdout, stderr string) (ibc.Wallet, error)

	// ParseRestoreKeyOutput extracts the address from the output of RestoreKey.
	ParseRestoreKeyOutput(stdout, stderr string) string

	// ParseGetChannelsOutput processes the output of GetChannels
	// to produce the channel output values.
	ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error)

	// ParseGetConnectionsOutput processes the output of GetConnections
	// to produce the connection output values.
	ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error)

	// ParseGetClientsOutput processes the output of GetClients
	// to produce the client output values.
	ParseGetClientsOutput(stdout, stderr string) (ibc.ClientOutputs, error)

	// Init is the command to run on the first call to AddChainConfiguration.
	// If the returned command is nil or empty, nothing will be executed.
	Init(homeDir string) []string

	// The remaining methods produce the command to run inside the container.

	AddChainConfiguration(containerFilePath, homeDir string) []string
	AddKey(chainID, keyName, coinType, homeDir string) []string
	CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string
	CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string
	CreateConnections(pathName, homeDir string) []string
	FlushAcknowledgements(pathName, channelID, homeDir string) []string
	FlushPackets(pathName, channelID, homeDir string) []string
	GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string
	UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string
	GetChannels(chainID, homeDir string) []string
	GetConnections(chainID, homeDir string) []string
	GetClients(chainID, homeDir string) []string
	LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) []string
	RestoreKey(chainID, keyName, coinType, mnemonic, homeDir string) []string
	StartRelayer(homeDir string, pathNames ...string) []string
	UpdateClients(pathName, homeDir string) []string
	CreateWallet(keyName, address, mnemonic string) ibc.Wallet
}
