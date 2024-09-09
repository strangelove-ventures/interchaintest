package thorchain_test

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/strangelove-ventures/interchaintest/v9/ibc"
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

// We are running many nodes, using many resources. This function is similar to
// testutils.WaitForBlocks(), but does not hammer calls as fast as possible.
func NiceWaitForBlocks(ctx context.Context, delta int64, chain ibc.Chain) error {
	startingHeight, err := chain.Height(ctx)
	if err != nil {
		return err
	}

	currentHeight := startingHeight
	for currentHeight < startingHeight+delta {
		time.Sleep(time.Millisecond * 200)
		currentHeight, err = chain.Height(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
