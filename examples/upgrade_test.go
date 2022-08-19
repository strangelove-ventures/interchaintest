package ibctest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/icza/dyno"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	haltHeight         = uint64(30)
	blocksAfterUpgrade = uint64(10)
	votingPeriod       = "10s"
)

func TestJunoUpgrade(t *testing.T) {
	CosmosChainUpgradeTest(t, "juno", "v6.0.0", "v7.0.0")
}

func CosmosChainUpgradeTest(t *testing.T, chainName, initialVersion, upgradeVersion string) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:          chainName,
			ChainName:     chainName,
			Version:       initialVersion,
			ModifyGenesis: modifyGenesisVotingPeriod(votingPeriod),
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*cosmos.CosmosChain)

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

	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain)
	chainUser := users[0]

	proposal := ibc.SoftwareUpgradeProposal{
		Deposit:     "500000000" + chain.Config().Denom,
		Title:       "Chain Upgrade 1",
		Name:        "chain-upgrade",
		Description: "First chain software upgrade",
		Height:      haltHeight,
	}

	upgradeTx, err := chain.UpgradeProposal(ctx, chainUser.KeyName, proposal)
	require.NoError(t, err, "error submitting software upgrade proposal tx")

	err = chain.VoteOnProposalAllValidators(ctx, upgradeTx.ProposalID, ibc.ProposalVoteYes)
	require.NoError(t, err, "failed to submit votes")

	timeoutCtx, timeoutCtxCancel := context.WithTimeout(ctx, time.Second*45)
	defer timeoutCtxCancel()

	height, err := chain.Height(ctx)
	require.NoError(t, err, "error fetching height before upgrade")

	err = test.WaitForBlocks(timeoutCtx, int(haltHeight-height)+1, chain)
	require.Error(t, err, "chain did not halt at halt height")

	height, err = chain.Height(ctx)
	require.NoError(t, err, "error fetching height after halt")

	require.Equal(t, haltHeight, height, "height is not equal to halt height")

	err = chain.StopAllNodes(ctx)
	require.NoError(t, err, "error stopping node(s)")

	chain.UpgradeVersion(ctx, client, upgradeVersion)

	err = chain.StartAllNodes(ctx)
	require.NoError(t, err, "error starting upgraded node(s)")

	timeoutCtx, timeoutCtxCancel = context.WithTimeout(ctx, time.Second*45)
	defer timeoutCtxCancel()

	err = test.WaitForBlocks(timeoutCtx, int(blocksAfterUpgrade), chain)
	require.NoError(t, err, "chain did not produce blocks after upgrade")

	height, err = chain.Height(ctx)
	require.NoError(t, err, "error fetching height after upgrade")

	require.GreaterOrEqual(t, height, haltHeight+blocksAfterUpgrade, "height did not increment enough after upgrade")
}

func modifyGenesisVotingPeriod(votingPeriod string) func([]byte) ([]byte, error) {
	return func(genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}
		if err := dyno.Set(g, votingPeriod, "app_state", "gov", "voting_params", "voting_period"); err != nil {
			return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
		}
		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
