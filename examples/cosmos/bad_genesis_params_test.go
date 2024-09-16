package cosmos_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	badGenesis = []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.gov.params.voting_period", "bad"),
	}
)

func TestBadInputParams(t *testing.T) {
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      "juno",
			ChainName: "juno",
			Version:   "v19.0.0-alpha.3",
			ChainConfig: ibc.ChainConfig{
				Denom:         "ujuno",
				Bech32Prefix:  "juno",
				CoinType:      "118",
				ModifyGenesis: cosmos.ModifyGenesis(badGenesis),
				GasPrices:     "0ujuno",
			},
			NumValidators: &numValsOne,
			NumFullNodes:  &numFullNodesZero,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	err = ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	})

	// failed to start chains: failed to start chain juno: PANIC: container juno-1-val-0-TestBadInputParams failed to start: panic: bad Duration: time: invalid duration "bad"
	require.Error(t, err)
	require.ErrorContains(t, err, "bad Duration")
	t.Log("err", err)

	t.Cleanup(func() {
		_ = ic.Close()
	})
}
