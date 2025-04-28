// Package rly provides an interface to the cosmos relayer running in a Docker container.
package rly

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moby/moby/client"
	"go.uber.org/zap"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
)

const (
	RlyDefaultUIDGID = "100:1000"
)

// CosmosRelayer is the ibc.Relayer implementation for github.com/cosmos/relayer.
type CosmosRelayer struct {
	// Embedded DockerRelayer so commands just work.
	*relayer.DockerRelayer
}

func NewCosmosRelayer(log *zap.Logger, testName string, cli *client.Client, networkID string, options ...relayer.RelayerOpt) *CosmosRelayer {
	c := &commander{log: log}

	dr, err := relayer.NewDockerRelayer(context.TODO(), log, testName, cli, networkID, c, options...)
	if err != nil {
		panic(err) // TODO: return
	}

	c.extraStartFlags = dr.GetExtraStartupFlags()

	r := &CosmosRelayer{
		DockerRelayer: dr,
	}

	return r
}

type CosmosRelayerChainConfigValue struct {
	AccountPrefix   string        `json:"account-prefix"`
	ChainID         string        `json:"chain-id"`
	Debug           bool          `json:"debug"`
	GRPCAddr        string        `json:"grpc-addr"`
	GasAdjustment   float64       `json:"gas-adjustment"`
	GasPrices       string        `json:"gas-prices"`
	Key             string        `json:"key"`
	KeyringBackend  string        `json:"keyring-backend"`
	OutputFormat    string        `json:"output-format"`
	RPCAddr         string        `json:"rpc-addr"`
	SignMode        string        `json:"sign-mode"`
	Timeout         string        `json:"timeout"`
	MinLoopDuration time.Duration `json:"min-loop-duration"`
}

type CosmosRelayerChainConfig struct {
	Type  string                        `json:"type"`
	Value CosmosRelayerChainConfigValue `json:"value"`
}

const (
	DefaultContainerImage   = "ghcr.io/cosmos/relayer"
	DefaultContainerVersion = "v2.5.2"
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
	if chainType == "polkadot" || chainType == "parachain" || chainType == "relaychain" {
		chainType = "substrate"
	}

	var err error
	loopDuration := time.Millisecond * 50
	for _, env := range chainConfig.Env {
		if strings.Contains(env, "ICTEST_RELAYER_LOOP_DURATION") {
			e := strings.Split(env, "=")
			if len(e) != 2 {
				panic(fmt.Sprintf("BUG: failed to parse %s", env))
			}

			loopDuration, err = time.ParseDuration(e[1])
			if err != nil {
				panic(fmt.Sprintf("BUG: failed to parse %s: %s", e[1], err))
			}
		}
	}

	return CosmosRelayerChainConfig{
		Type: chainType,
		Value: CosmosRelayerChainConfigValue{
			Key:             keyName,
			ChainID:         chainConfig.ChainID,
			RPCAddr:         rpcAddr,
			GRPCAddr:        gprcAddr,
			AccountPrefix:   chainConfig.Bech32Prefix,
			KeyringBackend:  keyring.BackendTest,
			GasAdjustment:   chainConfig.GasAdjustment,
			GasPrices:       chainConfig.GasPrices,
			Debug:           true,
			Timeout:         "10s",
			OutputFormat:    "json",
			SignMode:        "direct",
			MinLoopDuration: loopDuration,
		},
	}
}

// commander satisfies relayer.RelayerCommander.
type commander struct {
	log             *zap.Logger
	extraStartFlags []string
}

func NewCommander() relayer.RelayerCommander {
	return commander{}
}

func (commander) Name() string {
	return "rly"
}

func (commander) DockerUser() string {
	return RlyDefaultUIDGID // docker run -it --rm --entrypoint echo ghcr.io/cosmos/relayer "$(id -u):$(id -g)"
}

func (commander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	return []string{
		"rly", "chains", "add", "-f", containerFilePath,
		"--home", homeDir,
	}
}

func (commander) AddKey(chainID, keyName, coinType, signingAlgorithm, homeDir string) []string {
	return []string{
		"rly", "keys", "add", chainID, keyName,
		"--coin-type", fmt.Sprint(coinType),
		"--signing-algorithm", signingAlgorithm,
		"--home", homeDir,
	}
}

func (commander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	cmd := []string{
		"rly", "tx", "channel", pathName,
		"--src-port", opts.SourcePortName,
		"--dst-port", opts.DestPortName,
		"--order", opts.Order.String(),
		"--version", opts.Version,

		"--home", homeDir,
	}
	if opts.Override {
		cmd = append(cmd, "--override")
	}
	return cmd
}

func createClientOptsHelper(opts ibc.CreateClientOptions) []string {
	var clientOptions []string
	if opts.TrustingPeriod != "" {
		clientOptions = append(clientOptions, "--client-tp", opts.TrustingPeriod)
	}
	if opts.TrustingPeriodPercentage != 0 {
		clientOptions = append(clientOptions, "--client-tp-percentage", fmt.Sprint(opts.TrustingPeriodPercentage))
	}
	if opts.MaxClockDrift != "" {
		clientOptions = append(clientOptions, "--max-clock-drift", opts.MaxClockDrift)
	}
	if opts.Override {
		clientOptions = append(clientOptions, "--override")
	}

	return clientOptions
}

func (commander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	cmd := []string{"rly", "tx", "clients", pathName, "--home", homeDir}

	clientOptions := createClientOptsHelper(opts)
	cmd = append(cmd, clientOptions...)

	return cmd
}

func (commander) CreateClient(srcChainID, dstChainID, pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	cmd := []string{"rly", "tx", "client", srcChainID, dstChainID, pathName, "--home", homeDir}

	clientOptions := createClientOptsHelper(opts)
	cmd = append(cmd, clientOptions...)

	return cmd
}

func (commander) CreateConnections(pathName string, homeDir string) []string {
	return []string{
		"rly", "tx", "connection", pathName,
		"--home", homeDir,
	}
}

func (commander) Flush(pathName, channelID, homeDir string) []string {
	cmd := []string{"rly", "tx", "flush"}
	if pathName != "" {
		cmd = append(cmd, pathName)
		if channelID != "" {
			cmd = append(cmd, channelID)
		}
	}
	cmd = append(cmd, "--home", homeDir)
	return cmd
}

func (commander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	return []string{
		"rly", "paths", "new", srcChainID, dstChainID, pathName,
		"--home", homeDir,
	}
}

func (commander) UpdatePath(pathName, homeDir string, opts ibc.PathUpdateOptions) []string {
	command := []string{
		"rly", "paths", "update", pathName,
		"--home", homeDir,
	}

	if opts.ChannelFilter != nil {
		command = append(command,
			"--filter-rule", opts.ChannelFilter.Rule,
			"--filter-channels", strings.Join(opts.ChannelFilter.ChannelList, ","))
	}

	if opts.SrcChainID != nil {
		command = append(command, "--src-chain-id", *opts.SrcChainID)
	}
	if opts.DstChainID != nil {
		command = append(command, "--dst-chain-id", *opts.DstChainID)
	}
	if opts.SrcClientID != nil {
		command = append(command, "--src-client-id", *opts.SrcClientID)
	}
	if opts.DstClientID != nil {
		command = append(command, "--dst-client-id", *opts.DstClientID)
	}
	if opts.SrcConnID != nil {
		command = append(command, "--src-connection-id", *opts.SrcConnID)
	}
	if opts.DstConnID != nil {
		command = append(command, "--dst-connection-id", *opts.DstConnID)
	}

	return command
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

func (commander) GetClients(chainID, homeDir string) []string {
	return []string{
		"rly", "q", "clients", chainID,
		"--home", homeDir,
	}
}

func (commander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpt ibc.CreateClientOptions) []string {
	cmd := []string{
		"rly", "tx", "link", pathName,
		"--src-port", channelOpts.SourcePortName,
		"--dst-port", channelOpts.DestPortName,
		"--order", channelOpts.Order.String(),
		"--version", channelOpts.Version,
		"--debug",
		"--home", homeDir,
	}

	clientOptions := createClientOptsHelper(clientOpt)
	cmd = append(cmd, clientOptions...)

	return cmd
}

func (commander) RestoreKey(chainID, keyName, coinType, signingAlgorithm, mnemonic, homeDir string) []string {
	return []string{
		"rly", "keys", "restore", chainID, keyName, mnemonic,
		"--coin-type", fmt.Sprint(coinType),
		"--signing-algorithm", signingAlgorithm,
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
	cosmosRelayerChainConfig := ChainConfigToCosmosRelayerChainConfig(cfg, keyName, rpcAddr, grpcAddr)
	jsonBytes, err := json.Marshal(cosmosRelayerChainConfig)
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
	var wallet WalletModel
	err := json.Unmarshal([]byte(stdout), &wallet)
	rlyWallet := NewWallet("", wallet.Address, wallet.Mnemonic)
	return rlyWallet, err
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

func (c commander) ParseGetClientsOutput(stdout, stderr string) (ibc.ClientOutputs, error) {
	var clients ibc.ClientOutputs
	for _, client := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(client) == "" {
			continue
		}

		var clientOutput ibc.ClientOutput
		if err := json.Unmarshal([]byte(client), &clientOutput); err != nil {
			c.log.Error(
				"Error parsing client json",
				zap.Error(err),
			)

			continue
		}
		clients = append(clients, &clientOutput)
	}

	return clients, nil
}

func (commander) Init(homeDir string) []string {
	return []string{
		"rly", "config", "init",
		"--home", homeDir,
	}
}

func (c commander) CreateWallet(keyName, address, mnemonic string) ibc.Wallet {
	return NewWallet(keyName, address, mnemonic)
}
