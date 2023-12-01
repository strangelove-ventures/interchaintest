package cosmos

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

// BankSend sends tokens from one account to another.
func (tn *ChainNode) BankSend(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	_, err := tn.ExecTx(ctx,
		keyName, "bank", "send", keyName,
		amount.Address, fmt.Sprintf("%s%s", amount.Amount.String(), amount.Denom),
	)
	return err
}

// BankMultiSend sends an amount of token from one account to multiple accounts.
func (tn *ChainNode) BankMultiSend(ctx context.Context, keyName string, addresses []string, amount math.Int, denom string) error {
	cmd := append([]string{"bank", "multi-send", keyName}, addresses...)
	cmd = append(cmd, fmt.Sprintf("%s%s", amount, denom))

	_, err := tn.ExecTx(ctx, keyName, cmd...)
	return err
}

// GetBalance fetches the current balance for a specific account address and denom.
// Implements Chain interface
func (c *CosmosChain) GetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return sdkmath.Int{}, err
	}

	res, err := queryClient.Balance(ctx, &banktypes.QueryBalanceRequest{Address: address, Denom: denom})
	return res.Balance.Amount, err
}

// BankGetBalance is an alias for GetBalance
func (c *CosmosChain) BankGetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	return c.GetBalance(ctx, address, denom)
}

// AllBalances fetches an account address's balance for all denoms it holds
func (c *CosmosChain) BankAllBalances(ctx context.Context, address string) (types.Coins, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.AllBalances(ctx, &banktypes.QueryAllBalancesRequest{Address: address})
	return res.GetBalances(), err
}

// BankDenomMetadata fetches the metadata of a specific coin denomination
func (c *CosmosChain) BankDenomMetadata(ctx context.Context, denom string) (*banktypes.Metadata, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.DenomMetadata(ctx, &banktypes.QueryDenomMetadataRequest{Denom: denom})
	return &res.Metadata, err
}

func (c *CosmosChain) BankQueryDenomMetadataByQueryString(ctx context.Context, denom string) (*banktypes.Metadata, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.DenomMetadataByQueryString(ctx, &banktypes.QueryDenomMetadataByQueryStringRequest{Denom: denom})
	return &res.Metadata, err
}

func (c *CosmosChain) BankQueryDenomOwners(ctx context.Context, denom string) ([]*banktypes.DenomOwner, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.DenomOwners(ctx, &banktypes.QueryDenomOwnersRequest{Denom: denom})
	return res.DenomOwners, err
}

func (c *CosmosChain) BankQueryDenomsMetadata(ctx context.Context) ([]banktypes.Metadata, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.DenomsMetadata(ctx, &banktypes.QueryDenomsMetadataRequest{})
	return res.Metadatas, err
}

func (c *CosmosChain) BankQueryParams(ctx context.Context) (*banktypes.Params, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.Params(ctx, &banktypes.QueryParamsRequest{})
	return &res.Params, err
}

func (c *CosmosChain) BankQuerySendEnabled(ctx context.Context, denoms []string) ([]*banktypes.SendEnabled, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.SendEnabled(ctx, &banktypes.QuerySendEnabledRequest{
		Denoms: denoms,
	})
	return res.SendEnabled, err
}

func (c *CosmosChain) BankQuerySpendableBalance(ctx context.Context, address, denom string) (*types.Coin, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.SpendableBalanceByDenom(ctx, &banktypes.QuerySpendableBalanceByDenomRequest{
		Address: address,
		Denom:   denom,
	})
	return res.Balance, err
}

func (c *CosmosChain) BankQuerySpendableBalances(ctx context.Context, address string) (*types.Coins, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.SpendableBalances(ctx, &banktypes.QuerySpendableBalancesRequest{Address: address})
	return &res.Balances, err
}

func (c *CosmosChain) BankQueryTotalSupply(ctx context.Context, address string) (*types.Coins, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.TotalSupply(ctx, &banktypes.QueryTotalSupplyRequest{})
	return &res.Supply, err
}

func (c *CosmosChain) BankQueryTotalSupplyOf(ctx context.Context, address string) (*types.Coin, error) {
	queryClient, err := c.newBankQueryClient()
	if err != nil {
		return nil, err
	}

	res, err := queryClient.SupplyOf(ctx, &banktypes.QuerySupplyOfRequest{Denom: address})
	return &res.Amount, err
}

func (c *CosmosChain) newBankQueryClient() (banktypes.QueryClient, error) {
	grpcAddress := c.getFullNode().hostGRPCPort
	conn, err := grpc.Dial(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return banktypes.NewQueryClient(conn), nil
}
