package ibctest

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ory/dockertest/v3"
	"github.com/strangelove-ventures/ibctest/ibc"
	"golang.org/x/sync/errgroup"
)

// chainSet is an ordered collection of ibc.Chain,
// to group methods that apply actions against all chains in the set.
//
// The main purpose of the chainSet is to unify test setup when working with any number of chains.
type chainSet []ibc.Chain

// Initialize concurrently calls Initialize against each chain in the set.
// Each chain may run a docker pull command,
// so with a cold image cache, running concurrently may save some time.
func (cs chainSet) Initialize(testName string, homeDir string, pool *dockertest.Pool, networkID string) error {
	var eg errgroup.Group

	for _, c := range cs {
		c := c
		eg.Go(func() error {
			if err := c.Initialize(testName, homeDir, pool, networkID); err != nil {
				return fmt.Errorf("failed to initialize chain %s: %w", c.Config().Name, err)
			}

			return nil
		})
	}

	return eg.Wait()
}

// CreateKeys creates an ephemeral key for each chain in the set,
// returning the appropriate bech32 version of the key
// and the mnemonic behind it.
func (cs chainSet) CreateKeys() (bech32Addresses, mnemonics []string, err error) {
	bech32Addresses = make([]string, len(cs))
	mnemonics = make([]string, len(cs))

	kr := keyring.NewInMemory()

	// NOTE: this is hardcoded to the cosmos coin type.
	// In the future, we may need to get the coin type from the chain config.
	const coinType = types.CoinType

	for i, c := range cs {
		// The account name doesn't matter because the keyring is ephemeral,
		// but within the keyring's lifecycle, the name must be unique.
		accountName := fmt.Sprintf("acct-%d", i)

		info, mnemonic, err := kr.NewMnemonic(
			accountName,
			keyring.English,
			hd.CreateHDPath(coinType, 0, 0).String(),
			"", // Empty passphrase.
			hd.Secp256k1,
		)
		if err != nil {
			return nil, nil, err
		}

		bech32Addresses[i] = types.MustBech32ifyAddressBytes(c.Config().Bech32Prefix, info.GetAddress().Bytes())
		mnemonics[i] = mnemonic
	}

	return bech32Addresses, mnemonics, nil
}

// CreateCommonAccount creates a key with the given name on each chain in the set,
// and returns the bech32 representation of each account created.
//
// The keys are created concurrently because creating keys on one chain
// should have no effect on any other chain.
func (cs chainSet) CreateCommonAccount(ctx context.Context, keyName string) (bech32 []string, err error) {
	bech32 = make([]string, len(cs))

	eg, egCtx := errgroup.WithContext(ctx)

	for i, c := range cs {
		i, c := i, c
		eg.Go(func() error {
			config := c.Config()

			if err := c.CreateKey(egCtx, keyName); err != nil {
				return fmt.Errorf("failed to create key with name %q on chain %s: %w", keyName, config.Name, err)
			}

			addrBytes, err := c.GetAddress(egCtx, keyName)
			if err != nil {
				return fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, config.Name, err)
			}

			bech32[i], err = types.Bech32ifyAddressBytes(config.Bech32Prefix, addrBytes)
			if err != nil {
				return fmt.Errorf("failed to Bech32ifyAddressBytes on chain %s: %w", config.Name, err)
			}

			return nil
		})
	}

	return bech32, eg.Wait()
}

// Start concurrently calls Start against each chain in the set.
func (cs chainSet) Start(ctx context.Context, testName string, additionalGenesisWallets [][]ibc.WalletAmount) error {
	if len(additionalGenesisWallets) != len(cs) {
		panic(fmt.Errorf(
			"chainSet.Start called with %d additional set(s) of wallets; expected %d to match number of chains",
			len(additionalGenesisWallets), len(cs),
		))
	}

	eg, egCtx := errgroup.WithContext(ctx)

	for i, c := range cs {
		i, c := i, c
		eg.Go(func() error {
			if err := c.Start(testName, egCtx, additionalGenesisWallets[i]...); err != nil {
				return fmt.Errorf("failed to start chain %s: %w", c.Config().Name, err)
			}

			return nil
		})
	}

	return eg.Wait()
}
