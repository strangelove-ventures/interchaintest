// Package rly provides an interface to the cosmos relayer running in a Docker container.
package hyperspace

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/docker/docker/client"
	"github.com/pelletier/go-toml/v2"
	"github.com/strangelove-ventures/ibctest/v6/chain/polkadot"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/relayer"
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

	c.dockerRelayer = dr

	r := &HyperspaceRelayer{
		DockerRelayer: dr,
	}

	return r
}

type HyperspaceRelayerSubstrateChainConfig struct {
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

type KeyEntry struct {
	PublicKey  string `toml:"public_key"`
	PrivateKey string `toml:"private_key"`
	Account    string `toml:"account"`
	Address    []byte `toml:"address"`
}

type HyperspaceRelayerCosmosChainConfig struct {
	Type          string   `toml:"type"` //New
	Name          string   `toml:"name"`
	RPCUrl        string   `toml:"rpc_url"`
	GRPCUrl       string   `toml:"grpc_url"`
	WebsocketUrl  string   `toml:"websocket_url"`
	ChainID       string   `toml:"chain_id"`
	AccountPrefix string   `toml:"account_prefix"`
	StorePrefix   string   `toml:"store_prefix"`
	Mnemonic      string   `toml:"mnemonic"` //new
	MaxTxSize     uint64   `toml:"max_tx_size"`
	//ConnectionId string `toml:"connection_id"` // connection-1
	//ClientId string `toml:"client_id"` // 07-tendermint-0
	//WasmCodeId string `toml:"wasm_code_id"` // "Hash"
	//WasmClientType string `toml:"wasm_client_type` // 10-grandpa
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

const (
	HyperspaceDefaultContainerImage   = "hyperspace"
	HyperspaceDefaultContainerVersion = "local"
)

// HyperspaceCapabilities returns the set of capabilities of the Cosmos relayer.
//
// Note, this API may change if the rly package eventually needs
// to distinguish between multiple rly versions.
func HyperspaceCapabilities() map[relayer.Capability]bool {
	// RC1 matches the full set of capabilities as of writing.
	return nil // relayer.FullCapabilities()
}

func GenKeyEntry(bech32Prefix, coinType, mnemonic string) KeyEntry {
	coinType64, err := strconv.ParseUint(coinType, 10, 32)
	if err != nil {
		return KeyEntry{}
	}
	algo := keyring.SignatureAlgo(hd.Secp256k1)
	hdPath := hd.CreateHDPath(uint32(coinType64), 0, 0).String()

	// create master key and derive first key for keyring
	derivedPriv, err := algo.Derive()(mnemonic, "", hdPath)
	if err != nil {
		return KeyEntry{}
	}

	privKey := algo.Generate()(derivedPriv)
	address := types.AccAddress(privKey.PubKey().Address())
	bech32Addr := types.MustBech32ifyAddressBytes(bech32Prefix, address)

	return KeyEntry{
		PublicKey:  hex.EncodeToString(privKey.PubKey().Bytes()), //PubKeySecp256k1{0316AE4C34FB51C56AFB8126CB9AD725BCF0BD7FB4AD1684FD50DC45A67CBC0A7D}
		PrivateKey: hex.EncodeToString(privKey.Bytes()),  // i.e. ac26db8374e68403a3cf38cc2b196d688d2f094cec0908978b2460d4442062f7
		Account:    bech32Addr , // i.e. cosmos1g5r2vmnp6lta9cpst4lzc4syy3kcj2lj0nuhmy
		Address:    address.Bytes(), // i.e. [69 6 166 110 97 215 215 210 224 48 93 126 44 86 4 36 109 137 43 242]
	}
}

func ChainConfigToHyperspaceRelayerChainConfig(chainConfig ibc.ChainConfig, keyName, rpcAddr, gprcAddr string) interface{} {
	chainType := chainConfig.Type
	if chainType == "polkadot" || chainType == "parachain" || chainType == "relaychain" {
		chainType = "parachain"
	}

	if chainType == "parachain" {
		addrs := strings.Split(rpcAddr, ",")
		paraRpcAddr := rpcAddr
		relayRpcAddr := gprcAddr
		if len(addrs) > 1 {
			paraRpcAddr, relayRpcAddr = addrs[0], addrs[1]
		}
		return HyperspaceRelayerSubstrateChainConfig{
			Type:             chainType,
			Name:             chainConfig.Name,
			ParaID:           2000,
			ParachainRPCURL:  paraRpcAddr,
			RelayChainRPCURL: relayRpcAddr,
			//ClientID:         "10-grandpa-0",
			//ConnectionID:     "connection-0",
			CommitmentPrefix: "0x6962632f",
			PrivateKey:       "//Alice",
			SS58Version:      49,
			KeyType:          "sr25519",
			FinalityProtocol: "grandpa",
		}
	} else if chainType == "cosmos" {
		return HyperspaceRelayerCosmosChainConfig{
			Name:          chainConfig.Name,
			ChainID:       chainConfig.ChainID,
			AccountPrefix: chainConfig.Bech32Prefix,
			GRPCUrl:       gprcAddr,
			RPCUrl:        rpcAddr,
			StorePrefix:   "",
			MaxTxSize:     200000,
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
	log               *zap.Logger
	chainConfigPaths  []string
	extraStartFlags   []string
	dockerRelayer     *relayer.DockerRelayer
}

func (hyperspaceCommander) Name() string {
	return "hyperspace"
}

func (hyperspaceCommander) DockerUser() string {
	return "501:20" // docker run -it --rm --entrypoint echo ghcr.io/cosmos/relayer "$(id -u):$(id -g)"
}

func (c hyperspaceCommander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	fmt.Println("[hyperspace] AddChainConfiguration ", containerFilePath, homeDir)
	c.chainConfigPaths = append(c.chainConfigPaths, containerFilePath)
	return []string{
		"hyperspace",
		"-h",
	}
}


// Hyperspace doesn't not have this functionality
func (hyperspaceCommander) AddKey(chainID, keyName, coinType, homeDir string) []string {
	panic("[AddKey] Do not call me")
}

func (hyperspaceCommander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	panic("[CreateChannel] Implement me")
	/*fmt.Println("[hyperspace] CreateChannel", pathName, opts, homeDir)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "tx", "channel", pathName,
		// "--src-port", opts.SourcePortName,
		// "--dst-port", opts.DestPortName,
		// "--order", opts.Order.String(),
		// "--version", opts.Version,

		// "--home", homeDir,
	}*/
}

func (hyperspaceCommander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	panic("[CreateClients] Implement me")
	/*fmt.Println("[hyperspace] CreateClients", pathName, opts, homeDir)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "tx", "clients", pathName, "--client-tp", opts.TrustingPeriod,
		//"--home", homeDir,
	}*/
}

// CreateClient passing a value of 0 for customeClientTrustingPeriod will use default
func (hyperspaceCommander) CreateClient(pathName, homeDir, customeClientTrustingPeriod string) []string {
	panic("[CreateClient] Implement me")
	/*fmt.Println("[hyperspace] CreateClient", pathName, homeDir, customeClientTrustingPeriod)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "tx", "client", pathName, "--client-tp", customeClientTrustingPeriod,
		//"--home", homeDir,
	}*/
}

func (hyperspaceCommander) CreateConnections(pathName, homeDir string) []string {
	panic("[CreateConnections] Implement me")
	/*fmt.Println("[hyperspace] CreateConnections", pathName, homeDir)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "tx", "connection", pathName,
		//"--home", homeDir,
	}*/
}

// Hyperspace doesn't not have this functionality
func (hyperspaceCommander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	panic("[FlushAcknowledgements] Do not call me")
}

// Hyperspace doesn't not have this functionality
func (hyperspaceCommander) FlushPackets(pathName, channelID, homeDir string) []string {
	panic("[FlushPackets] Do not call me")
}

func (hyperspaceCommander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	panic("[GeneratePath] Implement me")
	/*fmt.Println("[hyperspace] GeneratePath", srcChainID, dstChainID, pathName, homeDir)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "paths", "new", srcChainID, dstChainID, pathName,
		// "--home", homeDir,
	}*/
}

func (hyperspaceCommander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	panic("[UpdatePath] Implement me")
	/*fmt.Println("[hyperspace] UpdatePath", pathName, homeDir, filter)
	return []string{
		"hyperspace",
		"-h",
		// "rly", "paths", "update", pathName,
		// "--home", homeDir,
		// "--filter-rule", filter.Rule,
		// "--filter-channels", strings.Join(filter.ChannelList, ","),
	}*/
}

func (hyperspaceCommander) GetChannels(chainID, homeDir string) []string {
	panic("[GetChannels] Test me")
	/*fmt.Println("[hyperspace] GetChannels", chainID, homeDir)
	return []string{
		"hyperspace",
		"query",
		"channels",
		chainID,
		"--config", "rococo-local.config",
		//"rly", "q", "channels", chainID,
		//"--home", homeDir,
	}*/
}

func (hyperspaceCommander) GetConnections(chainID, homeDir string) []string {
	panic("[GetConnections] Implement me")
	/*fmt.Println("[hyperspace] GetConnections", chainID, homeDir)
	return []string{
		"hyperspace",
		"-h",
		//"rly", "q", "connections", chainID,
		//"--home", homeDir,
	}*/
}

func (hyperspaceCommander) GetClients(chainID, homeDir string) []string {
	panic("[GetClients] Implement me")
}

func (hyperspaceCommander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpt ibc.CreateClientOptions) []string {
	panic("[LinkPath] Implement me")
	/*fmt.Println("[hyperspace] LinkPath", pathName, homeDir, channelOpts, clientOpt)
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
	}*/
}

// There is no hyperspace call to restore the key, so this can't return an executable.
// DockerRelayer's RestoreKey will restore the key in the chain's config file
// For now, we will hack this for cosmos chain's use case
func (hyperspaceCommander) RestoreKey(chainID, bech32Prefix, coinType, mnemonic, homeDir string) []string {
	keyEntry := GenKeyEntry(bech32Prefix, coinType, mnemonic)
	return []string{
		keyEntry.Account,
		keyEntry.PrivateKey,
		keyEntry.PublicKey,
		string(keyEntry.Address),
	}
}

func (c hyperspaceCommander) StartRelayer(homeDir string, pathNames ...string) []string {
	panic("[StartRelayer] Implement me")
	/*fmt.Println("[hyperspace] StartRelayer", homeDir, pathNames)
	if len(c.chainConfig) < 2 {
		panic("[StartRelayer] Needs two chains to start")
	}
	cmd := []string{
		"hyperspace",
		"relay",
		"--config-a", c.chainConfigs[0],
		"--config-b", c.chainConfigs[1]",
		"--config-core", homeDir + "/core.config",
	}
	cmd = append(cmd, c.extraStartFlags...)
	// cmd = append(cmd, pathNames...)
	return cmd*/
}

// Hyperspace doesn't not have this functionality
func (hyperspaceCommander) UpdateClients(pathName, homeDir string) []string {
	panic("[UpdateClients] Implement me")
}

func (c hyperspaceCommander) ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
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

func (c hyperspaceCommander) ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error) {
	panic("[ParseGetChannelsOutput] Test me")
	/*fmt.Println("[hyperspace] ParseGetChannelsOutput", stdout, stderr)
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

	return channels, nil*/
}

func (c hyperspaceCommander) ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error) {
	panic("[ParseGetConnectionsOutput] Test me")
	/*fmt.Println("[hyperspace] ParseGetConnectionsOutput", stdout, stderr)

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

	return connections, nil*/
}

func (c hyperspaceCommander) ParseGetClientsOutput(stdout, stderr string) (ibc.ClientOutputs, error) {
	panic("[ParseGetClientsOutput] Implement me")
}

func (c hyperspaceCommander) Init(homeDir string) []string {
	fmt.Println("[hyperspace] Init", homeDir)
	// Return hyperspace help to ensure hyperspace binary is accessible
	return []string{
		"hyperspace",
		"-h",
	}
}

func (c hyperspaceCommander) CreateWallet(keyName, address, mnemonic string) ibc.Wallet {
	kp, err := signature.KeyringPairFromSecret(mnemonic, polkadot.Ss58Format)
	if err != nil {
		return NewWallet("", "", "")
	}
	return NewWallet("", kp.Address, mnemonic)
}