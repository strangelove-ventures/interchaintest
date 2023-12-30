package cosmos

import (
	"context"
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"

	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
)

func (c *CosmosChain) AuthGetAccount(ctx context.Context, addr string) (*cdctypes.Any, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).Account(ctx, &authtypes.QueryAccountRequest{
		Address: addr,
	})
	return res.Account, err
}

func (c *CosmosChain) AuthParams(ctx context.Context, addr string) (*authtypes.Params, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).Params(ctx, &authtypes.QueryParamsRequest{})
	return &res.Params, err
}

func (c *CosmosChain) AuthModuleAccounts(ctx context.Context, addr string) ([]*cdctypes.Any, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).ModuleAccounts(ctx, &authtypes.QueryModuleAccountsRequest{})
	return res.Accounts, err
}

// AuthGetModuleAccount performs a query to get the account details of the specified chain module
func (c *CosmosChain) AuthGetModuleAccount(ctx context.Context, moduleName string) (authtypes.ModuleAccount, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).ModuleAccountByName(ctx, &authtypes.QueryModuleAccountByNameRequest{
		Name: moduleName,
	})
	if err != nil {
		return authtypes.ModuleAccount{}, err
	}

	if res.Account.TypeUrl == "/cosmos.auth.v1beta1.ModuleAccount" {
		var modAcc authtypes.ModuleAccount
		err := c.GetCodec().Unmarshal(res.Account.Value, &modAcc)
		fmt.Printf("modAcc: %+v\n", modAcc)

		return modAcc, err
	}

	return authtypes.ModuleAccount{}, fmt.Errorf("invalid module account type: %s", res.Account.TypeUrl)
}

// GetModuleAddress performs a query to get the address of the specified chain module
func (c *CosmosChain) AuthGetModuleAddress(ctx context.Context, moduleName string) (string, error) {
	queryRes, err := c.AuthGetModuleAccount(ctx, moduleName)
	if err != nil {
		return "", err
	}
	return queryRes.BaseAccount.Address, nil
}

// GetModuleAddress is an alias for AuthGetModuleAddress
func (c *CosmosChain) GetModuleAddress(ctx context.Context, moduleName string) (string, error) {
	return c.AuthGetModuleAddress(ctx, moduleName)
}

// GetGovernanceAddress performs a query to get the address of the chain's x/gov module
func (c *CosmosChain) GetGovernanceAddress(ctx context.Context) (string, error) {
	return c.GetModuleAddress(ctx, "gov")
}

func (c *CosmosChain) AuthBech32Prefix(ctx context.Context) (string, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).Bech32Prefix(ctx, &authtypes.Bech32PrefixRequest{})
	return res.Bech32Prefix, err
}

// AddressBytesToString
func (c *CosmosChain) AuthAddressBytesToString(ctx context.Context, addrBz []byte) (string, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).AddressBytesToString(ctx, &authtypes.AddressBytesToStringRequest{
		AddressBytes: addrBz,
	})
	return res.AddressString, err
}

// AddressStringToBytes
func (c *CosmosChain) AuthAddressStringToBytes(ctx context.Context, addr string) ([]byte, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).AddressStringToBytes(ctx, &authtypes.AddressStringToBytesRequest{
		AddressString: addr,
	})
	return res.AddressBytes, err
}

// AccountInfo
func (c *CosmosChain) AuthAccountInfo(ctx context.Context, addr string) (*authtypes.BaseAccount, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).AccountInfo(ctx, &authtypes.QueryAccountInfoRequest{
		Address: addr,
	})
	return res.Info, err
}

func (c *CosmosChain) AuthPrintAccountInfo(chain *CosmosChain, res *codectypes.Any) error {
	switch res.TypeUrl {
	case "/cosmos.auth.v1beta1.ModuleAccount":
		var modAcc authtypes.ModuleAccount
		err := chain.GetCodec().Unmarshal(res.Value, &modAcc)
		fmt.Printf("modAcc: %+v\n", modAcc)
		return err

	case "/cosmos.vesting.v1beta1.VestingAccount":
		var vestingAcc vestingtypes.BaseVestingAccount
		err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc)
		fmt.Printf("BaseVestingAccount: %+v\n", vestingAcc)
		return err

	case "/cosmos.vesting.v1beta1.PeriodicVestingAccount":
		var vestingAcc vestingtypes.PeriodicVestingAccount
		err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc)
		fmt.Printf("PeriodicVestingAccount: %+v\n", vestingAcc)
		return err

	case "/cosmos.vesting.v1beta1.ContinuousVestingAccount":
		var vestingAcc vestingtypes.ContinuousVestingAccount
		err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc)
		fmt.Printf("ContinuousVestingAccount: %+v\n", vestingAcc)
		return err

	case "/cosmos.vesting.v1beta1.DelayedVestingAccount":
		var vestingAcc vestingtypes.DelayedVestingAccount
		err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc)
		fmt.Printf("DelayedVestingAccount: %+v\n", vestingAcc)
		return err

	case "/cosmos.vesting.v1beta1.PermanentLockedAccount":
		var vestingAcc vestingtypes.PermanentLockedAccount
		err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc)
		fmt.Printf("PermanentLockedAccount: %+v\n", vestingAcc)
		return err

	default:
		var baseAcc authtypes.BaseAccount
		err := chain.GetCodec().Unmarshal(res.Value, &baseAcc)
		fmt.Printf("baseAcc: %+v\n", baseAcc)
		return err
	}
}
