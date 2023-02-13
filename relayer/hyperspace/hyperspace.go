// Package rly provides an interface to the cosmos relayer running in a Docker container.
package hyperspace

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"

	ibcexported "github.com/cosmos/ibc-go/v6/modules/core/03-connection/types"
	types23 "github.com/cosmos/ibc-go/v6/modules/core/23-commitment/types"
	"github.com/docker/docker/client"
	"github.com/misko9/go-substrate-rpc-client/v4/signature"
	"github.com/pelletier/go-toml/v2"
	"github.com/strangelove-ventures/ibctest/v6/chain/polkadot"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/relayer"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
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
	bech32Addr := types.MustBech32ifyAddressBytes(bech32Prefix, address)

	// Derive extended private key
	seed := bip39.NewSeed(mnemonic, "")
	masterKey, _ := bip32.NewMasterKey(seed)
	purposeKey, _ := masterKey.NewChildKey(0x8000002C) // 44'
	coinTypeKey, _ := purposeKey.NewChildKey(0x80000000 + uint32(coinType64)) // 118'
	accountKey, _ := coinTypeKey.NewChildKey(0x80000000) // 0'
	changeKey, _ := accountKey.NewChildKey(0) // 0
	indexKey, _ := changeKey.NewChildKey(0) // 0

	return KeyEntry{
		PublicKey:  indexKey.PublicKey().B58Serialize(), // i.e. "xpub6GNKSnPmR5zN3Ef3EqYkSJTZzjzGecb1n1SqJRUNnoFPsyxviG7QyoVzjEjP3gfqRu7AvRrEZMfXJazz8pZgmYP6yvvdRqC2pWmWpeQTMBP"
		PrivateKey: indexKey.B58Serialize(), // i.e. "xprvA3Ny3GrsaiS4pkaa8p1k5AWqSi9nF9sAQnXEW34mETiR1BdnAioAS1BWsx3uAXKT3NbY6cpY2mQL6N7R8se1GVHqNkpjwc7rv5VRaQ9x8EB"
		Account:    bech32Addr, // i.e. "cosmos1pyxjp07wc207l7jecyr3wcmq9cr54tqwhcwugm"
		Address:    address.Bytes(), // i.e. [9, 13, 32, 191, 206, 194, 159, 239, 250, 89, 193, 7, 23, 99, 96, 46, 7, 74, 172, 14]
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
			WebsocketUrl:   wsUrl,
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
	return "1000:1000" // docker run -it --rm --entrypoint echo ghcr.io/cosmos/relayer "$(id -u):$(id -g)"
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
	fmt.Println("[hyperspace] CreateChannel", pathName, homeDir)
	if(len(c.chainConfigPaths) < 2) {
		fmt.Println("ChainConfigPaths length: ", len(c.chainConfigPaths))
		panic("Hyperspace needs two chain configs")
	}
	// Temporarily force simd for chain A and rococo for chain B
	simd := 1
	if strings.Contains(c.chainConfigPaths[0], "simd") {
		simd = 0
	}
	return []string{
		"hyperspace",
		"create-channel",
		"--config-a",
		c.chainConfigPaths[simd],
		"--config-b",
		c.chainConfigPaths[(simd+1)%2],
		"--config-core",
		path.Join(homeDir, "core.config"),
		"--delay-period",
		"0",
		//"10",
		"--port-id",
		opts.SourcePortName,
		"--order",
		"unordered",
		"--version",
		opts.Version,
	}
}

func (c *hyperspaceCommander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	fmt.Println("[hyperspace] CreateClients", pathName, opts, homeDir)
	if(len(c.chainConfigPaths) < 2) {
		fmt.Println("ChainConfigPaths length: ", len(c.chainConfigPaths))
		panic("Hyperspace needs two chain configs")
	}
	// Temporarily force simd for chain A and rococo for chain B
	simd := 1
	if strings.Contains(c.chainConfigPaths[0], "simd") {
		simd = 0
	}
	return []string{
		"hyperspace",
		"create-clients",
		"--config-a",
		c.chainConfigPaths[simd],
		"--config-b",
		c.chainConfigPaths[(simd+1)%2],
		"--config-core",
		path.Join(homeDir, "core.config"),
		"--delay-period",
		"0",
		//"10",
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
	if(len(c.chainConfigPaths) < 2) {
		fmt.Println("ChainConfigPaths length: ", len(c.chainConfigPaths))
		panic("Hyperspace needs two chain configs")
	}
	// Temporarily force simd for chain A and rococo for chain B
	simd := 1
	if strings.Contains(c.chainConfigPaths[0], "simd") {
		simd = 0
	}
	return []string{
		"hyperspace",
		"create-connection",
		"--config-a",
		c.chainConfigPaths[simd],
		"--config-b",
		c.chainConfigPaths[(simd+1)%2],
		"--config-core",
		path.Join(homeDir, "core.config"),
		"--delay-period",
		"0",
		//"10",
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
	panic("Panic because hyperspace will panic")
	/*fmt.Println("[hyperspace] Get Channels")
	configFilePath := path.Join(homeDir, chainID + ".config")
	return []string{
		"hyperspace",
		"query",
		"channels",
		"--config",
		configFilePath,
	}*/
}

// Prints chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
func (hyperspaceCommander) GetConnections(chainID, homeDir string) []string {
	fmt.Println("[hyperspace] Get Connections")
	configFilePath := path.Join(homeDir, chainID + ".config")
	return []string{
		"cat",
		configFilePath,
	}
}

// Prints chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
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
	fmt.Println("[hyperspace] StartRelayer", homeDir, pathNames)
	if(len(c.chainConfigPaths) < 2) {
		fmt.Println("ChainConfigPaths length: ", len(c.chainConfigPaths))
		panic("Hyperspace needs two chain configs")
	}
	// Temporarily force simd for chain A and rococo for chain B
	simd := 1
	if strings.Contains(c.chainConfigPaths[0], "simd") {
		simd = 0
	}
	return []string{
		"hyperspace",
		"relay",
		"--config-a",
		c.chainConfigPaths[simd],
		"--config-b",
		c.chainConfigPaths[(simd+1)%2],
		"--config-core",
		path.Join(homeDir, "core.config"),
		"--delay-period",
		"0",
		//"10",
		"--port-id",
		"transfer",
		"--order",
		"unordered",
		"--version",
		"ics20-1",
	}
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
	panic("Re-add once hyperspace can query channels successfully")
/*	fmt.Println("Channels output: ", stdout)
	
	return []ibc.ChannelOutput{
		{
			State: "",
			Ordering: "",
			Counterparty: ibc.ChannelCounterparty{
				PortID: "",
				ChannelID: "",
			},
			ConnectionHops: []string{},
			Version: "",
			PortID: "",
			ChannelID: "",
		},
	}, nil*/
}

// Parses output of chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
func (hyperspaceCommander) ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error) {
	clientId := ""
	connectionId := ""
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "client_id") {
			fields := strings.Split(line, "\"")
			clientId = fields[1]
		}
		if strings.Contains(line, "connection_id") {
			fields := strings.Split(line, "\"")
			connectionId = fields[1]
		}
	}
	return ibc.ConnectionOutputs{
		&ibc.ConnectionOutput{
			ID: connectionId,
			ClientID: clientId,
			Versions: []*ibcexported.Version{
				{
					Identifier: "",
					Features: []string{},
				},
			},
			State: "",
			Counterparty: &ibcexported.Counterparty{
				ClientId: "",
				ConnectionId: "",
				Prefix: types23.MerklePrefix{
					KeyPrefix: []byte{},
				},
			},
			DelayPeriod: "10",
		},
	}, nil
}

// Parses output of chain config which is populated by hyperspace
// Ideally, there should be a command from hyperspace to get this output
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