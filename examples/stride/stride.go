package stride

import (
	icqtypes "github.com/Stride-Labs/stride/x/interchainquery/types"
	stakeibctypes "github.com/Stride-Labs/stride/x/stakeibc/types"
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/strangelove-ventures/ibctest/v3/chain/cosmos"
)

func StrideEncoding() *simappparams.EncodingConfig {
	cfg := cosmos.DefaultEncoding()

	icqtypes.RegisterInterfaces(cfg.InterfaceRegistry)
	stakeibctypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}
