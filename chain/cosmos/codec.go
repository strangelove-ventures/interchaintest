package cosmos

import (
	"cosmossdk.io/x/bank"
	"cosmossdk.io/x/consensus"
	distr "cosmossdk.io/x/distribution"
	"cosmossdk.io/x/gov"
	govclient "cosmossdk.io/x/gov/client"
	"cosmossdk.io/x/mint"
	"cosmossdk.io/x/params"
	paramsclient "cosmossdk.io/x/params/client"
	"cosmossdk.io/x/slashing"
	"cosmossdk.io/x/staking"
	"cosmossdk.io/x/upgrade"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"

	transfer "github.com/cosmos/ibc-go/v9/modules/apps/transfer"
	ibccore "github.com/cosmos/ibc-go/v9/modules/core"
	ibctm "github.com/cosmos/ibc-go/v9/modules/light-clients/07-tendermint"

	// ccvprovider "github.com/cosmos/interchain-security/v5/x/ccv/provider" // TODO:
	ibcwasm "github.com/strangelove-ventures/interchaintest/v9/chain/cosmos/08-wasm-types"
)

func DefaultEncoding() testutil.TestEncodingConfig {
	return testutil.MakeTestEncodingConfig(
		auth.AppModuleBasic{},
		genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
		bank.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distr.AppModuleBasic{},
		gov.NewAppModuleBasic(
			[]govclient.ProposalHandler{
				paramsclient.ProposalHandler,
			},
		),
		params.AppModuleBasic{},
		slashing.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		consensus.AppModuleBasic{},
		transfer.AppModuleBasic{},
		ibccore.AppModuleBasic{},
		ibctm.AppModuleBasic{},
		ibcwasm.AppModuleBasic{},
		// ccvprovider.AppModuleBasic{}, // TODO:
	)
}

func decodeTX(interfaceRegistry codectypes.InterfaceRegistry, txbz []byte) (sdk.Tx, error) {
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.DefaultTxDecoder(cdc)(txbz)
}

func encodeTxToJSON(interfaceRegistry codectypes.InterfaceRegistry, tx sdk.Tx) ([]byte, error) {
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.DefaultJSONTxEncoder(cdc)(tx)
}
