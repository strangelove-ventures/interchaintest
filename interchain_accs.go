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

	// Get both chains
	srcChain, err := GetChain(t.Name(), "icad", "master", "icad-1", 4, 0, log)
	require.NoError(t, err)

	dstChain, err := GetChain(t.Name(), "icad", "master", "icad-2", 4, 0, log)
	require.NoError(t, err)

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

	const srcKeyName = "icad-1"
	err = srcChain.CreateKey(ctx, srcKeyName)
	require.NoError(t, err)

	const srcUserKeyName = "user"
	err = srcChain.CreateKey(ctx, srcUserKeyName)
	require.NoError(t, err)

	const dstKeyName = "icad-2"
	err = dstChain.CreateKey(ctx, dstKeyName)
	require.NoError(t, err)

	addrBz, err := srcChain.GetAddress(ctx, srcUserKeyName)
	require.NoError(t, err)

	srcUser := types.MustBech32ifyAddressBytes(srcChain.Config().Bech32Prefix, addrBz)

	err = srcChain.Start(t.Name(), ctx,
		ibc.WalletAmount{
			Address: relayerSrcChain,
			Denom:   srcChain.Config().Denom,
			Amount:  1000000,
		},
		ibc.WalletAmount{
			Address: srcUser,
			Denom:   srcChain.Config().Denom,
			Amount:  1000000,
		},
	)
	require.NoError(t, err)

	err = dstChain.Start(t.Name(), ctx, ibc.WalletAmount{
		Address: relayerDstChain,
		Denom:   srcChain.Config().Denom,
		Amount:  1000000,
	})
	require.NoError(t, err)

	// Wait for chains to produce blocks
	err = test.WaitForBlocks(ctx, 5, srcChain, dstChain)
	require.NoError(t, err)

	// Configure chains in the relayer
	rpcAddr, grpcAddr := srcChain.GetRPCAddress(), srcChain.GetGRPCAddress()
	if !relayerImpl.UseDockerNetwork() {
		rpcAddr, grpcAddr = srcChain.GetHostRPCAddress(), srcChain.GetHostGRPCAddress()
	}

	err = relayerImpl.AddChainConfiguration(
		ctx,
		eRep,
		srcChain.Config(),
		relayerKeyName,
		rpcAddr,
		grpcAddr,
	)
	require.NoError(t, err)

	rpcAddr, grpcAddr = dstChain.GetRPCAddress(), dstChain.GetGRPCAddress()
	if !relayerImpl.UseDockerNetwork() {
		rpcAddr, grpcAddr = dstChain.GetHostRPCAddress(), dstChain.GetHostGRPCAddress()
	}

	err = relayerImpl.AddChainConfiguration(
		ctx,
		eRep,
		dstChain.Config(),
		relayerKeyName,
		rpcAddr,
		grpcAddr,
	)
	require.NoError(t, err)

	// Restore keys for both chains in the relayer
	err = relayerImpl.RestoreKey(
		ctx,
		eRep,
		srcChain.Config().ChainID,
		relayerKeyName,
		mnemonic,
	)
	require.NoError(t, err)

	err = relayerImpl.RestoreKey(
		ctx,
		eRep,
		dstChain.Config().ChainID,
		relayerKeyName,
		mnemonic,
	)
	require.NoError(t, err)

	// Generate path
	pathName := "ica-path"
	err = relayerImpl.GeneratePath(ctx, eRep, srcChain.Config().ChainID, dstChain.Config().ChainID, pathName)
	require.NoError(t, err)

	// Create new clients
	t.Log("Creating new clients")

	err = relayerImpl.CreateClients(ctx, eRep, pathName)
	require.NoError(t, err)

	err = test.WaitForBlocks(ctx, 5, srcChain, dstChain)
	require.NoError(t, err)

	// Create a new connection
	t.Log("Creating a new connection")

	err = relayerImpl.CreateConnections(ctx, eRep, pathName)
	require.NoError(t, err)

	err = test.WaitForBlocks(ctx, 5, srcChain)
	require.NoError(t, err)

	// Query for the newly created connection
	connections, err := relayerImpl.GetConnections(ctx, eRep, srcChain.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 1, len(connections))

	// Register a new interchain account
	t.Log("Registering a new interchain account")

	_, err = srcChain.RegisterInterchainAccount(ctx, srcUser, connections[0].ID)
	require.NoError(t, err)

	err = test.WaitForBlocks(ctx, 5, srcChain)
	require.NoError(t, err)

	// Start the relayer
	t.Log("Starting the relayer")

	err = relayerImpl.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := srcChain.Cleanup(ctx); err != nil {
			log.Warn("Chain cleanup failed", zap.String("chain", srcChain.Config().ChainID), zap.Error(err))
		}
		if err := dstChain.Cleanup(ctx); err != nil {
			log.Warn("Chain cleanup failed", zap.String("chain", dstChain.Config().ChainID), zap.Error(err))
		}
	})

	// Wait for relayer to start up and finish channel handshake
	err = test.WaitForBlocks(ctx, 15, srcChain)
	require.NoError(t, err)

	// Stop the relayer
	t.Log("Stopping the relayer")

	err = relayerImpl.StopRelayer(ctx, eRep)
	require.NoError(t, err)

	// Query for the new interchain account
	t.Log("Querying for the new interchain account address")

	icaAddress, err := srcChain.QueryInterchainAccount(ctx, connections[0].ID, srcUser)
	require.NoError(t, err)

	log.Info(icaAddress)
	require.NotEqual(t, "", icaAddress)
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
