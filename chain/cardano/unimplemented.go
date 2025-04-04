package cardano

import (
	"context"
	"errors"
	"runtime"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func errNotImplemented() error {
	pc, _, _, _ := runtime.Caller(1)
	return errors.New(runtime.FuncForPC(pc).Name() + " not implemented")
}

func (a *AdaChain) ExportState(ctx context.Context, height int64) (string, error) {
	panic(errNotImplemented())
}

func (a *AdaChain) GetGRPCAddress() string {
	panic(errNotImplemented())
}

func (a *AdaChain) GetHostGRPCAddress() string {
	panic(errNotImplemented())
}

func (a *AdaChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	panic(errNotImplemented())
}

func (a *AdaChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	panic(errNotImplemented())
}

func (a *AdaChain) Acknowledgements(ctx context.Context, height int64) ([]ibc.PacketAcknowledgement, error) {
	panic(errNotImplemented())
}

func (a *AdaChain) Timeouts(ctx context.Context, height int64) ([]ibc.PacketTimeout, error) {
	panic(errNotImplemented())
}
