package cosmos

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func SetSDKConfig(bech32Prefix string) *sdk.Config {
	var (
		bech32MainPrefix     = bech32Prefix
		bech32PrefixAccAddr  = bech32MainPrefix
		bech32PrefixAccPub   = bech32MainPrefix + sdk.PrefixPublic
		bech32PrefixValAddr  = bech32MainPrefix + sdk.PrefixValidator + sdk.PrefixOperator
		bech32PrefixValPub   = bech32MainPrefix + sdk.PrefixValidator + sdk.PrefixOperator + sdk.PrefixPublic
		bech32PrefixConsAddr = bech32MainPrefix + sdk.PrefixValidator + sdk.PrefixConsensus
		bech32PrefixConsPub  = bech32MainPrefix + sdk.PrefixValidator + sdk.PrefixConsensus + sdk.PrefixPublic
	)

	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(bech32PrefixAccAddr, bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(bech32PrefixValAddr, bech32PrefixValPub)
	cfg.SetBech32PrefixForConsensusNode(bech32PrefixConsAddr, bech32PrefixConsPub)
	return cfg
}
