package cosmos_test

import (
	"context"
	"testing"
	"time"

	ibctest "github.com/strangelove-ventures/ibctest/v6"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/conformance"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/relayer"
	"github.com/strangelove-ventures/ibctest/v6/testreporter"
	"github.com/strangelove-ventures/ibctest/v6/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestJunoUpgradeIBC(t *testing.T) {
	CosmosChainUpgradeIBCTest(t, "juno", "v6.0.0", "v8.0.0", "multiverse")
}

func CosmosChainUpgradeIBCTest(t *testing.T, chainName, initialVersion, upgradeVersion string, upgradeName string) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:      chainName,
			ChainName: chainName,
			Version:   initialVersion,
			ChainConfig: ibc.ChainConfig{
				ModifyGenesis: modifyGenesisShortProposals(votingPeriod, maxDepositPeriod),
			},
		},
		{
			Name:      "gaia",
			ChainName: "gaia",
			Version:   "v7.0.3",
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	client, network := ibctest.DockerSetup(t)

	chain, counterpartyChain := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	const (
		path        = "ibc-upgrade-test-path"
		relayerName = "relayer"
	)

	// Get a relayer instance
	rf := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.StartupFlags("-b", "100"),
	)

	r := rf.Build(t, client, network)

	ic := ibctest.NewInterchain().
		AddChain(chain).
		AddChain(counterpartyChain).
		AddRelayer(r, relayerName).
		AddLink(ibctest.InterchainLink{
			Chain1:  chain,
			Chain2:  counterpartyChain,
			Relayer: r,
			Path:    path,
		})

	ctx := context.Background()

	rep := testreporter.NewNopReporter()

	require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain)
	chainUser := users[0]

	// test IBC conformance before chain upgrade
	conformance.TestChainPair(t, ctx, client, network, chain, counterpartyChain, rf, rep, r, path)

	height, err := chain.Height(ctx)
	require.NoError(t, err, "error fetching height before submit upgrade proposal")

	haltHeight := height + haltHeightDelta

	proposal := cosmos.SoftwareUpgradeProposal{
		Deposit:     "500000000" + chain.Config().Denom, // greater than min deposit
		Title:       "Chain Upgrade 1",
		Name:        upgradeName,
		Description: "First chain software upgrade",
		Height:      haltHeight,
	}

	upgradeTx, err := chain.UpgradeProposal(ctx, chainUser.KeyName, proposal)
	require.NoError(t, err, "error submitting software upgrade proposal tx")

	err = chain.VoteOnProposalAllValidators(ctx, upgradeTx.ProposalID, cosmos.ProposalVoteYes)
	require.NoError(t, err, "failed to submit votes")

	_, err = cosmos.PollForProposalStatus(ctx, chain, height, height+haltHeightDelta, upgradeTx.ProposalID, cosmos.ProposalStatusPassed)
	require.NoError(t, err, "proposal status did not change to passed in expected number of blocks")

	height, err = chain.Height(ctx)
	require.NoError(t, err, "error fetching height before upgrade")

	timeoutCtx, timeoutCtxCancel := context.WithTimeout(ctx, time.Second*45)
	defer timeoutCtxCancel()

	// this should timeout due to chain halt at upgrade height.
	_ = testutil.WaitForBlocks(timeoutCtx, int(haltHeight-height)+1, chain)

	height, err = chain.Height(ctx)
	require.NoError(t, err, "error fetching height after chain should have halted")

	// make sure that chain is halted
	require.Equal(t, haltHeight, height, "height is not equal to halt height")

	// bring down nodes to prepare for upgrade
	err = chain.StopAllNodes(ctx)
	require.NoError(t, err, "error stopping node(s)")

	// upgrade version on all nodes
	chain.UpgradeVersion(ctx, client, upgradeVersion)

	// start all nodes back up.
	// validators reach consensus on first block after upgrade height
	// and chain block production resumes.
	err = chain.StartAllNodes(ctx)
	require.NoError(t, err, "error starting upgraded node(s)")

	timeoutCtx, timeoutCtxCancel = context.WithTimeout(ctx, time.Second*45)
	defer timeoutCtxCancel()

	err = testutil.WaitForBlocks(timeoutCtx, int(blocksAfterUpgrade), chain)
	require.NoError(t, err, "chain did not produce blocks after upgrade")

	// test IBC conformance after chain upgrade on same path
	conformance.TestChainPair(t, ctx, client, network, chain, counterpartyChain, rf, rep, r, path)
}
