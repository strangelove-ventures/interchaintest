package thorchain_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/math"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/examples/thorchain/features"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestThorchain(t *testing.T) {
	numThorchainValidators := 1
	numThorchainFullNodes  := 0

	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	// ------------------------
	// Setup EVM chains first
	// ------------------------
	ethChainName := common.ETHChain.String() // must use this name for test
	cf0 := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainName:   ethChainName,
			Name:        ethChainName,
			Version:     "latest",
			ChainConfig: ethereum.DefaultEthereumAnvilChainConfig(ethChainName),
		},
	})

	chains, err := cf0.Chains(t.Name())
	require.NoError(t, err)
	ethChain := chains[0].(*ethereum.EthereumChain)

	ic0 := interchaintest.NewInterchain().
		AddChain(ethChain)
	
	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic0.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic0.Close()
	})

	ethUserInitialAmount := math.NewInt(2 * ethereum.ETHER)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", ethUserInitialAmount, ethChain)
	ethUser := users[0]

	ethChain.SendFunds(ctx, "faucet", ibc.WalletAmount{
		Address: "0x1804c8ab1f12e6bbf3894d4083f33e07309d1f38",
		Amount: math.NewInt(ethereum.ETHER),
	})

	os.Setenv("ETHFAUCET", "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	stdout, _, err := ethChain.ForgeScript(ctx, ethUser.KeyName(), ethereum.ForgeScriptOpts{
		ContractRootDir: "contracts",
		SolidityContract: "script/Token.s.sol",
		RawOptions:       []string{"--sender", ethUser.FormattedAddress(), "--json"},
	})
	require.NoError(t, err)

	tokenContractAddress, err := GetEthAddressFromStdout(string(stdout))
	require.NoError(t, err)
	require.NotEmpty(t, tokenContractAddress)
	require.True(t, ethcommon.IsHexAddress(tokenContractAddress))

	fmt.Println("Token contract address:", tokenContractAddress)

	stdout, _, err = ethChain.ForgeScript(ctx, ethUser.KeyName(), ethereum.ForgeScriptOpts{
		ContractRootDir: "contracts",
		SolidityContract: "script/Router.s.sol",
		RawOptions:       []string{"--sender", ethUser.FormattedAddress(), "--json"},
	})
	require.NoError(t, err)

	ethRouterContractAddress, err := GetEthAddressFromStdout(string(stdout))
	require.NoError(t, err)
	require.NotEmpty(t, ethRouterContractAddress)
	require.True(t, ethcommon.IsHexAddress(ethRouterContractAddress))

	fmt.Println("Router contract address:", ethRouterContractAddress)


	// ----------------------------
	// Set up thorchain and others
	// ----------------------------
	thorchainChainSpec := ThorchainDefaultChainSpec(t.Name(), numThorchainValidators, numThorchainFullNodes, ethRouterContractAddress)
	// TODO: add router contracts to thorchain
	// Set ethereum RPC
	// Move other chains to above for setup too?
	//thorchainChainSpec.
	chainSpecs := []*interchaintest.ChainSpec{
		thorchainChainSpec,
		GaiaChainSpec(),
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), chainSpecs)

	chains, err = cf.Chains(t.Name())
	require.NoError(t, err)

	thorchain := chains[0].(*tc.Thorchain)
	gaia := chains[1].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(thorchain).
		AddChain(gaia)
	
	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	err = thorchain.StartAllValSidecars(ctx)
	require.NoError(t, err, "failed starting validator sidecars")

	doTxs(t, ctx, gaia) // Do 100 transactions

	defaultFundAmount := math.NewInt(100_000_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "default", defaultFundAmount, thorchain)
	thorchainUser := users[0]
	err = testutil.WaitForBlocks(ctx, 2, thorchain)
	require.NoError(t, err, "thorchain failed to make blocks")

	// --------------------------------------------------------
	// Check balances are correct
	// --------------------------------------------------------
	thorchainUserAmount, err := thorchain.GetBalance(ctx, thorchainUser.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	require.True(t, thorchainUserAmount.Equal(defaultFundAmount), "Initial thorchain user amount not expected")
	
	val0 := thorchain.GetNode()
	faucetAddr, err := val0.AccountKeyBech32(ctx, "faucet")
	require.NoError(t, err)
	faucetAmount, err := thorchain.GetBalance(ctx, faucetAddr, thorchain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, InitialFaucetAmount.Sub(defaultFundAmount).Sub(StaticGas), faucetAmount)

	// --------------------------------------------------------
	// Bootstrap pool
	// --------------------------------------------------------
	_, gaiaLper := features.DualLp(t, ctx, thorchain, gaia)
	_, _ = features.DualLp(t, ctx, thorchain, ethChain)

	// --------------------------------------------------------
	// Savers
	// --------------------------------------------------------
	saversFundAmount := math.NewInt(100_000_000_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "savers", saversFundAmount, gaia)
	gaiaSaver := users[0]

	saversFundAmount = math.NewInt(2 * ethereum.ETHER)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "savers", saversFundAmount, ethChain)
	ethSaver := users[0]
	
	pool, err := thorchain.ApiGetPool(common.ATOMAsset)
	require.NoError(t, err)
	saveAmount := math.NewUintFromString(pool.BalanceAsset).
		MulUint64(500).QuoUint64(10_000)

	saverQuote, err := thorchain.ApiGetSaverDepositQuote(common.ATOMAsset, saveAmount)
	require.NoError(t, err)
	
	// store expected range to fail if received amount is outside 5% tolerance
	quoteOut := math.NewUintFromString(saverQuote.ExpectedAmountDeposit)
	tolerance := quoteOut.QuoUint64(20)
	if saverQuote.Fees.Outbound != nil {
		outboundFee := math.NewUintFromString(*saverQuote.Fees.Outbound)
		quoteOut = quoteOut.Add(outboundFee)
	}
	minExpectedSaver := quoteOut.Sub(tolerance)
	maxExpectedSaver := quoteOut.Add(tolerance)

	// Alternate between memo and memoless
	memo := fmt.Sprintf("+:%s", "GAIA/ATOM")
	//memo = ""
	gaiaInboundAddr, _, err := thorchain.ApiGetInboundAddress("GAIA")
	require.NoError(t, err)
	_, err = gaia.SendFundsWithNote(ctx, gaiaSaver.KeyName(), ibc.WalletAmount{
		Address: gaiaInboundAddr,
		Denom: gaia.Config().Denom,
		Amount: math.Int(saveAmount).QuoRaw(100), // save amount is based on 8 dec
	}, memo)

	saverFound := false
	for count := 0; !saverFound; count++ {
		time.Sleep(time.Second)
		savers, err := thorchain.ApiGetSavers(common.ATOMAsset)
		require.NoError(t, err)
		for _, saver := range savers {
			if saver.AssetAddress != gaiaSaver.FormattedAddress() {
				continue
			}
			saverFound = true
			deposit := math.NewUintFromString(saver.AssetDepositValue)
			require.True(t, deposit.GTE(minExpectedSaver), fmt.Sprintf("Actual: %s, Min expected: %s", deposit, minExpectedSaver))
			require.True(t, deposit.LTE(maxExpectedSaver), fmt.Sprintf("Actual: %s, Max expected: %s", deposit, maxExpectedSaver))
		}
		require.Less(t, count, 30, "saver took longer than 30 sec to show")
	}

	// --- DO it again for ethSaver

	pool, err = thorchain.ApiGetPool(common.ETHAsset)
	require.NoError(t, err)
	saveAmount = math.NewUintFromString(pool.BalanceAsset).
		MulUint64(500).QuoUint64(10_000)

	saverQuote, err = thorchain.ApiGetSaverDepositQuote(common.ETHAsset, saveAmount)
	require.NoError(t, err)
	
	// store expected range to fail if received amount is outside 5% tolerance
	quoteOut = math.NewUintFromString(saverQuote.ExpectedAmountDeposit)
	tolerance = quoteOut.QuoUint64(20)
	if saverQuote.Fees.Outbound != nil {
		outboundFee := math.NewUintFromString(*saverQuote.Fees.Outbound)
		quoteOut = quoteOut.Add(outboundFee)
	}
	minExpectedSaver = quoteOut.Sub(tolerance)
	maxExpectedSaver = quoteOut.Add(tolerance)

	// Alternate between memo and memoless
	memo = fmt.Sprintf("+:%s", "ETH/ETH")
	//memo = ""
	ethInboundAddr, _, err := thorchain.ApiGetInboundAddress("ETH")
	require.NoError(t, err)
	_, err = ethChain.SendFundsWithNote(ctx, ethSaver.KeyName(), ibc.WalletAmount{
		Address: ethInboundAddr,
		Denom: ethChain.Config().Denom,
		Amount: math.Int(saveAmount.MulUint64(10_000_000_000)),
	}, memo)
	require.NoError(t, err)

	saverFound = false
	for count := 0; !saverFound; count++ {
		time.Sleep(time.Second)
		savers, err := thorchain.ApiGetSavers(common.ETHAsset)
		require.NoError(t, err)
		for _, saver := range savers {
			fmt.Println("Actual vs Expected:", saver.AssetAddress, ethSaver.FormattedAddress())
			if strings.ToLower(saver.AssetAddress) != strings.ToLower(ethSaver.FormattedAddress()) {
				continue
			}
			saverFound = true
			deposit := math.NewUintFromString(saver.AssetDepositValue)
			require.True(t, deposit.GTE(minExpectedSaver), fmt.Sprintf("Actual: %s, Min expected: %s", deposit, minExpectedSaver))
			require.True(t, deposit.LTE(maxExpectedSaver), fmt.Sprintf("Actual: %s, Max expected: %s", deposit, maxExpectedSaver))
		}
		require.Less(t, count, 30, "saver took longer than 30 sec to show")
	}
	// --------------------------------------------------------
	// Arb (implement after adding another chain)
	// --------------------------------------------------------
	err = thorchain.RecoverKey(ctx, "admin", strings.Repeat("master ", 23) + "notice")
	require.NoError(t, err)

	arbFundAmount := math.NewInt(1_000_000_000) // 1k (gaia), 10 (thorchain)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "arb", arbFundAmount, thorchain, gaia)
	thorchainArber := users[0]
	gaiaArber := users[1]
	
	ethArbFundAmount := math.NewInt(2 * ethereum.ETHER)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "arb2", ethArbFundAmount, ethChain)
	ethArber := users[0]
	
	mimirs, err := thorchain.ApiGetMimirs()
	require.NoError(t, err)

	if mimir, ok := mimirs["TradeAccountsEnabled"]; (ok && mimir != int64(1) || !ok) {
		err := thorchain.SetMimir(ctx, "admin", "TradeAccountsEnabled", "1")
		require.NoError(t, err)
	}

	memo = fmt.Sprintf("trade+:%s", thorchainArber.FormattedAddress())
	gaiaInboundAddr, _, err = thorchain.ApiGetInboundAddress("GAIA")
	require.NoError(t, err)
	_, err = ethChain.SendFundsWithNote(ctx, ethArber.KeyName(), ibc.WalletAmount{
		Address: ethInboundAddr,
		Denom: ethChain.Config().Denom,
		Amount: ethArbFundAmount.QuoRaw(10).MulRaw(9),
	}, memo)
	require.NoError(t, err)

	memo = fmt.Sprintf("trade+:%s", thorchainArber.FormattedAddress())
	ethInboundAddr, _, err = thorchain.ApiGetInboundAddress("ETH")
	require.NoError(t, err)
	_, err = gaia.SendFundsWithNote(ctx, gaiaArber.KeyName(), ibc.WalletAmount{
		Address: gaiaInboundAddr,
		Denom: gaia.Config().Denom,
		Amount: arbFundAmount.QuoRaw(10).MulRaw(9),
	}, memo)

	go func() {
		type Pool struct {
			BalanceRune math.Uint
			BalanceAsset math.Uint
		}
		originalPools := make(map[string]Pool)
		maxBasisPts := uint64(10_000)

		for {
			pools, err := thorchain.ApiGetPools()
			require.NoError(t, err)

			allPoolsSuspended := true
			arbPools := []tc.Pool{}
			for _, pool := range pools {
				if pool.Status != "Suspended" {
					allPoolsSuspended = false
				}

				// skip unavailable pools and those with no liquidity
				if pool.BalanceRune == "0" || pool.BalanceAsset == "0" || pool.Status != "Available" {
					continue
				}

				// if this is the first time we see the pool, store it to use as the target price
				if _, ok := originalPools[pool.Asset]; !ok {
					originalPools[pool.Asset] = Pool{
						BalanceRune:  math.NewUintFromString(pool.BalanceRune),
						BalanceAsset: math.NewUintFromString(pool.BalanceAsset),
					}
					continue
				}

				arbPools = append(arbPools, pool)
			}

			if allPoolsSuspended {
				return
			}

			if len(arbPools) < 2 {
				time.Sleep(time.Second * 5)
				continue
			}

			// sort pools by price change
			priceChangeBps := func(pool tc.Pool) int64 {
				originalPool := originalPools[pool.Asset]
				originalPrice := originalPool.BalanceRune.MulUint64(1e8).Quo(originalPool.BalanceAsset)
				currentPrice := math.NewUintFromString(pool.BalanceRune).MulUint64(1e8).Quo(math.NewUintFromString(pool.BalanceAsset))
				return int64(maxBasisPts) - int64(originalPrice.MulUint64(maxBasisPts).Quo(currentPrice).Uint64())
			}
			sort.Slice(arbPools, func(i, j int) bool {
				return priceChangeBps(arbPools[i]) > priceChangeBps(arbPools[j])
			})

			send := arbPools[0]
			receive := arbPools[len(arbPools)-1]

			// skip if none have diverged more than 10 basis points
			adjustmentBps := Min(Abs(priceChangeBps(send)), Abs(priceChangeBps(receive)))
			if adjustmentBps < 10 {
				// pools have not diverged enough
				time.Sleep(time.Second * 5)
				continue
			}

			// build the swap
			memo := fmt.Sprintf("=:%s", strings.Replace(receive.Asset, ".", "~", 1))
			asset, err := common.NewAsset(strings.Replace(send.Asset, ".", "~", 1))
			require.NoError(t, err)
			amount := math.NewUint(uint64(adjustmentBps / 2)).Mul(math.NewUintFromString(send.BalanceAsset)).QuoUint64(maxBasisPts)

			fmt.Println("Arbing:", amount, asset.String(), memo)
			err = thorchain.Deposit(ctx, thorchainArber.KeyName(), math.Int(amount), asset.String(), memo)
			if err != nil {
				fmt.Println("Error:", err)
			} else {
				fmt.Println("No arb error")
			}
			//require.NoError(t, err)

			time.Sleep(time.Second * 5)
		}
	}()

	// --------------------------------------------------------
	// Swap
	// --------------------------------------------------------
	swapperFundAmount := math.NewInt(1_000_000_000) // 1k (gaia), 10 (thorchain)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "swappers", swapperFundAmount, thorchain, gaia)
	thorchainSwapper := users[0]
	gaiaSwapper := users[1]
	
	// Get quote and calculate expected min/max output
	swapAmountAtomToRune := math.NewUint(500_000_000)
	swapQuote, err := thorchain.ApiGetSwapQuote(common.ATOMAsset, common.RuneNative, swapAmountAtomToRune.MulUint64(100)) // Thorchain has 8 dec for atom
	
	// store expected range to fail if received amount is outside 5% tolerance
	quoteOut = math.NewUintFromString(swapQuote.ExpectedAmountOut)
	tolerance = quoteOut.QuoUint64(20)
	if swapQuote.Fees.Outbound != nil {
		outboundFee := math.NewUintFromString(*swapQuote.Fees.Outbound)
		quoteOut = quoteOut.Add(outboundFee)

		// handle 2x gas rate fluctuation (add 1x outbound fee to tolerance)
		tolerance = tolerance.Add(outboundFee)
	}
	minExpectedRune := quoteOut.Sub(tolerance)
	maxExpectedRune := quoteOut.Add(tolerance)

	gaiaInboundAddr, _, err = thorchain.ApiGetInboundAddress("GAIA")
	require.NoError(t, err)
	memo = fmt.Sprintf("=:%s:%s", common.RuneNative.String(), thorchainSwapper.FormattedAddress())
	txHash, err := gaia.SendFundsWithNote(ctx, gaiaSwapper.KeyName(), ibc.WalletAmount{
		Address: gaiaInboundAddr,
		Denom: gaia.Config().Denom,
		Amount: math.Int(swapAmountAtomToRune),
	}, memo)
	require.NoError(t, err)

	// ----- VerifyOutbound -----
	stages, err := thorchain.ApiGetTxStages(txHash)
	require.NoError(t, err)
	count := 0
	for stages.SwapFinalised == nil || !stages.SwapFinalised.Completed {
	//for stages.OutboundSigned == nil || !stages.OutboundSigned.Completed { // Only for non-rune swaps
		time.Sleep(time.Second)
		stages, err = thorchain.ApiGetTxStages(txHash)
		require.NoError(t, err)
		count++
		require.Less(t, count, 60, "swap didn't complete in 60 seconds")
	}

	details, err := thorchain.ApiGetTxDetails(txHash)
	require.NoError(t, err)
	require.Equal(t, 1, len(details.OutTxs))
	require.Equal(t, 1, len(details.Actions))

	// verify outbound amount + max gas within expected range
	action := details.Actions[0]
	out := details.OutTxs[0]
	outAmountPlusMaxGas := math.NewUintFromString(out.Coins[0].Amount)
	maxGas := action.MaxGas[0]
	if maxGas.Asset == common.RuneNative.String() {
		outAmountPlusMaxGas = outAmountPlusMaxGas.Add(math.NewUintFromString(maxGas.Amount))
	} else { // shouldn't enter here for atom -> rune
		var maxGasAssetValue math.Uint
		maxGasAssetValue, err = thorchain.ConvertAssetAmount(maxGas, common.RuneNative.String())
		require.NoError(t, err)
		outAmountPlusMaxGas = outAmountPlusMaxGas.Add(maxGasAssetValue)
	}

	thorchainSwapperBalance, err := thorchain.GetBalance(ctx, thorchainSwapper.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	actualRune := thorchainSwapperBalance.Sub(swapperFundAmount)
	require.True(t, actualRune.GTE(math.Int(minExpectedRune)), fmt.Sprintf("Actual: %s rune, Min expected: %s rune", actualRune, minExpectedRune))
	require.True(t, actualRune.LTE(math.Int(maxExpectedRune)), fmt.Sprintf("Actual: %s rune, Max expected: %s rune", actualRune, maxExpectedRune))

	// --------------------------------------------------------
	// Saver Eject
	// --------------------------------------------------------
	// Reset mimirs
	mimirs, err = thorchain.ApiGetMimirs()
	require.NoError(t, err)

	if mimir, ok := mimirs["MaxSynthPerPoolDepth"]; (ok && mimir != int64(-1)) {
		err := thorchain.SetMimir(ctx, "admin", "MaxSynthPerPoolDepth", "-1")
		require.NoError(t, err)
	}

	if mimir, ok := mimirs["SaversEjectInterval"]; (ok && mimir != int64(-1)) {
		err := thorchain.SetMimir(ctx, "admin", "SaversEjectInterval", "-1")
		require.NoError(t, err)
	}

	saversEjectFundAmount := math.NewInt(100_000_000_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "savers", saversEjectFundAmount, gaia)
	gaiaSaverEjectUser := users[0]

	pool, err = thorchain.ApiGetPool(common.ATOMAsset)
	require.NoError(t, err)
	saveEjectAmount := math.NewUintFromString(pool.BalanceAsset).
		MulUint64(2000).QuoUint64(10_000)

	saverEjectQuote, err := thorchain.ApiGetSaverDepositQuote(common.ATOMAsset, saveEjectAmount)
	require.NoError(t, err)
	
	// store expected range to fail if received amount is outside 5% tolerance
	// question: does arbing make this flaky?
	saverEjectQuoteOut := math.NewUintFromString(saverEjectQuote.ExpectedAmountDeposit)
	toleranceEject := saverEjectQuoteOut.QuoUint64(20)
	if saverEjectQuote.Fees.Outbound != nil {
		outboundFee := math.NewUintFromString(*saverEjectQuote.Fees.Outbound)
		saverEjectQuoteOut = saverEjectQuoteOut.Add(outboundFee)
	}
	minExpectedSaverEject := saverEjectQuoteOut.Sub(toleranceEject)
	maxExpectedSaverEject := saverEjectQuoteOut.Add(toleranceEject)

	// Alternate between memo and memoless
	memo = fmt.Sprintf("+:%s", "GAIA/ATOM")
	//memo = ""
	gaiaInboundAddr, _, err = thorchain.ApiGetInboundAddress("GAIA")
	require.NoError(t, err)
	_, err = gaia.SendFundsWithNote(ctx, gaiaSaverEjectUser.KeyName(), ibc.WalletAmount{
		Address: gaiaInboundAddr,
		Denom: gaia.Config().Denom,
		Amount: math.Int(saveEjectAmount).QuoRaw(100), // save amount is based on 8 dec
	}, memo)

	saverEjectUserFound := false
	for count := 0; !saverEjectUserFound; count++ {
		time.Sleep(time.Second)
		savers, err := thorchain.ApiGetSavers(common.ATOMAsset)
		require.NoError(t, err)
		for _, saver := range savers {
			if saver.AssetAddress != gaiaSaverEjectUser.FormattedAddress() {
				continue
			}
			saverEjectUserFound = true
			deposit := math.NewUintFromString(saver.AssetDepositValue)
			require.True(t, deposit.GTE(minExpectedSaverEject), fmt.Sprintf("Actual: %s uatom, Min expected: %s uatom", deposit, minExpectedSaverEject))
			require.True(t, deposit.LTE(maxExpectedSaverEject), fmt.Sprintf("Actual: %s uatom, Max expected: %s uatom", deposit, maxExpectedSaverEject))
		}
		require.Less(t, count, 30, "saver took longer than 30 sec to show")
	}

	gaiaSaverEjectUserBalance, err := gaia.GetBalance(ctx, gaiaSaverEjectUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	gaiaSaverBalance, err := gaia.GetBalance(ctx, gaiaSaver.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)

	// Set mimirs
	if mimir, ok := mimirs["MaxSynthPerPoolDepth"]; (ok && mimir != int64(500) || !ok) {
		err := thorchain.SetMimir(ctx, "admin", "MaxSynthPerPoolDepth", "500")
		require.NoError(t, err)
	}

	if mimir, ok := mimirs["SaversEjectInterval"]; (ok && mimir != int64(1) || !ok) {
		err := thorchain.SetMimir(ctx, "admin", "SaversEjectInterval", "1")
		require.NoError(t, err)
	}

	for count := 0; true; count++ {
		time.Sleep(time.Second)
		savers, err := thorchain.ApiGetSavers(common.ATOMAsset)
		require.NoError(t, err)
		saverEjectUserFound := false
		for _, saver := range savers {
			if saver.AssetAddress != gaiaSaverEjectUser.FormattedAddress() {
				continue
			}
			saverEjectUserFound = true
		}
		if !saverEjectUserFound {
			break
		}
		require.Less(t, count, 30, "saver took longer than 30 sec to show")
	}

	err = PollForBalanceChange(ctx, gaia, 15, ibc.WalletAmount{
		Address: gaiaSaverEjectUser.FormattedAddress(),
		Denom: gaia.Config().Denom,
		Amount: gaiaSaverEjectUserBalance,
	})
	gaiaSaverEjectUserAfterBalance, err := gaia.GetBalance(ctx, gaiaSaverEjectUser.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaSaverEjectUserAfterBalance.GT(gaiaSaverEjectUserBalance), fmt.Sprintf("Balance (%s) must be greater after ejection: %s", gaiaSaverEjectUserAfterBalance, gaiaSaverEjectUserBalance))
	gaiaSaverAfterBalance, err := gaia.GetBalance(ctx, gaiaSaver.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaSaverBalance.Equal(gaiaSaverAfterBalance), fmt.Sprintf("Balance (%s) should be the same (%s)", gaiaSaverAfterBalance, gaiaSaverBalance))

	// --------------------------------------------------------
	// Ragnarok gaia
	// --------------------------------------------------------
	pools, err := thorchain.ApiGetPools()
	require.NoError(t, err)
	require.Equal(t, 2, len(pools), "only 2 pools are expected")

	gaiaLperBalanceBeforeRag, err := gaia.GetBalance(ctx, gaiaLper.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	
	err = thorchain.SetMimir(ctx, "admin", "RAGNAROK-GAIA-ATOM", "1")
	require.NoError(t, err)

	pools, err = thorchain.ApiGetPools()
	require.NoError(t, err)
	count = 0
	for len(pools) > 1 {
		if pools[0].Status == "Suspended" {
			break
		}
		require.Less(t, count, 6, "atom pool didn't get torn down or suspended in 60 seconds")
		time.Sleep(10 * time.Second)
		pools, err = thorchain.ApiGetPools()
		require.NoError(t, err)
		count++
	}

	err = PollForBalanceChange(ctx, gaia, 100, ibc.WalletAmount{
		Address: gaiaLper.FormattedAddress(),
		Denom: gaia.Config().Denom,
		Amount: gaiaLperBalanceBeforeRag,
	})
	require.NoError(t, err)
	gaiaLperBalanceAfterRag, err := gaia.GetBalance(ctx, gaiaLper.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaLperBalanceAfterRag.GT(gaiaLperBalanceBeforeRag), fmt.Sprintf("Lper balance (%s) should be greater after ragnarok (%s)", gaiaLperBalanceAfterRag, gaiaLperBalanceBeforeRag))
	
	err = PollForBalanceChange(ctx, gaia, 30, ibc.WalletAmount{
		Address: gaiaSaver.FormattedAddress(),
		Denom: gaia.Config().Denom,
		Amount: gaiaSaverBalance,
	})
	require.NoError(t, err)
	gaiaSaverAfterBalance, err = gaia.GetBalance(ctx, gaiaSaver.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaSaverAfterBalance.GT(gaiaSaverBalance), fmt.Sprintf("Saver balance (%s) should be greater after ragnarok (%s)", gaiaSaverAfterBalance, gaiaSaverBalance))
	
	//err = gaia.StopAllNodes(ctx)
	//require.NoError(t, err)

	//state, err := gaia.ExportState(ctx, -1)
	//require.NoError(t, err)
	//fmt.Println("State: ", state)


	//err = testutil.WaitForBlocks(ctx, 300, thorchain)
	//require.NoError(t, err, "thorchain failed to make blocks")
}

func Min[T int | uint | int64 | uint64](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func Abs[T int | int64](a T) T {
	if a < 0 {
		return -a
	}
	return a
}