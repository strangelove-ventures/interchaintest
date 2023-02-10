package hermes

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v6/ibc"
	"github.com/strangelove-ventures/interchaintest/v6/internal/dockerutil"
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
	paths map[string]*pathConfiguration
}

type pathConfiguration struct {
	chainA, chainB pathChainConfig
}

type pathChainConfig struct {
	chainID  string
	clientID string
}

func NewHermesRelayer(log *zap.Logger, testName string, cli *client.Client, networkID string, options ...relayer.RelayerOption) (*Relayer, error) {
	c := commander{log: log}
	//for _, opt := range options {
	//	switch o := opt.(type) {
	//	case relayer.RelayerOptionExtraStartFlags:
	//c.extraStartFlags = o.Flags
	//}
	//}
	dr, err := relayer.NewDockerRelayer(context.TODO(), log, testName, cli, networkID, c, options...)
	if err != nil {
		return nil, err
	}

	return &Relayer{
		DockerRelayer: dr,
	}, nil
}

func (r *Relayer) populatePathConfig(pathName string) error {
	//, ok := r.paths[pathName]

	// Query things
	return nil
}

func (*Relayer) AddChainConfiguration(ctx context.Context, rep ibc.RelayerExecReporter, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
	// We need a new implementation of AddChainConfiguration because the go relayer supports writing multiple chain config files
	// but hermes has them all in a single toml file. This function will get called once per chain and so will need to build up state somewhere and write
	// the final file at the end.
	return nil
}

func (r *Relayer) LinkPath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) error {
	_, ok := r.paths[pathName]
	if !ok {
		return fmt.Errorf("path %s not found", pathName)
	}

	//if err := r.CreateClients(ctx, rep, pathName, clientOpts); err != nil {
	//	return err
	//}

	//if err := r.CreateConnections(ctx, rep, pathName); err != nil {
	//	return err
	//}

	if err := r.CreateChannel(ctx, rep, pathName, channelOpts); err != nil {
		return err
	}

	return r.populatePathConfig(pathName)
}

func (r *Relayer) UpdateClients(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	pathConfig, ok := r.paths[pathName]
	if !ok {
		return fmt.Errorf("path %s not found", pathName)
	}
	updateChainACmd := []string{hermes, "--json", "update", "client", "--host-chain", pathConfig.chainA.chainID, "--client", pathConfig.chainA.clientID, "--home", r.HomeDir()}
	res := r.Exec(ctx, rep, updateChainACmd, nil)
	if res.Err != nil {
		return res.Err
	}
	updateChainBCmd := []string{hermes, "--json", "update", "client", "--host-chain", pathConfig.chainB.chainID, "--client", pathConfig.chainB.clientID, "--home", r.HomeDir()}
	return r.Exec(ctx, rep, updateChainBCmd, nil).Err
}

func (r *Relayer) CreateClients(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, opts ibc.CreateClientOptions) error {
	pathConfig := r.paths[pathName]
	chainACreateClientCmd := []string{hermes, "--json", "create", "client", "--host-chain", pathConfig.chainA.chainID, "--reference-chain", pathConfig.chainB.chainID}
	res := r.Exec(ctx, rep, chainACreateClientCmd, nil)
	if res.Err != nil {
		return res.Err
	}

	// TODO: parse res and update pathConfig?

	chainBCreateClientCmd := []string{hermes, "--json", "create", "client", "--host-chain", pathConfig.chainB.chainID, "--reference-chain", pathConfig.chainA.chainID}
	res = r.Exec(ctx, rep, chainBCreateClientCmd, nil)
	if res.Err != nil {
		return res.Err
	}
	// TODO: parse res and update pathConfig?

	return res.Err
}

func (r *Relayer) RestoreKey(ctx context.Context, rep ibc.RelayerExecReporter, chainID, keyName, coinType, mnemonic string) error {
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

	relativeMnemonicFilePath := "mnemonic.txt"
	fw := dockerutil.NewFileWriter(r.Logger(), r.Client(), r.TestName())
	if err := fw.WriteFile(ctx, r.VolumeName(), relativeMnemonicFilePath, []byte(mnemonic)); err != nil {
		return fmt.Errorf("failed to write mnemoic file: %w", err)
	}

	cmd := []string{hermes, "keys", "add", "--chain", chainID, "--hd-path", coinType, "--mnemonic-file", fmt.Sprintf("/mnt/dockervolume/%s", relativeMnemonicFilePath)}

	// Restoring a key should be near-instantaneous, so add a 1-minute timeout
	// to detect if Docker has hung.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}

	addrBytes := commander{}.ParseRestoreKeyOutput(string(res.Stdout), string(res.Stderr))

	r.wallets[chainID] = commander{}.CreateWallet("", addrBytes, mnemonic)

	return nil
}

var _ relayer.RelayerCommander = &commander{}

type commander struct {
	log   *zap.Logger
	paths map[string]pathConfiguration
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
	pathConfig := c.paths[pathName]
	// hermes create channel [OPTIONS] --a-chain <A_CHAIN_ID> --b-chain <B_CHAIN_ID> --a-port <A_PORT_ID> --b-port <B_PORT_ID> (--new-client-connection)
	return []string{hermes, "--json", "create", "channel", "--a-chain", pathConfig.chainA.chainID, "--a-port", opts.SourcePortName, "--b-port", opts.DestPortName, "--home", homeDir, "--new-client-connection"}
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
	return []string{hermes, "--json", "query", "channels", "--home", homeDir, "--chain", chainID}
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
	return []string{hermes, "--json", "query", "connections", "--chain", chainID, "--home", homeDir}
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
	return []string{hermes, "--json", "query", "clients", "--host-chain", chainID, "--home", homeDir}
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

	return []string{hermes, "keys", "add", "--chain", chainID, "--hd-path", coinType}
}

func (c commander) StartRelayer(homeDir string, pathNames ...string) []string {
	return []string{hermes, "start", "--full-scan", "--home", homeDir}
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
	panic("path does not exist in hermes")
}

// Not in Hermes
func (c commander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	panic("path does not exist in hermes")
}

// Not in Hermes
func (c commander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) []string {
	panic("path does not exist in hermes")
}
