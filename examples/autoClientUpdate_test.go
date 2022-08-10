package ibctest_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestAutoUpdateClient(t *testing.T) {
	ctx := context.Background()

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", ChainName: "gaia-1", Version: "v7.0.2", ChainConfig: ibc.ChainConfig{ChainID: "gaia-1"}},
		{Name: "gaia", ChainName: "gaia-2", Version: "v7.0.2", ChainConfig: ibc.ChainConfig{ChainID: "gaia-2"}},
	})

	client, network := ibctest.DockerSetup(t)
	r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia1, gaia2 := chains[0], chains[1]

	const ibcPath = "gaia1-gaia2"

	ic := ibctest.NewInterchain().
		AddChain(gaia1).
		AddChain(gaia2).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia1,
			Chain2:  gaia2,
			Relayer: r,
			Path:    ibcPath,
		})

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		HomeDir:   ibctest.TempDir(t),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,

		CreateClientOpts: ibc.CreateClientOptions{
			TrustingPeriod: "4m",
		}}))

	conns, err := r.GetConnections(ctx, eRep, "gaia-1")
	require.NoError(t, err)
	gaia1ClientID := conns[0].ClientID

	conns, err = r.GetConnections(ctx, eRep, "gaia-2")
	require.NoError(t, err)
	gaia2ClientID := conns[0].ClientID

	// log queriered client trusing period to verify tp (chain1)
	command := []string{
		"gaiad", "query", "ibc", "client", "state", gaia1ClientID,
		"--node", gaia1.GetHostRPCAddress(),
	}
	out, serr, err := gaia1.Exec(ctx, command, []string{})
	if err != nil {
		//NEED TO FIGURE OUT HOW LOGGING WORKS
		t.Log(out)
		t.Log(serr)
	}
	t.Log("OUT GAIA1: ", out)
	t.Log("SERR GAIA2: ", serr)

	// log queriered client trusing period to verify tp (chain2)
	command = []string{
		"gaiad", "query", "ibc", "client", "state", gaia2ClientID,
		"--node", gaia2.GetHostRPCAddress(),
	}
	out, serr, err = gaia2.Exec(ctx, command, []string{})
	if err != nil {
		//NEED TO FIGURE OUT HOW LOGGING WORKS
		fmt.Println(out)
		fmt.Println(serr)
	}

	// require client state active

	err = r.StartRelayer(ctx, eRep, ibcPath)
	require.NoError(t, err)

	// time.Sleep(5 * time.Minute)

	// require client state active
	// Try sending an IBC tx w/ relelvant client

}
