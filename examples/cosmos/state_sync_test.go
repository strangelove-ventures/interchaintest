package cosmos_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/strangelove-ventures/ibctest/v3"
	"github.com/strangelove-ventures/ibctest/v3/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v3/ibc"
	"github.com/strangelove-ventures/ibctest/v3/internal/configutil"
	"github.com/strangelove-ventures/ibctest/v3/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCosmosHubStateSync(t *testing.T) {
	CosmosChainStateSyncTest(t, "gaia", "v7.0.3")
}

const stateSyncSnapshotInterval = 10

func CosmosChainStateSyncTest(t *testing.T, chainName, version string) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	nf := 1

	configFileOverrides := make(map[string]any)
	appTomlOverrides := make(configutil.Toml)

	// state sync snapshots every stateSyncSnapshotInterval blocks.
	stateSync := make(configutil.Toml)
	stateSync["snapshot-interval"] = stateSyncSnapshotInterval
	appTomlOverrides["state-sync"] = stateSync

	// state sync snapshot interval must be a multiple of pruning keep every interval.
	appTomlOverrides["pruning"] = "custom"
	appTomlOverrides["pruning-keep-recent"] = stateSyncSnapshotInterval
	appTomlOverrides["pruning-keep-every"] = stateSyncSnapshotInterval
	appTomlOverrides["pruning-interval"] = stateSyncSnapshotInterval

	configFileOverrides["config/app.toml"] = appTomlOverrides

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:      chainName,
			ChainName: chainName,
			Version:   version,
			ChainConfig: ibc.ChainConfig{
				ConfigFileOverrides: configFileOverrides,
			},
			NumFullNodes: &nf,
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
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Wait for blocks so that nodes have a few state sync snapshot available
	require.NoError(t, test.WaitForBlocks(ctx, stateSyncSnapshotInterval*2, chain))

	latestHeight, err := chain.Height(ctx)
	require.NoError(t, err, "failed to fetch latest chain height")

	// Trusted height should be state sync snapshot interval blocks ago.
	trustHeight := int64(latestHeight) - stateSyncSnapshotInterval

	firstFullNode := chain.FullNodes[0]

	// Fetch block hash for trusted height.
	blockRes, err := firstFullNode.Client.Block(ctx, &trustHeight)
	require.NoError(t, err, "failed to fetch trusted block")
	trustHash := hex.EncodeToString(blockRes.BlockID.Hash)

	// Construct statesync parameters for new node to get in sync.
	configFileOverrides = make(map[string]any)
	configTomlOverrides := make(configutil.Toml)

	// Set trusted parameters and rpc servers for verification.
	stateSync = make(configutil.Toml)
	stateSync["trust_hash"] = trustHash
	stateSync["trust_height"] = trustHeight
	// State sync requires minimum of two RPC servers for verification. We can provide the same RPC twice though.
	stateSync["rpc_servers"] = fmt.Sprintf("tcp://%s:26657,tcp://%s:26657", firstFullNode.HostName(), firstFullNode.HostName())
	configTomlOverrides["statesync"] = stateSync

	configFileOverrides["config/config.toml"] = configTomlOverrides

	// Now that nodes are providing state sync snapshots, state sync a new node.
	require.NoError(t, chain.AddFullNodes(ctx, configFileOverrides, 1))

	// Wait for new node to be in sync.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	require.NoError(t, test.WaitForInSync(ctx, chain, chain.FullNodes[len(chain.FullNodes)-1]))
}
