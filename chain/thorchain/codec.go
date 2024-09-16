package thorchain

import (
	"cosmossdk.io/x/bank"
	"cosmossdk.io/x/consensus"
	distr "cosmossdk.io/x/distribution"
	"cosmossdk.io/x/mint"
	"cosmossdk.io/x/params"
	"cosmossdk.io/x/staking"
	"cosmossdk.io/x/upgrade"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/genutil"

	transfer "github.com/cosmos/ibc-go/v9/modules/apps/transfer"
	ibccore "github.com/cosmos/ibc-go/v9/modules/core"
	ibctm "github.com/cosmos/ibc-go/v9/modules/light-clients/07-tendermint"

	codectestutil "github.com/cosmos/cosmos-sdk/codec/testutil"
)

// TODO: ref sdk for this, does this work?
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
		params.AppModule{},
		upgrade.AppModule{},
		consensus.AppModule{},
		transfer.AppModule{},
		ibccore.AppModule{},
		ibctm.AppModule{},
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
