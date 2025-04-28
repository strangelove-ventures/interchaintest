package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-playground/validator"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Chains  []Chain    `json:"chains" yaml:"chains"`
	Relayer *Relayer   `json:"relayer" yaml:"relayer"`
	Server  RestServer `json:"server" yaml:"server"`
}

type AppStartConfig struct {
	Address string
	Port    uint16
	Cfg     *Config
	AuthKey string // optional password for API interaction
}

type RestServer struct {
	Host string `json:"host" yaml:"host"`
	Port string `json:"port" yaml:"port"`
}

type Relayer struct {
	Type         ibc.RelayerImplementation `json:"type" yaml:"type"`
	DockerImage  *ibc.DockerImage          `json:"docker_image" yaml:"docker_image"`
	StartupFlags *[]string                 `json:"startup_flags" yaml:"startup_flags"`
}

type IBCChannel struct {
	ChainID string             `json:"chain_id" yaml:"chain_id"`
	Channel *ibc.ChannelOutput `json:"channel" yaml:"channel"`
}

// ConfigFileOverrides overrides app toml configuration files.
type ConfigFileOverrides struct {
	File  string        `json:"file,omitempty" yaml:"file,omitempty"`
	Paths testutil.Toml `json:"paths" yaml:"paths"`
}

type Chain struct {
	Name                string                `json:"name" yaml:"name" validate:"min=1"`
	ChainID             string                `json:"chain_id" yaml:"chain_id" validate:"min=3"`
	DockerImage         ibc.DockerImage       `json:"docker_image" yaml:"docker_image" validate:"url"`
	GasPrices           string                `json:"gas_prices" yaml:"gas_prices"`
	GasAdjustment       float64               `json:"gas_adjustment" yaml:"gas_adjustment"`
	Genesis             Genesis               `json:"genesis,omitempty" yaml:"genesis,omitempty"`
	ConfigFileOverrides []ConfigFileOverrides `json:"config_file_overrides,omitempty" yaml:"config_file_overrides,omitempty"`
	IBCPaths            []string              `json:"ibc_paths,omitempty" yaml:"ibc_paths,omitempty"`
	NumberVals          int                   `json:"number_vals" yaml:"number_vals" validate:"gte=1"`
	NumberNode          int                   `json:"number_node" yaml:"number_node"`
	ChainType           string                `json:"chain_type" yaml:"chain_type" validate:"min=1"`
	CoinType            int                   `json:"coin_type" yaml:"coin_type" validate:"gt=0"`
	Binary              string                `json:"binary" yaml:"binary" validate:"min=1"`
	Bech32Prefix        string                `json:"bech32_prefix" yaml:"bech32_prefix" validate:"min=1"`
	Denom               string                `json:"denom" yaml:"denom" validate:"min=1"`
	TrustingPeriod      string                `json:"trusting_period" yaml:"trusting_period"`
	Debugging           bool                  `json:"debugging" yaml:"debugging"`
	BlockTime           string                `json:"block_time,omitempty" yaml:"block_time,omitempty"`
	HostPortOverride    map[string]string     `json:"host_port_override,omitempty" yaml:"host_port_override,omitempty"`
	ICSConsumerLink     string                `json:"ics_consumer_link,omitempty" yaml:"ics_consumer_link,omitempty"`
	ICSVersionOverride  ibc.ICSConfig         `json:"ics_version_override,omitempty" yaml:"ics_version_override,omitempty"`
	AdditionalStartArgs []string              `json:"additional_start_args,omitempty" yaml:"additional_start_args,omitempty"`
	Env                 []string              `json:"env,omitempty" yaml:"env,omitempty"`
}

func (chain *Chain) Validate() error {
	validate := validator.New()
	return validate.Struct(chain)
}

func (chain *Chain) SetChainDefaults() {
	if chain.ChainType == "" {
		chain.ChainType = "cosmos"
	}

	if chain.CoinType == 0 {
		chain.CoinType = 118
	}

	if chain.DockerImage.UIDGID == "" {
		chain.DockerImage.UIDGID = "1025:1025"
	}

	if chain.NumberVals == 0 {
		chain.NumberVals = 1
	}

	if chain.TrustingPeriod == "" {
		chain.TrustingPeriod = "112h"
	}

	if chain.BlockTime == "" {
		chain.BlockTime = "2s"
	}

	if chain.IBCPaths == nil {
		chain.IBCPaths = []string{}
	}

	// Genesis
	if chain.Genesis.StartupCommands == nil {
		chain.Genesis.StartupCommands = []string{}
	}
	if chain.Genesis.Accounts == nil {
		chain.Genesis.Accounts = []GenesisAccount{}
	}
	if chain.Genesis.Modify == nil {
		chain.Genesis.Modify = []cosmos.GenesisKV{}
	}

	if chain.Binary == "" {
		panic("'binary' is required in your config for " + chain.ChainID)
	}
	if chain.Denom == "" {
		panic("'denom' is required in your config for " + chain.ChainID)
	}
	if chain.Bech32Prefix == "" {
		panic("'bech32_prefix' is required in your config for " + chain.ChainID)
	}
}

// ChainsConfig is the chain configuration for the file.
type ChainsConfig struct {
	Chains []Chain `json:"chains" yaml:"chains"`
}

func NewChainsConfig(chains ...*Chain) ChainsConfig {
	updatedChains := make([]Chain, len(chains))
	for i, chain := range chains {
		updatedChains[i] = *chain
	}

	return ChainsConfig{
		Chains: updatedChains,
	}
}

// SaveJSON saves the chains config to a file.
func (cfg ChainsConfig) SaveJSON(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	bz, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal chains config: %w", err)
	}

	return os.WriteFile(file, bz, 0o777)
}

// SaveYAML saves the chains config to a file.
func (cfg ChainsConfig) SaveYAML(file string) error {
	bz, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal chains config: %w", err)
	}

	return os.WriteFile(file, bz, 0o777)
}

// MainLogs is the main runtime log file of the application.
type MainLogs struct {
	StartTime uint64       `json:"start_time" yaml:"start_time"`
	Chains    []LogOutput  `json:"chains" yaml:"chains"`
	Channels  []IBCChannel `json:"ibc_channels" yaml:"ibc_channels"`
}

// LogOutput is a subsection of the chains information for the parent logs.
type LogOutput struct {
	ChainID     string   `json:"chain_id" yaml:"chain_id"`
	ChainName   string   `json:"chain_name" yaml:"chain_name"`
	RPCAddress  string   `json:"rpc_address" yaml:"rpc_address"`
	RESTAddress string   `json:"rest_address" yaml:"rest_address"`
	GRPCAddress string   `json:"grpc_address" yaml:"grpc_address"`
	P2PAddress  string   `json:"p2p_address" yaml:"p2p_address"`
	IBCPath     []string `json:"ibc_paths" yaml:"ibc_paths"`
}
