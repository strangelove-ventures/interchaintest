package penumbra_test

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
)

//go:embed subnet-evm/srEXiWaHuhNyGwPUi444Tu47ZEDwxTWrbQiuD7FmgSAQ6X7Dy
var subnetevmVM []byte

//go:embed subnet-evm/genesis.json
var subnetevmGenesis []byte

func TestAvalancheChainStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()
	client, network := interchaintest.DockerSetup(t)

	nv := 5
	nf := 0

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "avalanche",
			Version: "v1.10.1",
			ChainConfig: ibc.ChainConfig{
				ChainID: "neto-123123",
				AvalancheSubnets: []ibc.AvalancheSubnetConfig{
					{
						Name:    "subnetevm",
						VM:      subnetevmVM,
						Genesis: subnetevmGenesis,
					},
				},
			},
			NumFullNodes:  &nf,
			NumValidators: &nv,
		},
	},
	).Chains(t.Name())

	require.NoError(t, err, "failed to get avalanche chain")
	require.Len(t, chains, 1)

	chain := chains[0]

	ctx := context.Background()

	err = chain.Initialize(ctx, t.Name(), client, network)
	require.NoError(t, err, "failed to initialize avalanche chain")

	err = chain.Start(t.Name(), ctx)
	require.NoError(t, err, "failed to start avalanche chain")

	subnetCtx := context.WithValue(ctx, "subnet", "0")

	eg := new(errgroup.Group)
	eg.Go(func() error {
		return chain.SendFunds(subnetCtx, "56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027", ibc.WalletAmount{
			Address: "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
			Amount:  1000000,
		})
	})
	eg.Go(func() error {
		return testutil.WaitForBlocks(subnetCtx, 2, chain)
	})

	require.NoError(t, eg.Wait(), "avalanche chain failed to make blocks")
}
