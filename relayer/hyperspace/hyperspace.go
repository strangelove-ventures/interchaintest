// Package rly provides an interface to the cosmos relayer running in a Docker container.
package hyperspace

import (
	"context"
	//"encoding/hex"
	"fmt"
	"path"
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
	dr, err := relayer.NewDockerRelayer(context.TODO(), log, testName, cli, networkID, &c, options...)
	if err != nil {
		panic(err) // TODO: return
	}

	//c.dockerRelayer = dr

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
	//ClientID         string   `toml:"client_id"`
	//ConnectionID     string   `toml:"connection_id"`
	BeefyActivation  uint32   `toml:"beefy_activation_block"`
	CommitmentPrefix string   `toml:"commitment_prefix"`
	PrivateKey       string   `toml:"private_key"`
	SS58Version      uint8    `toml:"ss58_version"`
	ChannelWhitelist []string `toml:"channel_whitelist"`
	FinalityProtocol string   `toml:"finality_protocol"`
	KeyType          string   `toml:"key_type"`
	//WasmCodeId       string   `toml:"wasm_code_id"`
}

type KeyEntry struct {
	PublicKey  string `toml:"public_key"`
	PrivateKey string `toml:"private_key"`
	Account    string `toml:"account"`
	Address    []byte `toml:"address"`
}

type HyperspaceRelayerCosmosChainConfig struct {
	Type           string   `toml:"type"` //New
	Name           string   `toml:"name"`
	RPCUrl         string   `toml:"rpc_url"`
	GRPCUrl        string   `toml:"grpc_url"`
	WebsocketUrl   string   `toml:"websocket_url"`
	ChainID        string   `toml:"chain_id"`
	AccountPrefix  string   `toml:"account_prefix"`
	StorePrefix    string   `toml:"store_prefix"`
	MaxTxSize      uint64   `toml:"max_tx_size"`
	WasmCodeId     string   `toml:"wasm_code_id"`
	//ConnectionId string `toml:"connection_id"` // connection-1
	//ClientId string `toml:"client_id"` // 07-tendermint-0
	Keybase        KeyEntry `toml:"keybase"`

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
	_ = types.MustBech32ifyAddressBytes(bech32Prefix, address)
	//bech32Addr := types.MustBech32ifyAddressBytes(bech32Prefix, address)

	// Use test keys temporarily
	return KeyEntry{
		PublicKey:  "spub4W7TSjsuqcUE17mSB2ajhZsbwkefsHWKsXCbERimu3z2QLN9EFgqqpppiBn4tTNPFoNVTo1b3BgCZAaFJuUgTZeFhzJjUHkK8X7kSC5c7yn",
		PrivateKey: "sprv8H873EM21Euvndgy513jLRvsPipBTpnUWJGzS3KALiT3XY2zgiNbJ2WLrvPzRhg7GuAoujHd5d6cpBe887vTbJghja8kmRdkHoNgamx6WWr",
		Account:    "cosmos1nnypkcfrvu3e9dhzeggpn4kh622l4cq7wwwrn0",
		Address:    []byte{156, 200, 27, 97, 35, 103, 35, 146, 182, 226, 202, 16, 25, 214, 215, 210, 149, 250, 224, 30},
		//PublicKey:  hex.EncodeToString(privKey.PubKey().Bytes()), // i.e. 02c1732ca9cb7c6efaa7c205887565b9787cab5ebdb7bc1dd872a21fc8c9efb56a
		//PrivateKey: hex.EncodeToString(privKey.Bytes()),  // i.e. ac26db8374e68403a3cf38cc2b196d688d2f094cec0908978b2460d4442062f7
		//Account:    bech32Addr , // i.e. cosmos1g5r2vmnp6lta9cpst4lzc4syy3kcj2lj0nuhmy
		//Address:    address.Bytes(), // i.e. [69 6 166 110 97 215 215 210 224 48 93 126 44 86 4 36 109 137 43 242]
	}
}

func ChainConfigToHyperspaceRelayerChainConfig(chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) interface{} {
	chainType := chainConfig.Type
	if chainType == "polkadot" || chainType == "parachain" || chainType == "relaychain" {
		chainType = "parachain"
	}

	if chainType == "parachain" {
		addrs := strings.Split(rpcAddr, ",")
		paraRpcAddr := rpcAddr
		relayRpcAddr := grpcAddr
		if len(addrs) > 1 {
			paraRpcAddr, relayRpcAddr = addrs[0], addrs[1]
		}
		return HyperspaceRelayerSubstrateChainConfig{
			Type:             chainType,
			Name:             chainConfig.Name,
			ParaID:           2000,
			ParachainRPCURL:  strings.Replace(strings.Replace(paraRpcAddr, "http", "ws", 1), "9933", "27451", 1),
			RelayChainRPCURL: strings.Replace(strings.Replace(relayRpcAddr, "http", "ws", 1),"9933", "27451", 1),
			//ClientID:         "10-grandpa-0",
			//ConnectionID:     "connection-0",
			CommitmentPrefix: "0x6962632f",
			PrivateKey:       "//Alice",
			SS58Version:      polkadot.Ss58Format,
			KeyType:          "sr25519",
			FinalityProtocol: "Grandpa",
		}
	} else if chainType == "cosmos" {
		wsUrl := strings.Replace(rpcAddr, "http", "ws", 1) + "/websocket"
		return HyperspaceRelayerCosmosChainConfig{
			Type:           chainType,
			Name:           chainConfig.Name,
			ChainID:        chainConfig.ChainID,
			AccountPrefix:  chainConfig.Bech32Prefix,
			GRPCUrl:        "http://"+grpcAddr,
			RPCUrl:         rpcAddr,
			StorePrefix:    "ibc",
			MaxTxSize:      200000,
			//WasmClientType: "10-grandpa",
			WebsocketUrl:   wsUrl,
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
	//dockerRelayer     *relayer.DockerRelayer
}

func (hyperspaceCommander) Name() string {
	return "hyperspace"
}

func (hyperspaceCommander) DockerUser() string {
	return "501:20" // docker run -it --rm --entrypoint echo ghcr.io/cosmos/relayer "$(id -u):$(id -g)"
}

func (c *hyperspaceCommander) AddChainConfiguration(containerFilePath, homeDir string) []string {
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

func (c *hyperspaceCommander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
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

func (c *hyperspaceCommander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	fmt.Println("[hyperspace] CreateClients", pathName, opts, homeDir)
	if(len(c.chainConfigPaths) < 2) {
		fmt.Println("ChainConfigPaths length: ", len(c.chainConfigPaths))
		panic("Hyperspace needs two chain configs")
	}
	return []string{
		"hyperspace",
		"create-clients",
		"--config-a",
		c.chainConfigPaths[0],
		"--config-b",
		c.chainConfigPaths[1],
		"--config-core",
		path.Join(homeDir, "core.config"),
		"--delay-period",
		"10",
		"--port-id",
		"transfer",
		"--order",
		"unordered",
	}
}

// Hyperspace doesn't implement this
func (hyperspaceCommander) CreateClient(pathName, homeDir, customClientTrustingPeriod string) []string {
	panic("[CreateClient] Do not use me")
}

func (c *hyperspaceCommander) CreateConnections(pathName, homeDir string) []string {
	fmt.Println("[hyperspace] CreateConnections", pathName, homeDir)
	return []string{
		"hyperspace",
		"create-connection",
		"--config-a",
		c.chainConfigPaths[0],
		"--config-b",
		c.chainConfigPaths[1],
		"--config-core",
		path.Join(homeDir, "core.config"),
		"--delay-period",
		"10",
		"--port-id",
		"transfer",
		"--order",
		"unordered",
	}
}

// Hyperspace doesn't not have this functionality
func (hyperspaceCommander) FlushAcknowledgements(pathName, channelID, homeDir string) []string {
	panic("[FlushAcknowledgements] Do not call me")
}

// Hyperspace doesn't not have this functionality
func (hyperspaceCommander) FlushPackets(pathName, channelID, homeDir string) []string {
	panic("[FlushPackets] Do not call me")
}

// Hyperspace does not have paths, just two configs
func (hyperspaceCommander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	panic("[GeneratePath] Do not call me")
}

// Hyperspace does not have paths, just two configs
func (hyperspaceCommander) UpdatePath(pathName, homeDir string, filter ibc.ChannelFilter) []string {
	panic("[UpdatePath] Do not call me")
	
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
	fmt.Println("[hyperspace] Get Clients")
	configFilePath := path.Join(homeDir, chainID + ".config")
	return []string{
		"cat",
		configFilePath,
	}
}

// Hyperspace does not have link cmd, call create clients, create connection, and create channel
func (hyperspaceCommander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpt ibc.CreateClientOptions) []string {
	panic("[LinkPath] Do not use me")
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

func (c *hyperspaceCommander) StartRelayer(homeDir string, pathNames ...string) []string {
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
	panic("[UpdateClients] Do not use me")
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

func (hyperspaceCommander) ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error) {
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

func (hyperspaceCommander) ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error) {
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

func (hyperspaceCommander) ParseGetClientsOutput(stdout, stderr string) (ibc.ClientOutputs, error) {
	clientId := ""
	chainId := ""
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "client_id") {
			fields := strings.Split(line, "\"")
			clientId = fields[1]
		}
		if strings.Contains(line, "chain_id") {
			fields := strings.Split(line, "\"")
			chainId = fields[1]
		}
	}
	return ibc.ClientOutputs{
		&ibc.ClientOutput{
			ClientID: clientId,
			ClientState: ibc.ClientState{
				ChainID: chainId, 
			},
		},
	}, nil
}

func (hyperspaceCommander) Init(homeDir string) []string {
	fmt.Println("[hyperspace] Init", homeDir)
	// Return hyperspace help to ensure hyperspace binary is accessible
	return []string{
		"hyperspace",
		"-h",
	}
}

func (hyperspaceCommander) CreateWallet(keyName, address, mnemonic string) ibc.Wallet {
	kp, err := signature.KeyringPairFromSecret(mnemonic, polkadot.Ss58Format)
	if err != nil {
		return NewWallet("", "", "")
	}
	return NewWallet("", kp.Address, mnemonic)
}