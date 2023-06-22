package polkadot_test

import (
	"context"
	"fmt"
	"testing"

	interchaintest "github.com/strangelove-ventures/interchaintest/v5"
	"github.com/strangelove-ventures/interchaintest/v5/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestWalletMethods(t *testing.T) {
	ctx := context.Background()
	nv := 5
	nf := 3

	chains, err := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainConfig: ibc.ChainConfig{
				Type:    "polkadot",
				Name:    "composable",
				ChainID: "rococo-local",
				Images: []ibc.DockerImage{
					{
						Repository: "seunlanlege/centauri-polkadot",
						Version:    "v0.9.27",
						UidGid:     "1025:1025",
					},
					{
						Repository: "seunlanlege/centauri-parachain",
						Version:    "v0.9.27",
						//UidGid: "1025:1025",
					},
				},
				Bin:            "polkadot",
				Bech32Prefix:   "composable",
				Denom:          "uDOT",
				GasPrices:      "",
				GasAdjustment:  0,
				TrustingPeriod: "",
				CoinType:       "354",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
	},
	).Chains(t.Name())

	require.NoError(t, err, "failed to get polkadot chain")
	require.Len(t, chains, 1)
	chain := chains[0]

	// BuildRelayerWallet test
	relayKeyName := "relayerWallet"
	relayWallet, err := chain.BuildRelayerWallet(ctx, relayKeyName)
	require.NoError(t, err, "Error building wallet")

	address, err := chain.GetAddress(ctx, relayKeyName)
	require.NoError(t, err, "Error getting relay address")
	fmt.Println("Relay wallet mnemonic: ", relayWallet.Mnemonic())
	fmt.Println("Relay wallet keyname: ", relayWallet.KeyName())
	fmt.Println("Relay wallet address: ", relayWallet.FormattedAddress())
	fmt.Println("Address: ", address)
	require.Equal(t, relayWallet.FormattedAddress(), string(address), "Relay addresses not equal")

	// BuildWallet test
	userKeyName := "userWallet"
	userWallet, err := chain.BuildRelayerWallet(ctx, userKeyName)
	require.NoError(t, err, "Error building wallet")

	address, err = chain.GetAddress(ctx, userKeyName)
	require.NoError(t, err, "Error getting user address")
	fmt.Println("Wallet mnemonic: ", userWallet.Mnemonic())
	fmt.Println("Wallet keyname: ", userWallet.KeyName())
	fmt.Println("Wallet address: ", userWallet.FormattedAddress())
	fmt.Println("Address: ", address)
	require.Equal(t, userWallet.FormattedAddress(), string(address), "User addresses not equal")

	// RecoverKey test
	recoverKeyName := "recoverWallet"
	err = chain.RecoverKey(ctx, recoverKeyName, userWallet.Mnemonic())
	require.NoError(t, err, "Error on RecoverKey")

	userAddress, err := chain.GetAddress(ctx, userKeyName)
	require.NoError(t, err, "Error getting user address for recover comparison")
	recoverAddress, err := chain.GetAddress(ctx, recoverKeyName)
	require.NoError(t, err, "Error getting recover address for recover comparison")
	require.Equal(t, userAddress, recoverAddress, "User and recover addresses not equal")
}
