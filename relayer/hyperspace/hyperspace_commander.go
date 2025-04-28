// Package hyperspace provides an interface to the hyperspace relayer running in a Docker container.
package hyperspace

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/misko9/go-substrate-rpc-client/v4/signature"
	"github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"

	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	types23 "github.com/cosmos/ibc-go/v8/modules/core/23-commitment/types"

	"github.com/strangelove-ventures/interchaintest/v8/chain/polkadot"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
)

// hyperspaceCommander satisfies relayer.RelayerCommander.
type hyperspaceCommander struct {
	log             *zap.Logger
	paths           map[string]*pathConfiguration
	extraStartFlags []string
}

func NewHyperspaceCommander() relayer.RelayerCommander {
	return &hyperspaceCommander{}
}

// pathConfiguration represents the concept of a "path" which is implemented at the interchain test level rather
// than the hyperspace level.
type pathConfiguration struct {
	chainA, chainB pathChainConfig
}

// pathChainConfig holds all values that will be required when interacting with a path.
type pathChainConfig struct {
	chainID string
}

func (hyperspaceCommander) Name() string {
	return "hyperspace"
}

func (hyperspaceCommander) DockerUser() string {
	return "1000:1000" // docker run -it --rm --entrypoint echo ghcr.io/cosmos/relayer "$(id -u):$(id -g)"
}

func (c *hyperspaceCommander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	c.log.Info("Hyperspace AddChainConfiguration",
		zap.String("container_file_path", containerFilePath),
		zap.String("home_dir", homeDir),
	)

	// c.chainConfigPaths = append(c.chainConfigPaths, containerFilePath)
	return []string{
		"hyperspace",
		"-h",
	}
}

// Hyperspace doesn't not have this functionality.
func (hyperspaceCommander) AddKey(chainID, keyName, coinType, signingAlgorithm, homeDir string) []string {
	panic("[AddKey] Do not call me")
}

func (c *hyperspaceCommander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	c.log.Info("Hyperspace CreateChannel",
		zap.String("path_name", pathName),
		zap.String("home_dir", homeDir),
	)

	_, ok := c.paths[pathName]
	if !ok {
		panic(fmt.Sprintf("path %s not found", pathName))
	}
	return []string{
		"hyperspace",
		"create-channel",
		"--config-a",
		configPath(homeDir, c.paths[pathName].chainA.chainID),
		"--config-b",
		configPath(homeDir, c.paths[pathName].chainB.chainID),
		"--config-core",
		path.Join(homeDir, "core.config"),
		"--port-id",
		opts.SourcePortName,
		"--order",
		"unordered",
		"--version",
		opts.Version,
	}
}

func (c *hyperspaceCommander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	c.log.Info("Hyperspace CreateClients",
		zap.String("path_name", pathName),
		zap.String("home_dir", homeDir),
		zap.String("trusting_period", opts.TrustingPeriod),
		zap.Int64("trusting_period_percentage", opts.TrustingPeriodPercentage),
		zap.String("max_clock_drift", opts.MaxClockDrift),
		zap.Bool("override", opts.Override),
	)

	_, ok := c.paths[pathName]
	if !ok {
		panic(fmt.Sprintf("path %s not found", pathName))
	}
	return []string{
		"hyperspace",
		"create-clients",
		"--config-a",
		configPath(homeDir, c.paths[pathName].chainA.chainID),
		"--config-b",
		configPath(homeDir, c.paths[pathName].chainB.chainID),
		"--config-core",
		path.Join(homeDir, "core.config"),
		"--port-id",
		"transfer",
		"--order",
		"unordered",
	}
}

// TODO: Implement if available in hyperspace relayer.
func (hyperspaceCommander) CreateClient(srcChainID, dstChainID, pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	panic("[CreateClient] Not Implemented")
}

func (c *hyperspaceCommander) CreateConnections(pathName, homeDir string) []string {
	c.log.Info("Hyperspace CreateConnections",
		zap.String("path_name", pathName),
		zap.String("home_dir", homeDir),
	)

	_, ok := c.paths[pathName]
	if !ok {
		panic(fmt.Sprintf("path %s not found", pathName))
	}
	return []string{
		"hyperspace",
		"create-connection",
		"--config-a",
		configPath(homeDir, c.paths[pathName].chainA.chainID),
		"--config-b",
		configPath(homeDir, c.paths[pathName].chainB.chainID),
		"--config-core",
		path.Join(homeDir, "core.config"),
		"--delay-period",
		"1",
	}
}

// Hyperspace doesn't not have this functionality.
func (hyperspaceCommander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	panic("[FlushAcknowledgements] Do not call me")
}

// Hyperspace doesn't not have this functionality.
func (hyperspaceCommander) FlushPackets(pathName, channelID, homeDir string) []string {
	panic("[FlushPackets] Do not call me")
}

// GeneratePath establishes an in memory path representation. The concept does not exist in hyperspace.
func (c *hyperspaceCommander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	if c.paths == nil {
		c.paths = map[string]*pathConfiguration{}
	}
	c.paths[pathName] = &pathConfiguration{
		chainA: pathChainConfig{
			chainID: srcChainID,
		},
		chainB: pathChainConfig{
			chainID: dstChainID,
		},
	}
	return []string{"true"}
}

// Hyperspace does not have paths, just two configs.
func (hyperspaceCommander) UpdatePath(pathName, homeDir string, opts ibc.PathUpdateOptions) []string {
	panic("[UpdatePath] Do not call me")
}

// Prints chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output.
func (c hyperspaceCommander) GetChannels(chainID, homeDir string) []string {
	c.log.Info("Hyperspace GetChannels",
		zap.String("chain_id", chainID),
		zap.String("home_dir", homeDir),
	)

	configFilePath := path.Join(homeDir, chainID+".config")
	return []string{
		"cat",
		configFilePath,
	}
}

// Prints chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output.
func (c hyperspaceCommander) GetConnections(chainID, homeDir string) []string {
	c.log.Info("Hyperspace GetConnections",
		zap.String("chain_id", chainID),
		zap.String("home_dir", homeDir),
	)

	configFilePath := path.Join(homeDir, chainID+".config")
	return []string{
		"cat",
		configFilePath,
	}
}

// Prints chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output.
func (c hyperspaceCommander) GetClients(chainID, homeDir string) []string {
	c.log.Info("Hyperspace GetClients",
		zap.String("chain_id", chainID),
		zap.String("home_dir", homeDir),
	)

	configFilePath := path.Join(homeDir, chainID+".config")
	return []string{
		"cat",
		configFilePath,
	}
}

// Hyperspace does not have link cmd, call create clients, create connection, and create channel.
func (hyperspaceCommander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpt ibc.CreateClientOptions) []string {
	panic("[LinkPath] Do not use me")
}

// There is no hyperspace call to restore the key, so this can't return an executable.
// HyperspaceRelayer's RestoreKey will restore the key in the chain's config file.
func (hyperspaceCommander) RestoreKey(chainID, bech32Prefix, coinType, signingAlgorithm, mnemonic, homeDir string) []string {
	panic("[RestoreKey] Do not use me")
}

// hyperspace can only start 1 path.
func (c *hyperspaceCommander) StartRelayer(homeDir string, pathNames ...string) []string {
	fields := []zap.Field{zap.String("home_dir", homeDir), zap.String("path_names", strings.Join(pathNames, ","))}

	c.log.Info("HyperSpace StartRelayer", fields...)

	if len(pathNames) != 1 {
		panic("Hyperspace's StartRelayer list of paths can only have 1 path")
	}
	pathName := pathNames[0]
	_, ok := c.paths[pathName]
	if !ok {
		panic(fmt.Sprintf("path %s not found", pathName))
	}
	return []string{
		"hyperspace",
		"relay",
		"--config-a",
		configPath(homeDir, c.paths[pathName].chainA.chainID),
		"--config-b",
		configPath(homeDir, c.paths[pathName].chainB.chainID),
		"--config-core",
		path.Join(homeDir, "core.config"),
	}
}

// Hyperspace doesn't not have this functionality.
func (hyperspaceCommander) UpdateClients(pathName, homeDir string) []string {
	panic("[UpdateClients] Do not use me")
}

func (c hyperspaceCommander) ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
	c.log.Info("Hyperspace ConfigContent",
		zap.String("rpc_addr", rpcAddr),
		zap.String("grpc_addr", grpcAddr),
		zap.String("key_name", keyName),
	)

	HyperspaceRelayerChainConfig := ChainConfigToHyperspaceRelayerChainConfig(cfg, keyName, rpcAddr, grpcAddr)
	bytes, err := toml.Marshal(HyperspaceRelayerChainConfig)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (hyperspaceCommander) DefaultContainerImage() string {
	return HyperspaceDefaultContainerImage
}

func (hyperspaceCommander) DefaultContainerVersion() string {
	return HyperspaceDefaultContainerVersion
}

// There is no hyperspace call to add key, so there is no stdout to parse.
// DockerRelayer's RestoreKey will restore the key in the chain's config file.
func (hyperspaceCommander) ParseAddKeyOutput(stdout, stderr string) (ibc.Wallet, error) {
	panic("[ParseAddKeyOutput] Do not call me")
}

// There is no hyperspace call to restore the key, so there is no stdout to parse.
// DockerRelayer's RestoreKey will restore the key in the chain's config file.
func (hyperspaceCommander) ParseRestoreKeyOutput(stdout, stderr string) string {
	panic("[ParseRestoreKeyOutput] Do not call me")
}

type ChannelsOutput struct {
	Channels [][]string `toml:"channel_whitelist"`
}

// Parses output of chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output.
func (hyperspaceCommander) ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error) {
	var cfg ChannelsOutput
	err := toml.Unmarshal([]byte(stdout), &cfg)
	if err != nil {
		return nil, err
	}

	outputs := make([]ibc.ChannelOutput, 0)
	for _, channel := range cfg.Channels {
		outputs = append(outputs, ibc.ChannelOutput{
			State:    "",
			Ordering: "",
			Counterparty: ibc.ChannelCounterparty{ // TODO: retrieve from hyperspace
				PortID:    "",
				ChannelID: "",
			},
			ConnectionHops: []string{},
			Version:        "",
			PortID:         channel[1],
			ChannelID:      channel[0],
		})
	}
	return outputs, nil
}

type ConnectionsOutput struct {
	ConnectionID string `toml:"connection_id"`
	ClientID     string `toml:"client_id"`
}

// Parses output of chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
// Only supports 1 connection and limited info.
func (hyperspaceCommander) ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error) {
	var cfg ConnectionsOutput
	err := toml.Unmarshal([]byte(stdout), &cfg)
	if err != nil {
		return nil, err
	}

	return ibc.ConnectionOutputs{
		&ibc.ConnectionOutput{
			ID:       cfg.ConnectionID,
			ClientID: cfg.ClientID,
			Versions: []*ibcexported.Version{
				{
					Identifier: "",
					Features:   []string{},
				},
			},
			State: "",
			Counterparty: &ibcexported.Counterparty{
				ClientId:     "",
				ConnectionId: "",
				Prefix: types23.MerklePrefix{
					KeyPrefix: []byte{},
				},
			},
			DelayPeriod: "0",
		},
	}, nil
}

type ClientOutput struct {
	ChainID  string `toml:"chain_id"`
	ClientID string `toml:"client_id"`
}

// Parses output of chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
// Only supports 1 client.
func (hyperspaceCommander) ParseGetClientsOutput(stdout, stderr string) (ibc.ClientOutputs, error) {
	var cfg ClientOutput
	err := toml.Unmarshal([]byte(stdout), &cfg)
	if err != nil {
		return nil, err
	}

	return ibc.ClientOutputs{
		&ibc.ClientOutput{
			ClientID: cfg.ClientID,
			ClientState: ibc.ClientState{
				ChainID: cfg.ChainID,
			},
		},
	}, nil
}

func (c hyperspaceCommander) Init(homeDir string) []string {
	c.log.Info("Hyperspace Init", zap.String("home_dir", homeDir))

	// Return hyperspace help to ensure hyperspace binary is accessible
	return []string{
		"hyperspace",
		"-h",
	}
}

func (hyperspaceCommander) CreateWallet(keyName, address, mnemonic string) ibc.Wallet {
	kp, err := signature.KeyringPairFromSecret(mnemonic, polkadot.Ss58Format)
	if err != nil {
		return NewWallet("", "", "")
	}
	return NewWallet("", kp.Address, mnemonic)
}

func (hyperspaceCommander) Flush(pathName, channelID, homeDir string) []string {
	panic("flush implemented in hyperspace not the commander")
}

func configPath(homeDir, chainID string) string {
	chainConfigFile := chainID + ".config"
	return path.Join(homeDir, chainConfigFile)
}
