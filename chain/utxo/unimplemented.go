package utxo

import (
	"context"
	"runtime"

	"github.com/strangelove-ventures/interchaintest/v9/ibc"
)

func PanicFunctionName() {
	pc, _, _, _ := runtime.Caller(1)
	panic(runtime.FuncForPC(pc).Name() + " not implemented")
}

func (c *UtxoChain) ExportState(ctx context.Context, height int64) (string, error) {
	PanicFunctionName()
	return "", nil
}

func (c *UtxoChain) GetGRPCAddress() string {
	PanicFunctionName()
	return ""
}

func (c *UtxoChain) GetHostGRPCAddress() string {
	PanicFunctionName()
	return ""
}

func (*UtxoChain) GetHostPeerAddress() string {
	PanicFunctionName()
	return ""
}

func (c *UtxoChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	PanicFunctionName()
	return 0
}

func (c *UtxoChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	PanicFunctionName()
	return nil
}

func (c *UtxoChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	PanicFunctionName()
	return ibc.Tx{}, nil
}

func (c *UtxoChain) Acknowledgements(ctx context.Context, height int64) ([]ibc.PacketAcknowledgement, error) {
	PanicFunctionName()
	return nil, nil
}

func (c *UtxoChain) Timeouts(ctx context.Context, height int64) ([]ibc.PacketTimeout, error) {
	PanicFunctionName()
	return nil, nil
}

func (c *UtxoChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	PanicFunctionName()
	return &UtxoWallet{}, nil
}
