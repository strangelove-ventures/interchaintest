package cosmos

import (
	"cosmossdk.io/x/bank"
	"cosmossdk.io/x/consensus"
	distr "cosmossdk.io/x/distribution"
	"cosmossdk.io/x/gov"

	"cosmossdk.io/x/mint"
	"cosmossdk.io/x/params"

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

	codectestutil "github.com/cosmos/cosmos-sdk/codec/testutil"
	transfer "github.com/cosmos/ibc-go/v9/modules/apps/transfer"
	ibccore "github.com/cosmos/ibc-go/v9/modules/core"
	ibctm "github.com/cosmos/ibc-go/v9/modules/light-clients/07-tendermint"
	// ccvprovider "github.com/cosmos/interchain-security/v5/x/ccv/provider" // TODO:
	// genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	// ibcwasm "github.com/strangelove-ventures/interchaintest/v9/chain/cosmos/08-wasm-types"
	// govclient "cosmossdk.io/x/gov/client"
	// paramsclient "cosmossdk.io/x/params/client"
)

func DefaultEncoding() testutil.TestEncodingConfig {
	return testutil.MakeTestEncodingConfig(
		codectestutil.CodecOptions{},
		auth.AppModule{},
		// genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
		genutil.AppModule{},
		bank.AppModule{},
		staking.AppModule{},
		mint.AppModule{},
		distr.AppModule{},
		gov.AppModule{},
		params.AppModule{},
		slashing.AppModule{},
		upgrade.AppModule{},
		consensus.AppModule{},
		transfer.AppModule{},
		ibccore.AppModule{},
		ibctm.AppModule{},
		// ibcwasm.AppModule{},
		// ccvprovider.AppModule{}, // TODO:
	)
}

func decodeTX(interfaceRegistry codectypes.InterfaceRegistry, txbz []byte) (sdk.Tx, error) {
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.DefaultJSONTxDecoder(cdc)(txbz)
}

func encodeTxToJSON(interfaceRegistry codectypes.InterfaceRegistry, tx sdk.Tx) ([]byte, error) {
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.DefaultJSONTxEncoder(cdc)(tx)
}
