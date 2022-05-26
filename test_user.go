package ibctest

import (
	"context"
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/stretchr/testify/require"
)

type User struct {
	Address []byte
	KeyName string
}

func (u *User) Bech32Address(bech32Prefix string) string {
	return types.MustBech32ifyAddressBytes(bech32Prefix, u.Address)
}

// generateUserWallet creates a new user wallet with the given key name on the given chain.
func generateUserWallet(ctx context.Context, keyName string, chain ibc.Chain) (*User, error) {
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
	var users []*User
	for _, chain := range chains {
		chainCfg := chain.Config()
		keyName := fmt.Sprintf("%s-%s-%s", keyNamePrefix, chainCfg.ChainID, dockerutil.RandLowerCaseLetterString(3))
		user, err := generateUserWallet(ctx, keyName, chain)
		require.NoError(t, err, "failed to get source user wallet")

		users = append(users, user)

		err = chain.SendFunds(ctx, FaucetAccountKeyName, ibc.WalletAmount{
			Address: user.Bech32Address(chainCfg.Bech32Prefix),
			Amount:  amount,
			Denom:   chainCfg.Denom,
		})
		require.NoError(t, err, "failed to get funds from faucet")
	}

	// TODO(nix 05-17-2022): Map with generics once using go 1.18
	chainHeights := make([]test.ChainHeighter, len(chains))
	for i := range chains {
		chainHeights[i] = chains[i]
	}

	require.NoError(t, test.WaitForBlocks(ctx, 5, chainHeights...), "failed to wait for blocks")

	return users
}
