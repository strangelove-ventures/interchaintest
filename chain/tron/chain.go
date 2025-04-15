package tron

import (
	"context"
	"cosmossdk.io/math"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"net"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/strangelove-ventures/interchaintest/v8/chain/tron/api"
	"go.uber.org/zap"
	"time"
)

const (
	FaucetKey = "0e4103112ae8f1b835ef3b6b80fe243ec5896c163e8617d8bd6ab5820779e3a6"
)

type Chain struct {
	logger   *zap.Logger
	config   ibc.ChainConfig
	testName string

	walletMutex sync.Mutex
	wallets map[string]*Wallet

	VolumeName   string
	NetworkID    string
	DockerClient *client.Client

	fullnode  *dockerutil.ContainerLifecycle
	supernode *dockerutil.ContainerLifecycle

	api *api.TronApi
}

func NewTronChain(
	testName string,
	chainConfig ibc.ChainConfig,
	_ int,
	_ int,
	logger *zap.Logger,
) *Chain {
	return &Chain{
		logger:   logger,
		config:   chainConfig,
		testName: testName,
		api:      api.NewTronApi("http://localhost:8090", time.Second*2),
		wallets:  make(map[string]*Wallet),
	}
}

func (c *Chain) Config() ibc.ChainConfig {
	return c.config
}

func (c *Chain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	image := c.Config().Images[0]

	c.fullnode = dockerutil.NewContainerLifecycle(c.logger, cli, c.Name())

	v, err := cli.VolumeCreate(ctx, volume.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel:   testName,
			dockerutil.NodeOwnerLabel: c.Name(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}

	c.VolumeName = v.Name
	c.NetworkID = networkID
	c.DockerClient = cli

	err = dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log:        c.logger,
		Client:     cli,
		VolumeName: v.Name,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UIDGID,
	})
	if err != nil {
		return fmt.Errorf("failed to set volume owner: %w", err)
	}

	if c.config.ChainID == "mocknet" {
		name := c.NameWithNodeType("super")
		c.supernode = dockerutil.NewContainerLifecycle(c.logger, cli, name)

		v, err = cli.VolumeCreate(ctx, volume.CreateOptions{
			Labels: map[string]string{
				dockerutil.CleanupLabel:   testName,
				dockerutil.NodeOwnerLabel: name,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create volume: %w", err)
		}
	}

	return nil
}

func (c *Chain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	if c.supernode != nil {
		err := c.supernode.CreateContainer(
			ctx,
			testName,
			c.NetworkID,
			c.config.Images[0],
			nat.PortMap{},
			"",
			[]string{
				fmt.Sprintf("%s:%s", c.VolumeName, "/home/tron/db"),
			},
			[]mount.Mount{},
			dockerutil.CondenseHostName(c.NameWithNodeType("super")),
			[]string{"entrypoint.sh"},
			[]string{
				"TRON_NODE_TYPE=mocknet-supernode",
			},
			[]string{},
		)
		if err != nil {
			return fmt.Errorf("failed to create supernode container: %w", err)
		}

		err = c.supernode.StartContainer(ctx)
		if err != nil {
			return err
		}
	}

	portMap := nat.PortMap{
		nat.Port("8090/tcp"): {},
		nat.Port("8091/tcp"): {},
	}

	for internal, external := range c.config.HostPortOverride {
		port := nat.Port(fmt.Sprintf("%d/tcp", internal))
		portMap[port] = []nat.PortBinding{{
			HostPort: fmt.Sprintf("%d", external),
		}}
	}

	c.logger.Info("starting container", zap.String("name", c.Name()))

	env := c.config.Env
	if c.supernode != nil {
		seed_node := c.NameWithNodeType("super") + ":18888"
		env = append(env, "TRON_SEED_NODE="+seed_node)
	}

	err := c.fullnode.CreateContainer(
		ctx,
		testName,
		c.NetworkID,
		c.config.Images[0],
		portMap,
		"",
		[]string{
			fmt.Sprintf("%s:%s", c.VolumeName, "/home/tron/db"),
		},
		[]mount.Mount{},
		dockerutil.CondenseHostName(c.Name()),
		[]string{"entrypoint.sh"},
		env,
		[]string{},
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	err = c.fullnode.StartContainer(ctx)
	if err != nil {
		return err
	}

	c.logger.Info("Waiting for chain to reach block 10")
	return c.WaitForBlock(ctx, 10, time.Minute)
}

func (c *Chain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	// TODO implement me
	panic("not implemented")
}

func (c *Chain) ExportState(ctx context.Context, height int64) (string, error) {
	panic("not implemented")
	return "", nil
}

func (c *Chain) GetAPIAddress() string{
	return fmt.Sprintf("http://%s:%s", dockerutil.CondenseHostName(c.Name()), "8090")
}

func (c *Chain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:%s", dockerutil.CondenseHostName(c.Name()), "8091")
}

func (c *Chain) GetGRPCAddress() string {
	panic("not implemented")
	return ""
}

func (c *Chain) GetHostRPCAddress() string {
	panic("not implemented")
	return ""
}

func (c *Chain) GetHostPeerAddress() string {
	panic("not implemented")
	return ""
}

func (c *Chain) GetHostGRPCAddress() string {
	panic("not implemented")
	return ""
}

func (c *Chain) HomeDir() string {
	return "/home/tron"
}

func (c *Chain) CreateKey(ctx context.Context, keyName string) error {
	var (
		err    error
		wallet *Wallet
	)

	if keyName == "faucet" {
		wallet, err = NewWalletFromKey(keyName, FaucetKey)
	} else {
		wallet, err = NewWallet(keyName)
	}
	if err != nil {
		c.logger.Error("failed to create wallet", zap.Error(err))
		return err
	}

	c.walletMutex.Lock()
	defer c.walletMutex.Unlock()
	c.wallets[keyName] = wallet
	return nil
}

func (c *Chain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	wallet, err := NewWalletFromMnemonic(keyName, mnemonic)
	if err != nil {
		c.logger.Error("failed to recover key", zap.Error(err))
		return err
	}
	c.walletMutex.Lock()
	defer c.walletMutex.Unlock()
	c.wallets[keyName] = wallet
	return nil
}

func (c *Chain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	c.walletMutex.Lock()
	wallet, ok := c.wallets[keyName]
	c.walletMutex.Unlock()
	if !ok {
		c.logger.Error("failed to find key", zap.String("keyName", keyName))
		return nil, fmt.Errorf("failed to find key, keyName: %s", keyName)
	}

	return crypto.FromECDSAPub(&wallet.key.PublicKey), nil
}

func (c *Chain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	_, err := c.SendFundsWithNote(ctx, keyName, amount, "")
	return err
}

func (c *Chain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	c.walletMutex.Lock()
	wallet, ok := c.wallets[keyName]
	c.walletMutex.Unlock()
	if !ok {
		err := fmt.Errorf("key not found")
		c.logger.Error("failed to find key", zap.String("name", keyName))
		return "", err
	}

	tx, err := c.api.CreateTransaction(
		wallet.FormattedAddress(),
		amount.Address,
		amount.Amount.Uint64(),
		note,
	)
	if err != nil {
		c.logger.Error("failed to create transaction", zap.Error(err))
		return "", err
	}

	hash, err := hex.DecodeString(tx.TxId)
	if err != nil {
		c.logger.Error("failed to decode hash", zap.Error(err))
		return "", err
	}

	signature, err := crypto.Sign(hash, wallet.key)
	if err != nil {
		c.logger.Error("failed to sign transaction", zap.Error(err))
		return "", err
	}

	tx.Signature = append(tx.Signature, hex.EncodeToString(signature))

	data, err := json.Marshal(tx)
	if err != nil {
		c.logger.Error("failed to marshal transaction", zap.Error(err))
		return "", err
	}

	response, err := c.api.BroadcastTransaction(data)
	if err != nil {
		c.logger.Error("failed to broadcast transaction", zap.Error(err))
		return "", err
	}

	return response.TxId, nil
}

func (c *Chain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	panic("not implemented")
	return ibc.Tx{}, nil
}

func (c *Chain) Height(ctx context.Context) (int64, error) {
	block, err := c.api.GetLatestBlock()
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block: %w", err)
	}

	return block.Header.RawData.Number, nil
}

func (c *Chain) GetBalance(_ context.Context, address string, denom string) (math.Int, error) {
	denom = strings.ToUpper(denom)
	if denom != "TRX" {
		return math.ZeroInt(), fmt.Errorf("denom not supported: '%s'", denom)
	}

	balance, err := c.api.GetBalance(address)
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to get balance: %w", err)
	}

	fmt.Println(balance)

	return math.NewIntFromUint64(balance), nil
}

func (c *Chain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	panic("not implemented")
	return 0
}

func (c *Chain) Acknowledgements(ctx context.Context, height int64) ([]ibc.PacketAcknowledgement, error) {
	panic("not implemented")
	return nil, nil
}

func (c *Chain) Timeouts(ctx context.Context, height int64) ([]ibc.PacketTimeout, error) {
	panic("not implemented")
	return nil, nil
}

func (c *Chain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic == "" {
		err := c.CreateKey(ctx, keyName)
		if err != nil {
			c.logger.Error("failed to create key", zap.Error(err))
			return nil, err
		}
	} else {
		err := c.RecoverKey(ctx, keyName, mnemonic)
		if err != nil {
			c.logger.Error("failed to recover key", zap.Error(err))
			return nil, err
		}
	}

	c.walletMutex.Lock()
	wallet := c.wallets[keyName]
	c.walletMutex.Unlock()
	return wallet, nil
}

func (c *Chain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	panic("not implemented")
	return nil, nil
}

func (c *Chain) Name() string {
	return c.NameWithNodeType("full")
}

func (c *Chain) NameWithNodeType(nodeType string) string {
	return fmt.Sprintf(
		"tron-%s-%s-%s",
		nodeType,
		c.config.ChainID,
		dockerutil.SanitizeContainerName(c.testName),
	)
}

func (c *Chain) WaitForBlock(
	ctx context.Context,
	targetHeight int64,
	timeout time.Duration,
) error {
	var height int64
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			err := fmt.Errorf("timed out waiting for block")
			c.logger.Error("failed to wait for block", zap.Error(err))
			return err
		default:
			_, err := net.DialTimeout("tcp", "localhost:8090", time.Second)
			if err == nil {
				height, err = c.Height(ctx)
				if height >= targetHeight {
					c.logger.Info(fmt.Sprintf("Block: %d", height))
					return nil
				}
			}

			if err != nil {
				c.logger.Error("failed to get height", zap.Error(err))
			}

			time.Sleep(time.Second)
		}
	}
}

func (c *Chain) WaitBlocks(
	ctx context.Context,
	amount int64,
	timeout time.Duration,
) error {
	height, err := c.Height(ctx)
	if err != nil {
		c.logger.Error("failed to get height <<", zap.Error(err))
		return err
	}

	return c.WaitForBlock(ctx, height+amount, timeout)
}
