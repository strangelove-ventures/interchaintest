package polkadot_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"cosmossdk.io/math"
	"github.com/icza/dyno"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	heightDelta      = uint64(20)
	votingPeriod     = "30s"
	maxDepositPeriod = "10s"
)

// Spin up a simd chain, push a contract, and get that contract code from chain
func TestPushWasmClientCode(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Override config files to support an ~2.5MB contract
	configFileOverrides := make(map[string]any)

	appTomlOverrides := make(testutil.Toml)
	configTomlOverrides := make(testutil.Toml)

	apiOverrides := make(testutil.Toml)
	apiOverrides["rpc-max-body-bytes"] = 2_000_000
	appTomlOverrides["api"] = apiOverrides

	rpcOverrides := make(testutil.Toml)
	rpcOverrides["max_body_bytes"] = 2_000_000
	rpcOverrides["max_header_bytes"] = 2_100_000
	configTomlOverrides["rpc"] = rpcOverrides

	//mempoolOverrides := make(testutil.Toml)
	//mempoolOverrides["max_tx_bytes"] = 6000000
	//configTomlOverrides["mempool"] = mempoolOverrides

	configFileOverrides["config/app.toml"] = appTomlOverrides
	configFileOverrides["config/config.toml"] = configTomlOverrides

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{ChainConfig: ibc.ChainConfig{
			Type:    "cosmos",
			Name:    "ibc-go-simd",
			ChainID: "simd",
			Images: []ibc.DockerImage{
				{
					Repository: "ghcr.io/strangelove-ventures/heighliner/ibc-go-simd",
					Version:    "feat-wasm-clients",
					UidGid:     "1025:1025",
				},
			},
			Bin:            "simd",
			Bech32Prefix:   "cosmos",
			Denom:          "stake",
			GasPrices:      "0.00stake",
			GasAdjustment:  1.3,
			TrustingPeriod: "504h",
			//EncodingConfig: WasmClientEncoding(),
			NoHostMount:         true,
			ConfigFileOverrides: configFileOverrides,
			ModifyGenesis:       modifyGenesisShortProposals(votingPeriod, maxDepositPeriod),
		},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	simd := chains[0]

	ic := interchaintest.NewInterchain().
		AddChain(simd)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true, // Skip path creation, so we can have granular control over the process
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and Fund User Wallets
	fundAmount := math.NewInt(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", fundAmount.Int64(), simd)
	simd1User := users[0]

	simd1UserBalInitial, err := simd.GetBalance(ctx, simd1User.FormattedAddress(), simd.Config().Denom)
	require.NoError(t, err)
	require.True(t, simd1UserBalInitial.Equal(fundAmount))

	simdChain := simd.(*cosmos.CosmosChain)

	// Verify a normal user cannot push a wasm light client contract
	_, err = simdChain.StoreClientContract(ctx, simd1User.KeyName(), "ics10_grandpa_cw.wasm")
	require.ErrorContains(t, err, "invalid authority")

	proposal := cosmos.TxProposalv1{
		Metadata: "none",
		Deposit:  "500000000" + simdChain.Config().Denom, // greater than min deposit
		Title:    "Grandpa Contract",
		Summary:  "new grandpa contract",
	}

	proposalTx, codeHash, err := simdChain.PushNewWasmClientProposal(ctx, simd1User.KeyName(), "ics10_grandpa_cw.wasm", proposal)
	require.NoError(t, err, "error submitting new wasm contract proposal tx")

	height, err := simdChain.Height(ctx)
	require.NoError(t, err, "error fetching height before submit upgrade proposal")

	err = simdChain.VoteOnProposalAllValidators(ctx, proposalTx.ProposalID, cosmos.ProposalVoteYes)
	require.NoError(t, err, "failed to submit votes")

	_, err = cosmos.PollForProposalStatus(ctx, simdChain, height, height+heightDelta, proposalTx.ProposalID, cosmos.ProposalStatusPassed)
	require.NoError(t, err, "proposal status did not change to passed in expected number of blocks")

	err = testutil.WaitForBlocks(ctx, 2, simd)
	require.NoError(t, err)

	var getCodeQueryMsgRsp GetCodeQueryMsgResponse
	err = simdChain.QueryClientContractCode(ctx, codeHash, &getCodeQueryMsgRsp)
	codeHashByte32 := sha256.Sum256(getCodeQueryMsgRsp.Code)
	codeHash2 := hex.EncodeToString(codeHashByte32[:])
	t.Logf("Contract codeHash from code: %s", codeHash2)
	require.NoError(t, err)
	require.NotEmpty(t, getCodeQueryMsgRsp.Code)
	require.Equal(t, codeHash, codeHash2)
}

type GetCodeQueryMsgResponse struct {
	Code []byte `json:"code"`
}

func modifyGenesisShortProposals(votingPeriod string, maxDepositPeriod string) func(ibc.ChainConfig, []byte) ([]byte, error) {
	return func(chainConfig ibc.ChainConfig, genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}
		if err := dyno.Set(g, votingPeriod, "app_state", "gov", "params", "voting_period"); err != nil {
			return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
		}
		if err := dyno.Set(g, maxDepositPeriod, "app_state", "gov", "params", "max_deposit_period"); err != nil {
			return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
		}
		if err := dyno.Set(g, chainConfig.Denom, "app_state", "gov", "params", "min_deposit", 0, "denom"); err != nil {
			return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
		}
		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
