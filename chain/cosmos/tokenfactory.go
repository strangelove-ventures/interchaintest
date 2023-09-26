package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
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
func TokenFactoryCreateDenom(c *CosmosChain, ctx context.Context, user ibc.Wallet, denomName string, gas uint64) (string, string, error) {
	cmd := []string{"tokenfactory", "create-denom", denomName}

	if gas != 0 {
		cmd = append(cmd, "--gas", strconv.FormatUint(gas, 10))
	}

	txHash, err := c.getFullNode().ExecTx(ctx, user.KeyName(), cmd...)
	if err != nil {
		return "", "", err
	}

	return "factory/" + user.FormattedAddress() + "/" + denomName, txHash, nil
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

func TokenFactoryMintDenomTo(c *CosmosChain, ctx context.Context, keyName, fullDenom string, amount uint64, toAddr string) (string, error) {
	return c.getFullNode().ExecTx(ctx, keyName,
		"tokenfactory", "mint-to", toAddr, convertToCoin(amount, fullDenom),
	)
}

func TokenFactoryMetadata(c *CosmosChain, ctx context.Context, keyName, fullDenom, ticker, description string, exponent uint64) (string, error) {
	return c.getFullNode().ExecTx(ctx, keyName,
		"tokenfactory", "modify-metadata", fullDenom, ticker, description, strconv.FormatUint(exponent, 10),
	)
}

func TokenFactoryGetAdmin(c *CosmosChain, ctx context.Context, fullDenom string) (*QueryDenomAuthorityMetadataResponse, error) {
	res := &QueryDenomAuthorityMetadataResponse{}
	stdout, stderr, err := c.getFullNode().ExecQuery(ctx, "tokenfactory", "denom-authority-metadata", fullDenom)
	if err != nil {
		return nil, fmt.Errorf("failed to query tokenfactory denom-authority-metadata: %w\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	if err := json.Unmarshal(stdout, res); err != nil {
		return nil, err
	}

	return res, nil
}

func convertToCoin(amount uint64, denom string) string {
	return strconv.FormatUint(amount, 10) + denom
}
