package workspace_optimizer

import (
	"context"
	"encoding/json"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos/wasm"
	"github.com/strangelove-ventures/interchaintest/v7/contract/cosmwasm"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestWorkspaceOptimizerContracts compiles a workspace's contracts using cosmwasm/workspace-optimizer
// It then spins up a juno chain and executes tests
func TestWorkspaceOptimizerContracts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	// Compile the workspace contracts, input is the relative path to the project
	// Using cosmwasm/workspace-optimizer v0.14.0 (default)
	// Output is the workspace object
	// Compilation runs in parallel with the chain setup, waiting if necessary before StoreContract
	workspace := cosmwasm.NewWorkspace("workspace").Compile()

	ctx := context.Background()

	// Chain Factory
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "juno",
			Version: "latest",
			ChainConfig: ibc.ChainConfig{
				GasPrices:      "0.00ujuno",
				EncodingConfig: wasm.WasmEncoding(),
			},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	juno := chains[0].(*cosmos.CosmosChain)

	client, network := interchaintest.DockerSetup(t)

	// Prep Interchain
	ic := interchaintest.NewInterchain().AddChain(juno)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and Fund User Wallets
	initBal := math.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", initBal, juno)
	junoUser := users[0]

	err = testutil.WaitForBlocks(ctx, 2, juno)
	require.NoError(t, err)

	// Verify balance
	junoUserBalInitial, err := juno.GetBalance(ctx, junoUser.FormattedAddress(), juno.Config().Denom)
	require.NoError(t, err)
	require.True(t, junoUserBalInitial.Equal(initBal))

	// Wait for contracts to finish compiling
	contractBinaries, err := workspace.WaitForCompile()
	require.NoError(t, err)

	// Store contract
	contractCodeId, err := juno.StoreContract(ctx, junoUser.KeyName(), contractBinaries["contract1"])
	require.NoError(t, err)

	// Instantiate contract
	contractAddr, err := juno.InstantiateContract(ctx, junoUser.KeyName(), contractCodeId, "{}", true)
	require.NoError(t, err)

	// Query current contract owner
	var queryOwnerResp QueryOwnerResponseData
	queryOwnerMsg := QueryMsg{Owner: &Owner{}}
	err = juno.QueryContract(ctx, contractAddr, queryOwnerMsg, &queryOwnerResp)
	require.NoError(t, err)
	require.Equal(t, junoUser.FormattedAddress(), queryOwnerResp.Data.Address)

	// Set a new contract owner
	newContractOwnerAddr := "juno1kmmr2nu0f2nha6qwhu8s6y5l6yfr3cx505jf25"
	changeContractOwnerMsg := ExecuteMsg{
		ChangeContractOwner: &ChangeContractOwner{
			NewOwner: newContractOwnerAddr,
		},
	}

	msgBz, err := json.Marshal(changeContractOwnerMsg)
	require.NoError(t, err)
	_, err = juno.ExecuteContract(ctx, junoUser.KeyName(), contractAddr, string(msgBz))
	require.NoError(t, err)

	// Query the new contract owner
	err = juno.QueryContract(ctx, contractAddr, queryOwnerMsg, &queryOwnerResp)
	require.NoError(t, err)
	require.Equal(t, newContractOwnerAddr, queryOwnerResp.Data.Address)
}

type ExecuteMsg struct {
	ChangeContractOwner *ChangeContractOwner `json:"change_contract_owner,omitempty"`
}

type ChangeContractOwner struct {
	NewOwner string `json:"new_owner"`
}

type QueryMsg struct {
	Owner *Owner `json:"owner,omitempty"`
}

type Owner struct{}

type QueryOwnerResponseData struct {
	Data QueryOwnerResponse `json:"data,omitempty"`
}

type QueryOwnerResponse struct {
	Address string `json:"address,omitempty"`
}
