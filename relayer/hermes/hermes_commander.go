package hermes

import (
	"context"
	"encoding/json"

	"github.com/strangelove-ventures/interchaintest/v6/ibc"
	"github.com/strangelove-ventures/interchaintest/v6/relayer"
	"go.uber.org/zap"
)

var _ relayer.RelayerCommander = &commander{}

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
	panic("implement me")
}

func (c commander) ParseGetClientsOutput(stdout, stderr string) (ibc.ClientOutputs, error) {
	panic("implement me")
}

func (c commander) Init(homeDir string) []string {
	return nil
}

func (c commander) GetChannels(chainID, homeDir string) []string {
	// the --verbose and --show-counterparty options are required to get enough information to correctly populate
	// the path.
	return []string{hermes, "--json", "query", "channels", "--chain", chainID, "--show-counterparty", "--verbose"}
}

func (c commander) GetConnections(chainID, homeDir string) []string {
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

func (c commander) StartRelayer(homeDir string, pathNames ...string) []string {
	// TODO: only look at paths (probably needs to happen at relayer level not commander)
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
	// TODO: implement
	return nil
}

func (c commander) CreateWallet(keyName, address, mnemonic string) ibc.Wallet {
	return NewWallet(keyName, address, mnemonic)
}

func (c commander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	// TODO: figure out how to implement this.
	panic("implement me")
}

func (c commander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	panic("generate path implemented in hermes relayer not the commander")
}

// the following methods do not have a single command that cleanly maps to a single hermes command without
// additional logic wrapping them. They have been implemented one layer up in the hermes relayer.

func (c commander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) []string {
	panic("link path implemented in hermes relayer not the commander")
}

func (c commander) RestoreKey(chainID, keyName, coinType, mnemonic, homeDir string) []string {
	panic("restore key implemented in hermes relayer not the commander")
}

func (c commander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	panic("add chain configuration implemented in hermes relayer not the commander")
}

func (c commander) AddKey(chainID, keyName, coinType, homeDir string) []string {
	panic("add key implemented in hermes relayer not the commander")
}

func (c commander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	panic("create channel implemented in hermes relayer not the commander")
}

func (c commander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	panic("create clients implemented in hermes relayer not the commander")
}

func (c commander) CreateConnections(pathName string, homeDir string) []string {
	panic("create connections implemented in hermes relayer not the commander")
}

func (c commander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	panic("flush acks implemented in hermes relayer not the commander")
}

func (c commander) FlushPackets(pathName, channelID, homeDir string) []string {
	panic("flush packets implemented in hermes relayer not the commander")
}

func (c commander) ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
	panic("config content implemented in hermes relayer not the commander")
}

func (c commander) ParseAddKeyOutput(stdout, stderr string) (ibc.Wallet, error) {
	panic("add key implemented in Hermes Relayer")
}

// ParseRestoreKeyOutput extracts the address from the hermes output.
func (c commander) ParseRestoreKeyOutput(stdout, stderr string) string {
	panic("implemented in Hermes Relayer")
}
