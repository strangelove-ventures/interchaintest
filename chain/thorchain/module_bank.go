package thorchain

import (
	"context"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/strangelove-ventures/interchaintest/v9/ibc"
)

// Deprecated: use BankSend instead.
func (tn *ChainNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	return tn.BankSend(ctx, keyName, amount)
}

// GetBalance fetches the current balance for a specific account address and denom.
// Implements Chain interface.
func (c *Thorchain) GetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	return c.BankQueryBalance(ctx, address, denom)
}

// BankGetBalance is an alias for GetBalance.
func (c *Thorchain) BankQueryBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).Balance(ctx, &banktypes.QueryBalanceRequest{Address: address, Denom: denom})
	if err != nil {
		return sdkmath.ZeroInt(), err
	}
	return res.Balance.Amount, nil
}

// AllBalances fetches an account address's balance for all denoms it holds.
func (c *Thorchain) BankQueryAllBalances(ctx context.Context, address string) (types.Coins, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).AllBalances(ctx, &banktypes.QueryAllBalancesRequest{Address: address})
	if err != nil {
		return nil, err
	}
	return res.GetBalances(), nil
}

// BankDenomMetadata fetches the metadata of a specific coin denomination.
func (c *Thorchain) BankQueryDenomMetadata(ctx context.Context, denom string) (*banktypes.Metadata, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).DenomMetadata(ctx, &banktypes.QueryDenomMetadataRequest{Denom: denom})
	if err != nil {
		return nil, err
	}
	return &res.Metadata, nil
}

func (c *Thorchain) BankQueryDenomMetadataByQueryString(ctx context.Context, denom string) (*banktypes.Metadata, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).DenomMetadataByQueryString(ctx, &banktypes.QueryDenomMetadataByQueryStringRequest{Denom: denom})
	if err != nil {
		return nil, err
	}
	return &res.Metadata, nil
}

func (c *Thorchain) BankQueryDenomOwners(ctx context.Context, denom string) ([]*banktypes.DenomOwner, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).DenomOwners(ctx, &banktypes.QueryDenomOwnersRequest{Denom: denom})
	if err != nil {
		return nil, err
	}
	return res.DenomOwners, nil
}

func (c *Thorchain) BankQueryDenomsMetadata(ctx context.Context) ([]banktypes.Metadata, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).DenomsMetadata(ctx, &banktypes.QueryDenomsMetadataRequest{})
	if err != nil {
		return nil, err
	}
	return res.Metadatas, nil
}

func (c *Thorchain) BankQueryParams(ctx context.Context) (*banktypes.Params, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).Params(ctx, &banktypes.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}
	return &res.Params, nil
}

func (c *Thorchain) BankQuerySendEnabled(ctx context.Context, denoms []string) ([]*banktypes.SendEnabled, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).SendEnabled(ctx, &banktypes.QuerySendEnabledRequest{
		Denoms: denoms,
	})
	if err != nil {
		return nil, err
	}
	return res.SendEnabled, nil
}

func (c *Thorchain) BankQuerySpendableBalance(ctx context.Context, address, denom string) (*types.Coin, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).SpendableBalanceByDenom(ctx, &banktypes.QuerySpendableBalanceByDenomRequest{
		Address: address,
		Denom:   denom,
	})
	if err != nil {
		return nil, err
	}
	return res.Balance, nil
}

func (c *Thorchain) BankQuerySpendableBalances(ctx context.Context, address string) (*types.Coins, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).SpendableBalances(ctx, &banktypes.QuerySpendableBalancesRequest{Address: address})
	if err != nil {
		return nil, err
	}
	return &res.Balances, nil
}

func (c *Thorchain) BankQueryTotalSupply(ctx context.Context) (*types.Coins, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).TotalSupply(ctx, &banktypes.QueryTotalSupplyRequest{})
	if err != nil {
		return nil, err
	}
	return &res.Supply, nil
}

func (c *Thorchain) BankQueryTotalSupplyOf(ctx context.Context, address string) (*types.Coin, error) {
	res, err := banktypes.NewQueryClient(c.GetNode().GrpcConn).SupplyOf(ctx, &banktypes.QuerySupplyOfRequest{Denom: address})
	if err != nil {
		return nil, err
	}
	return &res.Amount, nil
}
