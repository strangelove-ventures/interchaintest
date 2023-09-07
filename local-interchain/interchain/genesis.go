package interchain

import (
	"context"
	"log"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	types "github.com/strangelove-ventures/localinterchain/interchain/types"
)

func AddGenesisKeysToKeyring(ctx context.Context, config *types.Config, chains []ibc.Chain) {
	for idx, chain := range config.Chains {
		chainObj := chains[idx].(*cosmos.CosmosChain)

		for _, acc := range chain.Genesis.Accounts {
			if err := chainObj.RecoverKey(ctx, acc.Name, acc.Mnemonic); err != nil {
				panic(err)
			}
		}
	}
}

func PostStartupCommands(ctx context.Context, config *types.Config, chains []ibc.Chain) {
	for idx, chain := range config.Chains {
		chainObj := chains[idx].(*cosmos.CosmosChain)

		for _, cmd := range chain.Genesis.StartupCommands {
			log.Println("Running startup command", chainObj.Config().ChainID, cmd)

			cmd = strings.ReplaceAll(cmd, "%HOME%", chainObj.Validators[0].HomeDir())
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
	}
}

func SetupGenesisWallets(config *types.Config, chains []ibc.Chain) map[ibc.Chain][]ibc.WalletAmount {
	// iterate all chains chain's configs & setup accounts
	additionalWallets := make(map[ibc.Chain][]ibc.WalletAmount)
	for idx, chain := range config.Chains {
		chainObj := chains[idx].(*cosmos.CosmosChain)

		for _, acc := range chain.Genesis.Accounts {
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
	}
	return additionalWallets
}
