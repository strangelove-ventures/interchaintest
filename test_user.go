package ibctest

import (
	"context"
	"fmt"
	"testing"

	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/stretchr/testify/require"
)

// generateUserWallet creates a new user wallet with the given key name on the given chain.
// a user is recovered if a mnemonic is specified.
func generateUserWallet(ctx context.Context, keyName, mnemonic string, chain ibc.Chain) (*ibc.Wallet, error) {
	if mnemonic != "" {
		if err := chain.RecoverKey(ctx, keyName, mnemonic); err != nil {
			return nil, fmt.Errorf("failed to recover key on source chain: %w", err)
		}
	} else {
		if err := chain.CreateKey(ctx, keyName); err != nil {
			return nil, fmt.Errorf("failed to create key on source chain: %w", err)
		}
	}
	userAccountAddressBytes, err := chain.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get source user account address: %w", err)
	}
	user := ibc.Wallet{
		KeyName: keyName,
		Address: string(userAccountAddressBytes),
	}
	return &user, nil
}

// GetAndFundTestUserWithMnemonic restores a user using the given mnemonic
// and funds it with the native chain denom.
// The caller should wait for some blocks to complete before the funds will be accessible.
func GetAndFundTestUserWithMnemonic(
	t *testing.T,
	ctx context.Context,
	keyNamePrefix, mnemonic string,
	amount int64,
	chain ibc.Chain,
) *ibc.Wallet {
	chainCfg := chain.Config()
	keyName := fmt.Sprintf("%s-%s-%s", keyNamePrefix, chainCfg.ChainID, dockerutil.RandLowerCaseLetterString(3))
	user, err := generateUserWallet(ctx, keyName, mnemonic, chain)
	require.NoError(t, err, "failed to get source user wallet")

	err = chain.SendFunds(ctx, FaucetAccountKeyName, ibc.WalletAmount{
		Address: user.Bech32Address(chainCfg.Bech32Prefix),
		Amount:  amount,
		Denom:   chainCfg.Denom,
	})
	require.NoError(t, err, "failed to get funds from faucet")

	return user
}

// GetAndFundTestUsers generates and funds chain users with the native chain denom.
// The caller should wait for some blocks to complete before the funds will be accessible.
func GetAndFundTestUsers(
	t *testing.T,
	ctx context.Context,
	keyNamePrefix string,
	amount int64,
	chains ...ibc.Chain,
) []*ibc.Wallet {
	var users []*ibc.Wallet
	for _, chain := range chains {
		user := GetAndFundTestUserWithMnemonic(t, ctx, keyNamePrefix, "", amount, chain)
		users = append(users, user)
	}

	// TODO(nix 05-17-2022): Map with generics once using go 1.18
	chainHeights := make([]test.ChainHeighter, len(chains))
	for i := range chains {
		chainHeights[i] = chains[i]
	}
	return users
}
