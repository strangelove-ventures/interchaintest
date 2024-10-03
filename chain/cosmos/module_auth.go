package cosmos

import (
	"context"
	"fmt"

	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

// AuthQueryAccount performs a query to get the account details of the specified address.
func (c *CosmosChain) AuthQueryAccount(ctx context.Context, addr string) (*cdctypes.Any, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).Account(ctx, &authtypes.QueryAccountRequest{
		Address: addr,
	})
	return res.Account, err
}

// AuthQueryParams performs a query to get the auth module parameters.
func (c *CosmosChain) AuthQueryParams(ctx context.Context) (*authtypes.Params, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).Params(ctx, &authtypes.QueryParamsRequest{})
	return &res.Params, err
}

// AuthQueryModuleAccounts performs a query to get the account details of all the chain modules.
func (c *CosmosChain) AuthQueryModuleAccounts(ctx context.Context) ([]authtypes.ModuleAccount, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).ModuleAccounts(ctx, &authtypes.QueryModuleAccountsRequest{})

	maccs := make([]authtypes.ModuleAccount, len(res.Accounts))

	for i, acc := range res.Accounts {
		var macc authtypes.ModuleAccount
		err := c.GetCodec().Unmarshal(acc.Value, &macc)
		if err != nil {
			return nil, err
		}
		maccs[i] = macc
	}

	return maccs, err
}

// AuthGetModuleAccount performs a query to get the account details of the specified chain module.
func (c *CosmosChain) AuthQueryModuleAccount(ctx context.Context, moduleName string) (authtypes.ModuleAccount, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).ModuleAccountByName(ctx, &authtypes.QueryModuleAccountByNameRequest{
		Name: moduleName,
	})
	if err != nil {
		return authtypes.ModuleAccount{}, err
	}

	var modAcc authtypes.ModuleAccount
	err = c.GetCodec().Unmarshal(res.Account.Value, &modAcc)

	return modAcc, err
}

// GetModuleAddress performs a query to get the address of the specified chain module.
func (c *CosmosChain) AuthQueryModuleAddress(ctx context.Context, moduleName string) (string, error) {
	queryRes, err := c.AuthQueryModuleAccount(ctx, moduleName)
	if err != nil {
		return "", err
	}
	return queryRes.BaseAccount.Address, nil
}

// Deprecated: use AuthQueryModuleAddress instead.
func (c *CosmosChain) GetModuleAddress(ctx context.Context, moduleName string) (string, error) {
	return c.AuthQueryModuleAddress(ctx, moduleName)
}

// GetGovernanceAddress performs a query to get the address of the chain's x/gov module
// Deprecated: use AuthQueryModuleAddress(ctx, "gov") instead.
func (c *CosmosChain) GetGovernanceAddress(ctx context.Context) (string, error) {
	return c.GetModuleAddress(ctx, "gov")
}

func (c *CosmosChain) AuthQueryBech32Prefix(ctx context.Context) (string, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).Bech32Prefix(ctx, &authtypes.Bech32PrefixRequest{})
	return res.Bech32Prefix, err
}

// AddressBytesToString converts a byte array address to a string.
func (c *CosmosChain) AuthAddressBytesToString(ctx context.Context, addrBz []byte) (string, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).AddressBytesToString(ctx, &authtypes.AddressBytesToStringRequest{
		AddressBytes: addrBz,
	})
	return res.AddressString, err
}

// AddressStringToBytes converts a string address to a byte array.
func (c *CosmosChain) AuthAddressStringToBytes(ctx context.Context, addr string) ([]byte, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).AddressStringToBytes(ctx, &authtypes.AddressStringToBytesRequest{
		AddressString: addr,
	})
	return res.AddressBytes, err
}

// AccountInfo queries the account information of the given address.
func (c *CosmosChain) AuthQueryAccountInfo(ctx context.Context, addr string) (*authtypes.BaseAccount, error) {
	res, err := authtypes.NewQueryClient(c.GetNode().GrpcConn).AccountInfo(ctx, &authtypes.QueryAccountInfoRequest{
		Address: addr,
	})
	return res.Info, err
}

func (c *CosmosChain) AuthPrintAccountInfo(chain *CosmosChain, res *cdctypes.Any) error {
	switch res.TypeUrl {
	case "/cosmos.auth.v1beta1.ModuleAccount":
		var modAcc authtypes.ModuleAccount
		if err := chain.GetCodec().Unmarshal(res.Value, &modAcc); err != nil {
			return err
		}
		fmt.Printf("ModuleAccount: %+v\n", modAcc)
		return nil

	case "/cosmos.vesting.v1beta1.VestingAccount":
		var vestingAcc vestingtypes.BaseVestingAccount
		if err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc); err != nil {
			return err
		}
		fmt.Printf("BaseVestingAccount: %+v\n", vestingAcc)
		return nil

	case "/cosmos.vesting.v1beta1.PeriodicVestingAccount":
		var vestingAcc vestingtypes.PeriodicVestingAccount
		if err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc); err != nil {
			return err
		}
		fmt.Printf("PeriodicVestingAccount: %+v\n", vestingAcc)
		return nil

	case "/cosmos.vesting.v1beta1.ContinuousVestingAccount":
		var vestingAcc vestingtypes.ContinuousVestingAccount
		if err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc); err != nil {
			return err
		}
		fmt.Printf("ContinuousVestingAccount: %+v\n", vestingAcc)
		return nil

	case "/cosmos.vesting.v1beta1.DelayedVestingAccount":
		var vestingAcc vestingtypes.DelayedVestingAccount
		if err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc); err != nil {
			return err
		}
		fmt.Printf("DelayedVestingAccount: %+v\n", vestingAcc)
		return nil

	case "/cosmos.vesting.v1beta1.PermanentLockedAccount":
		var vestingAcc vestingtypes.PermanentLockedAccount
		if err := chain.GetCodec().Unmarshal(res.Value, &vestingAcc); err != nil {
			return err
		}
		fmt.Printf("PermanentLockedAccount: %+v\n", vestingAcc)
		return nil

	default:
		var baseAcc authtypes.BaseAccount
		if err := chain.GetCodec().Unmarshal(res.Value, &baseAcc); err != nil {
			return err
		}
		fmt.Printf("BaseAccount: %+v\n", baseAcc)
		return nil
	}
}
