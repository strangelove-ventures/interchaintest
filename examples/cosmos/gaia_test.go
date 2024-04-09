package cosmos

import (
	"context"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v6"
	"github.com/strangelove-ventures/interchaintest/v6/ibc"
	"github.com/strangelove-ventures/interchaintest/v6/relayer"
	"github.com/strangelove-ventures/interchaintest/v6/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	cosmosDockerRepository = "ghcr.io/strangelove-ventures/heighliner/gaia"
	cosmosDockerVersion    = "v15.1.0"
)

var (
	chainACfg = &interchaintest.ChainSpec{
		Name: "gaia",
		ChainConfig: ibc.ChainConfig{
			Type:                   "cosmos",
			Name:                   "gaia",
			ChainID:                "chainA",
			Bin:                    "gaiad",
			Bech32Prefix:           "cosmos",
			Denom:                  "uatom",
			GasPrices:              "0.5uatom",
			GasAdjustment:          50,
			TrustingPeriod:         "504hours",
			NoHostMount:            false,
			UsingNewGenesisCommand: true,
			Images:                 cosmosDockerImages(),
		},
		NumValidators: numValidators(),
		NumFullNodes:  numFullNodes(),
	}

	chainBCfg = &interchaintest.ChainSpec{
		Name: "gaia",
		ChainConfig: ibc.ChainConfig{
			Type:                   "cosmos",
			Name:                   "gaia",
			ChainID:                "chianB",
			Bin:                    "gaiad",
			Bech32Prefix:           "cosmos",
			Denom:                  "uatom",
			GasPrices:              "0.5uatom",
			GasAdjustment:          50,
			TrustingPeriod:         "504hours",
			NoHostMount:            false,
			UsingNewGenesisCommand: true,
			Images:                 cosmosDockerImages(),
		},
		NumValidators: numValidators(),
		NumFullNodes:  numFullNodes(),
	}
)

func numFullNodes() *int {
	n := 1
	return &n
}

func numValidators() *int {
	n := 1
	return &n
}

func cosmosDockerImages() []ibc.DockerImage {
	return []ibc.DockerImage{
		{
			Repository: cosmosDockerRepository,
			Version:    cosmosDockerVersion,
			UidGid:     "1025:1025",
		},
	}
}

func TestGaia(t *testing.T) {
	var (
		ctx             = context.Background()
		client, network = interchaintest.DockerSetup(t)
		rep             = testreporter.NewNopReporter()
		eRep            = rep.RelayerExecReporter(t)
		ibcPath         = "chainA-chainB"
	)

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		chainACfg,
		chainBCfg,
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chainA, chainB := chains[0], chains[1]

	i := ibc.DockerImage{
		Repository: "relayer",
		Version:    "local",
		UidGid:     "1025:1025",
	}

	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.CustomDockerImage(i.Repository, i.Version, i.UidGid),
		relayer.ImagePull(false),
	).Build(t, client, network)

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  chainA,
			Chain2:  chainB,
			Relayer: r,
			Path:    ibcPath,
		})

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}))
}
