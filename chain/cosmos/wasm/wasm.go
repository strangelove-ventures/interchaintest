package wasm

import (
	wasmtypes "github.com/ODIN-PROTOCOL/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"

	// simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
)

func WasmEncoding() *testutil.TestEncodingConfig {
	cfg := cosmos.DefaultEncoding()

	wasmtypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}
