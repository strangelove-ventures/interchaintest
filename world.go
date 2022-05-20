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
	chains   map[string]ibc.Chain
	relayers map[string]ibc.Relayer

	// Key: relayer and path name; Value: the two chains being linked.
	links map[relayerPath][2]string
}

// NewWorld returns a new World.
func NewWorld() *World {
	return &World{
		chains:   make(map[string]ibc.Chain),
		relayers: make(map[string]ibc.Relayer),

		links: make(map[relayerPath][2]string),
	}
}

// relayerPath is a tuple of a relayer name and a path name.
type relayerPath struct {
	Relayer string
	Path    string
}

// AddChain adds the given chain with the given name to the World.
func (w *World) AddChain(name string, chain ibc.Chain) *World {
	if _, exists := w.chains[name]; exists {
		panic(fmt.Errorf("chain with name %q already exists", name))
	}

	w.chains[name] = chain
	return w
}

// AddRelayer adds the given relayer with the given name to the World.
func (w *World) AddRelayer(name string, relayer ibc.Relayer) *World {
	if _, exists := w.relayers[name]; exists {
		panic(fmt.Errorf("relayer with name %q already exists", name))
	}

	w.relayers[name] = relayer
	return w
}

// WorldLink describes a link between two chains,
// by specifying the chain names, the relayer name,
// and the name of the path to create.
type WorldLink struct {
	// Names of chains involved.
	Chain1, Chain2 string

	// Name of relayer to use for link.
	Relayer string

	// Name of path to create.
	Path string
}

// AddLink adds the given link to the World.
// If any validation fails, AddLink panics.
func (w *World) AddLink(link WorldLink) *World {
	if _, exists := w.chains[link.Chain1]; !exists {
		panic(fmt.Errorf("chain with name %q does not exist", link.Chain1))
	}
	if _, exists := w.chains[link.Chain2]; !exists {
		panic(fmt.Errorf("chain with name %q does not exist", link.Chain2))
	}
	if _, exists := w.relayers[link.Relayer]; !exists {
		panic(fmt.Errorf("relayer with name %q does not exist", link.Relayer))
	}

	if link.Chain1 == link.Chain2 {
		panic(fmt.Errorf("chain names must be different (both were %q)", link.Chain1))
	}

	key := relayerPath{
		Relayer: link.Relayer,
		Path:    link.Path,
	}

	if _, exists := w.links[key]; exists {
		panic(fmt.Errorf("relayer %q already has a path named %q", key.Relayer, key.Path))
	}

	chains := [2]string{link.Chain1, link.Chain2}
	if chains[0] > chains[1] {
		chains[0], chains[1] = chains[1], chains[0]
	}

	w.links[key] = chains
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
	res.generateWallets(w.chains, relayerChains)

	// Build a chainSet for chain construction.
	// However, because the chainSet wants to act like a slice,
	// and the World wants to act like a map,
	// we have this hacky chainIndices map to track names to slice index.
	chainIndices := make(map[string]int, len(w.chains))
	cs := make(chainSet, 0, len(w.chains))
	for name, c := range w.chains {
		chainIndices[name] = len(cs)
		cs = append(cs, c)
	}

	// Initialize the chains (pull docker images, etc.).
	if err := cs.Initialize(opts.TestName, opts.HomeDir, opts.Pool, opts.NetworkID); err != nil {
		return nil, fmt.Errorf("failed to initialize chains: %w", err)
	}

	// Start chains from genesis.
	// This would also be considerably simpler if the chain set was aware of the name->chain mapping.
	walletsByChain := res.walletsByChain()
	walletAmounts := make([][]ibc.WalletAmount, len(cs))
	for name, c := range w.chains {
		wallets := walletsByChain[name]
		amounts := make([]ibc.WalletAmount, len(wallets))
		for i, w := range wallets {
			amounts[i] = ibc.WalletAmount{
				Address: w.Address,
				Denom:   c.Config().Denom,
				Amount:  10_000_000_000_000, // Every wallet gets 10b units of denom.
			}
		}
		walletAmounts[chainIndices[name]] = amounts
	}

	if err := cs.Start(ctx, opts.TestName, walletAmounts); err != nil {
		return nil, fmt.Errorf("failed to start chains: %w", err)
	}

	// One pass through the chains to configure each relayer
	// for the chains it should know about.
	for rName, chains := range w.relayerChains() {
		for _, cName := range chains {
			r := w.relayers[rName]
			c := w.chains[cName]
			rpcAddr, grpcAddr := c.GetRPCAddress(), c.GetGRPCAddress()
			// TODO: handle relayer outside of Docker
			// (the UseDockerNetwork() method is on the factory, not the relayer).

			keyName := cName
			if err := r.AddChainConfiguration(ctx,
				rep,
				c.Config(), keyName,
				rpcAddr, grpcAddr,
			); err != nil {
				return nil, fmt.Errorf("failed to configure relayer %s for chain %s: %w", rName, cName, err)
			}

			if err := r.RestoreKey(ctx,
				rep,
				c.Config().ChainID, keyName,
				res.RelayerWallets[[2]string{rName, cName}].Mnemonic,
			); err != nil {
				return nil, fmt.Errorf("failed to restore key to relayer %s for chain %s: %w", rName, cName, err)
			}
		}
	}

	// For every relayer link, teach the relayer about the link and create the link.
	for rp, chains := range w.links {
		r := w.relayers[rp.Relayer]
		c0 := w.chains[chains[0]]
		c1 := w.chains[chains[1]]
		if err := r.GeneratePath(ctx, rep, c0.Config().ChainID, c1.Config().ChainID, rp.Path); err != nil {
			return nil, fmt.Errorf(
				"failed to generate path %s on relayer %s between chains %s and %s: %w",
				rp.Path, rp.Relayer, chains[0], chains[1], err,
			)
		}

		if err := r.LinkPath(ctx, rep, rp.Path); err != nil {
			return nil, fmt.Errorf(
				"failed to link path %s on relayer %s between chains %s and %s: %w",
				rp.Path, rp.Relayer, chains[0], chains[1], err,
			)
		}
	}

	return res, nil
}

// WorldResult describes the addresses and mnemonics
// of the faucet and relayer wallets created during (*World).Build.
type WorldResult struct {
	// Keyed by chain name.
	FaucetWallets map[string]ibc.RelayerWallet

	// Keyed by [relayer name, chain name].
	RelayerWallets map[[2]string]ibc.RelayerWallet
}

// generateWallets builds one faucet wallet on each chain
// and a wallet on each chain for each relayer.
func (r *WorldResult) generateWallets(chains map[string]ibc.Chain, relayerChains map[string][]string) {
	kr := keyring.NewInMemory()

	r.FaucetWallets = make(map[string]ibc.RelayerWallet, len(chains))
	for name, c := range chains {
		// The account name doesn't matter because the keyring is ephemeral,
		// but within the keyring's lifecycle, the name must be unique.
		accountName := "faucet-" + name

		r.FaucetWallets[name] = buildWallet(kr, accountName, c.Config())
	}

	r.RelayerWallets = make(map[[2]string]ibc.RelayerWallet, len(relayerChains))
	for relayer, cs := range relayerChains {
		for _, c := range cs {
			accountName := "relayer-" + relayer + "-" + c

			config := chains[c].Config()
			r.RelayerWallets[[2]string{relayer, c}] = buildWallet(kr, accountName, config)
		}
	}
}

// walletsByChain returns a mapping of chain names to a slice of wallets to create.
func (r *WorldResult) walletsByChain() map[string][]ibc.RelayerWallet {
	wallets := make(map[string][]ibc.RelayerWallet, len(r.FaucetWallets))

	// Every chain has a faucet wallet, so this is a new slice for each chain.
	for c, w := range r.FaucetWallets {
		wallets[c] = []ibc.RelayerWallet{w}
	}

	for rc, w := range r.RelayerWallets {
		c := rc[1]
		wallets[c] = append(wallets[c], w)
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

// relayerChains builds a mapping of relayer names to which chains they connect to.
// The order of the chains is arbitrary.
func (w *World) relayerChains() map[string][]string {
	// Use a Set of chain names first just to avoid deduplication.
	uniq := make(map[string]map[string]struct{}, len(w.relayers))

	for rp, chains := range w.links {
		r := rp.Relayer
		if uniq[r] == nil {
			uniq[r] = make(map[string]struct{}, 2) // Adding at least 2 chains on it.
		}
		uniq[r][chains[0]] = struct{}{}
		uniq[r][chains[1]] = struct{}{}
	}

	out := make(map[string][]string, len(uniq))
	for r, m := range uniq {
		chains := make([]string, 0, len(m))
		for chain := range m {
			chains = append(chains, chain)
		}

		out[r] = chains
	}
	return out
}
