package cosmos_test

import (
	"context"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	wallet   = "dym"
	denom    = "udym"
	display  = "DYM"
	decimals = 18
)

func TestEthermintChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	numVals := 1
	numFullNodes := 0

	cosmos.SetSDKConfig(wallet)

	genesis := []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.gov.voting_params.voting_period", "1m"),
		cosmos.NewGenesisKV("app_state.gov.deposit_params.min_deposit.0.denom", denom),
		cosmos.NewGenesisKV("app_state.gov.deposit_params.min_deposit.0.amount", "1"),

		cosmos.NewGenesisKV("app_state.rollapp.params.dispute_period_in_blocks", "2"),

		cosmos.NewGenesisKV("app_state.staking.params.max_validators", 110),

		cosmos.NewGenesisKV("consensus_params.block.max_gas", "40000000"),
		cosmos.NewGenesisKV("app_state.feemarket.params.no_base_fee", true),
		cosmos.NewGenesisKV("app_state.evm.params.evm_denom", denom),
		cosmos.NewGenesisKV("app_state.evm.params.enable_create", false),

		cosmos.NewGenesisKV("app_state.epochs.epochs", []evmEpoch{
			newEvmEpoch("week", "604800s"),
			newEvmEpoch("day", "86400s"),
			newEvmEpoch("hour", "3600s"),
			newEvmEpoch("minute", "60s"),
		}),

		cosmos.NewGenesisKV("app_state.incentives.params.distr_epoch_identifier", "minute"),
		cosmos.NewGenesisKV("app_state.poolincentives.params.minted_denom", denom),
		cosmos.NewGenesisKV("app_state.poolincentives.lockable_durations", []string{"3600s"}),

		cosmos.NewGenesisKV("app_state.crisis.constant_fee.denom", denom),
		cosmos.NewGenesisKV("app_state.poolmanager.params.pool_creation_fee.0.denom", denom),

		cosmos.NewGenesisKV("app_state.bank.denom_metadata", []banktypes.Metadata{
			{
				Description: "Denom metadata",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    denom,
						Exponent: 0,
					},
					{
						Denom:    display,
						Exponent: decimals,
					},
				},
				Base:    denom,
				Display: display,
				Name:    display,
				Symbol:  display,
				URI:     "",
				URIHash: "",
			},
		}),
	}

	decimals := int64(decimals)
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name: "dymension",
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				ChainID:        "dymension_100-1",
				Images:         []ibc.DockerImage{{Repository: "ghcr.io/strangelove-ventures/heighliner/dymension", Version: "854ef84", UidGid: "1025:1025"}},
				Bin:            "dymd",
				Bech32Prefix:   wallet,
				Denom:          denom,
				CoinType:       "60",
				GasPrices:      "0" + denom,
				GasAdjustment:  1.5,
				TrustingPeriod: "168h0m0s",
				ModifyGenesis:  cosmos.ModifyGenesis(genesis),
				CoinDecimals:   &decimals,
			},
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", sdkmath.NewInt(10_000_000_000), chain, chain)
	user := users[0]

	balance, err := chain.GetNode().Chain.GetBalance(ctx, user.FormattedAddress(), denom)
	require.NoError(t, err)
	require.Equal(t, "10000000000", balance.String())
}

type evmEpoch struct {
	Identifier              string    `json:"identifier"`
	StartTime               time.Time `json:"start_time"`
	Duration                string    `json:"duration"`
	CurrentEpoch            string    `json:"current_epoch"`
	CurrentEpochStartTime   time.Time `json:"current_epoch_start_time"`
	EpochCountingStarted    bool      `json:"epoch_counting_started"`
	CurrentEpochStartHeight string    `json:"current_epoch_start_height"`
}

func newEvmEpoch(identifier string, duration string) evmEpoch {
	return evmEpoch{
		Identifier:              identifier,
		StartTime:               time.Time{},
		Duration:                duration,
		CurrentEpoch:            "0",
		CurrentEpochStartTime:   time.Time{},
		EpochCountingStarted:    false,
		CurrentEpochStartHeight: "0",
	}
}
