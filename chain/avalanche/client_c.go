package avalanche

import (
	"context"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

type CChainClient struct {
}

func NewCChainClient(rpcHost string, pk string) (ibc.AvalancheSubnetClient, error) {
	return new(CChainClient), nil
}

func (cchain *CChainClient) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	panic("not implemented")
}

func (cchain *CChainClient) Height(ctx context.Context) (uint64, error) {
	panic("not implemented")
}

func (cchain *CChainClient) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	panic("not implemented")
}
