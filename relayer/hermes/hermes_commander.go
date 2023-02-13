package hermes

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/strangelove-ventures/interchaintest/v6/ibc"
	"github.com/strangelove-ventures/interchaintest/v6/relayer"
	"go.uber.org/zap"
)

var _ relayer.RelayerCommander = &commander{}

var (

	// parseRestoreKeyOutput extracts the address from the hermes output.
	// SUCCESS Restored key 'g2-2' (cosmos1czklnpzwaq3hfxtv6ne4vas2p9m5q3p3fgkz8e) on chain g2-2
	parseRestoreKeyOutput = regexp.MustCompile("\\((.*)\\)")
)

type commander struct {
	log   *zap.Logger
	paths map[string]*pathConfiguration
}

func (c commander) Name() string {
	return hermes
}

func (c commander) DefaultContainerImage() string {
	return defaultContainerImage
}

func (c commander) DefaultContainerVersion() string {
	return DefaultContainerVersion
}

func (c commander) DockerUser() string {
	return hermesDefaultUidGid
}

func (c commander) ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
	panic("shouldn't be called::ConfigContent")
}

func (c commander) ParseAddKeyOutput(stdout, stderr string) (ibc.Wallet, error) {
	fmt.Println(stdout)
	fmt.Println(stderr)
	//TODO implement me
	panic("implement me")
}

// ParseRestoreKeyOutput extracts the address from the hermes output.
func (c commander) ParseRestoreKeyOutput(stdout, stderr string) string {
	fullMatchIdx, addressGroupIdx := 0, 1
	return parseRestoreKeyOutput.FindAllStringSubmatch(stdout, -1)[fullMatchIdx][addressGroupIdx]
}

type ChannelOutputResult struct {
	Result []ChannelResult `json:"result"`
	Status string          `json:"status"`
}

type ChannelResult struct {
	ChannelEnd             ChannelEnd `json:"channel_end"`
	CounterPartyChannelEnd ChannelEnd `json:"counterparty_channel_end"`
}

type ChannelEnd struct {
	ConnectionHops []string         `json:"connection_hops"`
	Ordering       string           `json:"ordering"`
	State          string           `json:"state"`
	Version        string           `json:"version"`
	Remote         ChannelAndPortId `json:"remote"`
}

type ChannelAndPortId struct {
	ChannelID string `json:"channel_id"`
	PortID    string `json:"port_id"`
}

func (c commander) ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error) {
	var result ChannelOutputResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return nil, err
	}

	var ibcChannelOutput []ibc.ChannelOutput
	for _, r := range result.Result {
		ibcChannelOutput = append(ibcChannelOutput, ibc.ChannelOutput{
			State:    r.ChannelEnd.State,
			Ordering: r.ChannelEnd.Ordering,
			Counterparty: ibc.ChannelCounterparty{
				PortID:    r.CounterPartyChannelEnd.Remote.PortID,
				ChannelID: r.CounterPartyChannelEnd.Remote.ChannelID,
			},
			ConnectionHops: r.ChannelEnd.ConnectionHops,
			Version:        r.ChannelEnd.Version,
			PortID:         r.ChannelEnd.Remote.PortID,
			ChannelID:      r.ChannelEnd.Remote.ChannelID,
		})
	}

	return ibcChannelOutput, nil
}

func (c commander) ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error) {
	fmt.Println(stdout)
	fmt.Println(stderr)
	panic("implement me")
}

func (c commander) ParseGetClientsOutput(stdout, stderr string) (ibc.ClientOutputs, error) {
	fmt.Println(stdout)
	fmt.Println(stderr)
	panic("implement me")
}

func (c commander) Init(homeDir string) []string {
	return nil
}

func (c commander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	return nil
}

func (c commander) AddKey(chainID, keyName, coinType, homeDir string) []string {
	// hermes keys add --chain foo --key-file <KEY_FILE>
	return nil
}

func (c commander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	panic("don't call me")
	//pathConfig := c.paths[pathName]
	//// hermes create channel [OPTIONS] --a-chain <A_CHAIN_ID> --b-chain <B_CHAIN_ID> --a-port <A_PORT_ID> --b-port <B_PORT_ID> (--new-client-connection)
	//return []string{hermes, "--json", "create", "channel", "--a-chain", pathConfig.chainA.chainID, "--b-chain", pathConfig.chainB.chainID, "--a-port", opts.SourcePortName, "--b-port", opts.DestPortName, "--new-client-connection"}
}

func (c commander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	pathConfig := c.paths[pathName]

	// hermes create client [OPTIONS] --host-chain <HOST_CHAIN_ID> --reference-chain <REFERENCE_CHAIN_ID>
	return []string{hermes, "--json", "create", "client", "--host-chain", pathConfig.chainA.chainID, "--reference-chain", ""}
}

func (c commander) CreateConnections(pathName string, homeDir string) []string {
	//DESCRIPTION:
	//	Create a new connection between two chains
	//
	//USAGE:
	//	hermes create connection [OPTIONS] --a-chain <A_CHAIN_ID> --b-chain <B_CHAIN_ID>
	//
	//		hermes create connection [OPTIONS] --a-chain <A_CHAIN_ID> --a-client <A_CLIENT_ID> --b-client <B_CLIENT_ID>
	//
	//		OPTIONS:
	//	--delay <DELAY>    Delay period parameter for the new connection (seconds) [default: 0]
	//	-h, --help             Print help information
	//
	//	FLAGS:
	//	--a-chain <A_CHAIN_ID>      Identifier of the side `a` chain for the new connection
	//	--a-client <A_CLIENT_ID>    Identifier of client hosted on chain `a`; default: None (creates
	//	a new client)
	//	--b-chain <B_CHAIN_ID>      Identifier of the side `b` chain for the new connection
	//	--b-client <B_CLIENT_ID>    Identifier of client hosted on chain `b`; default: None (creates
	//	a new client)

	pathConfig := c.paths[pathName]
	return []string{hermes, "--json", "create", "connection", "--a-chain", pathConfig.chainA.chainID, "--b-chain", pathConfig.chainB.chainID, "--home", homeDir}
}

func (c commander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	panic("implemented one layer above")
}

func (c commander) FlushPackets(pathName, channelID, homeDir string) []string {
	panic("implemented one layer above")
}

func (c commander) GetChannels(chainID, homeDir string) []string {
	//DESCRIPTION:
	//	Query the identifiers of all channels on a given chain
	//
	//USAGE:
	//	hermes query channels [OPTIONS] --chain <CHAIN_ID>
	//
	//		OPTIONS:
	//	--counterparty-chain <COUNTERPARTY_CHAIN_ID>
	//		Filter the query response by the this counterparty chain
	//
	//	-h, --help
	//	Print help information
	//
	//	--show-counterparty
	//	Show the counterparty chain, port, and channel
	//
	//	--verbose
	//	Enable verbose output, displaying the client and connection ids for each channel in the
	//	response
	//
	//REQUIRED:
	//	--chain <CHAIN_ID>    Identifier of the chain to query
	return []string{hermes, "--json", "query", "channels", "--chain", chainID, "--show-counterparty", "--verbose"}
}

func (c commander) GetConnections(chainID, homeDir string) []string {
	//DESCRIPTION:
	//	Query the identifiers of all connections on a chain
	//
	//USAGE:
	//	hermes query connections [OPTIONS] --chain <CHAIN_ID>
	//
	//		OPTIONS:
	//	--counterparty-chain <COUNTERPARTY_CHAIN_ID>
	//		Filter the query response by the counterparty chain
	//
	//	-h, --help
	//	Print help information
	//
	//	--verbose
	//	Enable verbose output, displaying the client for each connection in the response
	//
	//REQUIRED:
	//	--chain <CHAIN_ID>    Identifier of the chain to query
	return []string{hermes, "--json", "query", "connections", "--chain", chainID}
}

func (c commander) GetClients(chainID, homeDir string) []string {
	//DESCRIPTION:
	//	Query the identifiers of all clients on a chain
	//
	//USAGE:
	//	hermes query clients [OPTIONS] --host-chain <HOST_CHAIN_ID>
	//
	//		OPTIONS:
	//	-h, --help
	//	Print help information
	//
	//	--omit-chain-ids
	//	Omit printing the reference (or target) chain for each client
	//
	//	--reference-chain <REFERENCE_CHAIN_ID>
	//		Filter for clients which target a specific chain id (implies '--omit-chain-ids')
	//
	//REQUIRED:
	//	--host-chain <HOST_CHAIN_ID>    Identifier of the chain to query
	return []string{hermes, "--json", "query", "clients", "--host-chain", chainID}
}

func (c commander) RestoreKey(chainID, keyName, coinType, mnemonic, homeDir string) []string {
	//DESCRIPTION:
	//	Adds key to a configured chain or restores a key to a configured chain using a mnemonic
	//
	//USAGE:
	//	hermes keys add [OPTIONS] --chain <CHAIN_ID> --key-file <KEY_FILE>
	//
	//		hermes keys add [OPTIONS] --chain <CHAIN_ID> --mnemonic-file <MNEMONIC_FILE>
	//
	//		OPTIONS:
	//	-h, --help                   Print help information
	//	--hd-path <HD_PATH>      Derivation path for this key [default: m/44'/118'/0'/0/0]
	//	--key-name <KEY_NAME>    Name of the key (defaults to the `key_name` defined in the config)
	//	--overwrite              Overwrite the key if there is already one with the same key name
	//
	//	FLAGS:
	//	--chain <CHAIN_ID>                 Identifier of the chain
	//	--key-file <KEY_FILE>              Path to the key file
	//	--mnemonic-file <MNEMONIC_FILE>    Path to file containing mnemonic to restore the key from

	return []string{hermes, "keys", "add", "--chain", chainID}
}

func (c commander) StartRelayer(homeDir string, pathNames ...string) []string {
	return []string{hermes, "--json", "start", "--full-scan"}
}

func (c commander) UpdateClients(pathName, homeDir string) []string {
	//DESCRIPTION:
	//	Update an IBC client
	//
	//USAGE:
	//	hermes update client [OPTIONS] --host-chain <HOST_CHAIN_ID> --client <CLIENT_ID>
	//
	//		OPTIONS:
	//	-h, --help
	//	Print help information
	//
	//	--height <REFERENCE_HEIGHT>
	//		The target height of the client update. Leave unspecified for latest height.
	//
	//	--trusted-height <REFERENCE_TRUSTED_HEIGHT>
	//		The trusted height of the client update. Leave unspecified for latest height.
	//
	//		REQUIRED:
	//	--client <CLIENT_ID>            Identifier of the chain targeted by the client
	//	--host-chain <HOST_CHAIN_ID>    Identifier of the chain that hosts the client
	return nil
}

func (c commander) CreateWallet(keyName, address, mnemonic string) ibc.Wallet {
	return NewWallet(keyName, address, mnemonic)
}

// Not in Hermes
func (r *Relayer) GeneratePath(ctx context.Context, rep ibc.RelayerExecReporter, srcChainID, dstChainID, pathName string) error {
	if r.paths == nil {
		r.paths = map[string]*pathConfiguration{}
	}
	r.paths[pathName] = &pathConfiguration{
		chainA: pathChainConfig{
			chainID:  srcChainID,
			clientID: "",
		},
		chainB: pathChainConfig{
			chainID:  dstChainID,
			clientID: "",
		},
	}
	return nil
}

// Not in Hermes
func (c commander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	panic("path does not exist in hermes::GeneratePath")
}

// Not in Hermes
func (c commander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	panic("path does not exist in hermes::UpdatePath")
}

// Not in Hermes
func (c commander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) []string {
	panic("path does not exist in hermes::LinkPath")
}
