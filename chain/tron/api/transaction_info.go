package api

type TransactionInfo struct {
	Fee     uint64 `json:"fee"`
	Receipt struct {
		EnergyPenaltyTotal int64 `json:"energy_penalty_total"`
		EnergyUsageTotal   int64 `json:"energy_usage_total"`
	} `json:"receipt"`
}

func (i *TransactionInfo) GetBaseEnergy() int64 {
	energy := i.Receipt.EnergyUsageTotal - i.Receipt.EnergyPenaltyTotal
	if energy < 0 {
		return 0
	}
	return energy
}
