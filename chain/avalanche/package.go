package avalanche

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"go.uber.org/zap"
)

var _ ibc.Chain = AvalancheChain{}

type (
	AvalancheChain struct {
		log           *zap.Logger
		testName      string
		cfg           ibc.ChainConfig
		numValidators int
		numFullNodes  int
		nodes         AvalancheNodes
	}
)

func NewAvalancheChain(log *zap.Logger, testName string, chainConfig ibc.ChainConfig, numValidators int, numFullNodes int) (*AvalancheChain, error) {
	if numValidators < 5 {
		return nil, fmt.Errorf("numValidators must be more or equal 5, have: %d", numValidators)
	}
	return &AvalancheChain{
		log:           log,
		testName:      testName,
		cfg:           chainConfig,
		numValidators: numValidators,
		numFullNodes:  numFullNodes,
	}, nil
}

func (c *AvalancheChain) node() *AvalancheNode {
	if len(c.nodes) > c.numValidators {
		return &c.nodes[c.numValidators]
	}
	return &c.nodes[0]
}

// Config fetches the chain configuration.
func (c AvalancheChain) Config() ibc.ChainConfig {
	return c.cfg
}

// Initialize initializes node structs so that things like initializing keys can be done before starting the chain
func (c AvalancheChain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	count := c.numFullNodes + c.numValidators
	c.nodes = make([]AvalancheNode, count)
	chainCfg := c.Config()
	for _, image := range chainCfg.Images {
		rc, err := cli.ImagePull(
			ctx,
			image.Repository+":"+image.Version,
			types.ImagePullOptions{},
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
	var prevNode *AvalancheNode = nil
	for i := 0; i < count*2; i += 2 {
		n, err := NewAvalancheNode(ctx, i/2, 9650+i, 9650+i+1, cli, networkID, testName, chainCfg.Images[0], prevNode)
		if err != nil {
			return err
		}
		c.nodes[i] = *n
		prevNode = n
	}
	return nil
}

// Start sets up everything needed (validators, gentx, fullnodes, peering, additional accounts) for chain to start from genesis.
func (c AvalancheChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	panic("ToDo: implement me")
}

// Exec runs an arbitrary command using Chain's docker environment.
// Whether the invoked command is run in a one-off container or execing into an already running container
// is up to the chain implementation.
//
// "env" are environment variables in the format "MY_ENV_VAR=value"
func (c AvalancheChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	return c.node().Exec(ctx, cmd, env)
}

// ExportState exports the chain state at specific height.
func (c AvalancheChain) ExportState(ctx context.Context, height int64) (string, error) {
	panic("ToDo: implement me")
}

// GetRPCAddress retrieves the rpc address that can be reached by other containers in the docker network.
func (c AvalancheChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:%s", c.node().HostName(), c.node().RPCPort())
}

// GetGRPCAddress retrieves the grpc address that can be reached by other containers in the docker network.
func (c AvalancheChain) GetGRPCAddress() string {
	return fmt.Sprintf("http://%s:%s", c.node().HostName(), c.node().GRPCPort())
}

// GetHostRPCAddress returns the rpc address that can be reached by processes on the host machine.
// Note that this will not return a valid value until after Start returns.
func (c AvalancheChain) GetHostRPCAddress() string {
	panic("ToDo: implement me")
}

// GetHostGRPCAddress returns the grpc address that can be reached by processes on the host machine.
// Note that this will not return a valid value until after Start returns.
func (c AvalancheChain) GetHostGRPCAddress() string {
	panic("ToDo: implement me")
}

// HomeDir is the home directory of a node running in a docker container. Therefore, this maps to
// the container's filesystem (not the host).
func (c AvalancheChain) HomeDir() string {
	panic("ToDo: implement me")
}

// CreateKey creates a test key in the "user" node (either the first fullnode or the first validator if no fullnodes).
func (c AvalancheChain) CreateKey(ctx context.Context, keyName string) error {
	return c.node().CreateKey(ctx, keyName)
}

// RecoverKey recovers an existing user from a given mnemonic.
func (c AvalancheChain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	return c.node().RecoverKey(ctx, name, mnemonic)
}

// GetAddress fetches the bech32 address for a test key on the "user" node (either the first fullnode or the first validator if no fullnodes).
func (c AvalancheChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	return c.node().GetAddress(ctx, keyName)
}

// SendFunds sends funds to a wallet from a user account.
func (c AvalancheChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	return c.node().SendFunds(ctx, keyName, amount)
}

// SendIBCTransfer sends an IBC transfer returning a transaction or an error if the transfer failed.
func (c AvalancheChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	return c.node().SendIBCTransfer(ctx, channelID, keyName, amount, options)
}

// Height returns the current block height or an error if unable to get current height.
func (c AvalancheChain) Height(ctx context.Context) (uint64, error) {
	return c.node().Height(ctx)
}

// GetBalance fetches the current balance for a specific account address and denom.
func (c AvalancheChain) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	return c.node().GetBalance(ctx, address, denom)
}

// GetGasFeesInNativeDenom gets the fees in native denom for an amount of spent gas.
func (c AvalancheChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	// ToDo: ask how to calculate???
	panic("ToDo: implement me")
}

// Acknowledgements returns all acknowledgements in a block at height.
func (c AvalancheChain) Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error) {
	panic("ToDo: implement me")
}

// Timeouts returns all timeouts in a block at height.
func (c AvalancheChain) Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error) {
	panic("ToDo: implement me")
}

// BuildWallet will return a chain-specific wallet
// If mnemonic != "", it will restore using that mnemonic
// If mnemonic == "", it will create a new key, mnemonic will not be populated
func (c AvalancheChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
			return nil, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	} else {
		if err := c.CreateKey(ctx, keyName); err != nil {
			return nil, fmt.Errorf("failed to create key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	}

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}

	return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
}

// BuildRelayerWallet will return a chain-specific wallet populated with the mnemonic so that the wallet can
// be restored in the relayer node using the mnemonic. After it is built, that address is included in
// genesis with some funds.
func (c AvalancheChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	// ToDo: what functionality?
	panic("ToDo: implement me")
}
