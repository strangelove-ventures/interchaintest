package interchain

import (
	"context"
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	types "github.com/strangelove-ventures/localinterchain/interchain/types"
)

func VerifyIBCPaths(ibcpaths map[string][]int) error {
	for k, v := range ibcpaths {
		if len(v) == 1 {
			return fmt.Errorf("ibc path '%s' has only 1 chain", k)
		}
		if len(v) > 2 {
			return fmt.Errorf("ibc path '%s' has more than 2 chains", k)
		}
	}
	return nil
}

// TODO: Allow for a single chain to IBC between multiple chains
func LinkIBCPaths(ibcpaths map[string][]int, chains []ibc.Chain, ic *interchaintest.Interchain, r ibc.Relayer) {
	for path, c := range ibcpaths {
		chain1 := chains[c[0]]
		chain2 := chains[c[1]]

		interLink := interchaintest.InterchainLink{
			Chain1:  chain1,
			Chain2:  chain2,
			Path:    path,
			Relayer: r,
		}

		ic = ic.AddLink(interLink)
	}
}

// TODO: Get all channels a chain is connected too. Map it to the said chain_id. Then output to Logs.
func GetChannelConnections(ctx context.Context, ibcpaths map[string][]int, chains []ibc.Chain, ic *interchaintest.Interchain, r ibc.Relayer, eRep ibc.RelayerExecReporter) []types.IBCChannel {
	if len(ibcpaths) == 0 {
		return []types.IBCChannel{}
	}

	channels := []types.IBCChannel{}

	for _, c := range ibcpaths {
		chain1 := chains[c[0]]
		chain2 := chains[c[1]]

		channel1, err := ibc.GetTransferChannel(ctx, r, eRep, chain1.Config().ChainID, chain2.Config().ChainID)
		if err != nil {
			panic(err)
		}

		channels = append(channels, types.IBCChannel{
			ChainID: chain1.Config().ChainID,
			Channel: channel1,
		})

		// this a duplicate?
		channel2, err := ibc.GetTransferChannel(ctx, r, eRep, chain2.Config().ChainID, chain1.Config().ChainID)
		if err != nil {
			panic(err)
		}
		channels = append(channels, types.IBCChannel{
			ChainID: chain2.Config().ChainID,
			Channel: channel2,
		})
	}

	return channels
}
