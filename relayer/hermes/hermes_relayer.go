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
	defaultContainerImage   = "docker.io/informalsystems/hermes"
	DefaultContainerVersion = "1.0.0"

	// TODO: this was taken from RlyDefaultUidGid. Figure out what value should be used.
	hermesDefaultUidGid = "1000:1000"
	hermesConfigPath    = ".hermes/config.toml"
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

func NewHermesRelayer(log *zap.Logger, testName string, cli *client.Client, networkID string, options ...relayer.RelayerOption) *Relayer {
	c := commander{log: log}
	//for _, opt := range options {
	//	switch o := opt.(type) {
	//	case relayer.RelayerOptionExtraStartFlags:
	//c.extraStartFlags = o.Flags
	//}
	//}
	options = append(options, relayer.RelayerHomeDir("/home/hermes"))
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
	//cmd := []string{hermes, "config", "validate"}
	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (r *Relayer) AddChainConfiguration(ctx context.Context, rep ibc.RelayerExecReporter, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
	configContent, err := commander{}.ConfigContent(ctx, chainConfig, keyName, rpcAddr, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to generate config content: %w", err)
	}

	r.Logger().Info(string(configContent))

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
		return fmt.Errorf("failed to write mnemoic file: %w", err)
	}

	//cmd := []string{hermes, "--config", fmt.Sprintf("%s/%s", r.HomeDir(), hermesConfigPath), "keys", "add", "--chain", chainID, "--mnemonic-file", fmt.Sprintf("%s/%s", r.HomeDir(), relativeMnemonicFilePath)}
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

	r.Wallets[chainID] = commander{}.CreateWallet("", addrBytes, mnemonic)

	return nil
}
