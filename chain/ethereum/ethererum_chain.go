package ethereum

import (
	"context"
	"fmt"
	"io"
	"runtime"

	sdkmath "cosmossdk.io/math"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/internal/dockerutil"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
)

var _ ibc.Chain = &EthereumChain{}

const (
	blockTime = 2 // seconds
	apiPort = "8545/tcp"
)

var natPorts = nat.PortSet{
	nat.Port(apiPort): {},
}

type EthereumChain struct {
	testName string
	cfg ibc.ChainConfig
	
	log *zap.Logger

	VolumeName string
	NetworkID string
	DockerClient *dockerclient.Client

	containerLifecycle *dockerutil.ContainerLifecycle

	hostAPIPort string

	genesisWallets GenesisWallets
}

func NewEthereumAnvilChainConfig(
	name string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type: "ethereum",
		Name: name,
		ChainID: "ethereum-1",
		Bech32Prefix: "eth",
		CoinType: "60",
		Denom: "eth",
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
		NetworkID: "", // Populated in Initialize
	}

}

func PanicFunctionName() {
	pc, _, _, _ := runtime. Caller(1)
	panic(runtime.FuncForPC(pc).Name() + " not implemented")
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
	// TODO Add configurations
	/*chainCfg := c.Config()

	decimalPow := int64(math.Pow10(int(*chainCfg.CoinDecimals)))

	genesisAmount := types.Coin{
		Amount: sdkmath.NewInt(10_000_000).MulRaw(decimalPow),
		Denom:  chainCfg.Denom,
	}

	genesisSelfDelegation := types.Coin{
		Amount: sdkmath.NewInt(5_000_000).MulRaw(decimalPow),
		Denom:  chainCfg.Denom,
	}

	if chainCfg.ModifyGenesisAmounts != nil {
		genesisAmount, genesisSelfDelegation = chainCfg.ModifyGenesisAmounts()
	}

	genesisAmounts := []types.Coin{genesisAmount}

	configFileOverrides := chainCfg.ConfigFileOverrides
	// configFile, modifiedConfig
	// modifiedToml

	// PreGenesis?

	// ModifyGenesis?
	
	*/

	cmd := []string{c.cfg.Bin, "--host", "0.0.0.0"}
	c.containerLifecycle.CreateContainer(ctx, c.testName, c.NetworkID, c.cfg.Images[0], natPorts, []string{}, dockerutil.CondenseHostName(c.Name()), cmd, nil)

	c.log.Info("Starting container", zap.String("container", c.Name()))

	if err := c.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := c.containerLifecycle.GetHostPorts(ctx, apiPort)
	if err != nil {
		return err
	}

	fmt.Println("Host ports: ", hostPorts)
	c.hostAPIPort = hostPorts[0]

	fmt.Println("Host API port: ", c.hostAPIPort)

	return nil
}
func (c *EthereumChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	PanicFunctionName()
	return nil, nil, nil
}
func (c *EthereumChain) ExportState(ctx context.Context, height int64) (string, error) {
	PanicFunctionName()
	return "", nil
}
func (c *EthereumChain) GetRPCAddress() string {
	PanicFunctionName()
	return ""
}
func (c *EthereumChain) GetGRPCAddress() string {
	PanicFunctionName()
	return ""
}
func (c *EthereumChain) GetHostRPCAddress() string {
	PanicFunctionName()
	return ""
}
func (c *EthereumChain) GetHostGRPCAddress() string {
	PanicFunctionName()
	return ""
}
func (c *EthereumChain) HomeDir() string {
	PanicFunctionName()
	return ""
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
func (c *EthereumChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	PanicFunctionName()
	return ibc.Tx{}, nil
}
func (c *EthereumChain) Height(ctx context.Context) (uint64, error) {
	PanicFunctionName()
	return 0, nil
}
func (c *EthereumChain) GetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	PanicFunctionName()
	return sdkmath.ZeroInt(), nil
}
func (c *EthereumChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	PanicFunctionName()
	return 0
}
func (c *EthereumChain) Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error) {
	PanicFunctionName()
	return nil, nil
}
func (c *EthereumChain) Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error) {
	PanicFunctionName()
	return nil, nil
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
func (c *EthereumChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	PanicFunctionName()
	return nil, nil
}