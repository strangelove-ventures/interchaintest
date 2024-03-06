package templates

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	interchaintestwasm "github.com/strangelove-ventures/interchaintest/v8/chain/cosmos/wasm"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
)

// TODO: Test FeeShare from Juno here

func registerFeeShareContract(t *testing.T, ctx context.Context, node *cosmos.ChainNode, user ibc.Wallet, contract, withdrawAddr string) {
	cmd := []string{
		"feeshare", "register", contract, withdrawAddr,
	}

	txHash, err := node.ExecTx(ctx, user.KeyName(), cmd...)
	require.NoError(t, err)

	fmt.Println(txHash)
}

// go test -timeout 3000s -run TestCustomModule github.com/strangelove-ventures/interchaintest/v8/templates -v
func TestCustomModule(t *testing.T) {
	t.Parallel()

	var (
		numVals      = 1
		numFullNodes = 0
		sdk47Genesis = []cosmos.GenesisKV{
			// Set base cosmos-sdk params
			cosmos.NewGenesisKV("app_state.gov.params.voting_period", "15s"),
			cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", "10s"),
			cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", "ujuno"),
			cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.amount", "1"),
			// Set custom module params to 50% of gas fees
			cosmos.NewGenesisKV("app_state.feeshare.params.developer_shares", "0.5"),
		}
	)

	// Create a single chain
	chains := interchaintest.CreateChainWithConfig(t, numVals, numFullNodes, "juno", "v16.0.0", ibc.ChainConfig{
		ChainID:        "localjuno-1",
		Name:           "single-chain",
		GasPrices:      "0ujuno",
		ModifyGenesis:  cosmos.ModifyGenesis(sdk47Genesis),
		EncodingConfig: interchaintestwasm.WasmEncoding(),
	})

	enableBlockDB := false
	ctx, _, _, _ := interchaintest.BuildInitialChain(t, chains, enableBlockDB)

	// grab the chain
	chain := chains[0].(*cosmos.CosmosChain)

	// Chains
	juno := chains[0].(*cosmos.CosmosChain)
	node := juno.GetNode()

	nativeDenom := juno.Config().Denom

	// Users
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", math.NewInt(10_000_000), juno)
	granter := users[0]
	feeRcvAddr := "juno1v75wlkccpv7le3560zw32v2zjes5n0e7csr4qh"

	// Upload contract byte code to the chain
	codeId, err := chain.StoreContract(ctx, granter.KeyName(), "contracts/cw_template.wasm")
	require.NoError(t, err)

	// Setup contract instance
	noAdminFlag := true
	contractAddr, err := chain.InstantiateContract(ctx, granter.KeyName(), codeId, `{"count":0}`, noAdminFlag)
	require.NoError(t, err)

	// register contract to a random address (since we are the creator, though not the admin)
	registerFeeShareContract(t, ctx, node, granter, contractAddr, feeRcvAddr)
	if balance, err := juno.GetBalance(ctx, feeRcvAddr, nativeDenom); err != nil {
		t.Fatal(err)
	} else if balance.Int64() != 0 {
		t.Fatal("balance not 0")
	}

	// execute with a 10000 fee (so 5000 denom should be in the contract now with 50% feeshare default)
	res, err := chain.ExecuteContract(ctx, granter.KeyName(), contractAddr, `{"increment":{}}`, "--fees", "10000"+nativeDenom)
	require.NoError(t, err)

	fmt.Println(res)

	// check balance of nativeDenom now
	if balance, err := juno.GetBalance(ctx, feeRcvAddr, nativeDenom); err != nil {
		t.Fatal(err)
	} else if balance.Int64() != 5000 {
		t.Fatal("balance not 5,000. it is ", balance, nativeDenom)
	}
}
