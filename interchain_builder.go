package interchaintest

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"

	"github.com/strangelove-ventures/interchaintest/v9/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
	"github.com/strangelove-ventures/interchaintest/v9/relayer"
	"github.com/strangelove-ventures/interchaintest/v9/testreporter"
)

type codecRegistry func(registry codectypes.InterfaceRegistry)

// RegisterInterfaces registers the interfaces for the input codec register functions.
func RegisterInterfaces(codecIR ...codecRegistry) *testutil.TestEncodingConfig {
	cfg := cosmos.DefaultEncoding()
	for _, r := range codecIR {
		r(cfg.InterfaceRegistry)
	}
	return &cfg
}

// CreateChainWithConfig builds a single chain from the given ibc config.
func CreateChainWithConfig(t *testing.T, numVals, numFull int, name, version string, config ibc.ChainConfig) []ibc.Chain {
	t.Helper()

	if version == "" {
		if len(config.Images) == 0 {
			version = "latest"
			t.Logf("no image version specified in config, using %s", version)
		} else {
			version = config.Images[0].Version
		}
	}

	cf := NewBuiltinChainFactory(zaptest.NewLogger(t), []*ChainSpec{
		{
			Name:          name,
			ChainName:     name,
			Version:       version,
			ChainConfig:   config,
			NumValidators: &numVals,
			NumFullNodes:  &numFull,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	return chains
}

// CreateChainsWithChainSpecs builds multiple chains from the given chain specs.
func CreateChainsWithChainSpecs(t *testing.T, chainSpecs []*ChainSpec) []ibc.Chain {
	t.Helper()

	cf := NewBuiltinChainFactory(zaptest.NewLogger(t), chainSpecs)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	return chains
}

func BuildInitialChainWithRelayer(
	t *testing.T,
	chains []ibc.Chain,
	enableBlockDB bool,
	rly ibc.RelayerImplementation,
	relayerFlags []string,
	links []InterchainLink,
	skipPathCreations bool,
) (context.Context, *Interchain, ibc.Relayer, *testreporter.Reporter, *testreporter.RelayerExecReporter, *client.Client, string) {
	t.Helper()

	ctx := context.Background()
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)
	client, network := DockerSetup(t)

	ic := NewInterchain()
	for _, chain := range chains {
		ic = ic.AddChain(chain)
	}

	var r ibc.Relayer
	if links != nil {
		r = NewBuiltinRelayerFactory(
			rly,
			zaptest.NewLogger(t),
			relayer.StartupFlags(relayerFlags...),
		).Build(t, client, network)

		ic.AddRelayer(r, "relayer")

		for _, link := range links {
			link.Relayer = r
			ic = ic.AddLink(link)
		}
	}

	opt := InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: skipPathCreations,
	}
	if enableBlockDB {
		// This can be used to write to the block database which will index all block data e.g. txs, msgs, events, etc.
		opt.BlockDatabaseFile = DefaultBlockDatabaseFilepath()
	}

	err := ic.Build(ctx, eRep, opt)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = ic.Close()

		if r != nil {
			if err := r.StopRelayer(ctx, eRep); err != nil {
				t.Logf("an error occurred while stopping the relayer: %s", err)
			}
		}
	})

	return ctx, ic, r, rep, eRep, client, network
}

func BuildInitialChain(t *testing.T, chains []ibc.Chain, enableBlockDB bool) (context.Context, *Interchain, *client.Client, string) {
	t.Helper()
	ctx, ic, _, _, _, client, network := BuildInitialChainWithRelayer(t, chains, enableBlockDB, 0, nil, nil, true)
	return ctx, ic, client, network
}
