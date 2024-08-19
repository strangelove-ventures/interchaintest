package geth

import (
	"context"
	"fmt"
	"io"
	"time"

	"path"
	"strings"

	sdkmath "cosmossdk.io/math"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"go.uber.org/zap"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/common"
)

var _ ibc.Chain = &EthereumChain{}

const (
	blockTime = 2 // seconds
	rpcPort   = "8545/tcp"
)

var (
	GWEI  = sdkmath.NewInt(1_000_000_000)
	ETHER = GWEI.MulRaw(1_000_000_000)
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
	rpcClient *ethclient.Client

	keynameToAcctNum map[string]int
	acctNumToAddr map[int]string
	nextAcctNum int
}

func DefaultEthereumGethChainConfig(
	name string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "ethereum",
		Name:           name,
		ChainID:        "1337", // default geth chain-id
		Bech32Prefix:   "n/a",
		CoinType:       "60",
		Denom:          "wei",
		GasPrices:      "0",
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "ethereum/client-go",
				Version:    "v1.14.7",
				UidGid:     "1025:1025",
			},
		},
		Bin: "geth",
	}
}

func NewEthereumChain(testName string, chainConfig ibc.ChainConfig, log *zap.Logger) *EthereumChain {
	return &EthereumChain{
		testName:       testName,
		cfg:            chainConfig,
		log:            log,
		keynameToAcctNum: map[string]int{
			"faucet":0,
		},
		acctNumToAddr: make(map[int]string),
		nextAcctNum: 1,
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
	return fmt.Sprintf("geth-%s-%s", c.cfg.ChainID, dockerutil.SanitizeContainerName(c.testName))
}

func (c *EthereumChain) HomeDir() string {
	return "/home/geth"
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
	//   * add support for ConfigFileOverrides
	//		* block time
	//   * add support for custom gas-price

	cmd := []string{c.cfg.Bin,
		"--dev" ,"--dev.period", "2", "--verbosity", "4", "--networkid", "15", "--datadir", c.HomeDir(),
		"-http", "--http.addr", "0.0.0.0", "--http.port", "8545", "--allow-insecure-unlock",
		"--http.api", "eth,net,web3,miner,personal,txpool,debug", "--http.corsdomain", "\"*\"", "-nodiscover", "--http.vhosts=\"*\"",
	}

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

	err := c.containerLifecycle.CreateContainer(ctx, c.testName, c.NetworkID, c.cfg.Images[0], usingPorts, c.Bind(), []mount.Mount{}, c.HostName(), cmd, nil, []string{})
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

	// wait for rpc to come up
	time.Sleep(time.Second * 2)
	
	// dial the rpc host
	c.rpcClient, err = ethclient.Dial(c.GetHostRPCAddress())
	if err != nil {
		return fmt.Errorf("failed to dial ETH rpc host(%s): %w", c.GetHostRPCAddress(), err)
	}

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

func (c *EthereumChain) JavaScriptExec(ctx context.Context, jsCmd string) (stdout, stderr []byte, err error) {
	cmd := []string{
		c.cfg.Bin, "--exec", jsCmd, "--datadir", c.HomeDir(), "attach",
	}
	return c.Exec(ctx, cmd, nil)
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

func (c *EthereumChain) CreateKey(ctx context.Context, keyName string) error {
	_, ok := c.keynameToAcctNum[keyName]
	if ok {
		return fmt.Errorf("Keyname (%s) already used", keyName)
	}

	cmd := []string{
		"sh",
		"-c",
		fmt.Sprintf(`cat <<EOF | geth account new --datadir %s


EOF
`, c.HomeDir())}
	_, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}

	c.keynameToAcctNum[keyName] = c.nextAcctNum
	c.nextAcctNum++

	return nil
}

func (c *EthereumChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	panic("Geth, RecoverKey unimplemented")
	//return nil
}

// Get address of account, cast to a string to use
func (c *EthereumChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	accountNum, found := c.keynameToAcctNum[keyName]
	if !found {
		return nil, fmt.Errorf("GetAddress(): Keyname (%s) not found", keyName)
	}

	addr, found := c.acctNumToAddr[accountNum]
	if found {
		return hexutil.MustDecode(addr), nil
	}

	stdout, _, err := c.JavaScriptExec(ctx, fmt.Sprintf("eth.accounts[%d]", accountNum))
	if err != nil {
		return nil, err
	}
	
	for count := 0; strings.TrimSpace(string(stdout)) == "undefined"; count++ {
		time.Sleep(time.Second)
		stdout, _, err = c.JavaScriptExec(ctx, fmt.Sprintf("eth.accounts[%d]", accountNum))
		if err != nil {
			return nil, err
		}
		if count > 3 {
			return nil, fmt.Errorf("GetAddress(): Keyname (%s) with account (%d) not found", keyName, accountNum)
		}
	}

	return hexutil.MustDecode(strings.Trim(strings.TrimSpace(string(stdout)), "\"")), nil
}

func (c *EthereumChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	accountNum, found := c.keynameToAcctNum[keyName]
	if !found {
		return fmt.Errorf("SendFunds(): Keyname (%s) not found", keyName)
	}

	_, _, err := c.JavaScriptExec(ctx,
		fmt.Sprintf("eth.sendTransaction({from: eth.accounts[%d],to: %q,value: %s});", accountNum, amount.Address, amount.Amount),
	)
	if err != nil {
		return err
	}
	return testutil.WaitForBlocks(ctx, 1, c)
}

type TransactionReceipt struct {
	TxHash string `json:"transactionHash"`
}

func (c *EthereumChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	return "", nil
}

func (c *EthereumChain) Height(ctx context.Context) (int64, error) {
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

func (c *EthereumChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		err := c.RecoverKey(ctx, keyName, mnemonic)
		if err != nil {
			return nil, err
		}
	} else {
		// faucet is created when the chain starts and will be account #0
		if keyName == "faucet" {
			return NewWallet(keyName, []byte{}), nil
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
	return NewWallet(keyName, address), nil
}
