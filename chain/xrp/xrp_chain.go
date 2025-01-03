package xrp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"

	sdkmath "cosmossdk.io/math"

	xrpclient "github.com/strangelove-ventures/interchaintest/v8/chain/xrp/client"
	xrpwallet "github.com/strangelove-ventures/interchaintest/v8/chain/xrp/wallet"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Chain = &XrpChain{}

const (
	//blockTime         = 2 // seconds
	rpcAdminLocalPort = "5005/tcp"
	wsAdminLocalPort  = "6006/tcp"
	wsPublicPort      = "80/tcp"
	peerPort          = "51235/tcp"
	rpcPort           = "51234/tcp"
	faucetKeyName     = "faucet"
)

var natPorts = nat.PortMap{
	nat.Port(rpcAdminLocalPort): {},
	nat.Port(wsAdminLocalPort): {},
	nat.Port(wsPublicPort): {},
	nat.Port(peerPort): {},
	nat.Port(rpcPort): {},
}

type XrpChain struct {
	testName string
	cfg      ibc.ChainConfig
	cancel   context.CancelFunc

	log *zap.Logger

	VolumeName   string
	NetworkID    string
	DockerClient *dockerclient.Client

	containerLifecycle *dockerutil.ContainerLifecycle

	hostRPCPort string
	hostWSPort string

	xrpClient *xrpclient.XrpClient

	// cli arguments
	RippledCli         string
	ValidatorKeysCli   string
	AddrToKeyNameMap   map[string]string
	KeyNameToWalletMap map[string]*xrpwallet.XrpWallet

	ValidatorKeyInfo *ValidatorKeyOutput
	ValidatorToken string

	// Mutex for reading/writing AddrToKeyNameMap and KeyNameToWalletMap
	MapAccess sync.Mutex
}

func NewXrpChain(testName string, chainConfig ibc.ChainConfig, log *zap.Logger) *XrpChain {
	bins := strings.Split(chainConfig.Bin, ",")
	if len(bins) != 2 {
		panic(fmt.Sprintf("%s chain must set the daemon and cli binaries (i.e. appd,app-cli)", chainConfig.Name))
	}

	return &XrpChain{
		testName:             testName,
		cfg:                  chainConfig,
		log:                  log,
		RippledCli:           bins[0],
		ValidatorKeysCli:     bins[1],
		AddrToKeyNameMap:     make(map[string]string),
		KeyNameToWalletMap:   make(map[string]*xrpwallet.XrpWallet),
	}
}

func (c *XrpChain) Config() ibc.ChainConfig {
	return c.cfg
}

func (c *XrpChain) Initialize(ctx context.Context, testName string, cli *dockerclient.Client, networkID string) error {
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

func (c *XrpChain) Name() string {
	return fmt.Sprintf("xrp-%s-%s", c.cfg.ChainID, dockerutil.SanitizeContainerName(c.testName))
}

func (c *XrpChain) HomeDir() string {
	return "/home/xrp"
}

func (c *XrpChain) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", c.VolumeName, c.HomeDir())}
}

func (c *XrpChain) pullImages(ctx context.Context, cli *dockerclient.Client) {
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

func (c *XrpChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
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

	// env := []string{}
	// if c.cfg.Images[0].UIDGID != "" {
	// 	uidGID := strings.Split(c.cfg.Images[0].UIDGID, ":")
	// 	if len(uidGID) != 2 {
	// 		panic(fmt.Sprintf("%s chain does not have valid UidGid", c.cfg.Name))
	// 	}
	// 	env = []string{
	// 		fmt.Sprintf("UID=%s", uidGID[0]),
	// 		fmt.Sprintf("GID=%s", uidGID[1]),
	// 	}
	// }
	if err := c.CreateRippledConfig(ctx); err != nil {
		return err
	}

	//entrypoint := []string{"/entrypoint.sh"}
	entrypoint := []string{}
	cmd := []string{
		c.RippledCli,
		"--conf", fmt.Sprintf("%s/config/rippled.cfg", c.HomeDir()),
		"--standalone",
	}
	err := c.containerLifecycle.CreateContainer(ctx, c.testName, c.NetworkID, c.cfg.Images[0],
		usingPorts, "", c.Bind(), []mount.Mount{}, c.HostName(), cmd, c.cfg.Env, entrypoint)
	if err != nil {
		return err
	}

	c.log.Info("Starting container", zap.String("container", c.Name()))

	if err := c.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	// hostPorts, err := c.containerLifecycle.GetHostPorts(ctx, rpcPort, wsPublicPort)
	// if err != nil {
	// 	return err
	// }

	// c.hostRPCPort = strings.Split(hostPorts[0], ":")[1]
	// c.hostWSPort = strings.Split(hostPorts[1], ":")[1]
	c.hostRPCPort = "5005"
	c.hostWSPort = "8001"

	// Wait for rpc to come up
	time.Sleep(time.Second * 5)
	c.xrpClient = xrpclient.NewXrpClient(c.GetHostRPCAddress())

	resp, err := c.xrpClient.GetServerInfo()
	if err != nil {
		fmt.Println("server info error:", err)
	} else {
		fmt.Println("server info resp:", resp.Info.CompleteLedgers)
	}

	height, err := c.Height(ctx)
	if err != nil {
		fmt.Println("height error:", err)
	} else {
		fmt.Println("height", height)
	}
	
	go func() {
		// Don't use ctx from Start(), it gets cancelled soon after returning.
		goRoutineCtx := context.Background()
		goRoutineCtx, c.cancel = context.WithCancel(goRoutineCtx)

		xrpBlockTime := time.Second * 2
		timer := time.NewTimer(xrpBlockTime)
		defer timer.Stop()
		for {
			select {
			case <-goRoutineCtx.Done():
				return
			case <-timer.C:
				if err := c.xrpClient.ForceLedgerClose(); err != nil {
					fmt.Println("error force ledger close,", err)
				}
				timer.Reset(xrpBlockTime)
			}
		}
	}()
	
	time.Sleep(time.Second * 15)
	resp, err = c.xrpClient.GetServerInfo()
	if err != nil {
		fmt.Println("server info error:", err)
	} else {
		fmt.Println("server info resp:", resp.Info.CompleteLedgers)
	}

	height, err = c.Height(ctx)
	if err != nil {
		fmt.Println("height error:", err)
	} else {
		fmt.Println("height", height)
	}
	
	// Then wait the standard 2 blocks which also gives the faucet a starting balance of 100 coins
	// for height, err := c.Height(ctx); err == nil && height < int64(102); {
	// 	time.Sleep(time.Second)
	// 	height, err = c.Height(ctx)
	// }
	return err
}

func (c *XrpChain) HostName() string {
	return dockerutil.CondenseHostName(c.Name())
}

func (c *XrpChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	//logger := zap.NewNop()
	//if cmd[len(cmd)-1] != "getblockcount" && cmd[len(cmd)-3] != "generatetoaddress" { // too much logging, maybe switch to an rpc lib in the future
	//	logger = c.logger()
	//}
	job := dockerutil.NewImage(c.logger(), c.DockerClient, c.NetworkID, c.testName, c.cfg.Images[0].Repository, c.cfg.Images[0].Version)
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: c.Bind(),
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (c *XrpChain) logger() *zap.Logger {
	return c.log.With(
		zap.String("chain_id", c.cfg.ChainID),
		zap.String("test", c.testName),
	)
}

func (c *XrpChain) GetRPCAddress() string {
	rpcPortNumber := strings.Split(rpcPort, "/")
	return fmt.Sprintf("http://%s:%s", c.HostName(), rpcPortNumber)
}

func (c *XrpChain) GetWSAddress() string {
	wsPortNumber := strings.Split(wsPublicPort, "/")
	return fmt.Sprintf("ws://%s:%s", c.HostName(), wsPortNumber)
}

func (c *XrpChain) GetHostRPCAddress() string {
	return "http://127.0.0.1:" + c.hostRPCPort
}

func (c *XrpChain) GetHostWSAddress() string {
	return "ws://127.0.0.1:" + c.hostWSPort
}

func (c *XrpChain) CreateKey(ctx context.Context, keyName string) error {
	c.MapAccess.Lock()
	defer c.MapAccess.Unlock()

	// If wallet already exists, just return
	if c.KeyNameToWalletMap[keyName] != nil {
		return nil
	}

	if keyName == "faucet" {
		rootAccount := xrpwallet.GetRootAccount(keyName)
		c.AddrToKeyNameMap[rootAccount.AccountID] = keyName
		c.KeyNameToWalletMap[keyName] = rootAccount
		return nil
	}

	newAccount, err := xrpwallet.GenerateAccount(keyName, "ed25519")
	if err != nil {
		return fmt.Errorf("error creating new account, %v", err)
	}
	c.AddrToKeyNameMap[newAccount.AccountID] = keyName
	c.KeyNameToWalletMap[keyName] = newAccount

	return nil
}

// Get address of account, cast to a string to use.
func (c *XrpChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	c.MapAccess.Lock()
	defer c.MapAccess.Unlock()
	wallet, ok := c.KeyNameToWalletMap[keyName]
	if ok {
		return wallet.Address(), nil
	}

	// Pre-start GetAddress doesn't matter
	// TODO: do we still enter here?
	if keyName == "faucet" {
		return []byte(keyName), nil
	}

	return nil, fmt.Errorf("keyname: %s's address not found", keyName)
}

func (c *XrpChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	c.MapAccess.Lock()
	srcWallet := c.KeyNameToWalletMap[keyName]
	c.MapAccess.Unlock()

	if srcWallet == nil {
		return fmt.Errorf("invalid keyname")
	}
	//_, err := c.SendFundsWithNote(ctx, keyName, amount, "")
	// Get the next sequence number
    sequence, err := c.xrpClient.GetAccountSequence(srcWallet.FormattedAddress())
    if err != nil {
        fmt.Printf("Error getting sequence: %v\n", err)
        return err
    }

    // Create payment transaction
    payment := &xrpclient.Payment{
        TransactionType: "Payment",
        Account:         srcWallet.FormattedAddress(),
        Destination:     amount.Address,
        Amount:          amount.Amount.String(),
        Sequence:        sequence,
        Fee:             "10",
        NetworkID:       1234,
        SigningPubKey:   srcWallet.PublicKeyHex,
    }
	fmt.Println("asdf", payment.Account)
	panic("not implemented")

    // Sign and submit
    // err = c.xrpClient.SignAndSubmitPayment(srcWallet.MasterSeedHex, payment)
    // if err != nil {
    //     fmt.Printf("Error submitting payment: %v\n", err)
    //     return err
    // }
	
	// return nil
}

// func (c *XrpChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
// 	c.MapAccess.Lock()
// 	srcWallet := c.KeyNameToWalletMap[keyName]
// 	c.MapAccess.Unlock()

// 	if srcWallet == nil {
// 		return fmt.Errorf("invalid keyname")
// 	}
// 	//_, err := c.SendFundsWithNote(ctx, keyName, amount, "")
// 	tx := Transaction{
// 		TransactionType: "Payment",
// 		Account: srcWallet.FormattedAddress(),
// 		Destination: amount.Address,
// 		Amount: amount.Amount.String(),
// 	}
// 	txJson, err := json.Marshal(tx)
// 	if err != nil {
// 		return fmt.Errorf("send funds error on tx marshal, %v", err)
// 	}
// 	cmd := []string{
// 		c.RippledCli, "submit", "snoPBrXtMeMyMHUVTgbuqAfg1SUTb", fmt.Sprintf("'%s'", string(txJson)),
// 	}
// 	fmt.Println("transaction to sign:", txJson)
// 	fmt.Println("cmd:", cmd)
// 	stdout, _, err := c.Exec(ctx, cmd, nil)
// 	if err != nil {
// 		return fmt.Errorf("send funds error on exec, %v", err)
// 	}
// 	fmt.Println("submit output:", string(stdout))

// 	var submitResponse SubmitResponse
// 	if err := json.Unmarshal(stdout, &submitResponse); err != nil {
// 		return fmt.Errorf("error unmarshal submit response, %v", err)
// 	}
// 	if submitResponse.Result.Status != "success" || submitResponse.Result.EngineResultCode != 0 {
// 		return fmt.Errorf("send funds failed,\nengine_result: %s\nengine_result_code: %d\nengine_result_message: %s\nstatus: %s",
// 			submitResponse.Result.EngineResult,
// 			submitResponse.Result.EngineResultCode,
// 			submitResponse.Result.EngineResultMessage,
// 			submitResponse.Result.Status,
// 		)
// 	}
// 	return nil
// }

func (c *XrpChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	// partialCoin := amount.Amount.ModRaw(int64(math.Pow10(int(*c.Config().CoinDecimals))))
	// fullCoins := amount.Amount.Sub(partialCoin).QuoRaw(int64(math.Pow10(int(*c.Config().CoinDecimals))))
	// sendAmountFloat := float64(fullCoins.Int64()) + float64(partialCoin.Int64())/math.Pow10(int(*c.Config().CoinDecimals))

	// if err := c.LoadWallet(ctx, keyName); err != nil {
	// 	return "", err
	// }

	// wallet.txLock.Lock()
	// defer wallet.txLock.Unlock()

	// err = testutil.WaitForBlocks(ctx, 1, c)
	// return txHash, err
	return "", fmt.Errorf("SendFundsWithNote not implemented")
}

func (c *XrpChain) Height(ctx context.Context) (int64, error) {
	time.Sleep(time.Millisecond * 200) // TODO: slow down WaitForBlocks instead of here
	
	return c.xrpClient.GetCurrentLedger()
}

func (c *XrpChain) GetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	accountInfo, err := c.xrpClient.GetAccountInfo(address, false)
	if err != nil {
		return sdkmath.ZeroInt(), err
	}
	// TODO: check for accountInfo errors (i.e. account not found)
	balance, ok := sdkmath.NewIntFromString(accountInfo.AccountData.Balance)
	if !ok {
		return sdkmath.ZeroInt(), fmt.Errorf("balance not okay")
	}
	return balance, nil
}

func (c *XrpChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
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
	c.MapAccess.Lock()
	defer c.MapAccess.Unlock()
	wallet := c.KeyNameToWalletMap[keyName]

	return wallet, nil
}

func (c *XrpChain) Stop() {
	c.cancel()
}

// WriteFile accepts file contents in a byte slice and writes the contents to
// the docker filesystem. relPath describes the location of the file in the
// docker volume relative to the home directory.
func (c *XrpChain) WriteFile(ctx context.Context, content []byte, relPath string) error {
	fw := dockerutil.NewFileWriter(c.logger(), c.DockerClient, c.testName)
	return fw.WriteFile(ctx, c.VolumeName, relPath, content)
}