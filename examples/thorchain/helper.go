package thorchain_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func GetEthAddressFromStdout(stdout string) (string, error) {
	// Define the regular expression pattern
	re := regexp.MustCompile(`"value":"(0x[0-9a-fA-F]+)"`)

	// Find the first match
	matches := re.FindStringSubmatch(stdout)
	if len(matches) <= 1 {
		return "", fmt.Errorf("failed to parse out contract address")
	}
	// Extract the value
	return matches[1], nil
}

func sendFunds(ctx context.Context, keyName string, toAddr string, amount ibc.WalletAmount, val0 *cosmos.ChainNode) {
	memo := strings.Repeat("Hello World ", 10)
	command := []string{"bank", "send", keyName, toAddr, fmt.Sprintf("%s%s", amount.Amount.String(), amount.Denom), "--note", memo}
	_, _, _ = val0.Exec(ctx, val0.TxCommand(keyName, command...), val0.Chain.Config().Env)
}
