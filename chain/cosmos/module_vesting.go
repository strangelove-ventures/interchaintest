package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	vestingcli "github.com/cosmos/cosmos-sdk/x/auth/vesting/client/cli"

	"github.com/strangelove-ventures/interchaintest/v9/dockerutil"
)

// VestingCreateAccount creates a new vesting account funded with an allocation of tokens. The account can either be a delayed or continuous vesting account, which is determined by the '--delayed' flag.
// All vesting accounts created will have their start time set by the committed block's time. The end_time must be provided as a UNIX epoch timestamp.
func (tn *ChainNode) VestingCreateAccount(ctx context.Context, keyName string, toAddr string, coin string, endTime int64, flags ...string) error {
	cmd := []string{
		"vesting", "create-vesting-account", toAddr, coin, fmt.Sprintf("%d", endTime),
	}

	if len(flags) > 0 {
		cmd = append(cmd, flags...)
	}

	_, err := tn.ExecTx(ctx, keyName, cmd...)
	return err
}

// VestingCreatePermanentLockedAccount creates a new vesting account funded with an allocation of tokens that are locked indefinitely.
func (tn *ChainNode) VestingCreatePermanentLockedAccount(ctx context.Context, keyName string, toAddr string, coin string, flags ...string) error {
	cmd := []string{
		"vesting", "create-permanent-locked-account", toAddr, coin,
	}

	if len(flags) > 0 {
		cmd = append(cmd, flags...)
	}

	_, err := tn.ExecTx(ctx, keyName, cmd...)
	return err
}

// VestingCreatePeriodicAccount is a sequence of coins and period length in seconds.
// Periods are sequential, in that the duration of a period only starts at the end of the previous period.
// The duration of the first period starts upon account creation.
func (tn *ChainNode) VestingCreatePeriodicAccount(ctx context.Context, keyName string, toAddr string, periods vestingcli.VestingData, flags ...string) error {
	file := "periods.json"
	periodsJSON, err := json.MarshalIndent(periods, "", " ")
	if err != nil {
		return err
	}

	fw := dockerutil.NewFileWriter(tn.logger(), tn.DockerClient, tn.TestName)
	if err := fw.WriteFile(ctx, tn.VolumeName, file, periodsJSON); err != nil {
		return fmt.Errorf("writing periods JSON file to docker volume: %w", err)
	}

	cmd := []string{
		"vesting", "create-periodic-vesting-account", toAddr, path.Join(tn.HomeDir(), file),
	}

	if len(flags) > 0 {
		cmd = append(cmd, flags...)
	}

	_, err = tn.ExecTx(ctx, keyName, cmd...)
	return err
}
