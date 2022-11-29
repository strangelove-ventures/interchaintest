package ibctest

import (
	"context"
	"testing"

	ibctest "github.com/strangelove-ventures/ibctest/v5"
	"github.com/strangelove-ventures/ibctest/v5/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/internal/dockerutil"
	"github.com/strangelove-ventures/ibctest/v5/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestStrangeIndex(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:      "strange",
			ChainName: "strange",
			ChainConfig: ibc.ChainConfig{
				Type:         "cosmos_external",
				Name:         "strange",
				Address:      "http://strange.goc.strange.love:26657",
				ChainID:      "strange-1",
				Denom:        "ustrange",
				Bech32Prefix: "cosmos",

				Images: []ibc.DockerImage{{
					Repository: "ghcr.io/strangelove-ventures/heighliner/strange",
					Version:    "v0.1.0",
					UidGid:     dockerutil.GetHeighlinerUserString(),
				}},

				Bin:            "stranged",
				TrustingPeriod: "504h",
				GasPrices:      "0.0025ustrange",
				GasAdjustment:  1.1,
			},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*cosmos.CosmosExternalChain)

	ic := ibctest.NewInterchain().
		AddChain(chain)

	ctx := context.Background()

	client, network := ibctest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, nil, ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	require.NoError(t, test.WaitForBlocks(ctx, 2000, chain))

}
