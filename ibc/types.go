package ibc

import ibcexported "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"

type ChainConfig struct {
	Type           string
	Name           string
	ChainID        string
	Images         []DockerImage
	Bin            string
	Bech32Prefix   string
	Denom          string
	GasPrices      string
	GasAdjustment  float64
	TrustingPeriod string
	NoHostMount    bool
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

	if other.GasPrices != "" {
		c.GasPrices = other.GasPrices
	}

	// Skip GasAdjustment, so that 0.0 can be distinguished.

	if other.TrustingPeriod != "" {
		c.TrustingPeriod = other.TrustingPeriod
	}

	// Skip NoHostMount so that false can be distinguished.

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
	Repository string
	Version    string
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

type ContractStateModels struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DumpContractStateResponse struct {
	Models []ContractStateModels `json:"models"`
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

type RelayerWallet struct {
	Mnemonic string `json:"mnemonic"`
	Address  string `json:"address"`
}

type RelayerImplementation int64

const (
	CosmosRly RelayerImplementation = iota
	Hermes
)
