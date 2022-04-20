package ibc

type ChainConfig struct {
	Type           string
	Name           string
	ChainID        string
	Repository     string
	Version        string
	Bin            string
	Bech32Prefix   string
	Denom          string
	GasPrices      string
	GasAdjustment  float64
	TrustingPeriod string
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

type RelayerWallet struct {
	Mnemonic string `json:"mnemonic"`
	Address  string `json:"address"`
}

type RelayerImplementation int64

const (
	CosmosRly RelayerImplementation = iota
	Hermes
)
