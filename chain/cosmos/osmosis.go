package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/strangelove-ventures/interchaintest/v8/internal/dockerutil"
)

// OsmosisPoolParams defines parameters for creating an osmosis gamm liquidity pool
type OsmosisPoolParams struct {
	Weights        string `json:"weights"`
	InitialDeposit string `json:"initial-deposit"`
	SwapFee        string `json:"swap-fee"`
	ExitFee        string `json:"exit-fee"`
	FutureGovernor string `json:"future-governor"`
}

func OsmosisCreatePool(c *CosmosChain, ctx context.Context, keyName string, params OsmosisPoolParams) (string, error) {
	tn := c.getFullNode()
	poolbz, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	poolFile := "pool.json"

	fw := dockerutil.NewFileWriter(tn.logger(), tn.DockerClient, tn.TestName)
	if err := fw.WriteFile(ctx, tn.VolumeName, poolFile, poolbz); err != nil {
		return "", fmt.Errorf("failed to write pool file: %w", err)
	}

	if _, err := tn.ExecTx(ctx, keyName,
		"gamm", "create-pool",
		"--pool-file", filepath.Join(tn.HomeDir(), poolFile), "--gas", "700000",
	); err != nil {
		return "", fmt.Errorf("failed to create pool: %w", err)
	}

	stdout, _, err := tn.ExecQuery(ctx, "gamm", "num-pools")
	if err != nil {
		return "", fmt.Errorf("failed to query num pools: %w", err)
	}
	var res map[string]string
	if err := json.Unmarshal(stdout, &res); err != nil {
		return "", fmt.Errorf("failed to unmarshal query response: %w", err)
	}

	numPools, ok := res["num_pools"]
	if !ok {
		return "", fmt.Errorf("could not find number of pools in query response: %w", err)
	}
	return numPools, nil
}

func OsmosisSwapExactAmountIn(c *CosmosChain, ctx context.Context, keyName string, coinIn string, minAmountOut string, poolIDs []string, swapDenoms []string) (string, error) {
	return c.getFullNode().ExecTx(ctx, keyName,
		"gamm", "swap-exact-amount-in",
		coinIn, minAmountOut,
		"--swap-route-pool-ids", strings.Join(poolIDs, ","),
		"--swap-route-denoms", strings.Join(swapDenoms, ","),
	)
}

// OsmosisQueryPoolIds get a list of pool IDs
// PoolType - Balancer or StableSwap
// MinLiquidity - String of the coins in single string seperated by comma. Ex) 10uatom,100uosmo
func OsmosisQueryPoolIds(c *CosmosChain, ctx context.Context, minLiquidity, poolType string) ([]uint64, error) {
	tn := c.getFullNode()

	stdout, _, err := tn.ExecQuery(ctx, "gamm", "pools-with-filter", minLiquidity, poolType)
	if err != nil {
		return nil, fmt.Errorf("failed to query pools: %w", err)
	}
	var res map[string]interface{}
	if err := json.Unmarshal(stdout, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query response: %w", err)
	}

	poolsMap, ok := res["pools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("could not parse query response: %w", err)
	}

	ids := []uint64{}
	for _, currPool := range poolsMap {
		iPoolsMap, ok := currPool.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("could not parse query response: %w", err)
		}

		cpm, ok := iPoolsMap["id"].(string)
		if !ok {
			return nil, fmt.Errorf("could not parse query response: %w", err)
		}
		id, err := strconv.ParseUint(cpm, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("pool id unexpected type: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}
