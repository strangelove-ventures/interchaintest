package cosmos_test

import (
	"context"
	"testing"

	ibctest "github.com/strangelove-ventures/interchaintest/v3"
	"github.com/strangelove-ventures/interchaintest/v3/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v3/ibc"
	"github.com/strangelove-ventures/interchaintest/v3/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v3/testutil"
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
			Name:      "noble",
			ChainName: "noble",
			ChainConfig: ibc.ChainConfig{
				Type:         "cosmos_external",
				Name:         "noble",
				Address:      "https://rpc.testnet.noble.strange.love:443",
				ChainID:      "grand-1",
				Denom:        "uusdc",
				Bech32Prefix: "noble",

				Images: []ibc.DockerImage{{
					Repository: "ghcr.io/strangelove-ventures/heighliner/noble",
					Version:    "v0.5.1",
					UidGid:     dockerutil.GetHeighlinerUserString(),
				}},

				Bin:            "nobled",
				TrustingPeriod: "504h",
				GasPrices:      "0.00uusdc",
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
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	require.NoError(t, testutil.WaitForBlocks(ctx, 2000, chain))

}
