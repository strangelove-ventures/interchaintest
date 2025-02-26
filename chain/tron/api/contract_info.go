package api

type ContractInfo struct {
	SmartContract struct {
		Name            string `json:"name"`
		ContractAddress string `json:"contract_address"`
	} `json:"smart_contract"`
	ContractState struct {
		EnergyFactor int64 `json:"energy_factor"`
		EnergyUsage  int64 `json:"energy_usage"`
		UpdateCycle  int64 `json:"update_cycle"`
	} `json:"contract_state"`
}
