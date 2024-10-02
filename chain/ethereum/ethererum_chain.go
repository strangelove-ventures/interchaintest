package ethereum

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/avast/retry-go/v4"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"go.uber.org/zap"

	sdkmath "cosmossdk.io/math"
)

const (
	rpcPort = "8545/tcp"
)

var natPorts = nat.PortMap{
	nat.Port(rpcPort): {},
}

var (
	GWEI  = sdkmath.NewInt(1_000_000_000)
	ETHER = GWEI.MulRaw(1_000_000_000)
)

type EthereumChain struct {
	testName string
	cfg      ibc.ChainConfig

	log *zap.Logger

	volumeName   string
	networkID    string
	dockerClient *dockerclient.Client

	containerLifecycle *dockerutil.ContainerLifecycle

	hostRPCPort string
	rpcClient   *ethclient.Client
}

func NewEthereumChain(testName string, chainConfig ibc.ChainConfig, log *zap.Logger) *EthereumChain {
	return &EthereumChain{
		testName: testName,
		cfg:      chainConfig,
		log:      log,
	}
}

func (c *EthereumChain) Config() ibc.ChainConfig {
	return c.cfg
}

func (c *EthereumChain) Initialize(ctx context.Context, testName string, cli *dockerclient.Client, networkID string) error {
	chainCfg := c.Config()
	c.pullImages(ctx, cli)
	image := chainCfg.Images[0]

	c.containerLifecycle = dockerutil.NewContainerLifecycle(c.log, cli, c.Name())

	v, err := cli.VolumeCreate(ctx, volume.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel: testName,

			dockerutil.NodeOwnerLabel: c.Name(),
		},
	})
	if err != nil {
		return fmt.Errorf("creating volume for chain node: %w", err)
	}
	c.volumeName = v.Name
	c.networkID = networkID
	c.dockerClient = cli

	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: c.log,

		Client: cli,

		VolumeName: v.Name,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UidGid,
	}); err != nil {
		return fmt.Errorf("set volume owner: %w", err)
	}

	return nil
}

func (c *EthereumChain) Name() string {
	return fmt.Sprintf("%s-%s-%s-%s", c.cfg.Name, c.cfg.Bin, c.cfg.ChainID, dockerutil.SanitizeContainerName(c.testName))
}

func (c *EthereumChain) HomeDir() string {
	return "/home/ethereum"
}

func (c *EthereumChain) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", c.volumeName, c.HomeDir())}
}

func (c *EthereumChain) pullImages(ctx context.Context, cli *dockerclient.Client) {
	for _, image := range c.Config().Images {
		rc, err := cli.ImagePull(
			ctx,
			image.Repository+":"+image.Version,
			dockertypes.ImagePullOptions{},
		)
		if err != nil {
			c.log.Error("Failed to pull image",
				zap.Error(err),
				zap.String("repository", image.Repository),
				zap.String("tag", image.Version),
			)
		} else {
			_, _ = io.Copy(io.Discard, rc)
			_ = rc.Close()
		}
	}
}

func (c *EthereumChain) Start(ctx context.Context, cmd []string, mount []mount.Mount) error {
	usingPorts := nat.PortMap{}
	for k, v := range natPorts {
		usingPorts[k] = v
	}

	if c.cfg.HostPortOverride != nil {
		for intP, extP := range c.cfg.HostPortOverride {
			usingPorts[nat.Port(fmt.Sprintf("%d/tcp", intP))] = []nat.PortBinding{
				{
					HostPort: fmt.Sprintf("%d", extP),
				},
			}
		}

		fmt.Printf("Port Overrides: %v. Using: %v\n", c.cfg.HostPortOverride, usingPorts)
	}

	err := c.containerLifecycle.CreateContainer(ctx, c.testName, c.networkID, c.cfg.Images[0], usingPorts, c.Bind(), mount, c.HostName(), cmd, nil, []string{})
	if err != nil {
		return err
	}

	c.log.Info("Starting container", zap.String("container", c.Name()))

	if err := c.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := c.containerLifecycle.GetHostPorts(ctx, rpcPort)
	if err != nil {
		return err
	}

	c.hostRPCPort = hostPorts[0]

	c.rpcClient, err = ethclient.Dial(c.GetRPCAddress())
	if err != nil {
		return fmt.Errorf("failed to dial ETH rpc: %w", err)
	}

	// Wait for RPC to be available
	if err := retry.Do(func() error {
		_, err := c.rpcClient.ChainID(ctx)
		return err
	}, retry.Attempts(10), retry.Delay(time.Second*2)); err != nil {
		return fmt.Errorf("rpc unreachable after max attempts (%s): %w", c.GetHostRPCAddress(), err)
	}

	return testutil.WaitForBlocks(ctx, 2, c)
}

func (c *EthereumChain) HostName() string {
	return dockerutil.CondenseHostName(c.Name())
}

func (c *EthereumChain) NewJob() *dockerutil.Image {
	return dockerutil.NewImage(c.Logger(), c.dockerClient, c.networkID, c.testName, c.cfg.Images[0].Repository, c.cfg.Images[0].Version)
}

func (c *EthereumChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	job := c.NewJob()
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: c.Bind(),
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (c *EthereumChain) Logger() *zap.Logger {
	return c.log.With(
		zap.String("chain_id", c.cfg.ChainID),
		zap.String("test", c.testName),
	)
}

func (c *EthereumChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:8545", c.HostName())
}

func (c *EthereumChain) GetWSAddress() string {
	return fmt.Sprintf("ws://%s:8545", c.HostName())
}

func (c *EthereumChain) GetHostRPCAddress() string {
	return "http://" + c.hostRPCPort
}

func (c *EthereumChain) GetHostWSAddress() string {
	return "ws://" + c.hostRPCPort
}

func (c *EthereumChain) Height(ctx context.Context) (int64, error) {
	time.Sleep(time.Millisecond * 200) // TODO: slow down WaitForBlocks instead of here
	height, err := c.rpcClient.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get height: %w", err)
	}
	return int64(height), nil
}

func (c *EthereumChain) GetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	balance, err := c.rpcClient.BalanceAt(ctx, common.Address(hexutil.MustDecode(address)), nil)
	if err != nil {
		return sdkmath.Int{}, fmt.Errorf("failed to get height: %w", err)
	}
	return sdkmath.NewIntFromBigInt(balance), nil
}
