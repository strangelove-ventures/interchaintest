package cosmos

import (
	"context"
	"fmt"
	"strconv"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/strangelove-ventures/poa"
)

// POASetPower
func (tn *ChainNode) POASetPower(ctx context.Context, keyName, valoper string, power uint64, unsafe bool, flags ...string) error {
	cmd := []string{"poa", "set-power", valoper, fmt.Sprintf("%d", power)}
	if unsafe {
		cmd = append(cmd, "--unsafe")
	}
	cmd = append(cmd, flags...)

	_, err := tn.ExecTx(ctx,
		keyName, cmd...,
	)
	return err
}

// POARemove
func (tn *ChainNode) POARemove(ctx context.Context, keyName, valoper string, flags ...string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "poa", "remove", valoper,
	)
	return err
}

// POARemovePending
func (tn *ChainNode) POARemovePending(ctx context.Context, keyName, valoper string, flags ...string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "poa", "remove-pending", valoper,
	)
	return err
}

type POACreatePendingValidatorOpts struct {
	// Ed25519PubKey is just the key, not the entire json object
	// e.g. pl3Q8OQwtC7G2dSqRqsUrO5VZul7l40I+MKUcejqRsg=
	Ed25519PubKey           string
	Moniker                 string  `default:"moniker"`
	CommissionRate          float64 `default:"0.1"`
	CommissionMaxRate       float64 `default:"0.2"`
	CommissionMaxChangeRate float64 `default:"0.01"`
}

// POACreatePendingValidator
func (tn *ChainNode) POACreatePendingValidator(ctx context.Context, keyName string, opts POACreatePendingValidatorOpts) error {
	file := "validator_file.json"

	cr := strconv.FormatFloat(opts.CommissionRate, 'f', -1, 64)
	cmr := strconv.FormatFloat(opts.CommissionMaxRate, 'f', -1, 64)
	cmcr := strconv.FormatFloat(opts.CommissionMaxChangeRate, 'f', -1, 64)

	content := fmt.Sprintf(`{
		"pubkey": {"@type":"/cosmos.crypto.ed25519.PubKey","key":"%s"},
		"amount": "0%s",
		"moniker": "%s",
		"identity": "",
		"website": "https://website.com",
		"security": "security@cosmos.xyz",
		"details": "description",
		"commission-rate": "%s",
		"commission-max-rate": "%s",
		"commission-max-change-rate": "%s",
		"min-self-delegation": "1"

	}`, opts.Ed25519PubKey, tn.Chain.Config().Denom, opts.Moniker, cr, cmr, cmcr)

	if err := tn.WriteFile(ctx, []byte(content), file); err != nil {
		return err
	}

	_, err := tn.ExecTx(ctx,
		keyName, "poa", "create-validator", fmt.Sprintf("%s/%s", tn.HomeDir(), file),
	)
	return err
}

// POAUpdateParams
func (tn *ChainNode) POAUpdateParams(ctx context.Context, keyName, valoper string, admins []string, gracefulExit bool) error {
	gracefulParam := "true"
	if !gracefulExit {
		gracefulParam = "false"
	}

	// admin1,admin2,admin3
	adminList := ""
	for _, admin := range admins {
		adminList += admin + ","
	}
	adminList = adminList[:len(adminList)-1]

	_, err := tn.ExecTx(ctx,
		keyName, "poa", "update-params", adminList, gracefulParam,
	)
	return err
}

// POAUpdateStakingParams
func (tn *ChainNode) POAUpdateStakingParams(ctx context.Context, keyName string, sp stakingtypes.Params) error {
	command := []string{"tx", "poa", "update-staking-params",
		sp.UnbondingTime.String(),
		fmt.Sprintf("%d", sp.MaxValidators),
		fmt.Sprintf("%d", sp.MaxEntries),
		fmt.Sprintf("%d", sp.HistoricalEntries),
		sp.BondDenom,
		fmt.Sprintf("%d", sp.MinCommissionRate),
	}

	_, err := tn.ExecTx(ctx,
		keyName, command...,
	)
	return err
}

// POAQueryParams performs a query to get the POA module parameters
func (c *CosmosChain) POAQueryParams(ctx context.Context, addr string) (poa.Params, error) {
	res, err := poa.NewQueryClient(c.GetNode().GrpcConn).Params(ctx, &poa.QueryParamsRequest{})
	return res.Params, err
}

// POAQueryConsensusPower performs a query to get the total consensus power of the active validator.
func (c *CosmosChain) POAQueryConsensusPower(ctx context.Context, addr string) (int64, error) {
	res, err := poa.NewQueryClient(c.GetNode().GrpcConn).ConsensusPower(ctx, &poa.QueryConsensusPowerRequest{
		ValidatorAddress: addr,
	})
	return res.ConsensusPower, err
}

// POAQueryPendingValidators performs a query to get the pending validators waiting to get into the active set by the admin.
func (c *CosmosChain) POAQueryPendingValidators(ctx context.Context, addr string) ([]poa.Validator, error) {
	res, err := poa.NewQueryClient(c.GetNode().GrpcConn).PendingValidators(ctx, &poa.QueryPendingValidatorsRequest{})
	return res.Pending, err
}
