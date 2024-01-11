package ethereum

import (
	"context"
	"runtime"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func PanicFunctionName() {
	pc, _, _, _ := runtime.Caller(1)
	panic(runtime.FuncForPC(pc).Name() + " not implemented")
}

func (c *EthereumChain) ExportState(ctx context.Context, height int64) (string, error) {
	PanicFunctionName()
	return "", nil
}

func (c *EthereumChain) GetGRPCAddress() string {
	PanicFunctionName()
	return ""
}

func (c *EthereumChain) GetHostGRPCAddress() string {
	PanicFunctionName()
	return ""
}

func (*EthereumChain) GetHostPeerAddress() string {
	PanicFunctionName()
	return ""
}

// cast wallet import requires a password prompt which docker isn't properly handling. For now, we only use CreateKey().
func (c *EthereumChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	/*cmd := []string{"cast", "wallet", "import", keyName, "--mnemonic", mnemonic, "--password", ""}
	stdout, stderr, err := c.Exec(ctx, cmd, nil)
	fmt.Println("stdout: ", string(stdout))
	fmt.Println("stderr: ", string(stderr))
	if err != nil {
		return err
	}*/
	PanicFunctionName()
	return nil
}

func (c *EthereumChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	PanicFunctionName()
	return 0
}

func (c *EthereumChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	PanicFunctionName()
	return ibc.Tx{}, nil
}

func (c *EthereumChain) Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error) {
	PanicFunctionName()
	return nil, nil
}

func (c *EthereumChain) Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error) {
	PanicFunctionName()
	return nil, nil
}

func (c *EthereumChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	PanicFunctionName()
	return nil, nil
}
