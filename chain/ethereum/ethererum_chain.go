package ethereum

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	sdkmath "cosmossdk.io/math"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
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
	rpcPort   = "8545/tcp"
	GWEI      = 1_000_000_000
	ETHER     = 1_000_000_000 * GWEI
)

var natPorts = nat.PortMap{
	nat.Port(rpcPort): {},
}

type EthereumChain struct {
	testName string
	cfg      ibc.ChainConfig

	log *zap.Logger

	VolumeName   string
	NetworkID    string
	DockerClient *dockerclient.Client

	containerLifecycle *dockerutil.ContainerLifecycle

	hostRPCPort string

	genesisWallets GenesisWallets

	keystoreMap map[string]string
}

func DefaultEthereumAnvilChainConfig(
	name string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "ethereum",
		Name:           name,
		ChainID:        "31337", // default anvil chain-id
		Bech32Prefix:   "n/a",
		CoinType:       "60",
		Denom:          "wei",
		GasPrices:      "0",
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "ghcr.io/foundry-rs/foundry",
				Version:    "latest",
				UidGid:     "1000:1000",
			},
		},
		Bin: "anvil",
	}
}

func NewEthereumChain(testName string, chainConfig ibc.ChainConfig, log *zap.Logger) *EthereumChain {
	return &EthereumChain{
		testName:       testName,
		cfg:            chainConfig,
		log:            log,
		genesisWallets: NewGenesisWallet(),
		keystoreMap:    make(map[string]string),
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
	return fmt.Sprintf("anvil-%s-%s", c.cfg.ChainID, dockerutil.SanitizeContainerName(c.testName))
}

func (c *EthereumChain) HomeDir() string {
	return "/home/foundry"
}

func (c *EthereumChain) KeystoreDir() string {
	return path.Join(c.HomeDir(), ".foundry", "keystores")
}

func (c *EthereumChain) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", c.VolumeName, c.HomeDir())}
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
	//   * add support for custom chain id, must be an int?
	//   * add support for custom gas-price
	// Maybe add code-size-limit configuration for larger contracts

	// IBC support, add when necessary
	//   * add additionalGenesisWallet support for relayer wallet, either add genesis accounts or tx after chain starts

	cmd := []string{c.cfg.Bin,
		"--host", "0.0.0.0", // Anyone can call
		"--block-time", "2", // 2 second block times
		"--accounts", "10", // We current only use the first account for the faucet, but tests may expect the default
		"--balance", "10000000", // Genesis accounts loaded with 10mil ether, change as needed
	}

	var mounts []mount.Mount
	if loadState, ok := c.cfg.ConfigFileOverrides["--load-state"].(string); ok {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		localJsonFile := filepath.Join(pwd, loadState)
		dockerJsonFile := path.Join(c.HomeDir(), path.Base(loadState))
		mounts = []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: localJsonFile,
				Target: dockerJsonFile,
			},
		}
		cmd = append(cmd, "--load-state", dockerJsonFile)
	}

	usingPorts := natPorts

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

	err := c.containerLifecycle.CreateContainer(ctx, c.testName, c.NetworkID, c.cfg.Images[0], usingPorts, c.Bind(), mounts, c.HostName(), cmd, nil)
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
		Binds: c.Bind(),
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

func (c *EthereumChain) GetWSAddress() string {
	return fmt.Sprintf("ws://%s:8545", c.HostName())
}

func (c *EthereumChain) GetHostRPCAddress() string {
	return "http://" + c.hostRPCPort
}

func (c *EthereumChain) GetHostWSAddress() string {
	return "ws://" + c.hostRPCPort
}

type NewWalletOutput struct {
	Address string `json:"address"`
	Path    string `json:"path"`
}

func (c *EthereumChain) MakeKeystoreDir(ctx context.Context) error {
	cmd := []string{"mkdir", "-p", c.KeystoreDir()}
	_, _, err := c.Exec(ctx, cmd, nil)
	return err
}

func (c *EthereumChain) CreateKey(ctx context.Context, keyName string) error {
	err := c.MakeKeystoreDir(ctx) // Ensure keystore directory is created
	if err != nil {
		return err
	}

	cmd := []string{"cast", "wallet", "new", c.KeystoreDir(), "--unsafe-password", "", "--json"}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}

	newWallet := []NewWalletOutput{}
	err = json.Unmarshal(stdout, &newWallet)
	if err != nil {
		return err
	}

	c.keystoreMap[keyName] = newWallet[0].Path

	return nil
}

// Get address of account, cast to a string to use
func (c *EthereumChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {

	cmd := []string{"cast", "wallet", "address", "--keystore", c.keystoreMap[keyName], "--password", ""}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(string(stdout))), nil
}

func (c *EthereumChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	cmd := []string{"cast", "send", amount.Address, "--value", amount.Amount.String()}
	if keyName == "faucet" {
		cmd = append(cmd,
			"--private-key", "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			"--rpc-url", c.GetRPCAddress(),
		)

	} else {
		cmd = append(cmd,
			"--keystore", c.keystoreMap[keyName],
			"--password", "",
			"--rpc-url", c.GetRPCAddress(),
		)
	}
	_, _, err := c.Exec(ctx, cmd, nil)
	return err
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

func (c *EthereumChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		err := c.RecoverKey(ctx, keyName, mnemonic)
		if err != nil {
			return nil, err
		}
	} else {
		// Use the genesis account
		if keyName == "faucet" {
			// TODO: implement RecoverKey() so faucet can be saved to keystore
			return c.genesisWallets.GetFaucetWallet(keyName), nil
		} else {
			// Create new account
			err := c.CreateKey(ctx, keyName)
			if err != nil {
				return nil, err
			}
		}
	}

	address, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, err
	}
	return NewWallet(keyName, string(address)), nil
}
