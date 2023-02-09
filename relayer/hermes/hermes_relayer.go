package hermes

import (
	"context"

	"github.com/strangelove-ventures/interchaintest/v6/ibc"
	"github.com/strangelove-ventures/interchaintest/v6/relayer"
	"go.uber.org/zap"
)

const (
	hermes                  = "hermes"
	defaultContainerImage   = "foo"
	defaultContainerVersion = "v1.0.0"

	// TODO: this was taken from RlyDefaultUidGid. Figure out what value should be used.
	hermesDefaultUidGid = "100:1000"
)

var _ ibc.Relayer = &Relayer{}

// Relayer is the ibc.Relayer implementation for hermes.
type Relayer struct {
	*relayer.DockerRelayer
}

func (*Relayer) AddChainConfiguration(ctx context.Context, rep ibc.RelayerExecReporter, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
	// We need a new implementation of AddChainConfiguration because the go relayer supports writing multiple chain config files
	// but hermes has them all in a single toml file. This function will get called once per chain and so will need to build up state somewhere and write
	// the final file at the end.
	return nil
}

func (r *Relayer) LinkPath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions, connectionOpts ibc.CreateConnectionOptions) error {
	if err := r.CreateClients(ctx, rep, pathName, clientOpts); err != nil {
		return err
	}

	if err := r.CreateConnections(ctx, rep, pathName, connectionOpts); err != nil {
		return err
	}

	if err := r.CreateChannel(ctx, rep, pathName, channelOpts); err != nil {
		return err
	}

	return nil
}

var _ relayer.RelayerCommander = &commander{}

type commander struct {
	log *zap.Logger
}

func (c commander) Name() string {
	return hermes
}

func (c commander) DefaultContainerImage() string {
	return defaultContainerImage
}

func (c commander) DefaultContainerVersion() string {
	return defaultContainerVersion
}

func (c commander) DockerUser() string {
	return hermesDefaultUidGid
}

func (c commander) ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
	return nil, nil
}

func (c commander) ParseAddKeyOutput(stdout, stderr string) (ibc.Wallet, error) {
	//TODO implement me
	panic("implement me")
}

func (c commander) ParseRestoreKeyOutput(stdout, stderr string) string {
	//TODO implement me
	panic("implement me")
}

func (c commander) ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error) {
	//TODO implement me
	panic("implement me")
}

func (c commander) ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error) {
	//TODO implement me
	panic("implement me")
}

func (c commander) ParseGetClientsOutput(stdout, stderr string) (ibc.ClientOutputs, error) {
	//TODO implement me
	panic("implement me")
}

func (c commander) Init(homeDir string) []string {
	return []string{
		hermes, "config", "init",
		"--home", homeDir,
	}
}

func (c commander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	return nil
}

func (c commander) AddKey(chainID, keyName, coinType, homeDir string) []string {
	// hermes keys add --chain foo --key-file <KEY_FILE>
	return nil
}

func (c commander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	// hermes create channel [OPTIONS] --a-chain <A_CHAIN_ID> --b-chain <B_CHAIN_ID> --a-port <A_PORT_ID> --b-port <B_PORT_ID> (--new-client-connection)
	return []string{hermes, "create", "channel", "--a-chain", opts.ChainAID, "--a-port", opts.SourcePortName, "--b-port", opts.DestPortName, "--home", homeDir}
}

func (c commander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	// hermes create client [OPTIONS] --host-chain <HOST_CHAIN_ID> --reference-chain <REFERENCE_CHAIN_ID>
	return nil
}

func (c commander) CreateConnections(_ string, connectionOptions ibc.CreateConnectionOptions, homeDir string) []string {
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

	//hermes create connection [OPTIONS] --a-chain <A_CHAIN_ID> --b-chain <B_CHAIN_ID>
	//hermes create connection [OPTIONS] --a-chain <A_CHAIN_ID> --a-client <A_CLIENT_ID> --b-client <B_CLIENT_ID>
	return []string{hermes, "create", "connection", "--a-chain", connectionOptions.ChainAID, "--b-chain", connectionOptions.ChainBID, "--home", homeDir}
}

func (c commander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	return nil
}

func (c commander) FlushPackets(pathName, channelID, homeDir string) []string {
	return nil
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
	return []string{hermes, "query", "channels", "--home", homeDir, "--chain", chainID}
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
	return []string{hermes, "query", "connections", "--chain", chainID, "--home", homeDir}
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
	return []string{hermes, "query", "clients", "--host-chain", chainID, "--home", homeDir}
}

func (c commander) RestoreKey(chainID, keyName, coinType, mnemonic, homeDir string) []string {
	return nil
}

func (c commander) StartRelayer(homeDir string, pathNames ...string) []string {
	return []string{hermes, "start", "--full-scan", "--home", homeDir}
}

func (c commander) UpdateClients(pathName, homeDir string) []string {
	return nil
}

func (c commander) CreateWallet(keyName, address, mnemonic string) ibc.Wallet {
	return NewWallet(keyName, address, mnemonic)
}

// Not in Hermes
func (r *Relayer) GeneratePath(ctx context.Context, rep ibc.RelayerExecReporter, srcChainID, dstChainID, pathName string) error {
	// generate path gets called in interchain.Build. Hermes doesn't have this concept so something will need to be changed here.
	return nil
}

// Not in Hermes
func (c commander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	return nil
}

// Not in Hermes
func (c commander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	return nil
}

// Not in Hermes
func (c commander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions, connectionOpts ibc.CreateConnectionOptions) []string {
	return nil
}
