package ibctest

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestInterchainAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	home := ibctest.TempDir(t)
	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Get both chains
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "icad", Version: "master"},
		{Name: "icad", Version: "master"},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain1, chain2 := chains[0], chains[1]

	r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network,
	)

	const pathName = "test-path"

	ic := ibctest.NewInterchain().
		AddChain(chain1).
		AddChain(chain2).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  chain1,
			Chain2:  chain2,
			Relayer: r,
			Path:    pathName,
		})

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		HomeDir:   home,
		Client:    client,
		NetworkID: network,

		SkipPathCreation: true,
	}))

	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain1)
	chain1User := users[0]

	// Generate path
	err = r.GeneratePath(ctx, eRep, chain1.Config().ChainID, chain2.Config().ChainID, pathName)
	require.NoError(t, err)

	// Create new clients
	err = r.CreateClients(ctx, eRep, pathName)
	require.NoError(t, err)

	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Create a new connection
	err = r.CreateConnections(ctx, eRep, pathName)
	require.NoError(t, err)

	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Query for the newly created connection
	connections, err := r.GetConnections(ctx, eRep, chain1.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 1, len(connections))

	// Register a new interchain account
	_, err = chain1.RegisterInterchainAccount(ctx, chain1User.Bech32Address(chain1.Config().Bech32Prefix), connections[0].ID)
	require.NoError(t, err)

	// Start the relayer on both paths
	err = r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
			for _, c := range chains {
				if err = c.Cleanup(ctx); err != nil {
					t.Logf("an error occured while stopping chain %s: %s", c.Config().ChainID, err)
				}
			}
		},
	)

	// Wait for relayer to start up and finish channel handshake
	err = test.WaitForBlocks(ctx, 15, chain1, chain2)
	require.NoError(t, err)

	// Query for the new interchain account
	icaAddress, err := chain1.QueryInterchainAccount(ctx, connections[0].ID, chain1User.Bech32Address(chain1.Config().Bech32Prefix))
	require.NoError(t, err)
	require.NotEqual(t, "", icaAddress)
}
