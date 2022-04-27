package relayertest

import (
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

const (
	srcAccountKeyName  = "src-chain"
	dstAccountKeyName  = "dst-chain"
	userAccountKeyName = "user"
	testPathName       = "test-path"
)

func TestRelayer_FromScratch(t *testing.T, cf ibctest.ChainFactory, rf ibctest.RelayerFactory) {
	// This test contains many subtests,
	// so the chains and initial setup will be coupled to this test.
	rootTestName := t.Name()

	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoErrorf(t, err, "failed to set up test run")

	srcChain, dstChain, err := cf.Pair(rootTestName)
	require.NoError(t, err, "failed to get chain pair")

	// Most of this code was copied from StartChainsAndRelayerFromFactory.
	// TODO: extract some helpers?
	require.NoError(t, srcChain.Initialize(rootTestName, home, pool, network), "failed to initialize source chain")
	require.NoError(t, dstChain.Initialize(rootTestName, home, pool, network), "failed to initialize dest chain")

	srcChainCfg := srcChain.Config()
	dstChainCfg := dstChain.Config()

	kr := keyring.NewInMemory()

	// NOTE: this is hardcoded to the cosmos coin type.
	// We will need to choose other coin types for non-cosmos IBC once that happens.
	const coinType = types.CoinType

	// Create accounts out of band, because the chain genesis needs to know where to send initial funds.
	srcInfo, srcMnemonic, err := kr.NewMnemonic(srcAccountKeyName, keyring.English, hd.CreateHDPath(coinType, 0, 0).String(), "", hd.Secp256k1)
	require.NoError(t, err, "failed to create source account")
	srcAccount := types.MustBech32ifyAddressBytes(srcChainCfg.Bech32Prefix, srcInfo.GetAddress().Bytes())

	dstInfo, dstMnemonic, err := kr.NewMnemonic(dstAccountKeyName, keyring.English, hd.CreateHDPath(coinType, 0, 0).String(), "", hd.Secp256k1)
	require.NoError(t, err, "failed to create dest account")
	dstAccount := types.MustBech32ifyAddressBytes(dstChainCfg.Bech32Prefix, dstInfo.GetAddress().Bytes())

	// Fund relayer account on src chain
	srcRelayerWalletAmount := ibc.WalletAmount{
		Address: srcAccount,
		Denom:   srcChainCfg.Denom,
		Amount:  10000000,
	}

	// Fund relayer account on dst chain
	dstRelayerWalletAmount := ibc.WalletAmount{
		Address: dstAccount,
		Denom:   dstChainCfg.Denom,
		Amount:  10000000,
	}

	// Generate key to be used for "user" that will execute IBC transaction
	require.NoError(t, srcChain.CreateKey(ctx, userAccountKeyName), "failed to create key on source chain")

	srcUserAccountAddressBytes, err := srcChain.GetAddress(userAccountKeyName)
	require.NoError(t, err, "failed to get source user account address")

	srcUserAccountSrc, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, srcUserAccountAddressBytes)
	require.NoError(t, err)

	srcUserAccountDst, err := types.Bech32ifyAddressBytes(dstChainCfg.Bech32Prefix, srcUserAccountAddressBytes)
	require.NoError(t, err)

	require.NoError(t, dstChain.CreateKey(ctx, userAccountKeyName), "failed to create key on dest chain")

	dstUserAccountAddressBytes, err := dstChain.GetAddress(userAccountKeyName)
	require.NoError(t, err, "failed to get dest user account address")

	dstUserAccountSrc, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, dstUserAccountAddressBytes)
	require.NoError(t, err)

	dstUserAccountDst, err := types.Bech32ifyAddressBytes(dstChainCfg.Bech32Prefix, dstUserAccountAddressBytes)
	require.NoError(t, err)

	srcUser := ibctest.User{
		KeyName:         userAccountKeyName,
		SrcChainAddress: srcUserAccountSrc,
		DstChainAddress: srcUserAccountDst,
	}

	dstUser := ibctest.User{
		KeyName:         userAccountKeyName,
		SrcChainAddress: dstUserAccountSrc,
		DstChainAddress: dstUserAccountDst,
	}
	_, _ = srcUser, dstUser

	// Fund user account on src chain in order to relay from src to dst
	srcUserWalletAmount := ibc.WalletAmount{
		Address: srcUserAccountSrc,
		Denom:   srcChainCfg.Denom,
		Amount:  10000000000,
	}

	// Fund user account on dst chain in order to relay from dst to src
	dstUserWalletAmount := ibc.WalletAmount{
		Address: dstUserAccountDst,
		Denom:   dstChainCfg.Denom,
		Amount:  10000000000,
	}

	// start chains from genesis, wait until they are producing blocks
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := srcChain.Start(rootTestName, egCtx, []ibc.WalletAmount{srcRelayerWalletAmount, srcUserWalletAmount}); err != nil {
			return fmt.Errorf("failed to start source chain: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		if err := dstChain.Start(rootTestName, egCtx, []ibc.WalletAmount{dstRelayerWalletAmount, dstUserWalletAmount}); err != nil {
			return fmt.Errorf("failed to start dest chain: %w", err)
		}
		return nil
	})

	require.NoError(t, eg.Wait())

	// Now that the chains are running, we can start the relayer.
	// (We couldn't do this earlier,
	// because a non-docker relayer would not have had an address for the nodes.)
	srcRPCAddr, srcGRPCAddr := srcChain.GetRPCAddress(), srcChain.GetGRPCAddress()
	dstRPCAddr, dstGRPCAddr := dstChain.GetRPCAddress(), dstChain.GetGRPCAddress()
	if !rf.UseDockerNetwork() {
		srcRPCAddr, srcGRPCAddr = srcChain.GetHostRPCAddress(), srcChain.GetHostGRPCAddress()
		dstRPCAddr, dstGRPCAddr = dstChain.GetHostRPCAddress(), dstChain.GetHostGRPCAddress()
	}

	r := rf.Build(t, pool, network, home)

	require.NoError(t, r.AddChainConfiguration(ctx,
		srcChainCfg, srcAccountKeyName,
		srcRPCAddr, srcGRPCAddr,
	), "failed to configure relayer for source chain")

	require.NoError(t, r.AddChainConfiguration(ctx,
		dstChainCfg, dstAccountKeyName,
		dstRPCAddr, dstGRPCAddr,
	), "failed to configure relayer for dest chain")

	require.NoError(
		t,
		r.RestoreKey(ctx, srcChain.Config().ChainID, srcAccountKeyName, srcMnemonic),
		"failed to restore key to source chain",
	)
	require.NoError(
		t,
		r.RestoreKey(ctx, dstChain.Config().ChainID, dstAccountKeyName, dstMnemonic),
		"failed to restore key to dest chain",
	)
	require.NoError(
		t,
		r.GeneratePath(ctx, srcChainCfg.ChainID, dstChainCfg.ChainID, testPathName),
		"failed to generate path",
	)

	// TODO: fill in these tests, add capability checks.
	t.Run("create clients", func(t *testing.T) {
		t.Run("create connections", func(t *testing.T) {
			t.Run("create channels", func(t *testing.T) {
			})
		})
	})
}
