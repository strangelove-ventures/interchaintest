package ibc

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/ibctest/v6/internal/dockerutil"
	"go.uber.org/zap"
)

type Chain interface {
	// Config fetches the chain configuration.
	Config() ChainConfig

	// Initialize initializes node structs so that things like initializing keys can be done before starting the chain
	Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error

	// Start sets up everything needed (validators, gentx, fullnodes, peering, additional accounts) for chain to start from genesis.
	Start(testName string, ctx context.Context, additionalGenesisWallets ...WalletAmount) error

	// Exec runs an arbitrary command using Chain's docker environment.
	// Whether the invoked command is run in a one-off container or execing into an already running container
	// is up to the chain implementation.
	//
	// "env" are environment variables in the format "MY_ENV_VAR=value"
	Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error)

	// ExportState exports the chain state at specific height.
	ExportState(ctx context.Context, height int64) (string, error)

	// GetRPCAddress retrieves the rpc address that can be reached by other containers in the docker network.
	GetRPCAddress() string

	// GetGRPCAddress retrieves the grpc address that can be reached by other containers in the docker network.
	GetGRPCAddress() string

	// GetHostRPCAddress returns the rpc address that can be reached by processes on the host machine.
	// Note that this will not return a valid value until after Start returns.
	GetHostRPCAddress() string

	// GetHostGRPCAddress returns the grpc address that can be reached by processes on the host machine.
	// Note that this will not return a valid value until after Start returns.
	GetHostGRPCAddress() string

	// HomeDir is the home directory of a node running in a docker container. Therefore, this maps to
	// the container's filesystem (not the host).
	HomeDir() string

	// CreateKey creates a test key in the "user" node (either the first fullnode or the first validator if no fullnodes).
	CreateKey(ctx context.Context, keyName string) error

	// RecoverKey recovers an existing user from a given mnemonic.
	RecoverKey(ctx context.Context, name, mnemonic string) error

	// GetAddress fetches the bech32 address for a test key on the "user" node (either the first fullnode or the first validator if no fullnodes).
	GetAddress(ctx context.Context, keyName string) ([]byte, error)

	// SendFunds sends funds to a wallet from a user account.
	SendFunds(ctx context.Context, keyName string, amount WalletAmount) error

	// SendIBCTransfer sends an IBC transfer returning a transaction or an error if the transfer failed.
	SendIBCTransfer(ctx context.Context, channelID, keyName string, amount WalletAmount, timeout *IBCTimeout) (Tx, error)

	// Height returns the current block height or an error if unable to get current height.
	Height(ctx context.Context) (uint64, error)

	// GetBalance fetches the current balance for a specific account address and denom.
	GetBalance(ctx context.Context, address string, denom string) (int64, error)

	// GetGasFeesInNativeDenom gets the fees in native denom for an amount of spent gas.
	GetGasFeesInNativeDenom(gasPaid int64) int64

	// Acknowledgements returns all acknowledgements in a block at height.
	Acknowledgements(ctx context.Context, height uint64) ([]PacketAcknowledgement, error)

	// Timeouts returns all timeouts in a block at height.
	Timeouts(ctx context.Context, height uint64) ([]PacketTimeout, error)
}

// Toml is used for holding the decoded state of a toml config file.
type ChainUtilToml map[string]any

// recursiveModifyToml will apply toml modifications at the current depth,
// then recurse for new depths.
func recursiveModifyToml(c map[string]any, modifications ChainUtilToml) error {
	for key, value := range modifications {
		if reflect.ValueOf(value).Kind() == reflect.Map {
			cV, ok := c[key]
			if !ok {
				// Did not find section in existing config, populating fresh.
				cV = make(ChainUtilToml)
			}
			// Retrieve existing config to apply overrides to.
			cVM, ok := cV.(map[string]any)
			if !ok {
				return fmt.Errorf("failed to convert section to (map[string]any), found (%T)", cV)
			}
			if err := recursiveModifyToml(cVM, value.(ChainUtilToml)); err != nil {
				return err
			}
			c[key] = cVM
		} else {
			// Not a map, so we can set override value directly.
			c[key] = value
		}
	}
	return nil
}

// ModifyTomlConfigFile reads, modifies, then overwrites a toml config file, useful for config.toml, app.toml, etc.
func ModifyTomlConfigFile(
	ctx context.Context,
	logger *zap.Logger,
	dockerClient *client.Client,
	testName string,
	volumeName string,
	filePath string,
	modifications ChainUtilToml,
) error {
	fr := dockerutil.NewFileRetriever(logger, dockerClient, testName)
	config, err := fr.SingleFileContent(ctx, volumeName, filePath)
	if err != nil {
		return fmt.Errorf("failed to retrieve %s: %w", filePath, err)
	}

	var c ChainUtilToml
	if err := toml.Unmarshal(config, &c); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", filePath, err)
	}

	if err := recursiveModifyToml(c, modifications); err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(c); err != nil {
		return err
	}

	fw := dockerutil.NewFileWriter(logger, dockerClient, testName)
	if err := fw.WriteFile(ctx, volumeName, filePath, buf.Bytes()); err != nil {
		return fmt.Errorf("overwriting %s: %w", filePath, err)
	}

	return nil
}
