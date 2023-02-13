package hermes

import (
	"fmt"
	"strings"

	"github.com/strangelove-ventures/interchaintest/v6/ibc"
)

// https://github.com/informalsystems/hermes/blob/master/config.toml

func NewConfig(keyName, rpcAddr, grpcAddr string, chainConfigs ...ibc.ChainConfig) Config {
	var chains []Chain
	for _, chainCfg := range chainConfigs {
		chains = append(chains, Chain{
			ID:            chainCfg.ChainID,
			RPCAddr:       rpcAddr,
			GrpcAddr:      fmt.Sprintf("http://%s", grpcAddr),
			WebsocketAddr: strings.ReplaceAll(fmt.Sprintf("%s/websocket", rpcAddr), "http", "ws"),
			RPCTimeout:    "10s",
			AccountPrefix: chainCfg.Bech32Prefix,
			KeyName:       keyName,
			AddressType: AddressType{
				Derivation: "cosmos",
			},
			StorePrefix: "ibc",
			DefaultGas:  100000,
			MaxGas:      400000,
			GasPrice: GasPrice{
				Price: 0.001,
				Denom: "stake",
			},
			GasMultiplier:  1.1,
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
				ClearOnStart:   false,
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

//type Config struct {
//	Global    Global    `toml:"global"`
//	Mode      Mode      `toml:"mode"`
//	Rest      Rest      `toml:"rest"`
//	Telemetry Telemetry `toml:"telemetry"`
//	Chains    []Chain   `toml:"chains"`
//}
//
//type Global struct {
//	LogLevel string `toml:"log_level"`
//}
//
//type Clients struct {
//	Enabled      bool `toml:"enabled"`
//	Refresh      bool `toml:"refresh"`
//	Misbehaviour bool `toml:"misbehaviour"`
//}
//
//type Connections struct {
//	Enabled bool `toml:"enabled"`
//}
//
//type Channels struct {
//	Enabled bool `toml:"enabled"`
//}
//
//type Packets struct {
//	Enabled                       bool `toml:"enabled"`
//	ClearInterval                 int  `toml:"clear_interval"`
//	ClearOnStart                  bool `toml:"clear_on_start"`
//	TxConfirmation                bool `toml:"tx_confirmation"`
//	AutoRegisterCounterpartyPayee bool `toml:"auto_register_counterparty_payee"`
//}
//
//type Mode struct {
//	Clients     Clients     `toml:"clients"`
//	Connections Connections `toml:"connections"`
//	Channels    Channels    `toml:"channels"`
//	Packets     Packets     `toml:"packets"`
//}
//
//type Rest struct {
//	Enabled bool   `toml:"enabled"`
//	Host    string `toml:"host"`
//	Port    int    `toml:"port"`
//}
//
//type Telemetry struct {
//	Enabled bool   `toml:"enabled"`
//	Host    string `toml:"host"`
//	Port    int    `toml:"port"`
//}
//
//type AddressType struct {
//	Derivation string `toml:"derivation"`
//}
//
//type GasPrice struct {
//	Price float64 `toml:"price"`
//	Denom string  `toml:"denom"`
//}
//
//type TrustThreshold struct {
//	Numerator   string `toml:"numerator"`
//	Denominator string `toml:"denominator"`
//}
//
//type Chain struct {
//	ID             string         `toml:"id"`
//	RPCAddr        string         `toml:"rpc_addr"`
//	GrpcAddr       string         `toml:"grpc_addr"`
//	WebsocketAddr  string         `toml:"websocket_addr"`
//	RPCTimeout     string         `toml:"rpc_timeout"`
//	AccountPrefix  string         `toml:"account_prefix"`
//	KeyName        string         `toml:"key_name"`
//	AddressType    AddressType    `toml:"address_type"`
//	StorePrefix    string         `toml:"store_prefix"`
//	DefaultGas     int            `toml:"default_gas"`
//	MaxGas         int            `toml:"max_gas"`
//	GasPrice       GasPrice       `toml:"gas_price"`
//	GasMultiplier  float64        `toml:"gas_multiplier"`
//	MaxMsgNum      int            `toml:"max_msg_num"`
//	MaxTxSize      int            `toml:"max_tx_size"`
//	ClockDrift     string         `toml:"clock_drift"`
//	MaxBlockTime   string         `toml:"max_block_time"`
//	TrustingPeriod string         `toml:"trusting_period"`
//	TrustThreshold TrustThreshold `toml:"trust_threshold"`
//	MemoPrefix     string         `toml:"memo_prefix,omitempty"`
//}

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
