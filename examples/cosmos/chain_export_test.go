package cosmos_test

import (
	"context"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestJunoStateExport(t *testing.T) {
	// SDK v47
	CosmosChainStateExportTest(t, "juno", "v17.0.0")
}

func CosmosChainStateExportTest(t *testing.T, name, version string) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	numVals := 1
	numFullNodes := 0

	// defaults to Juno
	cfg := ibc.ChainConfig{}

	chains := interchaintest.CreateChainWithConfig(t, numVals, numFullNodes, name, version, cfg)
	chain := chains[0].(*cosmos.CosmosChain)

	enableBlockDB := false
	ctx, _, _, _ := interchaintest.BuildInitialChain(t, chains, enableBlockDB)

	HaltChainAndExportGenesis(ctx, t, chain, nil, 3)
}

func HaltChainAndExportGenesis(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, relayer ibc.Relayer, haltHeight int64) {
	timeoutCtx, timeoutCtxCancel := context.WithTimeout(ctx, time.Minute*2)
	defer timeoutCtxCancel()

	err := testutil.WaitForBlocks(timeoutCtx, int(haltHeight), chain)
	require.NoError(t, err, "chain did not halt at halt height")

	err = chain.StopAllNodes(ctx)
	require.NoError(t, err, "error stopping node(s)")

	state, err := chain.ExportState(ctx, int64(haltHeight))
	require.NoError(t, err, "error exporting state")

	appToml := make(testutil.Toml)

	for _, node := range chain.Nodes() {
		err := node.OverwriteGenesisFile(ctx, []byte(state))
		require.NoError(t, err)
	}

	for _, node := range chain.Nodes() {
		err := testutil.ModifyTomlConfigFile(
			ctx,
			zap.NewExample(),
			node.DockerClient,
			node.TestName,
			node.VolumeName,
			"config/app.toml",
			appToml,
		)
		require.NoError(t, err)
	}

	err = chain.StartAllNodes(ctx)
	require.NoError(t, err, "error starting node(s)")

	timeoutCtx, timeoutCtxCancel = context.WithTimeout(ctx, time.Minute*2)
	defer timeoutCtxCancel()

	err = testutil.WaitForBlocks(timeoutCtx, int(5), chain)
	require.NoError(t, err, "chain did not produce blocks after halt")

	height, err := chain.Height(ctx)
	require.NoError(t, err, "error getting height after halt")

	require.Greater(t, int64(height), haltHeight, "height did not increment after halt")
}
