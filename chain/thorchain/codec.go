package thorchain

import (
	"cosmossdk.io/x/upgrade"

	"github.com/cosmos/ibc-go/modules/capability"
	transfer "github.com/cosmos/ibc-go/v10/modules/apps/transfer"
	ibccore "github.com/cosmos/ibc-go/v10/modules/core"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/staking"
)

func DefaultEncoding() testutil.TestEncodingConfig {
	return testutil.MakeTestEncodingConfig(
		auth.AppModuleBasic{},
		genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distr.AppModuleBasic{},
		params.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		consensus.AppModuleBasic{},
		transfer.AppModuleBasic{},
		ibccore.AppModuleBasic{},
		ibctm.AppModuleBasic{},
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
