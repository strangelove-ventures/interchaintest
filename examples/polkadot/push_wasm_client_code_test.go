package polkadot_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/strangelove-ventures/ibctest/v6"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/testreporter"
	"github.com/strangelove-ventures/ibctest/v6/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	//simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
)

// Re-add once feat/wasm-client branch is on ibc-go v6
/*func WasmClientEncoding() *simappparams.EncodingConfig {
	cfg := cosmos.DefaultEncoding()

	wasmclient.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}*/

// Spin up a simd chain, push a contract, and get that contract code from chain
func TestPushWasmClientCode(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Override config files to support an ~2.5MB contract
	configFileOverrides := make(map[string]any)

	appTomlOverrides := make(testutil.Toml)
	configTomlOverrides := make(testutil.Toml)

	apiOverrides := make(testutil.Toml)
	apiOverrides["rpc-max-body-bytes"] = 1350000
	appTomlOverrides["api"] = apiOverrides

	rpcOverrides := make(testutil.Toml)
	rpcOverrides["max_body_bytes"] = 1350000
	rpcOverrides["max_header_bytes"] = 1400000
	configTomlOverrides["rpc"] = rpcOverrides

	//mempoolOverrides := make(testutil.Toml)
	//mempoolOverrides["max_tx_bytes"] = 6000000
	//configTomlOverrides["mempool"] = mempoolOverrides

	configFileOverrides["config/app.toml"] = appTomlOverrides
	configFileOverrides["config/config.toml"] = configTomlOverrides

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		/*{
			Name: "ibc-go-simd",
			Version: "feat/wasm-client",
			ChainConfig: ibc.ChainConfig{
				GasPrices:  "0.00stake",
				EncodingConfig: WasmClientEncoding(),
			}
		},*/
		{ChainConfig: ibc.ChainConfig{
			Type:    "cosmos",
			Name:    "ibc-go-simd",
			ChainID: "simd",
			Images: []ibc.DockerImage{
				{
					Repository: "ibc-go-simd",
					Version:    "feat-wasm-client",
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
		},
		},
	})

	t.Logf("Calling cf.Chains")
	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	simd := chains[0]

	t.Logf("NewInterchain")
	ic := ibctest.NewInterchain().
		AddChain(simd)

	t.Logf("Interchain build options")
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true, // Skip path creation, so we can have granular control over the process
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and Fund User Wallets
	fundAmount := int64(100_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, "default", int64(fundAmount), simd)
	simd1User := users[0]

	err = testutil.WaitForBlocks(ctx, 2, simd)
	require.NoError(t, err)

	simd1UserBalInitial, err := simd.GetBalance(ctx, simd1User.FormattedAddress(), simd.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, fundAmount, simd1UserBalInitial)

	err = testutil.WaitForBlocks(ctx, 2, simd)
	require.NoError(t, err)

	simdChain := simd.(*cosmos.CosmosChain)

	codeHash, err := simdChain.StoreClientContract(ctx, simd1User.KeyName(), "ics10_grandpa_cw.wasm")
	t.Logf("Contract codeHash: %s", codeHash)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 5, simd)
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
