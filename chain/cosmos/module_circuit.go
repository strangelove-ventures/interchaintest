package cosmos

import (
	"context"
	"fmt"
	"strings"

	circuittypes "cosmossdk.io/x/circuit/types"
)

// CircuitAuthorize executes the circuit authorize command.
func (tn *ChainNode) CircuitAuthorize(ctx context.Context, keyName, address string, permissionLevel int, typeUrls []string) error {
	if len(typeUrls) == 0 {
		return fmt.Errorf("CircuitAuthorize no typeUrls provided")
	}

	_, err := tn.ExecTx(ctx,
		keyName, "circuit", "authorize", address, fmt.Sprintf("%d", permissionLevel), minimizeTypeUrl(typeUrls),
	)
	return err
}

// CircuitDisable executes the circuit disable command.
func (tn *ChainNode) CircuitDisable(ctx context.Context, keyName string, typeUrls []string) error {
	if len(typeUrls) == 0 {
		return fmt.Errorf("CircuitDisable no typeUrls provided")
	}

	_, err := tn.ExecTx(ctx,
		keyName, "circuit", "disable", minimizeTypeUrl(typeUrls),
	)
	return err
}

func minimizeTypeUrl(typeUrls []string) string {
	updatedTypeUrls := make([]string, len(typeUrls))
	for i, typeUrl := range typeUrls {
		updatedTypeUrls[i] = strings.TrimPrefix(typeUrl, "/")
	}
	return strings.Join(updatedTypeUrls, ",")
}

// CircuitGetAccount returns a specific account's permissions.
func (c *CosmosChain) CircuitQueryAccount(ctx context.Context, addr string) (*circuittypes.AccountResponse, error) {
	res, err := circuittypes.NewQueryClient(c.GetNode().GrpcConn).Account(ctx, &circuittypes.QueryAccountRequest{
		Address: addr,
	})
	return res, err
}

// CircuitGetAccounts returns a list of all accounts with permissions.
func (c *CosmosChain) CircuitQueryAccounts(ctx context.Context, addr string) ([]*circuittypes.GenesisAccountPermissions, error) {
	res, err := circuittypes.NewQueryClient(c.GetNode().GrpcConn).Accounts(ctx, &circuittypes.QueryAccountsRequest{})
	return res.Accounts, err
}

// CircuitGetDisableList returns a list of all disabled message types.
func (c *CosmosChain) CircuitQueryDisableList(ctx context.Context) (*circuittypes.DisabledListResponse, error) {
	res, err := circuittypes.NewQueryClient(c.GetNode().GrpcConn).DisabledList(ctx, &circuittypes.QueryDisabledListRequest{})
	return res, err
}
