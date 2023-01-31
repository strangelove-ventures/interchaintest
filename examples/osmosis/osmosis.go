package osmosis

import (
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	balancertypes "github.com/osmosis-labs/osmosis/v12/x/gamm/pool-models/balancer"
	gammtypes "github.com/osmosis-labs/osmosis/v12/x/gamm/types"
	"github.com/strangelove-ventures/interchaintest/v3/chain/cosmos"
)

func OsmosisEncoding() *simappparams.EncodingConfig {
	cfg := cosmos.DefaultEncoding()

	balancertypes.RegisterInterfaces(cfg.InterfaceRegistry)
	gammtypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}
