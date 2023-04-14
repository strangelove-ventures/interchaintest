// Package hyperspace provides an interface to the hyperspace relayer running in a Docker container.
package hyperspace

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/docker/docker/client"
	"github.com/pelletier/go-toml/v2"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/relayer"
	"go.uber.org/zap"
)

var _ ibc.Relayer = &HyperspaceRelayer{}

// ******* DockerRelayer methods that will panic in hyperspace commander, no overrides yet *******
// FlushAcknowledgements() - no hyperspace implementation yet
// FlushPackets() - no hypersapce implementation yet
// UpdatePath() - hyperspace doesn't understand paths, may not be needed.
// UpdateClients() - no hyperspace implementation yet
// AddKey() - no hyperspace implementation yet

// HyperspaceRelayer is the ibc.Relayer implementation for github.com/ComposableFi/hyperspace.
type HyperspaceRelayer struct {
	// Embedded DockerRelayer so commands just work.
	*relayer.DockerRelayer
}

func NewHyperspaceRelayer(log *zap.Logger, testName string, cli *client.Client, networkID string, options ...relayer.RelayerOption) *HyperspaceRelayer {
	c := hyperspaceCommander{log: log}
	for _, opt := range options {
		switch o := opt.(type) {
		case relayer.RelayerOptionExtraStartFlags:
			c.extraStartFlags = o.Flags
		}
	}
	dr, err := relayer.NewDockerRelayer(context.TODO(), log, testName, cli, networkID, &c, options...)
	if err != nil {
		panic(err) // TODO: return
	}

	coreConfig := HyperspaceRelayerCoreConfig{
		PrometheusEndpoint: "",
	}
	bytes, err := toml.Marshal(coreConfig)
	if err != nil {
		panic(err) // TODO: return
	}
	err = dr.WriteFileToHomeDir(context.TODO(), "core.config", bytes)
	if err != nil {
		panic(err) // TODO: return
	}

	r := &HyperspaceRelayer{
		DockerRelayer: dr,
	}

	return r
}

// HyperspaceCapabilities returns the set of capabilities of the Cosmos relayer.
//
// Note, this API may change if the rly package eventually needs
// to distinguish between multiple rly versions.
func HyperspaceCapabilities() map[relayer.Capability]bool {
	// RC1 matches the full set of capabilities as of writing.
	return nil // relayer.FullCapabilities()
}

// LinkPath performs the operations that happen when a path is linked. This includes creating clients, creating connections
// and establishing a channel. This happens across multiple operations rather than a single link path cli command.
// Parachains need a Polkadot epoch/session before starting, do not link in interchain.Build()
func (r *HyperspaceRelayer) LinkPath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) error {
	if err := r.CreateClients(ctx, rep, pathName, clientOpts); err != nil {
		return err
	}

	if err := r.CreateConnections(ctx, rep, pathName); err != nil {
		return err
	}

	if err := r.CreateChannel(ctx, rep, pathName, channelOpts); err != nil {
		return err
	}

	return nil
}

func (r *HyperspaceRelayer) RestoreKey(ctx context.Context, rep ibc.RelayerExecReporter, cfg ibc.ChainConfig, keyName, mnemonic string) error {
	addrBytes := ""
	chainID := cfg.ChainID
	coinType := cfg.CoinType
	chainType := cfg.Type

	chainConfigFile := chainID + ".config"
	config, err := r.GetRelayerChainConfig(ctx, chainConfigFile, chainType)
	if err != nil {
		return err
	}
	switch chainType {
	case "cosmos":
		bech32Prefix := cfg.Bech32Prefix
		config.(*HyperspaceRelayerCosmosChainConfig).Keybase = GenKeyEntry(bech32Prefix, coinType, mnemonic)
	case "polkadot":
		config.(*HyperspaceRelayerSubstrateChainConfig).PrivateKey = mnemonic
	}

	err = r.SetRelayerChainConfig(ctx, chainConfigFile, config)
	if err != nil {
		return err
	}

	r.AddWallet(chainID, NewWallet(chainID, addrBytes, mnemonic))

	return nil
}

func (r *HyperspaceRelayer) SetClientContractHash(ctx context.Context, rep ibc.RelayerExecReporter, cfg ibc.ChainConfig, hash string) error {
	chainID := cfg.ChainID
	chainType := cfg.Type
	chainConfigFile := chainID + ".config"
	config, err := r.GetRelayerChainConfig(ctx, chainConfigFile, chainType)
	if err != nil {
		return err
	}
	switch chainType {
	case "cosmos":
		config.(*HyperspaceRelayerCosmosChainConfig).WasmCodeId = hash
	}

	return r.SetRelayerChainConfig(ctx, chainConfigFile, config)
}

func (r *HyperspaceRelayer) PrintCoreConfig(ctx context.Context, rep ibc.RelayerExecReporter) error {
	cmd := []string{
		"cat",
		path.Join(r.HomeDir(), "core.config"),
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}
	fmt.Println(string(res.Stdout))
	return nil
}

func (r *HyperspaceRelayer) PrintConfigs(ctx context.Context, rep ibc.RelayerExecReporter, chainID string) error {
	cmd := []string{
		"cat",
		path.Join(r.HomeDir(), chainID+".config"),
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}
	fmt.Println(string(res.Stdout))
	return nil
}

func (r *HyperspaceRelayer) GetRelayerChainConfig(
	ctx context.Context,
	filePath string,
	chainType string,
) (interface{}, error) {
	configRaw, err := r.ReadFileFromHomeDir(ctx, filePath)
	if err != nil {
		return nil, err
	}

	switch chainType {
	case "cosmos":
		var config HyperspaceRelayerCosmosChainConfig
		if err := toml.Unmarshal(configRaw, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", filePath, err)
		}
		return &config, nil
	case "polkadot":
		var config HyperspaceRelayerSubstrateChainConfig
		if err := toml.Unmarshal(configRaw, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", filePath, err)
		}
		return &config, nil
	}
	return nil, fmt.Errorf("unsupported chain config: %s", chainType)
}
func (r *HyperspaceRelayer) SetRelayerChainConfig(
	ctx context.Context,
	filePath string,
	config interface{},
) error {
	bytes, err := toml.Marshal(config)
	if err != nil {
		return err
	}

	return r.WriteFileToHomeDir(ctx, filePath, bytes)
}
