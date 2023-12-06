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
	cmd = append(cmd, extraFlags...)

	_, err := tn.ExecTx(ctx, keyName, cmd...)
	return err
}

// UpgradeCancel executes the upgrade cancel command.
func (tn *ChainNode) UpgradeCancel(ctx context.Context, keyName string, extraFlags ...string) error {
	cmd := []string{"upgrade", "cancel-software-upgrade"}
	cmd = append(cmd, extraFlags...)

	_, err := tn.ExecTx(ctx, keyName, cmd...)
	return err
}

// UpgradeGetPlan queries the current upgrade plan.
func (c *CosmosChain) UpgradeGetPlan(ctx context.Context, name string) (*upgradetypes.Plan, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).CurrentPlan(ctx, &upgradetypes.QueryCurrentPlanRequest{})
	return res.Plan, err
}

// UpgradeGetAppliedPlan queries a previously applied upgrade plan by its name.
func (c *CosmosChain) UpgradeGetAppliedPlan(ctx context.Context, name string) (*upgradetypes.QueryAppliedPlanResponse, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).AppliedPlan(ctx, &upgradetypes.QueryAppliedPlanRequest{
		Name: name,
	})
	return res, err

}

// UpgradeGetAuthority returns the account with authority to conduct upgrades
func (c *CosmosChain) UpgradeGetAuthority(ctx context.Context, name string) (string, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).Authority(ctx, &upgradetypes.QueryAuthorityRequest{})
	return res.Address, err
}

// UpgradeGetAllModuleVersions queries the list of module versions from state.
func (c *CosmosChain) UpgradeGetAllModuleVersions(ctx context.Context) ([]*upgradetypes.ModuleVersion, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).ModuleVersions(ctx, &upgradetypes.QueryModuleVersionsRequest{})
	return res.ModuleVersions, err
}

// UpgradeGetModuleVersion queries a specific module version from state.
func (c *CosmosChain) UpgradeGetModuleVersion(ctx context.Context, module string) (*upgradetypes.ModuleVersion, error) {
	res, err := upgradetypes.NewQueryClient(c.GetNode().GrpcConn).ModuleVersions(ctx, &upgradetypes.QueryModuleVersionsRequest{
		ModuleName: module,
	})
	if err != nil {
		return nil, err
	}

	return res.ModuleVersions[0], err
}
