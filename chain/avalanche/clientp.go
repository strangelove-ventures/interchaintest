package avalanche

import (
	"context"

	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

type PChainClient struct {
}

func NewPChainClient(rpcHost string, pk string) (ibc.AvalancheSubnetClient, error) {
	return new(PChainClient), nil
}

func (pchain *PChainClient) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	panic("not implemented")
}

func (pchain *PChainClient) Height(ctx context.Context) (uint64, error) {
	//platformvm.NewClient(fmt.Sprintf("http://127.0.0.1:%s", n.RPCPort())).GetHeight(ctx)
	panic("not implemented")
}

func (pchain *PChainClient) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	panic("not implemented")
}
