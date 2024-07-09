package avalanche

import (
	"context"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

type XChainClient struct {
}

func NewXChainClient(rpcHost string, pk string) (ibc.AvalancheSubnetClient, error) {
	return new(XChainClient), nil
}

func (xchain *XChainClient) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	panic("not implemented")
}

func (xchain *XChainClient) Height(ctx context.Context) (uint64, error) {
	panic("not implemented")
}

func (xchain *XChainClient) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	panic("not implemented")
}
