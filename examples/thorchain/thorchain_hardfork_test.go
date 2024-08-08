package thorchain_test

import (
	"context"
	_ "embed"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	//"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

//go:embed mainnet-genesis.json
var genesisBz []byte

func TestThorchainHardFork(t *testing.T) {
	numThorchainValidators := 2
	numThorchainFullNodes := 0

	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	client, network := interchaintest.DockerSetup(t)
	ctx := context.Background()

	// ----------------------------
	// Set up thorchain and others
	// ----------------------------
	thorchainChainSpec := ThorchainDefaultChainSpec(t.Name(), numThorchainValidators, numThorchainFullNodes, "")

	// Start from mainnet state
	thorchainChainSpec.Genesis = &ibc.GenesisConfig{
		Contents:      genesisBz,
		AllValidators: false, // only top 2/3 VP
	}

	// TODO: add router contracts to thorchain
	// Set ethereum RPC
	// Move other chains to above for setup too?

	chainSpecs := []*interchaintest.ChainSpec{
		thorchainChainSpec,
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), chainSpecs)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	thorchain := chains[0].(*tc.Thorchain)

	ic := interchaintest.NewInterchain().
		AddChain(thorchain)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	err = testutil.WaitForBlocks(ctx, 10, thorchain)
	require.NoError(t, err, "thorchain failed to make blocks")
}
