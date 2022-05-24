package ibc

import ibcexported "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"

type ChainConfig struct {
	Type           string
	Name           string
	ChainID        string
	Images         []ChainDockerImage
	Bin            string
	Bech32Prefix   string
	Denom          string
	GasPrices      string
	GasAdjustment  float64
	TrustingPeriod string
	NoHostMount    bool
}

type ChainDockerImage struct {
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
