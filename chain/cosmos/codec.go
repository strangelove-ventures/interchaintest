package cosmos

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
)

func DefaultEncoding() testutil.TestEncodingConfig {
	return testutil.TestEncodingConfig{}
	//return testutil.MakeTestEncodingConfig(
	//	auth.AppModuleBasic{},
	//	genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
	//	bank.AppModuleBasic{},
	//	staking.AppModuleBasic{},
	//	mint.AppModuleBasic{},
	//	distr.AppModuleBasic{},
	//	gov.NewAppModuleBasic(
	//		[]govclient.ProposalHandler{
	//			paramsclient.ProposalHandler,
	//		},
	//	),
	//	params.AppModuleBasic{},
	//	slashing.AppModuleBasic{},
	//	upgrade.AppModuleBasic{},
	//	consensus.AppModuleBasic{},
	//	transfer.AppModuleBasic{},
	//	ibccore.AppModuleBasic{},
	//	ibctm.AppModuleBasic{},
	//	ibcwasm.AppModuleBasic{},
	//)
}

func decodeTX(interfaceRegistry codectypes.InterfaceRegistry, txbz []byte) (sdk.Tx, error) {
	panic("foo")
	//cdc := codec.NewProtoCodec(interfaceRegistry)
	//return authTx.DefaultTxDecoder(cdc)(txbz)
}

func encodeTxToJSON(interfaceRegistry codectypes.InterfaceRegistry, tx sdk.Tx) ([]byte, error) {
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.DefaultJSONTxEncoder(cdc)(tx)
}
