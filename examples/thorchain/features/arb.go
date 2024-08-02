package features

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func Arb(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	exoChains ...ibc.Chain,
) (users []ibc.Wallet, err error) {
	chains := append(exoChains, thorchain)
	users = GetAndFundTestUsers(t, ctx, "arb", chains...)

	err = AddAdminIfNecessary(ctx, thorchain)
	require.NoError(t, err)

	mimirs, err := thorchain.ApiGetMimirs()
	require.NoError(t, err)

	if mimir, ok := mimirs["TradeAccountsEnabled"]; (ok && mimir != int64(1) || !ok) {
		err := thorchain.SetMimir(ctx, "admin", "TradeAccountsEnabled", "1")
		require.NoError(t, err)
	}

	thorUser := users[len(users)-1]

	var eg errgroup.Group
	for i, exoChain := range exoChains {
		i := i
		exoChain := exoChain
		eg.Go(func() error {
			exoChainType, err := common.NewChain(exoChain.Config().Name)
			require.NoError(t, err)

			exoUser := users[i]
	
			exoUserBalance, err := exoChain.GetBalance(ctx, exoUser.FormattedAddress(), exoChain.Config().Denom)
			require.NoError(t, err)

			memo := fmt.Sprintf("trade+:%s", thorUser.FormattedAddress())
			exoInboundAddr, _, err := thorchain.ApiGetInboundAddress(exoChainType.String())
			require.NoError(t, err)
			_, err = exoChain.SendFundsWithNote(ctx, exoUser.KeyName(), ibc.WalletAmount{
				Address: exoInboundAddr,
				Denom: exoChain.Config().Denom,
				Amount: exoUserBalance.QuoRaw(10).MulRaw(9),
			}, memo)

			return nil
		})
	}
	require.NoError(t, eg.Wait())

	go func() {
		type Pool struct {
			BalanceRune sdkmath.Uint
			BalanceAsset sdkmath.Uint
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
						BalanceRune:  sdkmath.NewUintFromString(pool.BalanceRune),
						BalanceAsset: sdkmath.NewUintFromString(pool.BalanceAsset),
					}
					continue
				}

				arbPools = append(arbPools, pool)
			}

			if allPoolsSuspended {
				return
			}

			if len(arbPools) < 2 {
				time.Sleep(time.Second * 2)
				continue
			}

			// sort pools by price change
			priceChangeBps := func(pool tc.Pool) int64 {
				originalPool := originalPools[pool.Asset]
				originalPrice := originalPool.BalanceRune.MulUint64(1e8).Quo(originalPool.BalanceAsset)
				currentPrice := sdkmath.NewUintFromString(pool.BalanceRune).MulUint64(1e8).Quo(sdkmath.NewUintFromString(pool.BalanceAsset))
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
				time.Sleep(time.Second * 2)
				continue
			}

			// build the swap
			memo := fmt.Sprintf("=:%s", strings.Replace(receive.Asset, ".", "~", 1))
			asset, err := common.NewAsset(strings.Replace(send.Asset, ".", "~", 1))
			require.NoError(t, err)
			amount := sdkmath.NewUint(uint64(adjustmentBps / 2)).Mul(sdkmath.NewUintFromString(send.BalanceAsset)).QuoUint64(maxBasisPts)

			fmt.Println("Arbing:", amount, asset.String(), memo)
			err = thorchain.Deposit(ctx, thorUser.KeyName(), sdkmath.Int(amount), asset.String(), memo)
			if err != nil {
				fmt.Println("Error:", err)
			} else {
				fmt.Println("No arb error")
			}
			//require.NoError(t, err)

			time.Sleep(time.Second * 2)
		}
	}()

	return users, nil
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