package ibctest

import (
	"context"
	"fmt"
	"sync"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ory/dockertest/v3"
	"github.com/strangelove-ventures/ibctest/ibc"
	"golang.org/x/sync/errgroup"
)

// chainSet is an unordered collection of ibc.Chain,
// to group methods that apply actions against all chains in the set.
//
// The main purpose of the chainSet is to unify test setup when working with any number of chains.
type chainSet map[ibc.Chain]struct{}

// Initialize concurrently calls Initialize against each chain in the set.
// Each chain may run a docker pull command,
// so with a cold image cache, running concurrently may save some time.
func (cs chainSet) Initialize(testName string, homeDir string, pool *dockertest.Pool, networkID string) error {
	var eg errgroup.Group

	for c := range cs {
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

// CreateCommonAccount creates a key with the given name on each chain in the set,
// and returns the bech32 representation of each account created.
// The typical use of CreateCommonAccount is to create a faucet account on each chain.
//
// The keys are created concurrently because creating keys on one chain
// should have no effect on any other chain.
func (cs chainSet) CreateCommonAccount(ctx context.Context, keyName string) (bech32 map[ibc.Chain]string, err error) {
	var mu sync.Mutex
	bech32 = make(map[ibc.Chain]string, len(cs))

	eg, egCtx := errgroup.WithContext(ctx)

	for c := range cs {
		c := c
		eg.Go(func() error {
			config := c.Config()

			if err := c.CreateKey(egCtx, keyName); err != nil {
				return fmt.Errorf("failed to create key with name %q on chain %s: %w", keyName, config.Name, err)
			}

			addrBytes, err := c.GetAddress(egCtx, keyName)
			if err != nil {
				return fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, config.Name, err)
			}

			b32, err := types.Bech32ifyAddressBytes(config.Bech32Prefix, addrBytes)
			if err != nil {
				return fmt.Errorf("failed to Bech32ifyAddressBytes on chain %s: %w", config.Name, err)
			}

			mu.Lock()
			bech32[c] = b32
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("failed to create common account with name %s: %w", keyName, err)
	}

	return bech32, nil
}

// Start concurrently calls Start against each chain in the set.
func (cs chainSet) Start(ctx context.Context, testName string, additionalGenesisWallets map[ibc.Chain][]ibc.WalletAmount) error {
	eg, egCtx := errgroup.WithContext(ctx)

	for c := range cs {
		c := c
		eg.Go(func() error {
			if err := c.Start(testName, egCtx, additionalGenesisWallets[c]...); err != nil {
				return fmt.Errorf("failed to start chain %s: %w", c.Config().Name, err)
			}

			return nil
		})
	}

	return eg.Wait()
}
