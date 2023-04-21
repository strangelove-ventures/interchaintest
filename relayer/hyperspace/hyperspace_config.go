package hyperspace

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/interchaintest/v7/chain/polkadot"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
)

type HyperspaceRelayerCoreConfig struct {
	PrometheusEndpoint string
}

type HyperspaceRelayerSubstrateChainConfig struct {
	Type             string   `toml:"type"`
	Name             string   `toml:"name"`
	ParaID           uint32   `toml:"para_id"`
	ParachainRPCURL  string   `toml:"parachain_rpc_url"`
	RelayChainRPCURL string   `toml:"relay_chain_rpc_url"`
	BeefyActivation  uint32   `toml:"beefy_activation_block"`
	CommitmentPrefix string   `toml:"commitment_prefix"`
	PrivateKey       string   `toml:"private_key"`
	SS58Version      uint8    `toml:"ss58_version"`
	FinalityProtocol string   `toml:"finality_protocol"`
	KeyType          string   `toml:"key_type"`
	ChannelWhitelist []string `toml:"channel_whitelist"`
}

type KeyEntry struct {
	PublicKey  string `toml:"public_key"`
	PrivateKey string `toml:"private_key"`
	Account    string `toml:"account"`
	Address    []byte `toml:"address"`
}

type HyperspaceRelayerCosmosChainConfig struct {
	Type             string   `toml:"type"`
	Name             string   `toml:"name"`
	RPCUrl           string   `toml:"rpc_url"`
	GRPCUrl          string   `toml:"grpc_url"`
	WebsocketUrl     string   `toml:"websocket_url"`
	ChainID          string   `toml:"chain_id"`
	AccountPrefix    string   `toml:"account_prefix"`
	FeeDenom         string   `toml:"fee_denom"`
	FeeAmount        string   `toml:"fee_amount"`
	GasLimit         uint64   `toml:"gas_limit"`
	StorePrefix      string   `toml:"store_prefix"`
	MaxTxSize        uint64   `toml:"max_tx_size"`
	WasmCodeId       string   `toml:"wasm_code_id"`
	Keybase          KeyEntry `toml:"keybase"`
	ChannelWhitelist []string `toml:"channel_whitelist"`
}

const (
	HyperspaceDefaultContainerImage   = "hyperspace"
	HyperspaceDefaultContainerVersion = "local"
)

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
	purposeKey, _ := masterKey.NewChildKey(0x8000002C)                        // 44'
	coinTypeKey, _ := purposeKey.NewChildKey(0x80000000 + uint32(coinType64)) // 118'
	accountKey, _ := coinTypeKey.NewChildKey(0x80000000)                      // 0'
	changeKey, _ := accountKey.NewChildKey(0)                                 // 0
	indexKey, _ := changeKey.NewChildKey(0)                                   // 0

	return KeyEntry{
		PublicKey:  indexKey.PublicKey().B58Serialize(), // i.e. "xpub6GNKSnPmR5zN3Ef3EqYkSJTZzjzGecb1n1SqJRUNnoFPsyxviG7QyoVzjEjP3gfqRu7AvRrEZMfXJazz8pZgmYP6yvvdRqC2pWmWpeQTMBP"
		PrivateKey: indexKey.B58Serialize(),             // i.e. "xprvA3Ny3GrsaiS4pkaa8p1k5AWqSi9nF9sAQnXEW34mETiR1BdnAioAS1BWsx3uAXKT3NbY6cpY2mQL6N7R8se1GVHqNkpjwc7rv5VRaQ9x8EB"
		Account:    bech32Addr,                          // i.e. "cosmos1pyxjp07wc207l7jecyr3wcmq9cr54tqwhcwugm"
		Address:    address.Bytes(),                     // i.e. [9, 13, 32, 191, 206, 194, 159, 239, 250, 89, 193, 7, 23, 99, 96, 46, 7, 74, 172, 14]
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
			RelayChainRPCURL: strings.Replace(strings.Replace(relayRpcAddr, "http", "ws", 1), "9933", "27451", 1),
			CommitmentPrefix: "0x6962632f",
			PrivateKey:       "//Alice",
			SS58Version:      polkadot.Ss58Format,
			KeyType:          "sr25519",
			FinalityProtocol: "Grandpa",
		}
	} else if chainType == "cosmos" {
		wsUrl := strings.Replace(rpcAddr, "http", "ws", 1) + "/websocket"
		return HyperspaceRelayerCosmosChainConfig{
			Type:          chainType,
			Name:          chainConfig.Name,
			ChainID:       chainConfig.ChainID,
			AccountPrefix: chainConfig.Bech32Prefix,
			FeeDenom:      "stake",
			FeeAmount:     "4000",
			GasLimit:      10_000_000,
			GRPCUrl:       "http://" + grpcAddr,
			RPCUrl:        rpcAddr,
			StorePrefix:   "ibc",
			MaxTxSize:     200000,
			WebsocketUrl:  wsUrl,
		}
	} else {
		panic(fmt.Sprintf("unsupported chain type %s", chainType))
	}
}
