package cosmos

import (
	"encoding/json"
	"fmt"

	govv1types "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/icza/dyno"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

func ModifyGenesisProposalTime(votingPeriod string, maxDepositPeriod string) func(ibc.ChainConfig, []byte) ([]byte, error) {
	return func(chainConfig ibc.ChainConfig, genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}

		govGsV1 := govv1types.GenesisState{}
		if err := json.Unmarshal(genbz, &govGsV1); err != nil {
			return nil, fmt.Errorf("failed to unmarshall genesis file: %w", err)
		}
		if govGsV1.Empty() {
			// if empty, the chain's gov module is on v1beta1, else it's on v1
			if err := dyno.Set(g, votingPeriod, "app_state", "gov", "voting_params", "voting_period"); err != nil {
				return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
			}
			if err := dyno.Set(g, maxDepositPeriod, "app_state", "gov", "deposit_params", "max_deposit_period"); err != nil {
				return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
			}
			if err := dyno.Set(g, chainConfig.Denom, "app_state", "gov", "deposit_params", "min_deposit", 0, "denom"); err != nil {
				return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
			}
		} else {
			if err := dyno.Set(g, votingPeriod, "app_state", "gov", "params", "voting_period"); err != nil {
				return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
			}
			if err := dyno.Set(g, maxDepositPeriod, "app_state", "gov", "params", "max_deposit_period"); err != nil {
				return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
			}
			if err := dyno.Set(g, chainConfig.Denom, "app_state", "gov", "params", "min_deposit", 0, "denom"); err != nil {
				return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
			}
		}
		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
