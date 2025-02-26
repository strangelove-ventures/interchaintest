package api

type BroadcastResponse struct {
	Result bool   `json:"result"`
	TxId   string `json:"txid"`
}

type EstimateEnergyResponse struct {
	Result struct {
		Result bool `json:"result"`
	} `json:"result"`
	Energy int64 `json:"energy_required"`
}

type ChainParametersResponse struct {
	Parameters []struct {
		Key   string `json:"key"`
		Value int64  `json:"value"`
	} `json:"chainParameter"`
}
