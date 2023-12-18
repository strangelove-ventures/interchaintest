package ethereum

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	sdkmath "cosmossdk.io/math"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"go.uber.org/zap"
)

var _ ibc.Chain = &EthereumChain{}

const (
	blockTime = 2 // seconds
	rpcPort = "8545/tcp"
)

var natPorts = nat.PortSet{
	nat.Port(rpcPort): {},
}

type EthereumChain struct {
	testName string
	cfg ibc.ChainConfig
	
	log *zap.Logger

	VolumeName string
	NetworkID string
	DockerClient *dockerclient.Client

	containerLifecycle *dockerutil.ContainerLifecycle

	hostRPCPort string

	genesisWallets GenesisWallets
}

func DefaultEthereumAnvilChainConfig(
	name string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type: "ethereum",
		Name: name,
		ChainID: "31337", // default anvil chain-id
		Bech32Prefix: "n/a",
		CoinType: "60",
		Denom: "wei",
		GasPrices: "0",
		GasAdjustment: 0,
		TrustingPeriod: "0",
		NoHostMount: false,
		Images: []ibc.DockerImage{
			{
				Repository: "ghcr.io/foundry-rs/foundry",
				Version: "latest",
				UidGid: "1000:1000",
			},
		},
		Bin: "anvil",
	}
}

func NewEthereumChain(testName string, chainConfig ibc.ChainConfig, log *zap.Logger) *EthereumChain {
	return &EthereumChain{
		testName: testName,
		cfg: chainConfig,
		log: log,
		genesisWallets: NewGenesisWallet(),
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
	c.VolumeName = v.Name
	c.NetworkID = networkID
	c.DockerClient = cli

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
	return fmt.Sprintf("%s-%s", c.cfg.ChainID, dockerutil.SanitizeContainerName(c.testName))
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

func (c *EthereumChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	// TODO: 
	//   * add support for different denom configuration, ether or wei, this will affect GetBalance, etc
	//   * add support for modifying genesis amount config, default is 10 ether
	//   * add support for ConfigFileOverrides
	//		* block time
	// 		* load state
	//   * add support for custom chain id, must be an int?
	//   * add support for custom gas-price
	// Maybe add code-size-limit configuration for larger contracts

	cmd := []string{c.cfg.Bin, "--host", "0.0.0.0", "--block-time", "2"}
	c.containerLifecycle.CreateContainer(ctx, c.testName, c.NetworkID, c.cfg.Images[0], natPorts, []string{}, c.HostName(), cmd, nil)

	c.log.Info("Starting container", zap.String("container", c.Name()))

	if err := c.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := c.containerLifecycle.GetHostPorts(ctx, rpcPort)
	if err != nil {
		return err
	}

	c.hostRPCPort = hostPorts[0]
	fmt.Println("Host RPC port: ", c.hostRPCPort)

	return testutil.WaitForBlocks(ctx, 2, c)
}

func (c *EthereumChain) HostName() string {
	return dockerutil.CondenseHostName(c.Name())
}

func (c *EthereumChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	job := dockerutil.NewImage(c.logger(), c.DockerClient, c.NetworkID, c.testName, c.cfg.Images[0].Repository, c.cfg.Images[0].Version)
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: []string{},
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (c *EthereumChain) logger() *zap.Logger {
	return c.log.With(
		zap.String("chain_id", c.cfg.ChainID),
		zap.String("test", c.testName),
	)
}

func (c *EthereumChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:8545", c.HostName())
}

func (c *EthereumChain) GetHostRPCAddress() string {
	return "http://" + c.hostRPCPort
}
func (c *EthereumChain) CreateKey(ctx context.Context, keyName string) error {
	PanicFunctionName()
	return nil
}
func (c *EthereumChain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	PanicFunctionName()
	return nil
}
func (c *EthereumChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	PanicFunctionName()
	return nil, nil
}
func (c *EthereumChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	PanicFunctionName()
	return nil
}

func (c *EthereumChain) Height(ctx context.Context) (uint64, error) {
	cmd := []string{"cast", "block-number", "--rpc-url", c.GetRPCAddress()}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(stdout)), 10, 64)
}

func (c *EthereumChain) GetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	cmd := []string{"cast", "balance", "--rpc-url", c.GetRPCAddress(), address}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return sdkmath.ZeroInt(), err
	}
	balance, ok := sdkmath.NewIntFromString(strings.TrimSpace(string(stdout)))
	if !ok {
		return sdkmath.ZeroInt(), fmt.Errorf("Error parsing string to sdk int")
	}
	return balance, nil
}

func (c *EthereumChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	PanicFunctionName()
	return 0
}

func (c *EthereumChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		// TODO Add support for a new wallet using mnemonic
		PanicFunctionName()
	} else {
		// Use a genesis account
		return c.genesisWallets.GetUnusedWallet(keyName), nil
	}
	return nil, nil
}