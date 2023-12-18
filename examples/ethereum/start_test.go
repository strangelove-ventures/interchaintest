package ethereum_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum"
	//"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestEthereum(t *testing.T) {

	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	// Log location
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainName: "ethereum", 
			Name: "ethereum",
			Version: "latest",
			ChainConfig: ethereum.DefaultEthereumAnvilChainConfig("ethereum"),
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	ethereumChain := chains[0].(*ethereum.EthereumChain)

	ic := interchaintest.NewInterchain().
		AddChain(ethereumChain)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true, // Skip path creation, so we can have granular control over the process
	}))
	fmt.Println("Interchain built, sleeping")

	time.Sleep(5 * time.Second)
	height, err := ethereumChain.Height(ctx)
	require.NoError(t, err)
	fmt.Println("Height: ", height)

	balance, err := ethereumChain.GetBalance(ctx, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", "")
	require.NoError(t, err)
	fmt.Println("Balance: ", balance)

	time.Sleep(5 * time.Second)
	height, err = ethereumChain.Height(ctx)
	require.NoError(t, err)
	fmt.Println("Height: ", height)

	time.Sleep(240 * time.Second)

}