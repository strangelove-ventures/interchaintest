package penumbra_test

import (
	"testing"

	"github.com/strangelove-ventures/ibctest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPenumbraChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoErrorf(t, err, "failed to set up test run")

	chain, err := ibctest.GetChain(t.Name(), "penumbra", "010-pasithee,v0.35.0", "penumbra-1", 4, 1, zap.NewNop())
	require.NoError(t, err, "failed to get penumbra chain")

	err = chain.Initialize(t.Name(), home, pool, network)
	require.NoError(t, err, "failed to initialize penumbra chain")

	err = chain.Start(t.Name(), ctx)
	require.NoError(t, err, "failed to start penumbra chain")

	_, err = chain.WaitForBlocks(50)
	require.NoError(t, err, "penumbra chain failed to make blocks")
}

// pcli -w /root/.penumbra/wallet wallet generate
// pcli -w /root/.penumbra/wallet addr new validator
