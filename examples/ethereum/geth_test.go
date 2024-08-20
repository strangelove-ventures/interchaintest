package ethereum_test

import (
	"context"

	"fmt"
	"strings"
	"testing"
	"time"

	//sdkmath "cosmossdk.io/math"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum/geth"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGeth(t *testing.T) {

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

	// Get default ethereum chain config for anvil
	gethConfig := geth.DefaultEthereumGethChainConfig("ethereum")


	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{ChainConfig: gethConfig},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	ethereumChain := chains[0].(*geth.EthereumChain)

	ic := interchaintest.NewInterchain().
		AddChain(ethereumChain)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))
	fmt.Println("Interchain built")

	// Create and fund a user using GetAndFundTestUsers
	ethUserInitialAmount := geth.ETHER.MulRaw(1000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", ethUserInitialAmount, ethereumChain)
	ethUser := users[0]

	// Check balances of user
	balance, err := ethereumChain.GetBalance(ctx, ethUser.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("User balance:", balance)

	ethUser2, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, "user2", strings.Repeat("dog ", 23)+"fossil", ethUserInitialAmount, ethereumChain)
	require.NoError(t, err)

	fmt.Println("ethUser2", ethUser2.FormattedAddress())
	balance, err = ethereumChain.GetBalance(ctx, ethUser2.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("User2 balance:", balance)

	txHash, err := ethereumChain.SendFundsWithNote(ctx, ethUser2.KeyName(), ibc.WalletAmount{
		Address: ethUser.FormattedAddress(),
		Amount: ethUserInitialAmount.QuoRaw(3),
		Denom: ethereumChain.Config().Denom,
	}, "hello")
	require.NoError(t, err)
	fmt.Println("Tx hash:", txHash)

	// Sleep for additional testing
	time.Sleep(1 * time.Second)

}

type ContractOutput struct {
	TxHash string `json:"transactionHash"`
}