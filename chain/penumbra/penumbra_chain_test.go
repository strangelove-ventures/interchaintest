package penumbra_test

import (
	"os"
	"testing"

	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	"github.com/strangelove-ventures/ibc-test-framework/log"
	"github.com/stretchr/testify/require"
)

func TestPenumbraChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoErrorf(t, err, "failed to set up test run")

	logger := log.New(os.Stderr, "console", "info")
	chain, err := ibctest.GetChain(t.Name(), "penumbra", "010-pasithee,v0.35.0", "penumbra-1", 4, 1, logger)
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
