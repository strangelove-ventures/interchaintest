package cosmos

import (
	"context"
	"fmt"
	"strconv"
)

func TokenFactoryBurnDenom(c *CosmosChain, ctx context.Context, keyName, fullDenom string, amount uint64) (string, error) {
	coin := strconv.FormatUint(amount, 10) + fullDenom
	return c.getFullNode().ExecTx(ctx, keyName,
		"tokenfactory", "burn", coin,
	)
}

func TokenFactoryBurnDenomFrom(c *CosmosChain, ctx context.Context, keyName, fullDenom string, amount uint64, fromAddr string) (string, error) {
	return c.getFullNode().ExecTx(ctx, keyName,
		"tokenfactory", "burn-from", fromAddr, convertToCoin(amount, fullDenom),
	)
}

func TokenFactoryChangeAdmin(c *CosmosChain, ctx context.Context, keyName, fullDenom, newAdmin string) (string, error) {
	return c.getFullNode().ExecTx(ctx, keyName,
		"tokenfactory", "change-admin", fullDenom, newAdmin,
	)
}

// create denom may require a lot of gas if the chain has the DenomCreationGasConsume param enabled
func TokenFactoryCreateDenom(c *CosmosChain, ctx context.Context, keyName, denomName, gas string) (string, error) {
	cmd := []string{"tokenfactory", "create-denom", denomName}

	if gas != "" {
		cmd = append(cmd, "--gas", gas)
	}

	return c.getFullNode().ExecTx(ctx, keyName, cmd...)
}

func TokenFactoryForceTransferDenom(c *CosmosChain, ctx context.Context, keyName, fullDenom string, amount uint64, fromAddr, toAddr string) (string, error) {
	return c.getFullNode().ExecTx(ctx, keyName,
		"tokenfactory", "force-transfer", convertToCoin(amount, fullDenom), fromAddr, toAddr,
	)
}

func TokenFactoryMintDenom(c *CosmosChain, ctx context.Context, keyName, fullDenom string, amount uint64) (string, error) {
	return c.getFullNode().ExecTx(ctx, keyName,
		"tokenfactory", "mint", convertToCoin(amount, fullDenom),
	)
}

func TokenFactoryMintDenomTo(c *CosmosChain, ctx context.Context, keyName, fullDenom, toAddr string, amount uint64) (string, error) {
	return c.getFullNode().ExecTx(ctx, keyName,
		"tokenfactory", "mint-to", toAddr, convertToCoin(amount, fullDenom),
	)
}

func TokenFactoryMetadata(c *CosmosChain, ctx context.Context, keyName, fullDenom, ticker, description, exponent string) (string, error) {
	return c.getFullNode().ExecTx(ctx, keyName,
		"tokenfactory", "modify-metadata", fullDenom, ticker, fmt.Sprintf("'%s'", description), exponent,
	)
}

func convertToCoin(amount uint64, denom string) string {
	return strconv.FormatUint(amount, 10) + denom
}
