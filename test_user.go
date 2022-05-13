package ibctest

import (
	"context"
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/ibc-test-framework/dockerutil"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/stretchr/testify/require"
)

type User struct {
	Address []byte
	KeyName string
}

func (u *User) Bech32Address(bech32Prefix string) string {
	return types.MustBech32ifyAddressBytes(bech32Prefix, u.Address)
}

// generate user wallet on chain
func getUserWallet(ctx context.Context, keyName string, chain ibc.Chain) (*User, error) {
	if err := chain.CreateKey(ctx, keyName); err != nil {
		return nil, fmt.Errorf("failed to create key on source chain: %w", err)
	}
	userAccountAddressBytes, err := chain.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get source user account address: %w", err)
	}
	user := User{
		KeyName: keyName,
		Address: userAccountAddressBytes,
	}
	return &user, nil
}

// generate and fund chain users
func GetAndFundTestUsers(
	t *testing.T,
	ctx context.Context,
	keyNamePrefix string,
	amount int64,
	chains ...ibc.Chain,
) []*User {
	users := []*User{}
	for _, chain := range chains {
		chainCfg := chain.Config()
		keyName := fmt.Sprintf("%s-%s-%s", keyNamePrefix, chainCfg.ChainID, dockerutil.RandLowerCaseLetterString(3))
		user, err := getUserWallet(ctx, keyName, chain)
		require.NoError(t, err, "failed to get source user wallet")

		users = append(users, user)

		err = GetFundsFromFaucet(chain, ctx, ibc.WalletAmount{
			Address: user.Bech32Address(chainCfg.Bech32Prefix),
			Amount:  amount,
			Denom:   chainCfg.Denom,
		})
		require.NoError(t, err, "failed to get funds from faucet")
	}

	require.NoError(t, WaitForBlocks(5, chains...), "failed to wait for blocks")

	return users
}

// get funds from faucet if it was initialized in genesis
func GetFundsFromFaucet(chain ibc.Chain, ctx context.Context, amount ibc.WalletAmount) error {
	return chain.SendFunds(ctx, faucetAccountKeyName, amount)
}
