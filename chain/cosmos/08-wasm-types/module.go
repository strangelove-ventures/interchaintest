package types

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	//grpc "github.com/cosmos/gogoproto/grpc"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
)

var _ module.AppModule = AppModule{}

// AppModule defines the basic application module used by the tendermint light client.
// Only the RegisterInterfaces function needs to be implemented. All other function perform
// a no-op.
type AppModule struct{}

// Name returns the tendermint module name.
func (AppModule) Name() string {
	return "08-wasm"
}

// RegisterLegacyAminoCodec performs a no-op. The Wasm client does not support amino.
func (AppModule) RegisterLegacyAminoCodec(*codec.LegacyAmino) {}

// RegisterInterfaces registers module concrete types into protobuf Any. This allows core IBC
// to unmarshal wasm light client types.
func (AppModule) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	RegisterInterfaces(registry)
}

// DefaultGenesis performs a no-op. Genesis is not supported for the tendermint light client.
func (AppModule) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return nil
}

// ValidateGenesis performs a no-op. Genesis is not supported for the tendermint light client.
func (AppModule) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	return nil
}

// RegisterGRPCGatewayRoutes performs a no-op.
func (AppModule) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {}

// GetTxCmd performs a no-op. Please see the 02-client cli commands.
func (AppModule) GetTxCmd() *cobra.Command {
	return nil
}

// GetQueryCmd performs a no-op. Please see the 02-client cli commands.
func (AppModule) GetQueryCmd() *cobra.Command {
	return nil
}

// IsAppModule implements module.AppModule.
func (a AppModule) IsAppModule() {
}

// IsOnePerModuleType implements module.AppModule.
func (a AppModule) IsOnePerModuleType() {
}
