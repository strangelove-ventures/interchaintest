package interchain

import (
	"context"
	"log"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	types "github.com/strangelove-ventures/interchaintest/local-interchain/interchain/types"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/chain/penumbra"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func AddGenesisKeysToKeyring(ctx context.Context, config *types.Config, chains []ibc.Chain) {
	for idx, chain := range config.Chains {
		switch chains[idx].(type) {
		case *cosmos.CosmosChain:
			chainObj := chains[idx].(*cosmos.CosmosChain)
			for _, acc := range chain.Genesis.Accounts {
				if acc.Mnemonic != "" {
					if err := chainObj.RecoverKey(ctx, acc.Name, acc.Mnemonic); err != nil {
						panic(err)
					}
				}
			}
		case *penumbra.PenumbraChain:
			chainObj := chains[idx].(*penumbra.PenumbraChain)
			for _, acc := range chain.Genesis.Accounts {
				if acc.Mnemonic != "" {
					if err := chainObj.RecoverKey(ctx, acc.Name, acc.Mnemonic); err != nil {
						panic(err)
					}
				}
			}
		default:
			continue
		}

	}
}

func PostStartupCommands(ctx context.Context, config *types.Config, chains []ibc.Chain) {
	for idx, chain := range config.Chains {
		switch chains[idx].(type) {
		case *cosmos.CosmosChain:
			chainObj := chains[idx].(*cosmos.CosmosChain)
			for _, cmd := range chain.Genesis.StartupCommands {
				startupCmds(ctx, cmd, chainObj, chainObj.Validators[0].HomeDir())
			}

		case *penumbra.PenumbraChain:
			chainObj := chains[idx].(*penumbra.PenumbraChain)
			for _, cmd := range chain.Genesis.StartupCommands {
				startupCmds(ctx, cmd, chainObj, chainObj.PenumbraNodes[0].TendermintNode.HomeDir())
			}
		}

	}
}

func startupCmds(ctx context.Context, cmd string, chainObj ibc.Chain, homeDir string) {
	log.Println("Running startup command", chainObj.Config().ChainID, cmd)

	cmd = strings.ReplaceAll(cmd, "%HOME%", homeDir)
	cmd = strings.ReplaceAll(cmd, "%CHAIN_ID%", chainObj.Config().ChainID)

	stdout, stderr, err := chainObj.Exec(ctx, strings.Split(cmd, " "), []string{})

	output := stdout
	if len(output) == 0 {
		output = stderr
	} else if err != nil {
		log.Println("Error running startup command", chainObj.Config().ChainID, cmd, err)
	}

	log.Println("Startup command output", chainObj.Config().ChainID, cmd, string(output))
}

func SetupGenesisWallets(config *types.Config, chains []ibc.Chain) map[ibc.Chain][]ibc.WalletAmount {
	// iterate all chains chain's configs & setup accounts
	additionalWallets := make(map[ibc.Chain][]ibc.WalletAmount)
	for idx, chain := range config.Chains {
		switch chainObj := chains[idx].(type) {
		case *cosmos.CosmosChain, *penumbra.PenumbraChain:
			additionalWallets = setupAccounts(chain.Genesis.Accounts, chainObj)
		default:
			continue
		}
	}
	return additionalWallets
}

func setupAccounts(genesisAccounts []types.GenesisAccount, chainObj ibc.Chain) map[ibc.Chain][]ibc.WalletAmount {
	additionalWallets := make(map[ibc.Chain][]ibc.WalletAmount)

	for _, acc := range genesisAccounts {
		amount, err := sdk.ParseCoinsNormalized(acc.Amount)
		if err != nil {
			panic(err)
		}

		for _, coin := range amount {
			additionalWallets[chainObj] = append(additionalWallets[chainObj], ibc.WalletAmount{
				Address: acc.Address,
				Amount:  coin.Amount,
				Denom:   coin.Denom,
			})
		}
	}

	return additionalWallets
}
