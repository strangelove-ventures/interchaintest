package ibctest

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ory/dockertest/v3"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/testreporter"
)

// World represents a full IBC network, encompassing a collection of
// one or more chains, one or more relayer instances, and initial account configuration.
type World struct {
	chains   map[ibc.Chain]string
	relayers map[ibc.Relayer]string

	// Key: relayer and path name; Value: the two chains being linked.
	links map[relayerPath][2]ibc.Chain
}

// NewWorld returns a new World.
func NewWorld() *World {
	return &World{
		chains:   make(map[ibc.Chain]string),
		relayers: make(map[ibc.Relayer]string),

		links: make(map[relayerPath][2]ibc.Chain),
	}
}

// relayerPath is a tuple of a relayer and a path name.
type relayerPath struct {
	Relayer ibc.Relayer
	Path    string
}

// AddChain adds the given chain with the given name to the World.
// If the given chain or name already exists, AddChain panics.
func (w *World) AddChain(chain ibc.Chain, name string) *World {
	for c, n := range w.chains {
		if c == chain {
			panic(fmt.Errorf("chain %v was already added", c))
		}
		if n == name {
			panic(fmt.Errorf("a chain with name %s already exists", n))
		}
	}

	w.chains[chain] = name
	return w
}

// AddRelayer adds the given relayer with the given name to the World.
func (w *World) AddRelayer(relayer ibc.Relayer, name string) *World {
	for r, n := range w.relayers {
		if r == relayer {
			panic(fmt.Errorf("relayer %v was already added", r))
		}
		if n == name {
			panic(fmt.Errorf("a relayer with name %s already exists", n))
		}
	}

	w.relayers[relayer] = name
	return w
}

// WorldLink describes a link between two chains,
// by specifying the chain names, the relayer name,
// and the name of the path to create.
type WorldLink struct {
	// Chains involved.
	Chain1, Chain2 ibc.Chain

	// Relayer to use for link.
	Relayer ibc.Relayer

	// Name of path to create.
	Path string
}

// AddLink adds the given link to the World.
// If any validation fails, AddLink panics.
func (w *World) AddLink(link WorldLink) *World {
	if _, exists := w.chains[link.Chain1]; !exists {
		panic(fmt.Errorf("chain %v was never added to World", link.Chain1))
	}
	if _, exists := w.chains[link.Chain2]; !exists {
		panic(fmt.Errorf("chain %v was never added to World", link.Chain2))
	}
	if _, exists := w.relayers[link.Relayer]; !exists {
		panic(fmt.Errorf("relayer %v was never added to World", link.Relayer))
	}

	if link.Chain1 == link.Chain2 {
		panic(fmt.Errorf("chains must be different (both were %v)", link.Chain1))
	}

	key := relayerPath{
		Relayer: link.Relayer,
		Path:    link.Path,
	}

	if _, exists := w.links[key]; exists {
		panic(fmt.Errorf("relayer %q already has a path named %q", key.Relayer, key.Path))
	}

	w.links[key] = [2]ibc.Chain{link.Chain1, link.Chain2}
	return w
}

// WorldBuildOptions describes configuration for (*World).Build.
type WorldBuildOptions struct {
	TestName string
	HomeDir  string

	Pool      *dockertest.Pool
	NetworkID string
}

// Build creates the defined world.
func (w *World) Build(ctx context.Context, rep *testreporter.RelayerExecReporter, opts WorldBuildOptions) (*WorldResult, error) {
	// Collect the set of relayer-chain mappings.
	relayerChains := w.relayerChains()
	res := new(WorldResult)
	res.generateWallets(relayerChains, w.chains, w.relayers)

	cs := make(chainSet, len(w.chains))
	for c := range w.chains {
		cs[c] = struct{}{}
	}

	// Initialize the chains (pull docker images, etc.).
	if err := cs.Initialize(opts.TestName, opts.HomeDir, opts.Pool, opts.NetworkID); err != nil {
		return nil, fmt.Errorf("failed to initialize chains: %w", err)
	}

	// Faucet addresses are created separately because they need to be explicitly added to the chains.
	faucetAddresses, err := cs.CreateCommonAccount(ctx, faucetAccountKeyName)
	if err != nil {
		return nil, fmt.Errorf("failed to create faucet accounts: %w", err)
	}

	// Wallet amounts for genesis.
	walletAmounts := make(map[ibc.Chain][]ibc.WalletAmount, len(cs))

	// Add faucet for each chain first.
	for c := range w.chains {
		// The values are nil at this point, so it is safe to directly assign the slice.
		walletAmounts[c] = []ibc.WalletAmount{
			{
				Address: faucetAddresses[c],
				Denom:   c.Config().Denom,
				Amount:  10_000_000_000_000, // Faucet wallet gets 10b units of denom.
			},
		}
	}

	// Then add all defined relayer wallets.
	for c, wallets := range res.relayerWalletsByChain() {
		for _, w := range wallets {
			// The wallets already exist because every chain has a faucet, so append relayer wallets.
			walletAmounts[c] = append(walletAmounts[c], ibc.WalletAmount{
				Address: w.Address,
				Denom:   c.Config().Denom,
				Amount:  1_000_000_000_000, // Every wallet gets 1b units of denom.
			})
		}
	}

	if err := cs.Start(ctx, opts.TestName, walletAmounts); err != nil {
		return nil, fmt.Errorf("failed to start chains: %w", err)
	}

	// Every relayer needs configured to be aware of its chains.
	for r, chains := range w.relayerChains() {
		for _, c := range chains {
			rpcAddr, grpcAddr := c.GetRPCAddress(), c.GetGRPCAddress()
			// TODO: handle relayer outside of Docker
			// (the UseDockerNetwork() method is on the factory, not the relayer).

			chainName := w.chains[c]
			if err := r.AddChainConfiguration(ctx,
				rep,
				c.Config(), chainName,
				rpcAddr, grpcAddr,
			); err != nil {
				return nil, fmt.Errorf("failed to configure relayer %s for chain %s: %w", w.relayers[r], chainName, err)
			}

			if err := r.RestoreKey(ctx,
				rep,
				c.Config().ChainID, chainName,
				res.RelayerWallets[RelayerChain{R: r, C: c}].Mnemonic,
			); err != nil {
				return nil, fmt.Errorf("failed to restore key to relayer %s for chain %s: %w", w.relayers[r], chainName, err)
			}
		}
	}

	// For every relayer link, teach the relayer about the link and create the link.
	for rp, chains := range w.links {
		c0 := chains[0]
		c1 := chains[1]
		if err := rp.Relayer.GeneratePath(ctx, rep, c0.Config().ChainID, c1.Config().ChainID, rp.Path); err != nil {
			return nil, fmt.Errorf(
				"failed to generate path %s on relayer %s between chains %s and %s: %w",
				rp.Path, rp.Relayer, chains[0], chains[1], err,
			)
		}

		if err := rp.Relayer.LinkPath(ctx, rep, rp.Path); err != nil {
			return nil, fmt.Errorf(
				"failed to link path %s on relayer %s between chains %s and %s: %w",
				rp.Path, rp.Relayer, chains[0], chains[1], err,
			)
		}
	}

	return res, nil
}

// WorldResult describes the addresses and mnemonics
// of the relayer wallets created during (*World).Build.
type WorldResult struct {
	RelayerWallets map[RelayerChain]ibc.RelayerWallet
}

// RelayerChain is a tuple of a Relayer and a Chain.
type RelayerChain struct {
	R ibc.Relayer
	C ibc.Chain
}

// generateWallets builds a wallet for each relayer-chain pairing.
func (r *WorldResult) generateWallets(relayerChains map[ibc.Relayer][]ibc.Chain, chainNames map[ibc.Chain]string, relayerNames map[ibc.Relayer]string) {
	kr := keyring.NewInMemory()

	r.RelayerWallets = make(map[RelayerChain]ibc.RelayerWallet, len(relayerChains))
	for relayer, chains := range relayerChains {
		for _, c := range chains {
			// Just an ephemeral unique name.
			accountName := relayerNames[relayer] + "-" + chainNames[c]

			config := c.Config()
			r.RelayerWallets[RelayerChain{R: relayer, C: c}] = buildWallet(kr, accountName, config)
		}
	}
}

// relayerWalletsByChain returns a mapping of chain names to a slice of relayer wallets to create.
func (r *WorldResult) relayerWalletsByChain() map[ibc.Chain][]ibc.RelayerWallet {
	wallets := make(map[ibc.Chain][]ibc.RelayerWallet)

	for rc, w := range r.RelayerWallets {
		wallets[rc.C] = append(wallets[rc.C], w)
	}

	return wallets
}

func buildWallet(kr keyring.Keyring, keyName string, config ibc.ChainConfig) ibc.RelayerWallet {
	// NOTE: this is hardcoded to the cosmos coin type.
	// In the future, we may need to get the coin type from the chain config.
	const coinType = types.CoinType

	info, mnemonic, err := kr.NewMnemonic(
		keyName,
		keyring.English,
		hd.CreateHDPath(coinType, 0, 0).String(),
		"", // Empty passphrase.
		hd.Secp256k1,
	)
	if err != nil {
		panic(fmt.Errorf("failed to create mnemonic: %w", err))
	}

	return ibc.RelayerWallet{
		Address: types.MustBech32ifyAddressBytes(config.Bech32Prefix, info.GetAddress().Bytes()),

		Mnemonic: mnemonic,
	}
}

// relayerChains builds a mapping of relayers to which chains they connect to.
// The order of the chains is arbitrary.
func (w *World) relayerChains() map[ibc.Relayer][]ibc.Chain {
	// Use a Set of chain names first just to avoid deduplication.
	uniq := make(map[ibc.Relayer]map[ibc.Chain]struct{}, len(w.relayers))

	for rp, chains := range w.links {
		r := rp.Relayer
		if uniq[r] == nil {
			uniq[r] = make(map[ibc.Chain]struct{}, 2) // Adding at least 2 chains on it.
		}
		uniq[r][chains[0]] = struct{}{}
		uniq[r][chains[1]] = struct{}{}
	}

	out := make(map[ibc.Relayer][]ibc.Chain, len(uniq))
	for r, chainSet := range uniq {
		chains := make([]ibc.Chain, 0, len(chainSet))
		for chain := range chainSet {
			chains = append(chains, chain)
		}

		out[r] = chains
	}
	return out
}
