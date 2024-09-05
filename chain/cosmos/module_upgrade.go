package cosmos

import (
	"context"
	"fmt"

	upgradetypes "cosmossdk.io/x/upgrade/types"
)

// UpgradeSoftware executes the upgrade software command.
func (tn *ChainNode) UpgradeSoftware(ctx context.Context, keyName, name, info string, height int, extraFlags ...string) error {
	cmd := []string{"upgrade", "software-upgrade", name}
	if height > 0 {
		cmd = append(cmd, "--upgrade-height", fmt.Sprintf("%d", height))
	}
	if info != "" {
		cmd = append(cmd, "--upgrade-info", info)
	}

	if len(extraFlags) > 0 {
		cmd = append(cmd, extraFlags...)
	}

	_, err := tn.ExecTx(ctx, keyName, cmd...)
	return err
}

// UpgradeCancel executes the upgrade cancel command.
func (tn *ChainNode) UpgradeCancel(ctx context.Context, keyName string, extraFlags ...string) error {
	cmd := []string{"upgrade", "cancel-software-upgrade"}

	if len(extraFlags) > 0 {
		cmd = append(cmd, extraFlags...)
	}

	_, err := tn.ExecTx(ctx, keyName, cmd...)
	return err
}

// UpgradeQueryPlan queries the current upgrade plan.
func (c *CosmosChain) UpgradeQueryPlan(ctx context.Context) (*upgradetypes.Plan, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).CurrentPlan(ctx, &upgradetypes.QueryCurrentPlanRequest{})
	return res.Plan, err
}

// UpgradeQueryAppliedPlan queries a previously applied upgrade plan by its name.
func (c *CosmosChain) UpgradeQueryAppliedPlan(ctx context.Context, name string) (*upgradetypes.QueryAppliedPlanResponse, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).AppliedPlan(ctx, &upgradetypes.QueryAppliedPlanRequest{
		Name: name,
	})
	return res, err
}

// UpgradeQueryAuthority returns the account with authority to conduct upgrades.
func (c *CosmosChain) UpgradeQueryAuthority(ctx context.Context) (string, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).Authority(ctx, &upgradetypes.QueryAuthorityRequest{})
	return res.Address, err
}

// UpgradeQueryAllModuleVersions queries the list of module versions from state.
func (c *CosmosChain) UpgradeQueryAllModuleVersions(ctx context.Context) ([]*upgradetypes.ModuleVersion, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).ModuleVersions(ctx, &upgradetypes.QueryModuleVersionsRequest{})
	return res.ModuleVersions, err
}

// UpgradeQueryModuleVersion queries a specific module version from state.
func (c *CosmosChain) UpgradeQueryModuleVersion(ctx context.Context, module string) (*upgradetypes.ModuleVersion, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).ModuleVersions(ctx, &upgradetypes.QueryModuleVersionsRequest{
		ModuleName: module,
	})
	if err != nil {
		return nil, err
	}

	return res.ModuleVersions[0], err
}
