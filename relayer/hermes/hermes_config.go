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

		var chainType string
		var accountPrefix string
		var trustingPeriod string
		switch chainCfg.Type {
		case "namada":
			chainType = "Namada"
			accountPrefix = ""
			trustingPeriod = "1000s"
		default:
			chainType = "CosmosSdk"
			accountPrefix = chainCfg.Bech32Prefix
			trustingPeriod = "14days"
		}
		chains = append(chains, Chain{
			ID:               chainCfg.ChainID,
			Type:             chainType,
			RPCAddr:          hermesCfg.rpcAddr,
			CCVConsumerChain: false,
			GrpcAddr:         fmt.Sprintf("http://%s", hermesCfg.grpcAddr),
			EventSource: EventSource{
				Mode:       "push",
				URL:        strings.ReplaceAll(fmt.Sprintf("%s/websocket", hermesCfg.rpcAddr), "http", "ws"),
				BatchDelay: "200ms",
			},
			RPCTimeout:    "10s",
			AccountPrefix: accountPrefix,
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
			TrustingPeriod: trustingPeriod,
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
		TracingServer: TracingServer{
			Enabled: false,
		},
		Chains: chains,
	}
}

type Config struct {
	Global        Global        `toml:"global"`
	Mode          Mode          `toml:"mode"`
	Rest          Rest          `toml:"rest"`
	Telemetry     Telemetry     `toml:"telemetry"`
	TracingServer TracingServer `toml:"tracing_server"`
	Chains        []Chain       `toml:"chains"`
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
	Enabled                       bool `toml:"enabled"`
	ClearInterval                 int  `toml:"clear_interval"`
	ClearOnStart                  bool `toml:"clear_on_start"`
	TxConfirmation                bool `toml:"tx_confirmation"`
	AutoRegisterCounterpartyPayee bool `toml:"auto_register_counterparty_payee"`
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

type TracingServer struct {
	Enabled bool `toml:"enabled"`
	Port    int  `toml:"port"`
}

type EventSource struct {
	Mode       string `toml:"mode"`
	URL        string `toml:"url"`
	BatchDelay string `toml:"batch_delay"`
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
	ID               string         `toml:"id"`
	Type             string         `toml:"type"`
	RPCAddr          string         `toml:"rpc_addr"`
	GrpcAddr         string         `toml:"grpc_addr"`
	EventSource      EventSource    `toml:"event_source"`
	CCVConsumerChain bool           `toml:"ccv_consumer_chain"`
	RPCTimeout       string         `toml:"rpc_timeout"`
	AccountPrefix    string         `toml:"account_prefix"`
	KeyName          string         `toml:"key_name"`
	AddressType      AddressType    `toml:"address_type"`
	StorePrefix      string         `toml:"store_prefix"`
	DefaultGas       int            `toml:"default_gas"`
	MaxGas           int            `toml:"max_gas"`
	GasPrice         GasPrice       `toml:"gas_price"`
	GasMultiplier    float64        `toml:"gas_multiplier"`
	MaxMsgNum        int            `toml:"max_msg_num"`
	MaxTxSize        int            `toml:"max_tx_size"`
	ClockDrift       string         `toml:"clock_drift"`
	MaxBlockTime     string         `toml:"max_block_time"`
	TrustingPeriod   string         `toml:"trusting_period"`
	TrustThreshold   TrustThreshold `toml:"trust_threshold"`
	MemoPrefix       string         `toml:"memo_prefix,omitempty"`
}
