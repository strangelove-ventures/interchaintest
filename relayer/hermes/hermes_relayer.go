package hermes

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/client"
	"github.com/pelletier/go-toml"
	"github.com/strangelove-ventures/interchaintest/v6/ibc"
	"github.com/strangelove-ventures/interchaintest/v6/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v6/relayer"
	"go.uber.org/zap"
)

const (
	hermes                  = "hermes"
	defaultContainerImage   = "docker.io/informalsystems/hermes"
	DefaultContainerVersion = "1.0.0"

	hermesDefaultUidGid = "1000:1000"
	hermesHome          = "/home/hermes"
	hermesConfigPath    = ".hermes/config.toml"
)

var _ ibc.Relayer = &Relayer{}

// Relayer is the ibc.Relayer implementation for hermes.
type Relayer struct {
	*relayer.DockerRelayer
	paths        map[string]*pathConfiguration
	chainConfigs []ChainConfig
}

type ChainConfig struct {
	cfg                        ibc.ChainConfig
	keyName, rpcAddr, grpcAddr string
}

type pathConfiguration struct {
	chainA, chainB pathChainConfig
}

type pathChainConfig struct {
	chainID  string
	clientID string
}

func NewHermesRelayer(log *zap.Logger, testName string, cli *client.Client, networkID string, options ...relayer.RelayerOption) *Relayer {
	c := commander{log: log}
	options = append(options, relayer.HomeDir(hermesHome))
	dr, err := relayer.NewDockerRelayer(context.TODO(), log, testName, cli, networkID, c, options...)
	if err != nil {
		panic(err)
	}

	return &Relayer{
		DockerRelayer: dr,
	}
}

func (r *Relayer) populatePathConfig(pathName string) error {
	//, ok := r.paths[pathName]

	// Query things
	return nil
}

func (r *Relayer) validateConfig(ctx context.Context, rep ibc.RelayerExecReporter) error {
	cmd := []string{hermes, "--config", fmt.Sprintf("%s/%s", r.HomeDir(), hermesConfigPath), "config", "validate"}
	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) configContent(cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
	r.chainConfigs = append(r.chainConfigs, ChainConfig{
		cfg:      cfg,
		keyName:  keyName,
		rpcAddr:  rpcAddr,
		grpcAddr: grpcAddr,
	})
	hermesConfig := NewConfig(r.chainConfigs...)
	bz, err := toml.Marshal(hermesConfig)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func (r *Relayer) AddChainConfiguration(ctx context.Context, rep ibc.RelayerExecReporter, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
	configContent, err := r.configContent(chainConfig, keyName, rpcAddr, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to generate config content: %w", err)
	}

	fw := dockerutil.NewFileWriter(r.Logger(), r.Client(), r.TestName())
	if err := fw.WriteFile(ctx, r.VolumeName(), hermesConfigPath, configContent); err != nil {
		return fmt.Errorf("failed to rly config: %w", err)
	}

	return r.validateConfig(ctx, rep)
}

func (r *Relayer) LinkPath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) error {
	_, ok := r.paths[pathName]
	if !ok {
		return fmt.Errorf("path %s not found", pathName)
	}

	if err := r.CreateChannel(ctx, rep, pathName, channelOpts); err != nil {
		return err
	}

	return r.populatePathConfig(pathName)
}

func (r *Relayer) CreateChannel(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, opts ibc.CreateChannelOptions) error {
	pathConfig := r.paths[pathName]
	cmd := []string{hermes, "--json", "create", "channel", "--a-chain", pathConfig.chainA.chainID, "--b-chain", pathConfig.chainB.chainID, "--a-port", opts.SourcePortName, "--b-port", opts.DestPortName, "--new-client-connection", "--yes"}
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

func (r *Relayer) UpdateClients(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	pathConfig, ok := r.paths[pathName]
	if !ok {
		return fmt.Errorf("path %s not found", pathName)
	}
	updateChainACmd := []string{hermes, "--json", "update", "client", "--host-chain", pathConfig.chainA.chainID, "--client", pathConfig.chainA.clientID}
	res := r.Exec(ctx, rep, updateChainACmd, nil)
	if res.Err != nil {
		return res.Err
	}
	updateChainBCmd := []string{hermes, "--json", "update", "client", "--host-chain", pathConfig.chainB.chainID, "--client", pathConfig.chainB.clientID}
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
		return fmt.Errorf("failed to write mnemonic file: %w", err)
	}

	cmd := []string{hermes, "keys", "add", "--chain", chainID, "--mnemonic-file", fmt.Sprintf("%s/%s", r.HomeDir(), relativeMnemonicFilePath)}

	// Restoring a key should be near-instantaneous, so add a 1-minute timeout
	// to detect if Docker has hung.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}

	addrBytes := commander{}.ParseRestoreKeyOutput(string(res.Stdout), string(res.Stderr))

	r.Wallets[chainID] = NewWallet(chainID, addrBytes, mnemonic)

	return nil
}

func (r *Relayer) FlushAcknowledgements(ctx context.Context, rep ibc.RelayerExecReporter, pathName, channelID string) error {
	return r.FlushPackets(ctx, rep, pathName, channelID)
}

func (r *Relayer) FlushPackets(ctx context.Context, rep ibc.RelayerExecReporter, pathName, channelID string) error {
	//DESCRIPTION:
	//	Clear outstanding packets (i.e., packet-recv and packet-ack) on a given channel in both directions.
	//		The channel is identified by the chain, port, and channel IDs at one of its ends
	//
	//USAGE:
	//	hermes clear packets [OPTIONS] --chain <CHAIN_ID> --port <PORT_ID> --channel <CHANNEL_ID>
	//
	//		OPTIONS:
	//	--counterparty-key-name <COUNTERPARTY_KEY_NAME>
	//		use the given signing key for the counterparty chain (default: `counterparty_key_name`
	//	config)
	//
	//	-h, --help
	//	Print help information
	//
	//	--key-name <KEY_NAME>
	//	use the given signing key for the specified chain (default: `key_name` config)
	//
	//	REQUIRED:
	//	--chain <CHAIN_ID>        Identifier of the chain
	//	--channel <CHANNEL_ID>    Identifier of the channel
	//	--port <PORT_ID>          Identifier of the port

	path := r.paths[pathName]
	cmd := []string{hermes, "clear", "packets", "--chain", path.chainA.chainID, "--channel", channelID, "--port", "transfer"}
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}
