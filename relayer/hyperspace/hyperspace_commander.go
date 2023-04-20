// Package hyperspace provides an interface to the hyperspace relayer running in a Docker container.
package hyperspace

import (
	"context"
	"fmt"
	"path"

	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	types23 "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	"github.com/misko9/go-substrate-rpc-client/v4/signature"
	"github.com/pelletier/go-toml/v2"
	"github.com/strangelove-ventures/interchaintest/v7/chain/polkadot"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"go.uber.org/zap"
)

// hyperspaceCommander satisfies relayer.RelayerCommander.
type hyperspaceCommander struct {
	log             *zap.Logger
	paths           map[string]*pathConfiguration
	extraStartFlags []string
}

// pathConfiguration represents the concept of a "path" which is implemented at the interchain test level rather
// than the hyperspace level.
type pathConfiguration struct {
	chainA, chainB pathChainConfig
}

// pathChainConfig holds all values that will be required when interacting with a path.
type pathChainConfig struct {
	chainID      string
}

func (hyperspaceCommander) Name() string {
	return "hyperspace"
}

func (hyperspaceCommander) DockerUser() string {
	return "1000:1000" // docker run -it --rm --entrypoint echo ghcr.io/cosmos/relayer "$(id -u):$(id -g)"
}

func (c *hyperspaceCommander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	fmt.Println("[hyperspace] AddChainConfiguration ", containerFilePath, homeDir)
	//c.chainConfigPaths = append(c.chainConfigPaths, containerFilePath)
	return []string{
		"hyperspace",
		"-h",
	}
}

// Hyperspace doesn't not have this functionality
func (hyperspaceCommander) AddKey(chainID, keyName, coinType, homeDir string) []string {
	panic("[AddKey] Do not call me")
}

func (c *hyperspaceCommander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	fmt.Println("[hyperspace] CreateChannel", pathName, homeDir)
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
		"--delay-period",
		"0",
		"--port-id",
		opts.SourcePortName,
		"--order",
		"unordered",
		"--version",
		opts.Version,
	}
}

func (c *hyperspaceCommander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	fmt.Println("[hyperspace] CreateClients", pathName, opts, homeDir)
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
		"--delay-period",
		"0",
		"--port-id",
		"transfer",
		"--order",
		"unordered",
	}
}

func (c *hyperspaceCommander) CreateConnections(pathName, homeDir string) []string {
	fmt.Println("[hyperspace] CreateConnections", pathName, homeDir)
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
		"0",
		"--port-id",
		"transfer",
		"--order",
		"unordered",
	}
}

// Hyperspace doesn't not have this functionality
func (hyperspaceCommander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	panic("[FlushAcknowledgements] Do not call me")
}

// Hyperspace doesn't not have this functionality
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

// Hyperspace does not have paths, just two configs
func (hyperspaceCommander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	panic("[UpdatePath] Do not call me")

}

// Prints chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
func (hyperspaceCommander) GetChannels(chainID, homeDir string) []string {
	fmt.Println("[hyperspace] Get Channels")
	configFilePath := path.Join(homeDir, chainID+".config")
	return []string{
		"cat",
		configFilePath,
	}
}

// Prints chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
func (hyperspaceCommander) GetConnections(chainID, homeDir string) []string {
	fmt.Println("[hyperspace] Get Connections")
	configFilePath := path.Join(homeDir, chainID+".config")
	return []string{
		"cat",
		configFilePath,
	}
}

// Prints chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
func (hyperspaceCommander) GetClients(chainID, homeDir string) []string {
	fmt.Println("[hyperspace] Get Clients")
	configFilePath := path.Join(homeDir, chainID+".config")
	return []string{
		"cat",
		configFilePath,
	}
}

// Hyperspace does not have link cmd, call create clients, create connection, and create channel
func (hyperspaceCommander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpt ibc.CreateClientOptions) []string {
	panic("[LinkPath] Do not use me")
}

// There is no hyperspace call to restore the key, so this can't return an executable.
// HyperspaceRelayer's RestoreKey will restore the key in the chain's config file
func (hyperspaceCommander) RestoreKey(chainID, bech32Prefix, coinType, mnemonic, homeDir string) []string {
	panic("[RestoreKey] Do not use me")
}

// hyperspace can only start 1 path
func (c *hyperspaceCommander) StartRelayer(homeDir string, pathNames ...string) []string {
	fmt.Println("[hyperspace] StartRelayer", homeDir, pathNames)
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
		"--delay-period",
		"0",
		"--port-id",
		"transfer",
		"--order",
		"unordered",
		"--version",
		"ics20-1",
	}
}

// Hyperspace doesn't not have this functionality
func (hyperspaceCommander) UpdateClients(pathName, homeDir string) []string {
	panic("[UpdateClients] Do not use me")
}

func (hyperspaceCommander) ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
	fmt.Println("[hyperspace] ConfigContent", cfg, keyName, rpcAddr, grpcAddr)
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
// DockerRelayer's RestoreKey will restore the key in the chain's config file
func (hyperspaceCommander) ParseAddKeyOutput(stdout, stderr string) (ibc.Wallet, error) {
	panic("[ParseAddKeyOutput] Do not call me")
}

// There is no hyperspace call to restore the key, so there is no stdout to parse.
// DockerRelayer's RestoreKey will restore the key in the chain's config file
func (hyperspaceCommander) ParseRestoreKeyOutput(stdout, stderr string) string {
	panic("[ParseRestoreKeyOutput] Do not call me")
}

type ChannelsOutput struct {
	Channels [][]string `toml:"channel_whitelist"`
}

// Parses output of chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
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
// Only supports 1 connection and limited info
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
// Only supports 1 client
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

func (hyperspaceCommander) Init(homeDir string) []string {
	fmt.Println("[hyperspace] Init", homeDir)
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
