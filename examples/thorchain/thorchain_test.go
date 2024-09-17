package thorchain_test

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto"
	"github.com/strangelove-ventures/interchaintest/v8"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/examples/thorchain/features"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// go test -timeout 20m -v -run TestThorchain examples/thorchain/*.go -count 1
func TestThorchainSim(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	// Start non-thorchain chains
	exoChains := StartExoChains(t, ctx, client, network)
	gaiaEg := SetupGaia(t, ctx, exoChains["GAIA"])
	ethRouterContractAddress, bscRouterContractAddress, err := SetupContracts(ctx, exoChains["ETH"], exoChains["BSC"])
	require.NoError(t, err)

	// Start thorchain
	thorchain := StartThorchain(t, ctx, client, network, exoChains, ethRouterContractAddress, bscRouterContractAddress)
	require.NoError(t, gaiaEg.Wait()) // Wait for 100 transactions before starting tests

	// --------------------------------------------------------
	// Bootstrap pool
	// --------------------------------------------------------
	eg, egCtx := errgroup.WithContext(ctx)
	for _, exoChain := range exoChains {
		exoChain := exoChain
		eg.Go(func() error {
			_, lper, err := features.DualLp(t, egCtx, thorchain, exoChain.chain)
			if err != nil {
				return err
			}
			exoChain.lpers = append(exoChain.lpers, lper)
			return nil
		})
	}
	require.NoError(t, eg.Wait())

	// --------------------------------------------------------
	// Savers
	// --------------------------------------------------------
	eg, egCtx = errgroup.WithContext(ctx)
	for _, exoChain := range exoChains {
		exoChain := exoChain
		eg.Go(func() error {
			saver, err := features.Saver(t, egCtx, thorchain, exoChain.chain)
			if err != nil {
				return err
			}
			exoChain.savers = append(exoChain.savers, saver)
			return nil
		})
	}
	require.NoError(t, eg.Wait())

	// --------------------------------------------------------
	// Arb
	// --------------------------------------------------------
	_, err = features.Arb(t, ctx, thorchain, exoChains.GetChains()...)
	require.NoError(t, err)

	// --------------------------------------------------------
	// Swap - only swaps non-rune assets for now
	// --------------------------------------------------------
	eg, egCtx = errgroup.WithContext(ctx)
	exoChainList := exoChains.GetChains()
	for i := range exoChainList {
		i := i
		fmt.Println("Chain:", i, "Name:", exoChainList[i].Config().Name)
		randomChain := rand.Intn(len(exoChainList))
		if i == randomChain && i == 0 {
			randomChain++
		} else if i == randomChain {
			randomChain--
		}
		eg.Go(func() error {
			return features.DualSwap(t, ctx, thorchain, exoChainList[i], exoChainList[randomChain])
		})
	}
	require.NoError(t, eg.Wait())

	// ------------------------------------------------------------
	// Saver Eject - must be done sequentially due to mimir states
	// ------------------------------------------------------------
	mimirLock := sync.Mutex{}
	eg, egCtx = errgroup.WithContext(ctx)
	for _, exoChain := range exoChains {
		exoChain := exoChain
		eg.Go(func() error {
			_, err = features.SaverEject(t, egCtx, &mimirLock, thorchain, exoChain.chain, exoChain.savers...)
			if err != nil {
				return err
			}
			return nil
		})
	}
	require.NoError(t, eg.Wait())

	// --------------------------------------------------------
	// Ragnarok
	// --------------------------------------------------------
	eg, egCtx = errgroup.WithContext(ctx)
	for _, exoChain := range exoChains {
		exoChain := exoChain
		eg.Go(func() error {
			refundWallets := append(exoChain.lpers, exoChain.savers...)
			return features.Ragnarok(t, egCtx, thorchain, exoChain.chain, refundWallets...)
		})
	}
	require.NoError(t, eg.Wait())

	//err = testutil.WaitForBlocks(ctx, 300, thorchain)
	//require.NoError(t, err, "thorchain failed to make blocks")
}

// go test -timeout 20m -v -run TestThorchainBankMsgSend examples/thorchain/*.go -count 1
func TestThorchainBankMsgSend(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	// Start non-thorchain chains
	exoChains := StartExoChains(t, ctx, client, network)
	gaiaEg := SetupGaia(t, ctx, exoChains["GAIA"])
	ethRouterContractAddress, bscRouterContractAddress, err := SetupContracts(ctx, exoChains["ETH"], exoChains["BSC"])
	require.NoError(t, err)

	// Start thorchain
	thorchain := StartThorchain(t, ctx, client, network, exoChains, ethRouterContractAddress, bscRouterContractAddress)
	require.NoError(t, gaiaEg.Wait()) // Wait for 100 transactions before starting tests
	strDenom := thorchain.Config().Denom
	fmt.Printf(strDenom)

	// Ensure there's an admin so mimirs can be configured
	if err := features.AddAdminIfNecessary(ctx, thorchain); err != nil {
		t.FailNow()
	}

	// Reset mimirs
	mimirLock := sync.Mutex{}
	mimirLock.Lock()
	mimirs, err := thorchain.ApiGetMimirs()
	require.NoError(t, err)

	// Enable RUNEPool mimir
	if mimir, ok := mimirs[strings.ToUpper("RUNEPOOLENABLED")]; ok && mimir != int64(1) || !ok {
		if err := thorchain.SetMimir(ctx, "admin", "RUNEPOOLENABLED", "1"); err != nil {
			mimirLock.Unlock()
			t.FailNow()
		}
	}

	users, err := features.GetAndFundTestUsers(t, ctx, "thorusr1", thorchain)
	require.NoError(t, err)
	thorUsr1 := users[0]

	users2, err := features.GetAndFundTestUsers(t, ctx, "thorusr2", thorchain)
	require.NoError(t, err)
	thorUsr2 := users2[0]

	usr1BalBefore, err := thorchain.GetBalance(ctx, thorUsr1.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)

	sendableAmount := usr1BalBefore.Quo(math.NewInt(10))

	usr2BalBefore, err := thorchain.GetBalance(ctx, thorUsr2.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)

	amount := ibc.WalletAmount{
		Address: thorUsr2.FormattedAddress(),
		Denom:   thorchain.Config().Denom,
		Amount:  sendableAmount,
	}

	// No error verifies that the route is enabled for a normal bank send
	err = thorchain.BankSendWithNote(ctx, thorUsr1.KeyName(), amount, "")
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 1, thorchain)
	require.NoError(t, err)

	usr1BalAfterFirstTx, err := thorchain.GetBalance(ctx, thorUsr1.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	usr2BalAfter, err := thorchain.GetBalance(ctx, thorUsr2.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	usr2BalExpected := usr2BalBefore.Add(sendableAmount)

	require.Greater(t, usr1BalBefore.Int64(), usr1BalAfterFirstTx.Int64())
	require.Equal(t, usr2BalExpected.Int64(), usr2BalAfter.Int64())

	usr1TxFee := usr1BalBefore.Sub(amount.Amount).Sub(usr1BalAfterFirstTx)

	thorAcc := sdk.AccAddress(crypto.AddressHash([]byte("thorchain")))
	thorAddr := sdk.MustBech32ifyAddressBytes("tthor", thorAcc)
	amountDeposit := ibc.WalletAmount{
		Address: thorAddr,
		Denom:   thorchain.Config().Denom,
		Amount:  sendableAmount,
	}
	usr1ExpectedBal := usr1BalAfterFirstTx.Sub(amountDeposit.Amount).Sub(usr1TxFee)

	// Perform a MsgDeposit to the RUNEpool using MsgSend with an embedded memo
	err = thorchain.BankSendWithNote(ctx, thorUsr1.KeyName(), amountDeposit, "pool+")
	require.NoError(t, err)

	// Verify the RUNE tokens are taken away from the user's bank balance
	usr1BalAfterSecondTx, err := thorchain.GetBalance(ctx, thorUsr1.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, usr1BalAfterSecondTx.Int64(), usr1ExpectedBal.Int64())

	// Verify the expected amount of RUNE tokens are deposited to RUNEpool for the user
	var runeProviderPosition tc.RUNEProvider
	provPosition, err := thorchain.ApiGetRuneProviders()
	require.NoError(t, err)
	for _, prov := range provPosition {
		if prov.RuneAddress == thorUsr1.FormattedAddress() {
			runeProviderPosition = prov
		}
	}
	require.NotNil(t, runeProviderPosition)
	require.Equal(t, runeProviderPosition.DepositAmount, amountDeposit.Amount.String())
}

// go test -timeout 20m -v -run TestThorchainMsgSend examples/thorchain/*.go -count 1
func TestThorchainMsgSend(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	// Start non-thorchain chains
	exoChains := StartExoChains(t, ctx, client, network)
	gaiaEg := SetupGaia(t, ctx, exoChains["GAIA"])
	ethRouterContractAddress, bscRouterContractAddress, err := SetupContracts(ctx, exoChains["ETH"], exoChains["BSC"])
	require.NoError(t, err)

	// Start thorchain
	thorchain := StartThorchain(t, ctx, client, network, exoChains, ethRouterContractAddress, bscRouterContractAddress)
	require.NoError(t, gaiaEg.Wait()) // Wait for 100 transactions before starting tests
	strDenom := thorchain.Config().Denom
	fmt.Printf(strDenom)

	// Ensure there's an admin so mimirs can be configured
	if err := features.AddAdminIfNecessary(ctx, thorchain); err != nil {
		t.FailNow()
	}

	// Reset mimirs
	mimirLock := sync.Mutex{}
	mimirLock.Lock()
	mimirs, err := thorchain.ApiGetMimirs()
	require.NoError(t, err)

	// Enable RUNEPool mimir
	if mimir, ok := mimirs[strings.ToUpper("RUNEPOOLENABLED")]; ok && mimir != int64(1) || !ok {
		if err := thorchain.SetMimir(ctx, "admin", "RUNEPOOLENABLED", "1"); err != nil {
			mimirLock.Unlock()
			t.FailNow()
		}
	}

	users, err := features.GetAndFundTestUsers(t, ctx, "thorusr1", thorchain)
	require.NoError(t, err)
	thorUsr1 := users[0]

	users2, err := features.GetAndFundTestUsers(t, ctx, "thorusr2", thorchain)
	require.NoError(t, err)
	thorUsr2 := users2[0]

	usr1BalBefore, err := thorchain.GetBalance(ctx, thorUsr1.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)

	sendableAmount := usr1BalBefore.Quo(math.NewInt(10))

	usr2BalBefore, err := thorchain.GetBalance(ctx, thorUsr2.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)

	amount := ibc.WalletAmount{
		Address: thorUsr2.FormattedAddress(),
		Denom:   thorchain.Config().Denom,
		Amount:  sendableAmount,
	}

	thorAcc := sdk.AccAddress(crypto.AddressHash([]byte("thorchain")))
	thorAccAddr := sdk.MustBech32ifyAddressBytes("tthor", thorAcc)

	// No error verifies that the route is enabled for a normal bank send
	_, err = thorchain.SendFundsWithNote(ctx, thorUsr1.KeyName(), amount, "")
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 1, thorchain)
	require.NoError(t, err)

	usr1BalAfterFirstTx, err := thorchain.GetBalance(ctx, thorUsr1.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	usr2BalAfter, err := thorchain.GetBalance(ctx, thorUsr2.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	usr2BalExpected := usr2BalBefore.Add(sendableAmount)

	require.Greater(t, usr1BalBefore.Int64(), usr1BalAfterFirstTx.Int64())
	require.Equal(t, usr2BalExpected.Int64(), usr2BalAfter.Int64())

	usr1TxFee := usr1BalBefore.Sub(amount.Amount).Sub(usr1BalAfterFirstTx)

	disallowedSendToThorModuleAddr := ibc.WalletAmount{
		Address: thorAccAddr,
		Denom:   thorchain.Config().Denom,
		Amount:  sendableAmount,
	}

	// Error verifies that sending tokens to the thor module address is disallowed
	_, err = thorchain.SendFundsWithNote(ctx, thorUsr1.KeyName(), disallowedSendToThorModuleAddr, "")
	require.Error(t, err)
	fmt.Printf("err: %+v\n", err)

	usr1BalAfterDeniedSend, err := thorchain.GetBalance(ctx, thorUsr1.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	expectedBal := usr1BalAfterFirstTx.Sub(usr1TxFee)
	require.Equal(t, expectedBal.String(), usr1BalAfterDeniedSend.String())

	amountDeposit := ibc.WalletAmount{
		Address: thorAccAddr,
		Denom:   thorchain.Config().Denom,
		Amount:  sendableAmount.Mul(math.NewInt(2)),
	}
	usr1ExpectedBal := usr1BalAfterFirstTx.Sub(amountDeposit.Amount).Sub(usr1TxFee).Sub(usr1TxFee)

	// Perform a MsgDeposit to the RUNEpool using MsgSend with an embedded memo
	_, err = thorchain.SendFundsWithNote(ctx, thorUsr1.KeyName(), amountDeposit, "pool+")
	require.NoError(t, err)

	// Verify the RUNE tokens are taken away from the user's bank balance
	usr1BalAfterSecondTx, err := thorchain.GetBalance(ctx, thorUsr1.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, usr1BalAfterSecondTx.Int64(), usr1ExpectedBal.Int64())

	// Verify the expected amount of RUNE tokens are deposited to RUNEpool for the user
	var runeProviderPosition tc.RUNEProvider
	provPosition, err := thorchain.ApiGetRuneProviders()
	require.NoError(t, err)
	for _, prov := range provPosition {
		if prov.RuneAddress == thorUsr1.FormattedAddress() {
			runeProviderPosition = prov
		}
	}
	require.NotNil(t, runeProviderPosition)
	require.Equal(t, runeProviderPosition.DepositAmount, amountDeposit.Amount.String())
}
