package handlers

import (
	"encoding/json"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

type IbcChainConfigAlias struct {
	Type           string  `json:"type"`
	Name           string  `json:"name"`
	ChainID        string  `json:"chain_id"`
	Bin            string  `json:"bin"`
	Bech32Prefix   string  `json:"bech32_prefix"`
	Denom          string  `json:"denom"`
	CoinType       string  `json:"coin_type"`
	GasPrices      string  `json:"gas_prices"`
	GasAdjustment  float64 `json:"gas_adjustment"`
	TrustingPeriod string  `json:"trusting_period"`
}

func (c *IbcChainConfigAlias) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

func MarshalIBCChainConfig(cfg ibc.ChainConfig) ([]byte, error) {
	jsonRes, err := json.MarshalIndent(IbcChainConfigAlias{
		Type:           cfg.Type,
		Name:           cfg.Name,
		ChainID:        cfg.ChainID,
		Bin:            cfg.Bin,
		Bech32Prefix:   cfg.Bech32Prefix,
		Denom:          cfg.Denom,
		CoinType:       cfg.CoinType,
		GasPrices:      cfg.GasPrices,
		GasAdjustment:  cfg.GasAdjustment,
		TrustingPeriod: cfg.TrustingPeriod,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return jsonRes, nil
}
