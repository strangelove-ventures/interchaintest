package tron

import (
	"context"
	"cosmossdk.io/math"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/mount"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

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

	wallets map[string]Wallet

	VolumeName   string
	NetworkID    string
	DockerClient *client.Client

	lifecycle *dockerutil.ContainerLifecycle

	api *api.TronApi
}

func NewTronChain(
	testName string,
	chainConfig ibc.ChainConfig,
	numValidators int,
	numFullNodes int,
	logger *zap.Logger,
) *Chain {
	return &Chain{
		logger:   logger,
		config:   chainConfig,
		testName: testName,
		api:      api.NewTronApi("http://localhost:1234", time.Second*2),
		wallets:  map[string]Wallet{},
	}
}

func (c *Chain) Config() ibc.ChainConfig {
	return c.config
}

func (c *Chain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	image := c.Config().Images[0]

	c.lifecycle = dockerutil.NewContainerLifecycle(c.logger, cli, c.Name())

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

	return nil
}

func (c *Chain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	// TODO ports

	c.logger.Info("starting container", zap.String("name", c.Name()))

	err := c.lifecycle.CreateContainer(
		ctx,
		testName,
		c.NetworkID,
		c.config.Images[0],
		nat.PortMap{},
		"",
		[]string{},
		[]mount.Mount{},
		"hostname",
		[]string{},
		[]string{},
		[]string{},
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	return nil
}

func (c *Chain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	// TODO implement me
	panic("not implemented")
}

func (c *Chain) ExportState(ctx context.Context, height int64) (string, error) {
	panic("not implemented")
	return "", nil
}

func (c *Chain) GetRPCAddress() string {
	panic("not implemented")
	return ""
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

func (c *Chain) CreateKey(ctx context.Context, name string) error {
	var (
		err    error
		wallet Wallet
	)

	if name == "faucet" {
		wallet, err = NewWalletFromKey(FaucetKey)
	} else {
		wallet, err = NewWallet()
	}
	if err != nil {
		c.logger.Error("failed to create wallet", zap.Error(err))
		return err
	}

	c.wallets[name] = wallet
	return nil
}

func (c *Chain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	wallet, err := NewWalletFromMnemonic(mnemonic)
	if err != nil {
		c.logger.Error("failed to recover key", zap.Error(err))
		return err
	}
	c.wallets[name] = wallet
	return nil
}

func (c *Chain) GetAddress(ctx context.Context, name string) ([]byte, error) {
	wallet, ok := c.wallets[name]
	if !ok {
		c.logger.Error("failed to find key", zap.String("name", name))
		return nil, fmt.Errorf("failed to find key: %s", name)
	}

	return crypto.FromECDSAPub(&wallet.key.PublicKey), nil
}

func (c *Chain) SendFunds(ctx context.Context, name string, amount ibc.WalletAmount) error {
	_, err := c.SendFundsWithNote(ctx, name, amount, "")
	return err
}

func (c *Chain) SendFundsWithNote(ctx context.Context, name string, amount ibc.WalletAmount, note string) (string, error) {
	wallet, ok := c.wallets[name]
	if !ok {
		err := fmt.Errorf("key not found")
		c.logger.Error("failed to find key", zap.String("name", name))
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
	if denom != "TRX" {
		return math.ZeroInt(), fmt.Errorf("only 'TRX' supported")
	}

	balance, err := c.api.GetBalance(address)
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to get balance: %w", err)
	}

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

	wallet := c.wallets[keyName]
	return wallet, nil
}

func (c *Chain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	panic("not implemented")
	return nil, nil
}

func (c *Chain) Name() string {
	return fmt.Sprintf("tron-%s-%s", c.config.ChainID, dockerutil.SanitizeContainerName(c.testName))
}
