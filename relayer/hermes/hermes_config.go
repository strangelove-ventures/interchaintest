package hermes

import (
	"fmt"
	"strconv"
	"strings"
)

// NewConfig returns a hermes Config with an entry for each of the provided ChainConfigs.
// The defaults were adapted from the sample config file found here: https://github.com/informalsystems/hermes/blob/master/config.toml
func NewConfig(chainConfigs ...ChainConfig) Config {
	var chains []Chain
	for _, hermesCfg := range chainConfigs {
		chainCfg := hermesCfg.cfg

		gasPricesStr, err := strconv.ParseFloat(strings.ReplaceAll(chainCfg.GasPrices, chainCfg.Denom, ""), 32)
		if err != nil {
			panic(err)
		}

		chains = append(chains, Chain{
			ID:            chainCfg.ChainID,
			RPCAddr:       hermesCfg.rpcAddr,
			GrpcAddr:      fmt.Sprintf("http://%s", hermesCfg.grpcAddr),
			WebsocketAddr: strings.ReplaceAll(fmt.Sprintf("%s/websocket", hermesCfg.rpcAddr), "http", "ws"),
			RPCTimeout:    "10s",
			AccountPrefix: chainCfg.Bech32Prefix,
			KeyName:       hermesCfg.keyName,
			AddressType: AddressType{
				Derivation: "cosmos",
			},
			StorePrefix: "ibc",
			DefaultGas:  100000,
			MaxGas:      400000,
			GasPrice: GasPrice{
				Price: gasPricesStr,
				Denom: chainCfg.Denom,
			},
			GasMultiplier:  chainCfg.GasAdjustment,
			MaxMsgNum:      30,
			MaxTxSize:      2097152,
			ClockDrift:     "5s",
			MaxBlockTime:   "30s",
			TrustingPeriod: "14days",
			TrustThreshold: TrustThreshold{
				Numerator:   "1",
				Denominator: "3",
			},
			MemoPrefix: "hermes",
		},
		)
	}

	return Config{
		Global: Global{
			LogLevel: "info",
		},
		Mode: Mode{
			Clients: Clients{
				Enabled:      true,
				Refresh:      true,
				Misbehaviour: true,
			},
			Connections: Connections{
				Enabled: true,
			},
			Channels: Channels{
				Enabled: true,
			},
			Packets: Packets{
				Enabled:        true,
				ClearInterval:  0,
				ClearOnStart:   true,
				TxConfirmation: false,
			},
		},
		Rest: Rest{
			Enabled: false,
		},
		Telemetry: Telemetry{
			Enabled: false,
		},
		Chains: chains,
	}
}

type Config struct {
	Global    Global    `toml:"global"`
	Mode      Mode      `toml:"mode"`
	Rest      Rest      `toml:"rest"`
	Telemetry Telemetry `toml:"telemetry"`
	Chains    []Chain   `toml:"chains"`
}

type Global struct {
	LogLevel string `toml:"log_level"`
}

type Clients struct {
	Enabled      bool `toml:"enabled"`
	Refresh      bool `toml:"refresh"`
	Misbehaviour bool `toml:"misbehaviour"`
}

type Connections struct {
	Enabled bool `toml:"enabled"`
}

type Channels struct {
	Enabled bool `toml:"enabled"`
}

type Packets struct {
	Enabled        bool `toml:"enabled"`
	ClearInterval  int  `toml:"clear_interval"`
	ClearOnStart   bool `toml:"clear_on_start"`
	TxConfirmation bool `toml:"tx_confirmation"`
}

type Mode struct {
	Clients     Clients     `toml:"clients"`
	Connections Connections `toml:"connections"`
	Channels    Channels    `toml:"channels"`
	Packets     Packets     `toml:"packets"`
}

type Rest struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
}

type Telemetry struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
}

type AddressType struct {
	Derivation string `toml:"derivation"`
}

type GasPrice struct {
	Price float64 `toml:"price"`
	Denom string  `toml:"denom"`
}

type TrustThreshold struct {
	Numerator   string `toml:"numerator"`
	Denominator string `toml:"denominator"`
}

type Chain struct {
	ID             string         `toml:"id"`
	RPCAddr        string         `toml:"rpc_addr"`
	GrpcAddr       string         `toml:"grpc_addr"`
	WebsocketAddr  string         `toml:"websocket_addr"`
	RPCTimeout     string         `toml:"rpc_timeout"`
	AccountPrefix  string         `toml:"account_prefix"`
	KeyName        string         `toml:"key_name"`
	AddressType    AddressType    `toml:"address_type"`
	StorePrefix    string         `toml:"store_prefix"`
	DefaultGas     int            `toml:"default_gas"`
	MaxGas         int            `toml:"max_gas"`
	GasPrice       GasPrice       `toml:"gas_price"`
	GasMultiplier  float64        `toml:"gas_multiplier"`
	MaxMsgNum      int            `toml:"max_msg_num"`
	MaxTxSize      int            `toml:"max_tx_size"`
	ClockDrift     string         `toml:"clock_drift"`
	MaxBlockTime   string         `toml:"max_block_time"`
	TrustingPeriod string         `toml:"trusting_period"`
	TrustThreshold TrustThreshold `toml:"trust_threshold"`
	MemoPrefix     string         `toml:"memo_prefix,omitempty"`
}
