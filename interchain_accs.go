package ibctest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestInterchainAccounts(t *testing.T, relayF RelayerFactory) {
	t.Parallel()

	pool, network := DockerSetup(t)
	home := t.TempDir() // Must be before chain cleanup to avoid test error during cleanup.

	log := newLogger()
	ctx := context.Background()

	rep := new(testreporter.Reporter)
	eRep := rep.RelayerExecReporter(t)

	// Setup relayer key store
	kr := keyring.NewInMemory()
	const coinType = types.CoinType
	const relayerKeyName = "default"

	info, mnemonic, err := kr.NewMnemonic(
		relayerKeyName,
		keyring.English,
		hd.CreateHDPath(coinType, 0, 0).String(),
		"",
		hd.Secp256k1,
	)

	//entropy, err := bip39.NewEntropy(256)
	//require.NoError(t, err)
	//
	//mnemonic, err := bip39.NewMnemonic(entropy)
	//require.NoError(t, err)

	// Get both chains
	srcChain, err := GetChain(t.Name(), "icad", "master", "icad-1", 4, 1, log)
	require.NoError(t, err)

	dstChain, err := GetChain(t.Name(), "icad", "master", "icad-2", 4, 1, log)
	require.NoError(t, err)

	relayerSrcChain := types.MustBech32ifyAddressBytes(srcChain.Config().Bech32Prefix, info.GetAddress())
	relayerDstChain := types.MustBech32ifyAddressBytes(dstChain.Config().Bech32Prefix, info.GetAddress())

	// Build relayer
	relayerImpl := relayF.Build(t, pool, network, home)

	// Initialize & start both chains
	err = srcChain.Initialize(t.Name(), home, pool, network)
	require.NoError(t, err)

	err = dstChain.Initialize(t.Name(), home, pool, network)
	require.NoError(t, err)

	err = srcChain.CreateKey(ctx, "icad-1")
	require.NoError(t, err)

	err = dstChain.CreateKey(ctx, "icad-2")
	require.NoError(t, err)

	err = srcChain.Start(t.Name(), ctx, ibc.WalletAmount{
		Address: relayerSrcChain,
		Denom:   srcChain.Config().Denom,
		Amount:  1000000,
	})
	require.NoError(t, err)

	err = dstChain.Start(t.Name(), ctx, ibc.WalletAmount{
		Address: relayerDstChain,
		Denom:   srcChain.Config().Denom,
		Amount:  1000000,
	})
	require.NoError(t, err)

	// Wait for chains to produce blocks
	err = test.WaitForBlocks(ctx, 5, srcChain)
	require.NoError(t, err)

	t.Log("Chains Started")

	// Configure chains in the relayer
	rpcAddr, grpcAddr := srcChain.GetRPCAddress(), srcChain.GetGRPCAddress()
	if !relayerImpl.UseDockerNetwork() {
		rpcAddr, grpcAddr = srcChain.GetHostRPCAddress(), srcChain.GetHostGRPCAddress()
	}

	err = relayerImpl.AddChainConfiguration(
		ctx,
		eRep,
		srcChain.Config(),
		"default",
		rpcAddr,
		grpcAddr,
	)
	require.NoError(t, err)

	t.Log("Added Config 1")

	rpcAddr, grpcAddr = dstChain.GetRPCAddress(), dstChain.GetGRPCAddress()
	if !relayerImpl.UseDockerNetwork() {
		rpcAddr, grpcAddr = dstChain.GetHostRPCAddress(), dstChain.GetHostGRPCAddress()
	}

	err = relayerImpl.AddChainConfiguration(
		ctx,
		eRep,
		dstChain.Config(),
		"default",
		rpcAddr,
		grpcAddr,
	)
	require.NoError(t, err)

	t.Log("Added Config 2")

	// Restore keys for both chains
	err = relayerImpl.RestoreKey(
		ctx,
		eRep,
		srcChain.Config().ChainID,
		"default",
		mnemonic,
	)
	require.NoError(t, err)

	t.Log("Restore Key 1")

	err = relayerImpl.RestoreKey(
		ctx,
		eRep,
		dstChain.Config().ChainID,
		"default",
		mnemonic,
	)
	require.NoError(t, err)

	t.Log("Restore Key 2")

	// Generate path
	pathName := "ica-path"
	err = relayerImpl.GeneratePath(ctx, eRep, srcChain.Config().ChainID, dstChain.Config().ChainID, pathName)
	require.NoError(t, err)
	t.Log("After gen path")

	// Clients handshake
	err = relayerImpl.CreateClients(ctx, eRep, pathName)
	require.NoError(t, err)
	t.Log("After create clients")

	test.WaitForBlocks(ctx, 5, srcChain, dstChain)

	// Connections handshake
	err = relayerImpl.CreateConnections(ctx, eRep, pathName)
	require.NoError(t, err)
	t.Log("After create connections")

	// Start relayer
	err = relayerImpl.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)
	t.Log("After relayer start")

	t.Cleanup(func() {
		if err := relayerImpl.StopRelayer(ctx, eRep); err != nil {
			t.Logf("error stopping relayer: %v", err)
		}
		if err := srcChain.Cleanup(ctx); err != nil {
			log.Warn("Chain cleanup failed", zap.String("chain", srcChain.Config().ChainID), zap.Error(err))
		}
		if err := dstChain.Cleanup(ctx); err != nil {
			log.Warn("Chain cleanup failed", zap.String("chain", dstChain.Config().ChainID), zap.Error(err))
		}
	})

	// Wait for relayer to start up
	err = test.WaitForBlocks(ctx, 5, srcChain)
	require.NoError(t, err)

	// Query connections
	connections, err := relayerImpl.GetConnections(ctx, eRep, srcChain.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 1, len(connections))

	// Register interchain account
	_, err = srcChain.RegisterInterchainAccount(ctx, "icad-1", connections[0].ID)

	// Query interchain account
	address, err := srcChain.GetAddress(ctx, "icad-1")
	require.NoError(t, err)

	icaAddress, err := srcChain.QueryInterchainAccount(ctx, connections[0].ID, string(address))
	require.NoError(t, err)

	log.Info(icaAddress)
}

func newLogger() *zap.Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format("2006-01-02T15:04:05.000000Z07:00"))
	}
	config.LevelKey = "lvl"

	enc := zapcore.NewConsoleEncoder(config)
	lvl := zap.NewAtomicLevel()
	return zap.New(zapcore.NewCore(enc, os.Stdout, lvl))
}
