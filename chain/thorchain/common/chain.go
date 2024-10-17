package common

import (
	"errors"
	"strings"
)

const (
	EmptyChain = Chain("")
	BNBChain   = Chain("BNB")
	BSCChain   = Chain("BSC")
	ETHChain   = Chain("ETH")
	BTCChain   = Chain("BTC")
	LTCChain   = Chain("LTC")
	BCHChain   = Chain("BCH")
	DOGEChain  = Chain("DOGE")
	THORChain  = Chain("THOR")
	TERRAChain = Chain("TERRA")
	GAIAChain  = Chain("GAIA")
	AVAXChain  = Chain("AVAX")
)

type Chain string

// Chains represent a slice of Chain.
type Chains []Chain

// Valid validates chain format, should consist only of uppercase letters.
func (c Chain) Valid() error {
	if len(c) < 3 {
		return errors.New("chain id len is less than 3")
	}
	if len(c) > 10 {
		return errors.New("chain id len is more than 10")
	}
	for _, ch := range string(c) {
		if ch < 'A' || ch > 'Z' {
			return errors.New("chain id can consist only of uppercase letters")
		}
	}
	return nil
}

// NewChain create a new Chain and default the siging_algo to Secp256k1.
func NewChain(chainID string) (Chain, error) {
	chain := Chain(strings.ToUpper(chainID))
	if err := chain.Valid(); err != nil {
		return chain, err
	}
	return chain, nil
}

// String implement fmt.Stringer.
func (c Chain) String() string {
	// convert it to upper case again just in case someone created a ticker via Chain("rune")
	return strings.ToUpper(string(c))
}

// GetGasAsset chain's base asset.
func (c Chain) GetGasAsset() Asset {
	switch c {
	case THORChain:
		return RuneNative
	case BNBChain:
		return BNBAsset
	case BSCChain:
		return BNBBEP20Asset
	case BTCChain:
		return BTCAsset
	case LTCChain:
		return LTCAsset
	case BCHChain:
		return BCHAsset
	case DOGEChain:
		return DOGEAsset
	case ETHChain:
		return ETHAsset
	case TERRAChain:
		return LUNAAsset
	case AVAXChain:
		return AVAXAsset
	case GAIAChain:
		return ATOMAsset
	case EmptyChain:
		return EmptyAsset
	default:
		return EmptyAsset
	}
}
