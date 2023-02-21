package interchaintest

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Interchain represents a full IBC network, encompassing a collection of
// one or more chains, one or more relayer instances, and initial account configuration.
type Interchain struct {
	log *zap.Logger

	// Map of chain reference to chain ID.
	chains map[ibc.Chain]string

	// Map of relayer reference to user-supplied instance name.
	relayers map[ibc.Relayer]string

	// Key: relayer and path name; Value: the two chains being linked.
	links map[relayerPath]interchainLink

	// Set to true after Build is called once.
	built bool

	// Map of relayer-chain pairs to address and mnemonic, set during Build().
	// Not yet exposed through any exported API.
	relayerWallets map[relayerChain]ibc.Wallet

	// Map of chain to additional genesis wallets to include at chain start.
	AdditionalGenesisWallets map[ibc.Chain][]ibc.WalletAmount

	// Set during Build and cleaned up in the Close method.
	cs *chainSet
}

type interchainLink struct {
	chains [2]ibc.Chain
	// If set, these options will be used when creating the client in the path link step.
	// If a zero value initialization is used, e.g. CreateClientOptions{},
	// then the default values will be used via ibc.DefaultClientOpts.
	createClientOpts ibc.CreateClientOptions

	// If set, these options will be used when creating the channel in the path link step.
	// If a zero value initialization is used, e.g. CreateChannelOptions{},
	// then the default values will be used via ibc.DefaultChannelOpts.
	createChannelOpts ibc.CreateChannelOptions
}

// NewInterchain returns a new Interchain.
//
// Typical usage involves multiple calls to AddChain, one or more calls to AddRelayer,
// one or more calls to AddLink, and then finally a single call to Build.
func NewInterchain() *Interchain {
	return &Interchain{
		log: zap.NewNop(),

		chains:   make(map[ibc.Chain]string),
		relayers: make(map[ibc.Relayer]string),

		links: make(map[relayerPath]interchainLink),
	}
}

// relayerPath is a tuple of a relayer and a path name.
type relayerPath struct {
	Relayer ibc.Relayer
	Path    string
}

// AddChain adds the given chain to the Interchain,
// using the chain ID reported by the chain's config.
// If the given chain already exists,
// or if another chain with the same configured chain ID exists, AddChain panics.
func (ic *Interchain) AddChain(chain ibc.Chain, additionalGenesisWallets ...ibc.WalletAmount) *Interchain {
	if chain == nil {
		panic(fmt.Errorf("cannot add nil chain"))
	}

	newID := chain.Config().ChainID
	newName := chain.Config().Name

	for c, id := range ic.chains {
		if c == chain {
			panic(fmt.Errorf("chain %v was already added", c))
		}
		if id == newID {
			panic(fmt.Errorf("a chain with ID %s already exists", id))
		}
		if c.Config().Name == newName {
			panic(fmt.Errorf("a chain with name %s already exists", newName))
		}
	}

	ic.chains[chain] = newID

	if len(additionalGenesisWallets) == 0 {
		return ic
	}

	if ic.AdditionalGenesisWallets == nil {
		ic.AdditionalGenesisWallets = make(map[ibc.Chain][]ibc.WalletAmount)
	}
	ic.AdditionalGenesisWallets[chain] = additionalGenesisWallets

	return ic
}

// AddRelayer adds the given relayer with the given name to the Interchain.
func (ic *Interchain) AddRelayer(relayer ibc.Relayer, name string) *Interchain {
	if relayer == nil {
		panic(fmt.Errorf("cannot add nil relayer"))
	}

	for r, n := range ic.relayers {
		if r == relayer {
			panic(fmt.Errorf("relayer %v was already added", r))
		}
		if n == name {
			panic(fmt.Errorf("a relayer with name %s already exists", n))
		}
	}

	ic.relayers[relayer] = name
	return ic
}

// InterchainLink describes a link between two chains,
// by specifying the chain names, the relayer name,
// and the name of the path to create.
type InterchainLink struct {
	// Chains involved.
	Chain1, Chain2 ibc.Chain

	// Relayer to use for link.
	Relayer ibc.Relayer

	// Name of path to create.
	Path string

	// If set, these options will be used when creating the client in the path link step.
	// If a zero value initialization is used, e.g. CreateClientOptions{},
	// then the default values will be used via ibc.DefaultClientOpts.
	CreateClientOpts ibc.CreateClientOptions

	// If set, these options will be used when creating the channel in the path link step.
	// If a zero value initialization is used, e.g. CreateChannelOptions{},
	// then the default values will be used via ibc.DefaultChannelOpts.
	CreateChannelOpts ibc.CreateChannelOptions
}

// AddLink adds the given link to the Interchain.
// If any validation fails, AddLink panics.
func (ic *Interchain) AddLink(link InterchainLink) *Interchain {
	if _, exists := ic.chains[link.Chain1]; !exists {
		cfg := link.Chain1.Config()
		panic(fmt.Errorf("chain with name=%s and id=%s was never added to Interchain", cfg.Name, cfg.ChainID))
	}
	if _, exists := ic.chains[link.Chain2]; !exists {
		cfg := link.Chain2.Config()
		panic(fmt.Errorf("chain with name=%s and id=%s was never added to Interchain", cfg.Name, cfg.ChainID))
	}
	if _, exists := ic.relayers[link.Relayer]; !exists {
		panic(fmt.Errorf("relayer %v was never added to Interchain", link.Relayer))
	}

	if link.Chain1 == link.Chain2 {
		panic(fmt.Errorf("chains must be different (both were %v)", link.Chain1))
	}

	key := relayerPath{
		Relayer: link.Relayer,
		Path:    link.Path,
	}

	if _, exists := ic.links[key]; exists {
		panic(fmt.Errorf("relayer %q already has a path named %q", key.Relayer, key.Path))
	}

	ic.links[key] = interchainLink{
		chains:            [2]ibc.Chain{link.Chain1, link.Chain2},
		createChannelOpts: link.CreateChannelOpts,
		createClientOpts:  link.CreateClientOpts,
	}
	return ic
}

// InterchainBuildOptions describes configuration for (*Interchain).Build.
type InterchainBuildOptions struct {
	TestName string

	Client    *client.Client
	NetworkID string

	// If set, ic.Build does not create paths or links in the relayer,
	// but it does still configure keys and wallets for declared relayer-chain links.
	// This is useful for tests that need lower-level access to configuring relayers.
	SkipPathCreation bool

	// Optional. Git sha for test invocation. Once Go 1.18 supported,
	// may be deprecated in favor of runtime/debug.ReadBuildInfo.
	GitSha string

	// If set, saves block history to a sqlite3 database to aid debugging.
	BlockDatabaseFile string
}

// Build starts all the chains and configures the relayers associated with the Interchain.
// It is the caller's responsibility to directly call StartRelayer on the relayer implementations.
//
// Calling Build more than once will cause a panic.
func (ic *Interchain) Build(ctx context.Context, rep *testreporter.RelayerExecReporter, opts InterchainBuildOptions) error {
	if ic.built {
		panic(fmt.Errorf("Interchain.Build called more than once"))
	}
	ic.built = true

	chains := make([]ibc.Chain, 0, len(ic.chains))
	for chain := range ic.chains {
		chains = append(chains, chain)
	}
	ic.cs = newChainSet(ic.log, chains)

	// Initialize the chains (pull docker images, etc.).
	if err := ic.cs.Initialize(ctx, opts.TestName, opts.Client, opts.NetworkID); err != nil {
		return fmt.Errorf("failed to initialize chains: %w", err)
	}

	err := ic.generateRelayerWallets(ctx) // Build the relayer wallet mapping.
	if err != nil {
		return err
	}

	walletAmounts, err := ic.genesisWalletAmounts(ctx)
	if err != nil {
		// Error already wrapped with appropriate detail.
		return err
	}

	if err := ic.cs.Start(ctx, opts.TestName, walletAmounts); err != nil {
		return fmt.Errorf("failed to start chains: %w", err)
	}

	if err := ic.cs.TrackBlocks(ctx, opts.TestName, opts.BlockDatabaseFile, opts.GitSha); err != nil {
		return fmt.Errorf("failed to track blocks: %w", err)
	}

	if err := ic.configureRelayerKeys(ctx, rep); err != nil {
		// Error already wrapped with appropriate detail.
		return err
	}

	// Some tests may want to configure the relayer from a lower level,
	// but still have wallets configured.
	if opts.SkipPathCreation {
		return nil
	}

	// For every relayer link, teach the relayer about the link and create the link.
	for rp, link := range ic.links {
		rp := rp
		link := link
		c0 := link.chains[0]
		c1 := link.chains[1]

		if err := rp.Relayer.GeneratePath(ctx, rep, c0.Config().ChainID, c1.Config().ChainID, rp.Path); err != nil {
			return fmt.Errorf(
				"failed to generate path %s on relayer %s between chains %s and %s: %w",
				rp.Path, rp.Relayer, ic.chains[c0], ic.chains[c1], err,
			)
		}
	}

	// Now link the paths in parallel
	// Creates clients, connections, and channels for each link/path.
	var eg errgroup.Group
	for rp, link := range ic.links {
		rp := rp
		link := link
		c0 := link.chains[0]
		c1 := link.chains[1]
		eg.Go(func() error {
			// If the user specifies a zero value CreateClientOptions struct then we fall back to the default
			// client options.
			if link.createClientOpts == (ibc.CreateClientOptions{}) {
				link.createClientOpts = ibc.DefaultClientOpts()
			}

			// Check that the client creation options are valid and fully specified.
			if err := link.createClientOpts.Validate(); err != nil {
				return err
			}

			// If the user specifies a zero value CreateChannelOptions struct then we fall back to the default
			// channel options for an ics20 fungible token transfer channel.
			if link.createChannelOpts == (ibc.CreateChannelOptions{}) {
				link.createChannelOpts = ibc.DefaultChannelOpts()
			}

			// Check that the channel creation options are valid and fully specified.
			if err := link.createChannelOpts.Validate(); err != nil {
				return err
			}

			if err := rp.Relayer.LinkPath(ctx, rep, rp.Path, link.createChannelOpts, link.createClientOpts); err != nil {
				return fmt.Errorf(
					"failed to link path %s on relayer %s between chains %s and %s: %w",
					rp.Path, rp.Relayer, ic.chains[c0], ic.chains[c1], err,
				)
			}
			return nil
		})
	}

	return eg.Wait()
}

// WithLog sets the logger on the interchain object.
// Usually the default nop logger is fine, but sometimes it can be helpful
// to see more verbose logs, typically by passing zaptest.NewLogger(t).
func (ic *Interchain) WithLog(log *zap.Logger) *Interchain {
	ic.log = log
	return ic
}

// Close cleans up any resources created during Build,
// and returns any relevant errors.
func (ic *Interchain) Close() error {
	return ic.cs.Close()
}

func (ic *Interchain) genesisWalletAmounts(ctx context.Context) (map[ibc.Chain][]ibc.WalletAmount, error) {
	// Faucet addresses are created separately because they need to be explicitly added to the chains.
	faucetAddresses, err := ic.cs.CreateCommonAccount(ctx, FaucetAccountKeyName)
	if err != nil {
		return nil, fmt.Errorf("failed to create faucet accounts: %w", err)
	}

	// Wallet amounts for genesis.
	walletAmounts := make(map[ibc.Chain][]ibc.WalletAmount, len(ic.cs.chains))

	// Add faucet for each chain first.
	for c := range ic.chains {
		// The values are nil at this point, so it is safe to directly assign the slice.
		walletAmounts[c] = []ibc.WalletAmount{
			{
				Address: faucetAddresses[c],
				Denom:   c.Config().Denom,
				Amount:  100_000_000_000_000, // Faucet wallet gets 100T units of denom.
			},
		}

		if ic.AdditionalGenesisWallets != nil {
			walletAmounts[c] = append(walletAmounts[c], ic.AdditionalGenesisWallets[c]...)
		}
	}

	// Then add all defined relayer wallets.
	for rc, wallet := range ic.relayerWallets {
		c := rc.C
		walletAmounts[c] = append(walletAmounts[c], ibc.WalletAmount{
			Address: wallet.FormattedAddress(),
			Denom:   c.Config().Denom,
			Amount:  1_000_000_000_000, // Every wallet gets 1t units of denom.
		})
	}

	return walletAmounts, nil
}

// generateRelayerWallets populates ic.relayerWallets.
func (ic *Interchain) generateRelayerWallets(ctx context.Context) error {
	if ic.relayerWallets != nil {
		panic(fmt.Errorf("cannot call generateRelayerWallets more than once"))
	}

	relayerChains := ic.relayerChains()
	ic.relayerWallets = make(map[relayerChain]ibc.Wallet, len(relayerChains))
	for r, chains := range relayerChains {
		for _, c := range chains {
			// Just an ephemeral unique name, only for the local use of the keyring.
			accountName := ic.relayers[r] + "-" + ic.chains[c]
			newWallet, err := c.BuildRelayerWallet(ctx, accountName)
			if err != nil {
				return err
			}
			ic.relayerWallets[relayerChain{R: r, C: c}] = newWallet
		}
	}

	return nil
}

// configureRelayerKeys adds the chain configuration for each relayer
// and adds the preconfigured key to the relayer for each relayer-chain.
func (ic *Interchain) configureRelayerKeys(ctx context.Context, rep *testreporter.RelayerExecReporter) error {
	// Possible optimization: each relayer could be configured concurrently.
	// But we are only testing with a single relayer so far, so we don't need this yet.

	for r, chains := range ic.relayerChains() {
		for _, c := range chains {
			rpcAddr, grpcAddr := c.GetRPCAddress(), c.GetGRPCAddress()
			if !r.UseDockerNetwork() {
				rpcAddr, grpcAddr = c.GetHostRPCAddress(), c.GetHostGRPCAddress()
			}

			chainName := ic.chains[c]
			if err := r.AddChainConfiguration(ctx,
				rep,
				c.Config(), chainName,
				rpcAddr, grpcAddr,
			); err != nil {
				return fmt.Errorf("failed to configure relayer %s for chain %s: %w", ic.relayers[r], chainName, err)
			}

			if err := r.RestoreKey(ctx,
				rep,
				c.Config(), chainName,
				ic.relayerWallets[relayerChain{R: r, C: c}].Mnemonic(),
			); err != nil {
				return fmt.Errorf("failed to restore key to relayer %s for chain %s: %w", ic.relayers[r], chainName, err)
			}
		}
	}

	return nil
}

// relayerChain is a tuple of a Relayer and a Chain.
type relayerChain struct {
	R ibc.Relayer
	C ibc.Chain
}

// relayerChains builds a mapping of relayers to the chains they connect to.
// The order of the chains is arbitrary.
func (ic *Interchain) relayerChains() map[ibc.Relayer][]ibc.Chain {
	// First, collect a mapping of relayers to sets of chains,
	// so we don't have to manually deduplicate entries.
	uniq := make(map[ibc.Relayer]map[ibc.Chain]struct{}, len(ic.relayers))

	for rp, link := range ic.links {
		r := rp.Relayer
		if uniq[r] == nil {
			uniq[r] = make(map[ibc.Chain]struct{}, 2) // Adding at least 2 chains per relayer.
		}
		uniq[r][link.chains[0]] = struct{}{}
		uniq[r][link.chains[1]] = struct{}{}
	}

	// Then convert the sets to slices.
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
