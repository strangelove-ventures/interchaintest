package utxo

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"go.uber.org/zap"

	sdkmath "cosmossdk.io/math"
)

var _ ibc.Chain = &UtxoChain{}

const (
	blockTime                 = 2 // seconds
	rpcPort                   = "18443/tcp"
	noDefaultKeyWalletVersion = 159_900
	namedFixWalletVersion     = 169_901
	faucetKeyName             = "faucet"
)

var natPorts = nat.PortMap{
	nat.Port(rpcPort): {},
}

type UtxoChain struct {
	testName string
	cfg      ibc.ChainConfig

	log *zap.Logger

	VolumeName   string
	NetworkID    string
	DockerClient *dockerclient.Client

	containerLifecycle *dockerutil.ContainerLifecycle

	hostRPCPort string

	// cli arguments
	BinDaemon          string
	BinCli             string
	RPCUser            string
	RPCPassword        string
	BaseCli            []string
	AddrToKeyNameMap   map[string]string
	KeyNameToWalletMap map[string]*NodeWallet

	WalletVersion        int
	unloadWalletAfterUse bool
}

func NewUtxoChain(testName string, chainConfig ibc.ChainConfig, log *zap.Logger) *UtxoChain {
	bins := strings.Split(chainConfig.Bin, ",")
	if len(bins) != 2 {
		panic(fmt.Sprintf("%s chain must set the daemon and cli binaries (i.e. appd,app-cli)", chainConfig.Name))
	}
	rpcUser := ""
	rpcPassword := ""
	for _, arg := range chainConfig.AdditionalStartArgs {
		if strings.Contains(arg, "-rpcuser") {
			rpcUser = arg
		}
		if strings.Contains(arg, "-rpcpassword") {
			rpcPassword = arg
		}
	}
	if rpcUser == "" || rpcPassword == "" {
		panic(fmt.Sprintf("%s chain must have -rpcuser and -rpcpassword set in config's AdditionalStartArgs", chainConfig.Name))
	}

	return &UtxoChain{
		testName:             testName,
		cfg:                  chainConfig,
		log:                  log,
		BinDaemon:            bins[0],
		BinCli:               bins[1],
		RPCUser:              rpcUser,
		RPCPassword:          rpcPassword,
		AddrToKeyNameMap:     make(map[string]string),
		KeyNameToWalletMap:   make(map[string]*NodeWallet),
		unloadWalletAfterUse: false,
	}
}

func (c *UtxoChain) Config() ibc.ChainConfig {
	return c.cfg
}

func (c *UtxoChain) Initialize(ctx context.Context, testName string, cli *dockerclient.Client, networkID string) error {
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
		UidGid:     image.UIDGID,
	}); err != nil {
		return fmt.Errorf("set volume owner: %w", err)
	}

	return nil
}

func (c *UtxoChain) Name() string {
	return fmt.Sprintf("utxo-%s-%s", c.cfg.ChainID, dockerutil.SanitizeContainerName(c.testName))
}

func (c *UtxoChain) HomeDir() string {
	return "/home/utxo"
}

func (c *UtxoChain) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", c.VolumeName, c.HomeDir())}
}

func (c *UtxoChain) pullImages(ctx context.Context, cli *dockerclient.Client) {
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

func (c *UtxoChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	cmd := []string{
		c.BinDaemon,
		"--regtest",
		"-printtoconsole",
		"-regtest=1",
		"-txindex",
		"-rpcallowip=0.0.0.0/0",
		"-rpcbind=0.0.0.0",
		"-deprecatedrpc=create_bdb",
		"-rpcport=18443",
	}

	cmd = append(cmd, c.cfg.AdditionalStartArgs...)

	usingPorts := nat.PortMap{}
	for k, v := range natPorts {
		usingPorts[k] = v
	}

	if c.cfg.HostPortOverride != nil {
		var fields []zap.Field

		i := 0
		for intP, extP := range c.cfg.HostPortOverride {
			port := nat.Port(fmt.Sprintf("%d/tcp", intP))

			usingPorts[port] = []nat.PortBinding{
				{
					HostPort: fmt.Sprintf("%d", extP),
				},
			}

			fields = append(fields, zap.String(fmt.Sprintf("port_overrides_%d", i), fmt.Sprintf("%s:%d", port, extP)))
			i++
		}

		c.log.Info("Port overrides", fields...)
	}

	env := []string{}
	if c.cfg.Images[0].UIDGID != "" {
		uidGID := strings.Split(c.cfg.Images[0].UIDGID, ":")
		if len(uidGID) != 2 {
			panic(fmt.Sprintf("%s chain does not have valid UidGid", c.cfg.Name))
		}
		env = []string{
			fmt.Sprintf("UID=%s", uidGID[0]),
			fmt.Sprintf("GID=%s", uidGID[1]),
		}
	}

	entrypoint := []string{"/entrypoint.sh"}
	if c.cfg.Images[0].Repository == "registry.gitlab.com/thorchain/devops/node-launcher" { // these images don't have "/entrypoint.sh"
		entrypoint = []string{}
		cmd = append(cmd, fmt.Sprintf("--datadir=%s", c.HomeDir()))
	}

	err := c.containerLifecycle.CreateContainer(ctx, c.testName, c.NetworkID, c.cfg.Images[0],
		usingPorts, c.Bind(), []mount.Mount{}, c.HostName(), cmd, env, entrypoint)
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

	c.hostRPCPort = strings.Split(hostPorts[0], ":")[1]

	c.BaseCli = []string{
		c.BinCli,
		"-regtest",
		c.RPCUser,
		c.RPCPassword,
		fmt.Sprintf("-rpcconnect=%s", c.HostName()),
		"-rpcport=18443",
	}

	// Wait for rpc to come up
	time.Sleep(time.Second * 5)

	go func() {
		ctx := context.Background()
		amount := "100"
		nextBlockHeight := 100
		if c.cfg.CoinType == "3" {
			amount = "1000" // Dogecoin needs more blocks for more coins
			nextBlockHeight = 1000
		}
		for {
			faucetWallet, found := c.KeyNameToWalletMap[faucetKeyName]
			if !found || !faucetWallet.ready {
				time.Sleep(time.Second)
				continue
			}

			// If faucet exists, chain is up and running. Any future error should return from this go routine.
			// If the chain stops, we will then error and return from this go routine
			// Don't use ctx from Start(), it gets cancelled soon after returning.
			cmd = append(c.BaseCli, "generatetoaddress", amount, faucetWallet.address)
			_, _, err = c.Exec(ctx, cmd, nil)
			if err != nil {
				c.logger().Error("generatetoaddress error", zap.Error(err))
				return
			}
			amount = "1"
			if nextBlockHeight == 431 && c.cfg.CoinType == "2" {
				keyName := "mweb"
				if err := c.CreateWallet(ctx, keyName); err != nil {
					c.logger().Error("error creating mweb wallet at block 431", zap.String("chain", c.cfg.ChainID), zap.Error(err))
					return
				}
				addr, err := c.GetNewAddress(ctx, keyName, true)
				if err != nil {
					c.logger().Error("error creating mweb wallet at block 431", zap.String("chain", c.cfg.ChainID), zap.Error(err))
					return
				}
				if err := c.sendToMwebAddress(ctx, faucetKeyName, addr, 1); err != nil {
					c.logger().Error("error sending to mweb wallet at block 431", zap.String("chain", c.cfg.ChainID), zap.Error(err))
					return
				}
			}
			nextBlockHeight++
			time.Sleep(time.Second * 2)
		}
	}()

	c.WalletVersion, _ = c.GetWalletVersion(ctx, "")

	if err := c.CreateWallet(ctx, faucetKeyName); err != nil {
		return err
	}

	if c.WalletVersion == 0 {
		c.WalletVersion, err = c.GetWalletVersion(ctx, faucetKeyName)
		if err != nil {
			return err
		}
	}

	addr, err := c.GetNewAddress(ctx, faucetKeyName, false)
	if err != nil {
		return err
	}

	if err := c.SetAccount(ctx, addr, faucetKeyName); err != nil {
		return err
	}

	// Wait for 100 blocks to be created, coins mature after 100 blocks and the faucet starts getting 50 spendable coins/block onwards
	// Then wait the standard 2 blocks which also gives the faucet a starting balance of 100 coins
	for height, err := c.Height(ctx); err == nil && height < int64(102); {
		time.Sleep(time.Second)
		height, err = c.Height(ctx)
	}
	return err
}

func (c *UtxoChain) HostName() string {
	return dockerutil.CondenseHostName(c.Name())
}

func (c *UtxoChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	logger := zap.NewNop()
	if cmd[len(cmd)-1] != "getblockcount" && cmd[len(cmd)-3] != "generatetoaddress" { // too much logging, maybe switch to an rpc lib in the future
		logger = c.logger()
	}
	job := dockerutil.NewImage(logger, c.DockerClient, c.NetworkID, c.testName, c.cfg.Images[0].Repository, c.cfg.Images[0].Version)
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: c.Bind(),
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (c *UtxoChain) logger() *zap.Logger {
	return c.log.With(
		zap.String("chain_id", c.cfg.ChainID),
		zap.String("test", c.testName),
	)
}

func (c *UtxoChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:18443", c.HostName())
}

func (c *UtxoChain) GetWSAddress() string {
	return fmt.Sprintf("ws://%s:18443", c.HostName())
}

func (c *UtxoChain) GetHostRPCAddress() string {
	return "http://0.0.0.0:" + c.hostRPCPort
}

func (c *UtxoChain) GetHostWSAddress() string {
	return "ws://0.0.0.0:" + c.hostRPCPort
}

func (c *UtxoChain) CreateKey(ctx context.Context, keyName string) error {
	if keyName == "faucet" {
		// chain has not started, cannot create wallet yet. Faucet will be created in Start().
		return nil
	}

	if err := c.CreateWallet(ctx, keyName); err != nil {
		return err
	}

	addr, err := c.GetNewAddress(ctx, keyName, false)
	if err != nil {
		return err
	}

	return c.SetAccount(ctx, addr, keyName)
}

// Get address of account, cast to a string to use.
func (c *UtxoChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	wallet, ok := c.KeyNameToWalletMap[keyName]
	if ok {
		return []byte(wallet.address), nil
	}

	// Pre-start GetAddress doesn't matter
	if keyName == "faucet" {
		return []byte(keyName), nil
	}

	return nil, fmt.Errorf("keyname: %s's address not found", keyName)
}

func (c *UtxoChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	_, err := c.SendFundsWithNote(ctx, keyName, amount, "")
	return err
}

func (c *UtxoChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	partialCoin := amount.Amount.ModRaw(int64(math.Pow10(int(*c.Config().CoinDecimals))))
	fullCoins := amount.Amount.Sub(partialCoin).QuoRaw(int64(math.Pow10(int(*c.Config().CoinDecimals))))
	sendAmountFloat := float64(fullCoins.Int64()) + float64(partialCoin.Int64())/math.Pow10(int(*c.Config().CoinDecimals))

	if err := c.LoadWallet(ctx, keyName); err != nil {
		return "", err
	}

	wallet, err := c.getWalletForUse(keyName)
	if err != nil {
		return "", err
	}
	wallet.txLock.Lock()
	defer wallet.txLock.Unlock()

	// get utxo
	listUtxo, err := c.ListUnspent(ctx, keyName)
	if err != nil {
		return "", err
	}

	rawTxHex, err := c.CreateRawTransaction(ctx, keyName, listUtxo, amount.Address, sendAmountFloat, []byte(note))
	if err != nil {
		return "", err
	}

	// sign raw transaction
	signedRawTxHex, err := c.SignRawTransaction(ctx, keyName, rawTxHex)
	if err != nil {
		return "", err
	}

	// send raw transaction
	txHash, err := c.SendRawTransaction(ctx, signedRawTxHex)
	if err != nil {
		return "", err
	}

	if err := c.UnloadWallet(ctx, keyName); err != nil {
		return "", err
	}

	err = testutil.WaitForBlocks(ctx, 1, c)
	return txHash, err
}

func (c *UtxoChain) Height(ctx context.Context) (int64, error) {
	time.Sleep(time.Millisecond * 200) // TODO: slow down WaitForBlocks instead of here
	cmd := append(c.BaseCli, "getblockcount")
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(string(stdout)), 10, 64)
}

func (c *UtxoChain) GetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	keyName, ok := c.AddrToKeyNameMap[address]
	if !ok {
		return sdkmath.Int{}, fmt.Errorf("wallet not found for address: %s", address)
	}

	var coinsWithDecimal float64
	if c.WalletVersion >= noDefaultKeyWalletVersion {
		if err := c.LoadWallet(ctx, keyName); err != nil {
			return sdkmath.Int{}, err
		}
		cmd := append(c.BaseCli, fmt.Sprintf("-rpcwallet=%s", keyName), "getbalance")
		stdout, _, err := c.Exec(ctx, cmd, nil)
		if err != nil {
			return sdkmath.Int{}, err
		}
		if err := c.UnloadWallet(ctx, keyName); err != nil {
			return sdkmath.Int{}, err
		}
		balance := strings.TrimSpace(string(stdout))
		coinsWithDecimal, err = strconv.ParseFloat(balance, 64)
		if err != nil {
			return sdkmath.Int{}, err
		}
	} else {
		listUtxo, err := c.ListUnspent(ctx, keyName)
		if err != nil {
			return sdkmath.Int{}, err
		}

		for _, utxo := range listUtxo {
			if utxo.Spendable {
				coinsWithDecimal += utxo.Amount
			}
		}
	}

	coinsScaled := int64(coinsWithDecimal * math.Pow10(int(*c.Config().CoinDecimals)))
	return sdkmath.NewInt(coinsScaled), nil
}

func (c *UtxoChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		err := c.RecoverKey(ctx, keyName, mnemonic)
		if err != nil {
			return nil, err
		}
	} else {
		// Create new account
		err := c.CreateKey(ctx, keyName)
		if err != nil {
			return nil, err
		}
	}

	address, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, err
	}
	return NewWallet(keyName, string(address)), nil
}
