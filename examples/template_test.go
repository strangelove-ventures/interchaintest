package ibctest_test

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestTemplate(t *testing.T) {
	ctx := context.Background()

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", ChainName: "gaia-1", Version: "v7.0.2"},
		{Name: "osmosis", ChainName: "gaia-2", Version: "v7.0.2"},
	})

	client, network := ibctest.DockerSetup(t)
	r1 := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia, osmosis := chains[0], chains[1]

	// CREATE USERS

	const ibcPath = "gaia-osmo-demo"

	ic := ibctest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r1, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r1,
			Path:    ibcPath,
		})

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false},
	),
	)

	// START RELAYER

	// SEND IBC TX

	// CONFIRM

}
