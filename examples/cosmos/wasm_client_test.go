package cosmos_test 

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/icza/dyno"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
)

const (
	heightDelta      = uint64(10)
	govVotingPeriod     = "10s"
	govMaxDepositPeriod = "10s"
)

func TestTendermintWasm(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name: "ibc-go-simd",
			ChainName: "ibc-go-simd",
			Version: "feat-wasm-clients-main",
			ChainConfig: ibc.ChainConfig{
				ModifyGenesis:       modifyGenesisShortProposalsNew(),
			},
		},
		{
			Name: "ibc-go-simd",
			ChainName: "fake-gaia", // Gaia needs to support ibc-go v7.2 so it can verify membership of counterparty chain's 08-wasm types
			Version: "feat-wasm-clients-main",
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	simdChain := chains[0].(*cosmos.CosmosChain)
	gaiaChain := chains[1].(*cosmos.CosmosChain)

	// Get a relayer instance
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.CustomDockerImage("ghcr.io/cosmos/relayer", "steve-wasm", "100:1000"),
	).Build(t, client, network)

	// Build the network; spin up the chains and configure the relayer
	const pathName = "simd-gaia"

	ic := interchaintest.NewInterchain().
		AddChain(simdChain).
		AddChain(gaiaChain).
		AddRelayer(r, "rly").
		AddLink(interchaintest.InterchainLink{
			Chain1:  simdChain,
			Chain2:  gaiaChain,
			Relayer: r,
			Path:    pathName,
		})

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true, // Skip path creation, so we can have granular control over the process
	}))
	fmt.Println("Interchain built")

	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create a proposal, vote, and wait for it to pass. Return code hash for relayer.
	codeHash := pushWasmContractViaGov(t, ctx, simdChain)

	// Create path for relayer between simd and gaia
	err = r.GeneratePath(ctx, eRep, simdChain.Config().ChainID, gaiaChain.Config().ChainID, pathName)
	require.NoError(t, err)

	// Link path: create clients, connection, and channel
	err = r.LinkPath(ctx, eRep, pathName, ibc.DefaultChannelOpts(), ibc.CreateClientOptions{
		TrustingPeriod: "0",
		SrcChainWasmCodeID: codeHash, // simd's wasm contract code id
		DstChainWasmCodeID: "", // gaia has no contract
	})

	// Start relayer
	r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = r.StopRelayer(ctx, eRep)
		if err != nil {
			panic(err)
		}
	})

	// Fund users on both simd and gaia
	fundAmount := int64(12_345_000)
	simdUser, gaiaUser := fundUsers(t, ctx, fundAmount, simdChain, gaiaChain)

	// Send 2.77 simd native token from simd user to gaia user
	sendNativeS2G := int64(2_770_000)
	transferS2G := ibc.WalletAmount{
		Address: gaiaUser.FormattedAddress(),
		Denom:   simdChain.Config().Denom,
		Amount:  sendNativeS2G,
	}
	tx, err := simdChain.SendIBCTransfer(ctx, "channel-0", simdUser.KeyName(), transferS2G, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate()) // test source wallet has decreased funds

	// Send 2.88 gaia native token from gaia user to simd user
	sendNativeG2S := int64(2_880_000)
	transferG2S := ibc.WalletAmount{
		Address: simdUser.FormattedAddress(),
		Denom:   simdChain.Config().Denom,
		Amount:  sendNativeG2S,
	}
	tx, err = gaiaChain.SendIBCTransfer(ctx, "channel-0", gaiaUser.KeyName(), transferG2S, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate()) // test source wallet has decreased funds

	err = testutil.WaitForBlocks(ctx, 2, simdChain, gaiaChain)
	require.NoError(t, err)

	simdDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", "channel-0", simdChain.Config().Denom))
	gaiaDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", "channel-0", gaiaChain.Config().Denom))
	
	// Send 1.22 simd/ibc token from gaia user to simd user
	reflectIbcG2S := int64(1_220_000)
	reflectTransferG2S := ibc.WalletAmount{
		Address: simdUser.FormattedAddress(),
		Denom:   simdDenomTrace.IBCDenom(),
		Amount:  reflectIbcG2S,
	}
	tx, err = gaiaChain.SendIBCTransfer(ctx, "channel-0", gaiaUser.KeyName(), reflectTransferG2S, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate()) // test source wallet has decreased funds

	// Send 1.22 gaia/ibc token from simd user to gaia user
	reflectIbcS2G := int64(1_220_000)
	reflectTransferS2G := ibc.WalletAmount{
		Address: gaiaUser.FormattedAddress(),
		Denom:   gaiaDenomTrace.IBCDenom(),
		Amount:  reflectIbcS2G,
	}
	tx, err = simdChain.SendIBCTransfer(ctx, "channel-0", simdUser.KeyName(), reflectTransferS2G, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate()) // test source wallet has decreased funds

	err = testutil.WaitForBlocks(ctx, 2, simdChain, gaiaChain)
	require.NoError(t, err)

	// Verify simd user's final native balance
	simdUserNativeBal, err := simdChain.GetBalance(ctx, simdUser.FormattedAddress(), simdChain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, fundAmount-sendNativeS2G+reflectIbcG2S, simdUserNativeBal)

	// Verify simd user's final gaia/ibc token balance
	simdUserGaiaBal, err := simdChain.GetBalance(ctx, simdUser.FormattedAddress(), gaiaDenomTrace.IBCDenom())
	require.NoError(t, err)
	require.Equal(t, sendNativeG2S-reflectIbcS2G, simdUserGaiaBal)

	// Verify gaia user's final native balance
	gaiaUserNativeBal, err := gaiaChain.GetBalance(ctx, gaiaUser.FormattedAddress(), gaiaChain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, fundAmount-sendNativeG2S+reflectIbcS2G, gaiaUserNativeBal)

	// Verify gaia user's final simd/ibc token balance
	gaiaUserSimdBal, err := gaiaChain.GetBalance(ctx, gaiaUser.FormattedAddress(), simdDenomTrace.IBCDenom())
	require.NoError(t, err)
	require.Equal(t, sendNativeS2G-reflectIbcG2S, gaiaUserSimdBal)
}

type GetCodeQueryMsgResponse struct {
	Code []byte `json:"code"`
}

func pushWasmContractViaGov(t *testing.T, ctx context.Context, cosmosChain *cosmos.CosmosChain) string {
	// Set up cosmos user for pushing new wasm code msg via governance
	fundAmountForGov := int64(10_000_000_000)
	contractUsers := interchaintest.GetAndFundTestUsers(t, ctx, "default", int64(fundAmountForGov), cosmosChain)
	contractUser := contractUsers[0]

	err := testutil.WaitForBlocks(ctx, 1, cosmosChain)
	require.NoError(t, err)

	contractUserBalInitial, err := cosmosChain.GetBalance(ctx, contractUser.FormattedAddress(), cosmosChain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, fundAmountForGov, contractUserBalInitial)

	proposal := cosmos.TxProposalv1{
		Metadata: "none",
		Deposit:  "500000000" + cosmosChain.Config().Denom, // greater than min deposit
		Title:    "Tendermint Contract",
		Summary:  "new tendermint contract",
	}

	proposalTx, codeHash, err := cosmosChain.PushNewWasmClientProposal(ctx, contractUser.KeyName(), "./wasm/ics07_tendermint_cw.wasm", proposal)
	require.NoError(t, err, "error submitting new wasm contract proposal tx")

	height, err := cosmosChain.Height(ctx)
	require.NoError(t, err, "error fetching height before submit upgrade proposal")

	err = cosmosChain.VoteOnProposalAllValidators(ctx, proposalTx.ProposalID, cosmos.ProposalVoteYes)
	require.NoError(t, err, "failed to submit votes")

	_, err = cosmos.PollForProposalStatus(ctx, cosmosChain, height, height+heightDelta, proposalTx.ProposalID, cosmos.ProposalStatusPassed)
	require.NoError(t, err, "proposal status did not change to passed in expected number of blocks")

	err = testutil.WaitForBlocks(ctx, 1, cosmosChain)
	require.NoError(t, err)

	var getCodeQueryMsgRsp GetCodeQueryMsgResponse
	err = cosmosChain.QueryClientContractCode(ctx, codeHash, &getCodeQueryMsgRsp)
	codeHashByte32 := sha256.Sum256(getCodeQueryMsgRsp.Code)
	codeHash2 := hex.EncodeToString(codeHashByte32[:])
	require.NoError(t, err)
	require.NotEmpty(t, getCodeQueryMsgRsp.Code)
	require.Equal(t, codeHash, codeHash2)

	return codeHash
}

func fundUsers(t *testing.T, ctx context.Context, fundAmount int64, cosmosChain1 ibc.Chain, cosmosChain2 ibc.Chain) (ibc.Wallet, ibc.Wallet) {
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", fundAmount, cosmosChain1, cosmosChain2)
	cosmosUser1, cosmosUser2 := users[0], users[1]
	err := testutil.WaitForBlocks(ctx, 1, cosmosChain1, cosmosChain2)
	require.NoError(t, err, "cosmos or polkadot chain failed to make blocks")

	// Check balances are correct
	cosmosUser1Amount, err := cosmosChain1.GetBalance(ctx, cosmosUser1.FormattedAddress(), cosmosChain1.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, fundAmount, cosmosUser1Amount, "Initial polkadot user amount not expected")
	cosmosUser2Amount, err := cosmosChain2.GetBalance(ctx, cosmosUser2.FormattedAddress(), cosmosChain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, fundAmount, cosmosUser2Amount, "Initial parachain user amount not expected")

	return cosmosUser1, cosmosUser2
}

func modifyGenesisShortProposalsNew() func(ibc.ChainConfig, []byte) ([]byte, error) {
	return func(chainConfig ibc.ChainConfig, genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}
		if err := dyno.Set(g, govVotingPeriod, "app_state", "gov", "params", "voting_period"); err != nil {
			return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
		}
		if err := dyno.Set(g, govMaxDepositPeriod, "app_state", "gov", "params", "max_deposit_period"); err != nil {
			return nil, fmt.Errorf("failed to set max deposit period in genesis json: %w", err)
		}
		if err := dyno.Set(g, chainConfig.Denom, "app_state", "gov", "params", "min_deposit", 0, "denom"); err != nil {
			return nil, fmt.Errorf("failed to set min deposit in genesis json: %w", err)
		}
		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
