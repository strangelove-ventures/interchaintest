package hermes

import (
	"context"

	"github.com/strangelove-ventures/interchaintest/v6/ibc"
	"github.com/strangelove-ventures/interchaintest/v6/relayer"
	"go.uber.org/zap"
)

const (
	name                    = "hermes"
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

func (r *Relayer) LinkPath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) error {
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

func (r *Relayer) GeneratePath(ctx context.Context, rep ibc.RelayerExecReporter, srcChainID, dstChainID, pathName string) error {
	// generate path gets called in interchain.Build. Hermes doesn't have this concept so something will need to be changed here.
	return nil
}

var _ relayer.RelayerCommander = &commander{}

type commander struct {
	log *zap.Logger
}

func (c commander) Name() string {
	return name
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
		name, "config", "init",
		"--home", homeDir,
	}
}

func (c commander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	//TODO implement me
	panic("implement me")
}

func (c commander) AddKey(chainID, keyName, coinType, homeDir string) []string {
	// hermes keys add --chain foo --key-file <KEY_FILE>
	panic("implement me")
}

func (c commander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {

	// hermes create channel [OPTIONS] --a-chain <A_CHAIN_ID> --a-connection <A_CONNECTION_ID> --a-port <A_PORT_ID> --b-port <B_PORT_ID>
	// return []string{name, "create", "channel", "--a-chain", "CHAIN_ID", "--a-connection", "<A_CONNECTION_ID>", "--a-port", opts.SourcePortName, "--b-port", opts.DestPortName}
	return []string{name, "create", "channel", "--a-chain", "?????", "--a-port", opts.SourcePortName, "--b-port", opts.DestPortName, "--new-client-connection"}

}

func (c commander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	//TODO implement me
	panic("implement me")
}

func (c commander) CreateConnections(pathName, homeDir string) []string {
	//TODO implement me
	panic("implement me")
}

func (c commander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	//TODO implement me
	panic("implement me")
}

func (c commander) FlushPackets(pathName, channelID, homeDir string) []string {
	//TODO implement me
	panic("implement me")
}

func (c commander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	//TODO implement me
	panic("implement me")
}

func (c commander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	panic("implement me")
}

func (c commander) GetChannels(chainID, homeDir string) []string {
	return []string{name, "query", "channels", "--home", homeDir, "--chain", chainID}
}

func (c commander) GetConnections(chainID, homeDir string) []string {
	panic("implement me")
}

func (c commander) GetClients(chainID, homeDir string) []string {
	panic("implement me")
}

func (c commander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) []string {
	panic("implement me")
}

func (c commander) RestoreKey(chainID, keyName, coinType, mnemonic, homeDir string) []string {
	panic("implement me")
}

func (c commander) StartRelayer(homeDir string, pathNames ...string) []string {
	return []string{"hermes", "start", "--full-scan", "--home", homeDir}
}

func (c commander) UpdateClients(pathName, homeDir string) []string {
	panic("implement me")
}

func (c commander) CreateWallet(keyName, address, mnemonic string) ibc.Wallet {
	return NewWallet(keyName, address, mnemonic)
}
