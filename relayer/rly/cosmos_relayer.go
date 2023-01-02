// Package rly provides an interface to the cosmos relayer running in a Docker container.
package rly

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/relayer"
	"go.uber.org/zap"
)

// CosmosRelayer is the ibc.Relayer implementation for github.com/cosmos/relayer.
type CosmosRelayer struct {
	// Embedded DockerRelayer so commands just work.
	*relayer.DockerRelayer
}

func NewCosmosRelayer(log *zap.Logger, testName string, cli *client.Client, networkID string, options ...relayer.RelayerOption) *CosmosRelayer {
	c := commander{log: log}
	for _, opt := range options {
		switch o := opt.(type) {
		case relayer.RelayerOptionExtraStartFlags:
			c.extraStartFlags = o.Flags
		}
	}
	dr, err := relayer.NewDockerRelayer(context.TODO(), log, testName, cli, networkID, c, options...)
	if err != nil {
		panic(err) // TODO: return
	}

	r := &CosmosRelayer{
		DockerRelayer: dr,
	}

	return r
}

type CosmosRelayerChainConfigValue struct {
	AccountPrefix  string  `json:"account-prefix" toml:"account_prefix"`
	ChainID        string  `json:"chain-id" toml:"chain_id"`
	Debug          bool    `json:"debug" toml:"debug"`
	GRPCAddr       string  `json:"grpc-addr" toml:"grpc_addr"`
	GasAdjustment  float64 `json:"gas-adjustment" toml:"gas_adjustment"`
	GasPrices      string  `json:"gas-prices" toml:"gas_prices"`
	Key            string  `json:"key" toml:"key"`
	KeyringBackend string  `json:"keyring-backend" toml:"keyring_backend"`
	OutputFormat   string  `json:"output-format" toml:"output_format"`
	RPCAddr        string  `json:"rpc-addr" toml:"rpc_addr"`
	SignMode       string  `json:"sign-mode" toml:"sign_mode"`
	Timeout        string  `json:"timeout" toml:"timeout"`
}

type SubstrateRelayerChainConfigValue struct {
	Key                  string  `json:"key" yaml:"key"`
	ChainName            string  `json:"chain-name" yaml:"chain-name"`
	ChainID              string  `json:"chain-id" yaml:"chain-id"`
	RPCAddr              string  `json:"rpc-addr" yaml:"rpc-addr"`
	RelayRPCAddr         string  `json:"relay-rpc-addr" yaml:"relay-rpc-addr"`
	AccountPrefix        string  `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend       string  `json:"keyring-backend" yaml:"keyring-backend"`
	KeyDirectory         string  `json:"key-directory" yaml:"key-directory"`
	GasPrices            string  `json:"gas-prices" yaml:"gas-prices"`
	GasAdjustment        float64 `json:"gas-adjustment" yaml:"gas-adjustment"`
	Debug                bool    `json:"debug" yaml:"debug"`
	Timeout              string  `json:"timeout" yaml:"timeout"`
	OutputFormat         string  `json:"output-format" yaml:"output-format"`
	Network              uint16  `json:"network" yaml:"network"`
	ParaID               uint32  `json:"para-id" yaml:"para-id"`
	BeefyActivationBlock uint32  `json:"beefy-activation-block" yaml:"beefy-activation-block"`
	RelayChain           int32   `json:"relay-chain" yaml:"relay-chain"`
	FinalityProtocol     string  `json:"finality-protocol" yaml:"finality-protocol"`
	SignMode             string  `json:"sign-mode" toml:"sign_mode"`
	GRPCAddr             string  `json:"grpc-addr" toml:"grpc_addr"`
}

type CosmosRelayerChainConfig struct {
	Type  string      `json:"type" toml:"type"`
	Value interface{} `json:"value"`
}

const (
	DefaultContainerImage   = "ghcr.io/cosmos/relayer"
	DefaultContainerVersion = "v2.1.2"
)

// Capabilities returns the set of capabilities of the Cosmos relayer.
//
// Note, this API may change if the rly package eventually needs
// to distinguish between multiple rly versions.
func Capabilities() map[relayer.Capability]bool {
	// RC1 matches the full set of capabilities as of writing.
	return relayer.FullCapabilities()
}

func ChainConfigToCosmosRelayerChainConfig(chainConfig ibc.ChainConfig, keyName, rpcAddr, gprcAddr string) CosmosRelayerChainConfig {
	chainType := chainConfig.Type
	return CosmosRelayerChainConfig{
		Type: chainType,
		Value: CosmosRelayerChainConfigValue{
			Key:     keyName,
			ChainID: chainConfig.ChainID,
			RPCAddr: rpcAddr,

			GRPCAddr:       gprcAddr,
			AccountPrefix:  chainConfig.Bech32Prefix,
			KeyringBackend: keyring.BackendTest,
			GasAdjustment:  chainConfig.GasAdjustment,
			GasPrices:      chainConfig.GasPrices,
			Debug:          true,
			Timeout:        "10s",
			OutputFormat:   "json",
			SignMode:       "direct",
		},
	}
}

func ChainConfigToSubstrateRelayerChainConfig(chainConfig ibc.ChainConfig, keyName, rpcAddr, gprcAddr string) CosmosRelayerChainConfig {
	chainType := chainConfig.Type
	return CosmosRelayerChainConfig{
		Type: chainType,
		Value: SubstrateRelayerChainConfigValue{
			Key:       keyName,
			ChainID:   chainConfig.ChainID,
			ChainName: chainConfig.Name,
			RPCAddr:   rpcAddr,

			GRPCAddr:         gprcAddr,
			AccountPrefix:    chainConfig.Bech32Prefix,
			KeyringBackend:   keyring.BackendTest,
			GasAdjustment:    chainConfig.GasAdjustment,
			GasPrices:        chainConfig.GasPrices,
			Debug:            true,
			Timeout:          "10s",
			OutputFormat:     "json",
			SignMode:         "direct",
			FinalityProtocol: "grandpa",
		},
	}
}

// commander satisfies relayer.RelayerCommander.
type commander struct {
	log             *zap.Logger
	extraStartFlags []string
}

func (commander) Name() string {
	return "rly"
}

func (commander) DockerUser() string {
	return "100:1000" // docker run -it --rm --entrypoint echo ghcr.io/cosmos/relayer "$(id -u):$(id -g)"
}

func (commander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	fmt.Println("AddChainConfiguration1")
	return []string{
		"rly", "chains", "add", "-f", containerFilePath,
		"--home", homeDir,
	}
}

func (commander) AddKey(chainID, keyName, homeDir string) []string {
	return []string{
		"rly", "keys", "add", chainID, keyName,
		"--home", homeDir,
	}
}

func (commander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	return []string{
		"rly", "tx", "channel", pathName,
		"--src-port", opts.SourcePortName,
		"--dst-port", opts.DestPortName,
		"--order", opts.Order.String(),
		"--version", opts.Version,

		"--home", homeDir,
	}
}

func (commander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	return []string{
		"rly", "tx", "clients", pathName, "--client-tp", opts.TrustingPeriod,
		"--home", homeDir,
	}
}

// passing a value of 0 for customeClientTrustingPeriod will use default
func (commander) CreateClient(pathName, homeDir, customeClientTrustingPeriod string) []string {
	return []string{
		"rly", "tx", "client", pathName, "--client-tp", customeClientTrustingPeriod,
		"--home", homeDir,
	}
}

func (commander) CreateConnections(pathName, homeDir string) []string {
	return []string{
		"rly", "tx", "connection", pathName,
		"--home", homeDir,
	}
}

func (commander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	return []string{
		"rly", "tx", "relay-acks", pathName, channelID,
		"--home", homeDir,
	}
}

func (commander) FlushPackets(pathName, channelID, homeDir string) []string {
	return []string{
		"rly", "tx", "relay-pkts", pathName, channelID,
		"--home", homeDir,
	}
}

func (commander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	return []string{
		"rly", "paths", "new", srcChainID, dstChainID, pathName,
		"--home", homeDir,
	}
}

func (commander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	return []string{
		"rly", "paths", "update", pathName,
		"--home", homeDir,
		"--filter-rule", filter.Rule,
		"--filter-channels", strings.Join(filter.ChannelList, ","),
	}
}

func (commander) GetChannels(chainID, homeDir string) []string {
	return []string{
		"rly", "q", "channels", chainID,
		"--home", homeDir,
	}
}

func (commander) GetConnections(chainID, homeDir string) []string {
	return []string{
		"rly", "q", "connections", chainID,
		"--home", homeDir,
	}
}

func (commander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpt ibc.CreateClientOptions) []string {
	return []string{
		"rly", "tx", "link", pathName,
		"--src-port", channelOpts.SourcePortName,
		"--dst-port", channelOpts.DestPortName,
		"--order", channelOpts.Order.String(),
		"--version", channelOpts.Version,
		"--client-tp", clientOpt.TrustingPeriod,

		"--home", homeDir,
	}
}

func (commander) RestoreKey(chainID, keyName, mnemonic, homeDir string) []string {
	return []string{
		"rly", "keys", "restore", chainID, keyName, mnemonic,
		"--home", homeDir,
	}
}

func (c commander) StartRelayer(homeDir string, pathNames ...string) []string {
	cmd := []string{
		"rly", "start", "--debug",
		"--home", homeDir,
	}
	cmd = append(cmd, c.extraStartFlags...)
	cmd = append(cmd, pathNames...)
	return cmd
}

func (commander) UpdateClients(pathName, homeDir string) []string {
	return []string{
		"rly", "tx", "update-clients", pathName,
		"--home", homeDir,
	}
}

func (commander) ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
	chainType := cfg.Type
	var config CosmosRelayerChainConfig
	if chainType == "polkadot" || chainType == "parachain" || chainType == "relaychain" || chainType == "substrate" {
		cfg.Type = "substrate"
		config = ChainConfigToSubstrateRelayerChainConfig(cfg, keyName, rpcAddr, grpcAddr)
	} else {
		config = ChainConfigToCosmosRelayerChainConfig(cfg, keyName, rpcAddr, grpcAddr)
	}
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

func (commander) DefaultContainerImage() string {
	return DefaultContainerImage
}

func (commander) DefaultContainerVersion() string {
	return DefaultContainerVersion
}

func (commander) ParseAddKeyOutput(stdout, stderr string) (ibc.Wallet, error) {
	var wallet ibc.Wallet
	err := json.Unmarshal([]byte(stdout), &wallet)
	return wallet, err
}

func (commander) ParseRestoreKeyOutput(stdout, stderr string) string {
	return strings.Replace(stdout, "\n", "", 1)
}

func (c commander) ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error) {
	var channels []ibc.ChannelOutput
	channelSplit := strings.Split(stdout, "\n")
	for _, channel := range channelSplit {
		if strings.TrimSpace(channel) == "" {
			continue
		}
		var channelOutput ibc.ChannelOutput
		err := json.Unmarshal([]byte(channel), &channelOutput)
		if err != nil {
			c.log.Error("Failed to parse channels json", zap.Error(err))
			continue
		}
		channels = append(channels, channelOutput)
	}

	return channels, nil
}

func (c commander) ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error) {
	var connections ibc.ConnectionOutputs
	for _, connection := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(connection) == "" {
			continue
		}

		var connectionOutput ibc.ConnectionOutput
		if err := json.Unmarshal([]byte(connection), &connectionOutput); err != nil {
			c.log.Error(
				"Error parsing connection json",
				zap.Error(err),
			)

			continue
		}
		connections = append(connections, &connectionOutput)
	}

	return connections, nil
}

func (commander) Init(homeDir string) []string {
	return []string{
		"rly", "config", "init",
		"--home", homeDir,
	}
}
