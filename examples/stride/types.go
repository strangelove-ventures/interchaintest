package stride_test

type HostZoneAccount struct {
	Address string `json:"address"`
	// Delegations [] `json:"delegations"`
	Target string `json:"target"`
}

type HostZoneValidator struct {
	Address              string `json:"address"`
	CommissionRate       string `json:"commissionRate"`
	DelegationAmt        string `json:"delegationAmt"`
	InternalExchangeRate string `json:"internalExchangeRate"`
	Name                 string `json:"name"`
	Status               string `json:"status"`
	Weight               string `json:"weight"`
}

type HostZoneWrapper struct {
	HostZone HostZone `json:"HostZone"`
}

type HostZone struct {
	HostDenom             string              `json:"HostDenom"`
	IBCDenom              string              `json:"IBCDenom"`
	LastRedemptionRate    string              `json:"LastRedemptionRate"`
	RedemptionRate        string              `json:"RedemptionRate"`
	Address               string              `json:"address"`
	Bech32prefix          string              `json:"bech32pref ix"`
	ChainID               string              `json:"chainId"`
	ConnectionID          string              `json:"connectionId"`
	DelegationAccount     HostZoneAccount     `json:"delegationAccount"`
	FeeAccount            HostZoneAccount     `json:"feeAccount"`
	RedemptionAccount     HostZoneAccount     `json:"redemptionAccount"`
	WithdrawalAccount     HostZoneAccount     `json:"withdrawalAccount"`
	StakedBal             string              `json:"stakedBal"`
	TransferChannelId     string              `json:"transferChannelId"`
	UnbondingFrequency    string              `json:"unbondingFrequency"`
	Validators            []HostZoneValidator `json:"validators"`
	BlacklistedValidators []HostZoneValidator `json:"blacklistedValidators"`
}

// type DepositRecordsWrapper struct {
// 	DepositRecords []DepositRecord `json:"DepositRecords"`

// }
