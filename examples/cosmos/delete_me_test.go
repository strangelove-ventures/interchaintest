package cosmos_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// go test -timeout 3000s -run ^TestSingleValBenchmark$ github.com/strangelove-ventures/interchaintest/v8/examples/cosmos -v -count=1
func TestSingleValBenchmark(t *testing.T) {
	now := time.Now()
	t.Parallel()

	ctx := context.Background()

	client, network := interchaintest.DockerSetup(t)
	icOpts := interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      "ibc-go-simd",
			ChainName: "ibc-go-simd",
			Version:   "v8.0.0", // SDK v50
			ChainConfig: ibc.ChainConfig{
				Denom:         denomMetadata.Base,
				Bech32Prefix:  baseBech32,
				CoinType:      "118",
				ModifyGenesis: cosmos.ModifyGenesis(sdk47Genesis),
				GasAdjustment: 1.5,
				OverrideGenesisStart: ibc.GenesisFileStart{
					GenesisFilePath: path.Join(t.Name(), "export.json"), // TODO: if this is not there, do we continue as normal? (or add an option here for PanicOnMissing)
					Client:          client,
					NetworkID:       network,
				},
			},
			NumValidators: &numValsOne,
			NumFullNodes:  &numFullNodesZero,
		},
	})
	fmt.Println("Chain Factory took", time.Since(now))

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chainA := chains[0].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(chainA)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	now = time.Now()
	require.NoError(t, ic.Build(ctx, eRep, icOpts))

	_, err = performExport(t, ctx, chainA, "export.json")
	require.NoError(t, err)

	fmt.Println("Export", time.Since(now))

	fmt.Println("Build", time.Since(now))
}

func performExport(t *testing.T, ctx context.Context, chain *cosmos.CosmosChain, fileName string) ([]byte, error) {
	// TODO: do for every node in the network? (shouldnt be needed tbh)
	val := chain.Validators[0]

	// perform export & save to the host machine
	height, err := val.Height(ctx)
	if err != nil {
		return nil, err
	}

	// stop node & wait\
	err = val.StopContainer(ctx)
	require.NoError(t, err)

	output, err := val.ExportState(ctx, height)
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	require.NoError(t, err)

	p := path.Join(cwd, t.Name(), fileName)

	err = os.MkdirAll(path.Dir(p), 0755)
	require.NoError(t, err)

	// save to output file
	// namespace with the testname
	err = os.WriteFile(p, []byte(output), 0644)
	require.NoError(t, err)

	// start it back up
	err = val.StartContainer(ctx)
	require.NoError(t, err)

	// fr := dockerutil.NewFileWriter(zaptest.NewLogger(t), val.DockerClient, t.Name())
	// err = fr.WriteFile(ctx, val.VolumeName, "export.json", []byte(output))
	return []byte(output), err
}
