package ibc

import (
	"reflect"
	"strconv"

	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
)

// ChainConfig defines the chain parameters requires to run an interchaintest testnet for a chain.
type ChainConfig struct {
	// Chain type, e.g. cosmos.
	Type string `yaml:"type"`
	// Chain name, e.g. cosmoshub.
	Name string `yaml:"name"`
	// Chain ID, e.g. cosmoshub-4
	ChainID string `yaml:"chain-id"`
	// Docker images required for running chain nodes.
	Images []DockerImage `yaml:"images"`
	// Binary to execute for the chain node daemon.
	Bin string `yaml:"bin"`
	// Bech32 prefix for chain addresses, e.g. cosmos.
	Bech32Prefix string `yaml:"bech32-prefix"`
	// Denomination of native currency, e.g. uatom.
	Denom string `yaml:"denom"`
	// Coin type
	CoinType string `default:"118" yaml:"coin-type"`
	// Minimum gas prices for sending transactions, in native currency denom.
	GasPrices string `yaml:"gas-prices"`
	// Adjustment multiplier for gas fees.
	GasAdjustment float64 `yaml:"gas-adjustment"`
	// Trusting period of the chain.
	TrustingPeriod string `yaml:"trusting-period"`
	// Do not use docker host mount.
	NoHostMount bool `yaml:"no-host-mount"`
	// When true, will skip validator gentx flow
	SkipGenTx bool
	// When provided, will run before performing gentx and genesis file creation steps for validators.
	PreGenesis func(ChainConfig) error
	// When provided, genesis file contents will be altered before sharing for genesis.
	ModifyGenesis func(ChainConfig, []byte) ([]byte, error)
	// Override config parameters for files at filepath.
	ConfigFileOverrides map[string]any
	// Non-nil will override the encoding config, used for cosmos chains only.
	EncodingConfig *testutil.TestEncodingConfig
	// Required when the chain uses the new sub commands for genesis (https://github.com/cosmos/cosmos-sdk/pull/14149)
	UsingNewGenesisCommand bool `yaml:"using-new-genesis-command"`
}

func (c ChainConfig) Clone() ChainConfig {
	x := c
	images := make([]DockerImage, len(c.Images))
	copy(images, c.Images)
	x.Images = images
	return x
}

func (c ChainConfig) VerifyCoinType() (string, error) {
	// If coin-type is left blank in the ChainConfig,
	// the Cosmos SDK default of 118 is used.
	if c.CoinType == "" {
		typ := reflect.TypeOf(c)
		f, _ := typ.FieldByName("CoinType")
		coinType := f.Tag.Get("default")
		_, err := strconv.ParseUint(coinType, 10, 32)
		if err != nil {
			return "", err
		}
		return coinType, nil
	} else {
		_, err := strconv.ParseUint(c.CoinType, 10, 32)
		if err != nil {
			return "", err
		}
		return c.CoinType, nil
	}
}

func (c ChainConfig) MergeChainSpecConfig(other ChainConfig) ChainConfig {
	// Make several in-place modifications to c,
	// which is a value, not a reference,
	// and return the updated copy.

	if other.Type != "" {
		c.Type = other.Type
	}

	// Skip Name, as that is held in ChainSpec.ChainName.

	if other.ChainID != "" {
		c.ChainID = other.ChainID
	}

	if len(other.Images) > 0 {
		c.Images = append([]DockerImage(nil), other.Images...)
	}

	if other.Bin != "" {
		c.Bin = other.Bin
	}

	if other.Bech32Prefix != "" {
		c.Bech32Prefix = other.Bech32Prefix
	}

	if other.Denom != "" {
		c.Denom = other.Denom
	}

	if other.CoinType != "" {
		c.CoinType = other.CoinType
	}

	if other.GasPrices != "" {
		c.GasPrices = other.GasPrices
	}

	if other.GasAdjustment > 0 && c.GasAdjustment == 0 {
		c.GasAdjustment = other.GasAdjustment
	}

	if other.TrustingPeriod != "" {
		c.TrustingPeriod = other.TrustingPeriod
	}

	// Skip NoHostMount so that false can be distinguished.

	if other.ModifyGenesis != nil {
		c.ModifyGenesis = other.ModifyGenesis
	}

	if other.SkipGenTx {
		c.SkipGenTx = true
	}

	if other.PreGenesis != nil {
		c.PreGenesis = other.PreGenesis
	}

	if other.ConfigFileOverrides != nil {
		c.ConfigFileOverrides = other.ConfigFileOverrides
	}

	if other.EncodingConfig != nil {
		c.EncodingConfig = other.EncodingConfig
	}

	return c
}

// IsFullyConfigured reports whether all required fields have been set on c.
// It is possible for some fields, such as GasAdjustment and NoHostMount,
// to be their respective zero values and for IsFullyConfigured to still report true.
func (c ChainConfig) IsFullyConfigured() bool {
	return c.Type != "" &&
		c.Name != "" &&
		c.ChainID != "" &&
		len(c.Images) > 0 &&
		c.Bin != "" &&
		c.Bech32Prefix != "" &&
		c.Denom != "" &&
		c.GasPrices != "" &&
		c.TrustingPeriod != ""
}

type DockerImage struct {
	Repository string `yaml:"repository"`
	Version    string `yaml:"version"`
	UidGid     string `yaml:"uid-gid"`
}

// Ref returns the reference to use when e.g. creating a container.
func (i DockerImage) Ref() string {
	if i.Version == "" {
		return i.Repository + ":latest"
	}

	return i.Repository + ":" + i.Version
}

type WalletAmount struct {
	Address string
	Denom   string
	Amount  int64
}

type IBCTimeout struct {
	NanoSeconds uint64
	Height      uint64
}

type ChannelCounterparty struct {
	PortID    string `json:"port_id"`
	ChannelID string `json:"channel_id"`
}

type ChannelOutput struct {
	State          string              `json:"state"`
	Ordering       string              `json:"ordering"`
	Counterparty   ChannelCounterparty `json:"counterparty"`
	ConnectionHops []string            `json:"connection_hops"`
	Version        string              `json:"version"`
	PortID         string              `json:"port_id"`
	ChannelID      string              `json:"channel_id"`
}

// ConnectionOutput represents the IBC connection information queried from a chain's state for a particular connection.
type ConnectionOutput struct {
	ID           string                    `json:"id,omitempty" yaml:"id"`
	ClientID     string                    `json:"client_id,omitempty" yaml:"client_id"`
	Versions     []*ibcexported.Version    `json:"versions,omitempty" yaml:"versions"`
	State        string                    `json:"state,omitempty" yaml:"state"`
	Counterparty *ibcexported.Counterparty `json:"counterparty" yaml:"counterparty"`
	DelayPeriod  string                    `json:"delay_period,omitempty" yaml:"delay_period"`
}

type ConnectionOutputs []*ConnectionOutput

type ClientOutput struct {
	ClientID    string      `json:"client_id"`
	ClientState ClientState `json:"client_state"`
}

type ClientState struct {
	ChainID string `json:"chain_id"`
}

type ClientOutputs []*ClientOutput

type Wallet interface {
	KeyName() string
	FormattedAddress() string
	Mnemonic() string
	Address() []byte
}

type RelayerImplementation int64

const (
	CosmosRly RelayerImplementation = iota
	Hermes
	Hyperspace
)

// ChannelFilter provides the means for either creating an allowlist or a denylist of channels on the src chain
// which will be used to narrow down the list of channels a user wants to relay on.
type ChannelFilter struct {
	Rule        string
	ChannelList []string
}
