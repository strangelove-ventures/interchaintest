package ibctest

import (
	"context"
	"fmt"
	"github.com/strangelove-ventures/ibctest/v6/dockerutil"
	"testing"

	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
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
	ctx context.Context,
	keyNamePrefix, mnemonic string,
	amount int64,
	chain ibc.Chain,
) (*ibc.Wallet, error) {
	chainCfg := chain.Config()
	keyName := fmt.Sprintf("%s-%s-%s", keyNamePrefix, chainCfg.ChainID, dockerutil.RandLowerCaseLetterString(3))
	user, err := generateUserWallet(ctx, keyName, mnemonic, chain)
	if err != nil {
		return nil, fmt.Errorf("failed to get source user wallet: %w", err)
	}

	err = chain.SendFunds(ctx, FaucetAccountKeyName, ibc.WalletAmount{
		Address: user.Bech32Address(chainCfg.Bech32Prefix),
		Amount:  amount,
		Denom:   chainCfg.Denom,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get funds from faucet: %w", err)
	}
	return user, nil
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
	users := make([]*ibc.Wallet, len(chains))
	var eg errgroup.Group
	for i, chain := range chains {
		i := i
		chain := chain
		eg.Go(func() error {
			user, err := GetAndFundTestUserWithMnemonic(ctx, keyNamePrefix, "", amount, chain)
			if err != nil {
				return err
			}
			users[i] = user
			return nil
		})
	}
	require.NoError(t, eg.Wait())

	// TODO(nix 05-17-2022): Map with generics once using go 1.18
	chainHeights := make([]testutil.ChainHeighter, len(chains))
	for i := range chains {
		chainHeights[i] = chains[i]
	}
	return users
}
