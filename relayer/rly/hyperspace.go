// Package rly provides an interface to the cosmos relayer running in a Docker container.
package rly

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	keys "github.com/cosmos/btcutil/hdkeychain"
	"github.com/docker/docker/client"
	"github.com/pelletier/go-toml/v2"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/relayer"
	"go.uber.org/zap"
)

// HyperspaceRelayer is the ibc.Relayer implementation for github.com/ComposableFi/hyperspace.
type HyperspaceRelayer struct {
	// Embedded DockerRelayer so commands just work.
	*relayer.DockerRelayer
}

func NewHyperspaceRelayer(log *zap.Logger, testName string, cli *client.Client, networkID string, options ...relayer.RelayerOption) *HyperspaceRelayer {
	c := hyperspaceCommander{log: log}
	for _, opt := range options {
		switch o := opt.(type) {
		case relayer.RelayerOptionExtraStartFlags:
			c.extraStartFlags = o.Flags
		}
	}
	dr, err := relayer.NewDockerRelayer(context.TODO(), log, testName, cli, networkID, c, options...)
	if err != nil {
		panic(err) // TODO: return
	}

	r := &HyperspaceRelayer{
		DockerRelayer: dr,
	}

	return r
}

type HyperspaceRelayerChainConfig struct {
	Type             string   `toml:"type"`
	Name             string   `toml:"name"`
	ParaID           uint32   `toml:"para_id"`
	ParachainRPCURL  string   `toml:"parachain_rpc_url"`
	RelayChainRPCURL string   `toml:"relay_chain_rpc_url"`
	ClientID         string   `toml:"client_id"`
	ConnectionID     string   `toml:"connection_id"`
	BeefyActivation  uint32   `toml:"beefy_activation_block"`
	CommitmentPrefix string   `toml:"commitment_prefix"`
	PrivateKey       string   `toml:"private_key"`
	SS58Version      uint8    `toml:"ss58_version"`
	ChannelWhitelist []string `toml:"channel_whitelist"`
	FinalityProtocol string   `toml:"finality_protocol"`
	KeyType          string   `toml:"key_type"`
}

/*
	/// Chain name
	pub name: String,
	/// rpc url for cosmos
	pub rpc_url: Url,
	/// grpc url for cosmos
	pub grpc_url: Url,
	/// websocket url for cosmos
	pub websocket_url: Url,
	/// Cosmos chain Id
	pub chain_id: String,
	/// Light client id on counterparty chain
	pub client_id: Option<String>,
	/// Connection Id
	pub connection_id: Option<String>,
	/// Account prefix
	pub account_prefix: String,
	/// Store prefix
	pub store_prefix: String,
	/// Maximun transaction size
	pub max_tx_size: usize,
	/// The key that signs transactions
	pub keybase: KeyEntry,

*/

type KeyEntry struct {
	PublicKey  string `toml:"public_key"`
	PrivateKey string `toml:"private_key"`
	Account    string `toml:"account"`
	Address    []byte `toml:"address"`
}

type HyperspaceRelayerCosmosChainConfigValue struct {
	Name          string   `toml:"name"`
	RPCUrl        string   `toml:"rpc_url"`
	GRPCUrl       string   `toml:"grpc_url"`
	WebsocketUrl  string   `toml:"websocket_url"`
	ChainID       string   ` toml:"chain_id"`
	AccountPrefix string   `toml:"account_prefix"`
	StorePrefix   string   `toml:"store_prefix"`
	MaxTxSize     uint64   `toml:"max_tx_size"`
	Keybase       KeyEntry `toml:"keybase"`

	//Debug          bool    `json:"debug" toml:"debug"`
	//GasAdjustment  float64 `json:"gas-adjustment" toml:"gas_adjustment"`
	//GasPrices      string  `json:"gas-prices" toml:"gas_prices"`
	//Key            string  `json:"key" toml:"key"`
	//KeyringBackend string  `json:"keyring-backend" toml:"keyring_backend"`
	//OutputFormat   string  `json:"output-format" toml:"output_format"`
	//SignMode       string  `json:"sign-mode" toml:"sign_mode"`
	//Timeout        string  `json:"timeout" toml:"timeout"`
}

type HyperspaceRelayerCoreConfig struct {
	PrometheusEndpoint string
}

type HyperspaceRelayerConfig struct {
	ChainA HyperspaceRelayerChainConfig `toml:"chain_a"`
	ChainB HyperspaceRelayerChainConfig `toml:"chain_b"`
	Core   HyperspaceRelayerCoreConfig  `toml:"core"`
}

const (
	HyperspaceDefaultContainerImage   = "hyperspace"
	HyperspaceDefaultContainerVersion = "latest"
)

// HyperspaceCapabilities returns the set of capabilities of the Cosmos relayer.
//
// Note, this API may change if the rly package eventually needs
// to distinguish between multiple rly versions.
func HyperspaceCapabilities() map[relayer.Capability]bool {
	// RC1 matches the full set of capabilities as of writing.
	return nil // relayer.FullCapabilities()
}

func GenKey() KeyEntry {
	testVec1MasterHex := "000102030405060708090a0b0c0d0e0f"
	masterSeed, err := hex.DecodeString(testVec1MasterHex)
	if err != nil {
		panic(err)
	}
	net := chaincfg.SimNetParams
	extKey, err := keys.NewMaster(masterSeed, &net)
	if err != nil {
		panic(err)
	}
	extKey, err = extKey.Derive(0)
	if err != nil {
		panic(err)
	}

	privStr := extKey.String()
	pubKey, err := extKey.Neuter()
	if err != nil {
		panic(err)
	}
	pubKey, err = pubKey.Neuter()
	if err != nil {
		panic(err)
	}
	pubStr := pubKey.String()

	address, err := pubKey.Address(&net)
	if err != nil {
		panic(err)
	}
	/*
		addrBytes, err := c.GetAddress(egCtx, keyName)
		b32, err := types.Bech32ifyAddressBytes(config.Bech32Prefix, addrBytes)
	*/
	//KeyBech32
	account1 := types.MustBech32ifyAddressBytes("cosmos", address.ScriptAddress())
	fmt.Println("account1", account1)

	//account := address.EncodeAddress()
	// sdk.AccAddressFromBech32(user.Bech32Address(b.chain.Config().Bech32Prefix))
	account2, err := types.AccAddressFromBech32(address.EncodeAddress())
	fmt.Println("account2", account2)
	fmt.Println("account2", account2.String())
	//account3, err := types.AccAddressFromBech32(address.)
	//fmt.Println("account3", account3)

	return KeyEntry{
		PublicKey:  pubStr,
		PrivateKey: privStr,
		Account:    account2.String(),
		Address:    address.ScriptAddress(),
	}
}

func ChainConfigToHyperspaceRelayerChainConfig(chainConfig ibc.ChainConfig, keyName, rpcAddr, gprcAddr string) interface{} {
	chainType := chainConfig.Type
	if chainType == "polkadot" || chainType == "parachain" || chainType == "relaychain" {
		chainType = "parachain"
	}
	addrs := strings.Split(rpcAddr, ",")
	paraRpcAddr := rpcAddr
	relayRpcAddr := gprcAddr
	if len(addrs) > 1 {
		paraRpcAddr, relayRpcAddr = addrs[0], addrs[1]
	}

	if chainType == "parachain" {
		return HyperspaceRelayerChainConfig{
			Type:             chainType,
			Name:             chainConfig.Name,
			ParaID:           2001,
			ParachainRPCURL:  paraRpcAddr,
			RelayChainRPCURL: relayRpcAddr,
			ClientID:         "10-grandpa-0",
			ConnectionID:     "connection-0",
			CommitmentPrefix: "0x6962632f",
			PrivateKey:       "//Alice",
			SS58Version:      49,
			KeyType:          "sr25519",
			FinalityProtocol: "grandpa",
		}
	} else if chainType == "cosmos" {
		return HyperspaceRelayerCosmosChainConfigValue{
			ChainID:       chainConfig.ChainID,
			AccountPrefix: chainConfig.Bech32Prefix,
			GRPCUrl:       gprcAddr,
			RPCUrl:        rpcAddr,
			StorePrefix:   "",
			MaxTxSize:     200000,
			Keybase:       GenKey(),
			//Debug:          true,
			//GasAdjustment:  chainConfig.GasAdjustment,
			//GasPrices:      chainConfig.GasPrices,
			//KeyringBackend: "test",
			//OutputFormat:   "toml",
			//SignMode:       "direct",
			//Timeout:        "10s",
		}
	} else {
		panic(fmt.Sprintf("unsupported chain type %s", chainType))
	}
}

// hyperspaceCommander satisfies relayer.RelayerCommander.
type hyperspaceCommander struct {
	log             *zap.Logger
	extraStartFlags []string
}

func (hyperspaceCommander) Name() string {
	return "rly"
}

func (hyperspaceCommander) DockerUser() string {
	return "501:20" // docker run -it --rm --entrypoint echo ghcr.io/cosmos/relayer "$(id -u):$(id -g)"
}

func (hyperspaceCommander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	fmt.Println("[hyperspace] AddChainConfiguration ", containerFilePath, homeDir)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "chains", "add", "-f", containerFilePath,
		// "--home", homeDir,
	}
}

func (hyperspaceCommander) AddKey(chainID, keyName, homeDir string) []string {
	fmt.Println("[hyperspace] AddKey", chainID, keyName, homeDir)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "keys", "add", chainID, keyName,
		// "--home", homeDir,
	}
}

func (hyperspaceCommander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	fmt.Println("[hyperspace] CreateChannel", pathName, opts, homeDir)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "tx", "channel", pathName,
		// "--src-port", opts.SourcePortName,
		// "--dst-port", opts.DestPortName,
		// "--order", opts.Order.String(),
		// "--version", opts.Version,

		// "--home", homeDir,
	}
}

func (hyperspaceCommander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	fmt.Println("[hyperspace] CreateClients", pathName, opts, homeDir)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "tx", "clients", pathName, "--client-tp", opts.TrustingPeriod,
		//"--home", homeDir,
	}
}

// CreateClient passing a value of 0 for customeClientTrustingPeriod will use default
func (hyperspaceCommander) CreateClient(pathName, homeDir, customeClientTrustingPeriod string) []string {
	fmt.Println("[hyperspace] CreateClient", pathName, homeDir, customeClientTrustingPeriod)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "tx", "client", pathName, "--client-tp", customeClientTrustingPeriod,
		//"--home", homeDir,
	}
}

func (hyperspaceCommander) CreateConnections(pathName, homeDir string) []string {
	fmt.Println("[hyperspace] CreateConnections", pathName, homeDir)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "tx", "connection", pathName,
		//"--home", homeDir,
	}
}

func (hyperspaceCommander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	fmt.Println("[hyperspace] FlushAcknowledgements", pathName, channelID, homeDir)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "tx", "relay-acks", pathName, channelID,
		//"--home", homeDir,
	}
}

func (hyperspaceCommander) FlushPackets(pathName, channelID, homeDir string) []string {
	fmt.Println("[hyperspace] FlushPackets", pathName, channelID, homeDir)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "tx", "relay-pkts", pathName, channelID,
		//"--home", homeDir,
	}
}

func (hyperspaceCommander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	fmt.Println("[hyperspace] GeneratePath", srcChainID, dstChainID, pathName, homeDir)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "paths", "new", srcChainID, dstChainID, pathName,
		// "--home", homeDir,
	}
}

func (hyperspaceCommander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	fmt.Println("[hyperspace] UpdatePath", pathName, homeDir, filter)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "paths", "update", pathName,
		// "--home", homeDir,
		// "--filter-rule", filter.Rule,
		// "--filter-channels", strings.Join(filter.ChannelList, ","),
	}
}

func (hyperspaceCommander) GetChannels(chainID, homeDir string) []string {

	fmt.Println("[hyperspace] GetChannels", chainID, homeDir)
	return []string{
		"hyperspace",
		"query",
		"channels",
		chainID,
		"--config", "rococo-local.config",
		//"rly", "q", "channels", chainID,
		//"--home", homeDir,
	}
}

func (hyperspaceCommander) GetConnections(chainID, homeDir string) []string {
	fmt.Println("[hyperspace] GetConnections", chainID, homeDir)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "q", "connections", chainID,
		//"--home", homeDir,
	}
}

func (hyperspaceCommander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpt ibc.CreateClientOptions) []string {
	fmt.Println("[hyperspace] LinkPath", pathName, homeDir, channelOpts, clientOpt)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "tx", "link", pathName,
		// "--src-port", channelOpts.SourcePortName,
		// "--dst-port", channelOpts.DestPortName,
		// "--order", channelOpts.Order.String(),
		// "--version", channelOpts.Version,
		// "--client-tp", clientOpt.TrustingPeriod,

		// "--home", homeDir,
	}
}

func (hyperspaceCommander) RestoreKey(chainID, keyName, mnemonic, homeDir string) []string {
	fmt.Println("[hyperspace] RestoreKey", chainID, keyName, mnemonic, homeDir)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "keys", "restore", chainID, keyName, mnemonic,
		//"--home", homeDir,
	}
}

func (c hyperspaceCommander) StartRelayer(homeDir string, pathNames ...string) []string {
	fmt.Println("[hyperspace] StartRelayer", homeDir, pathNames)
	cmd := []string{
		"hyperspace",
		"relay",
		"--config-a", homeDir + "/config_a.toml",
		"--config-b", homeDir + "/config_b.toml",
		"--config-core", homeDir + "/config_core.toml",
	}
	cmd = append(cmd, c.extraStartFlags...)
	// cmd = append(cmd, pathNames...)
	return cmd
}

func (hyperspaceCommander) UpdateClients(pathName, homeDir string) []string {
	fmt.Println("[hyperspace] UpdateClients", pathName, homeDir)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "tx", "update-clients", pathName,
		// "--home", homeDir,
	}
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

func (hyperspaceCommander) ParseAddKeyOutput(stdout, stderr string) (ibc.Wallet, error) {
	fmt.Println("[hyperspace] ParseAddKeyOutput", stdout, stderr)

	var wallet ibc.Wallet
	err := json.Unmarshal([]byte(stdout), &wallet)
	return wallet, err
}

func (hyperspaceCommander) ParseRestoreKeyOutput(stdout, stderr string) string {
	fmt.Println("[hyperspace] ParseRestoreKeyOutput", stdout, stderr)
	//return strings.Replace(stdout, "\n", "", 1)
	return "5DdfLppz85oT7jPPw3vANQmJ3HM1V545NXnAb2RBkjqc6hdH"
}

func (c hyperspaceCommander) ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error) {
	fmt.Println("[hyperspace] ParseGetChannelsOutput", stdout, stderr)
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

func (c hyperspaceCommander) ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error) {
	fmt.Println("[hyperspace] ParseGetConnectionsOutput", stdout, stderr)

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

func (hyperspaceCommander) Init(homeDir string) []string {
	fmt.Println("[hyperspace] Init", homeDir)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "config", "init",
		// "--home", homeDir,
	}
}
