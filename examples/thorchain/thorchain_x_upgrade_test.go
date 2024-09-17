package thorchain_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v8"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"

	//"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	haltHeightDelta    = 10 // will propose upgrade this many blocks in the future
	blocksAfterUpgrade = 10
)

func TestThorchainXUpgrade(t *testing.T) {

	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	client, network := interchaintest.DockerSetup(t)
	ctx := context.Background()

	// ----------------------------
	// Set up thorchain and others
	// ----------------------------
	const (
		numThorchainValidators = 2
		numThorchainFullNodes  = 1
		initialVersion         = "v2.136.0-mockupgradetest"
		upgradeVersion         = "v3.137.0-mockupgradetest"
		containerRepo          = "ghcr.io/strangelove-ventures/heighliner/thorchain"
		upgradeName            = "v3.137.0"
		upgradeInfo            = "mock upgrade info"
	)

	thorchainChainSpec := ThorchainDefaultChainSpec(t.Name(), numThorchainValidators, numThorchainFullNodes, "", "", nil, nil)

	thorchainChainSpec.Images[0].Repository = containerRepo
	thorchainChainSpec.Images[0].Version = initialVersion

	log := zaptest.NewLogger(t)

	cf := interchaintest.NewBuiltinChainFactory(log, []*interchaintest.ChainSpec{thorchainChainSpec})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	tc := chains[0].(*tc.Thorchain)

	ic := interchaintest.NewInterchain().
		AddChain(tc).WithLog(log)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	height, err := tc.Height(ctx)
	require.NoError(t, err, "error fetching height before submit upgrade proposal")

	haltHeight := height + haltHeightDelta

	// submit software upgrade proposal from first validator
	_, err = tc.Validators[0].ExecTx(ctx, "thorchain", "thorchain", "propose-upgrade", upgradeName, fmt.Sprint(haltHeight), "--info", `"`+upgradeInfo+`"`)
	require.NoError(t, err, "error submitting software upgrade proposal tx")

	// we have two validators, so we need to approve the upgrade
	// from the second validator to reach quorum schedule the upgrade.

	// the upgrade plan should not be scheduled on-chain yet.
	_, _, err = tc.GetNode().ExecQuery(ctx, "upgrade", "plan")
	require.Error(t, err, "expected error from upgrade plan query")

	// approve software upgrade from second validator
	_, err = tc.Validators[1].ExecTx(ctx, "thorchain", "thorchain", "approve-upgrade", upgradeName)
	require.NoError(t, err, "error submitting software upgrade approval tx")

	// ensure on-chain scheduled upgrade plan matches what was proposed and approved
	stdout, _, err := tc.GetNode().ExecQuery(ctx, "upgrade", "plan")
	require.NoError(t, err, "expected success from upgrade plan query")

	plan := make(map[string]any)
	require.NoError(t, json.Unmarshal(stdout, &plan))
	require.Equal(t, upgradeName, plan["name"], "upgrade name mismatch")
	require.Equal(t, fmt.Sprint(haltHeight), plan["height"], "upgrade height mismatch")
	require.Equal(t, upgradeInfo, plan["info"], "upgrade info mismatch")

	// now wait for chain to halt at upgrade height

	height, err = tc.Height(ctx)
	require.NoError(t, err, "error fetching height before upgrade")

	timeoutCtx, timeoutCtxCancel := context.WithTimeout(ctx, time.Second*45)
	defer timeoutCtxCancel()

	// this should timeout due to chain halt at upgrade height.
	_ = testutil.WaitForBlocks(timeoutCtx, int(haltHeight-height)+1, tc)

	height, err = tc.Height(ctx)
	require.NoError(t, err, "error fetching height after chain should have halted")

	// make sure that chain is halted
	require.Equal(t, haltHeight, height, "height is not equal to halt height")

	// bring down nodes to prepare for upgrade
	err = tc.StopAllNodes(ctx)
	require.NoError(t, err, "error stopping node(s)")

	// upgrade version on all nodes
	tc.UpgradeVersion(ctx, client, containerRepo, upgradeVersion)

	// start all nodes back up.
	// validators reach consensus on first block after upgrade height
	// and chain block production resumes.
	err = tc.StartAllNodes(ctx)
	require.NoError(t, err, "error starting upgraded node(s)")

	timeoutCtx, timeoutCtxCancel = context.WithTimeout(ctx, time.Second*45)
	defer timeoutCtxCancel()

	err = testutil.WaitForBlocks(timeoutCtx, int(blocksAfterUpgrade), tc)
	require.NoError(t, err, "chain did not produce blocks after upgrade")

}
