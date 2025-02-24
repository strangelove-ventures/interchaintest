package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
)

func (node *ChainNode) GetConsumerChainByChainId(ctx context.Context, chainId string) (string, error) {
	chains, err := node.ListConsumerChains(ctx)
	if err != nil {
		return "", err
	}
	for _, chain := range chains.Chains {
		if chain.ChainID == chainId {
			return chain.ConsumerID, nil
		}
	}
	return "", fmt.Errorf("chain not found")
}

func (node *ChainNode) ListConsumerChains(ctx context.Context) (ListConsumerChainsResponse, error) {
	queryRes, _, err := node.ExecQuery(
		ctx,
		"provider", "list-consumer-chains",
	)
	if err != nil {
		return ListConsumerChainsResponse{}, err
	}

	var queryResponse ListConsumerChainsResponse
	err = json.Unmarshal([]byte(queryRes), &queryResponse)
	if err != nil {
		return ListConsumerChainsResponse{}, err
	}

	return queryResponse, nil
}

type ListConsumerChainsResponse struct {
	Chains     []ConsumerChain `json:"chains"`
	Pagination Pagination      `json:"pagination"`
}

type ConsumerChain struct {
	ChainID            string   `json:"chain_id"`
	ClientID           string   `json:"client_id"`
	TopN               int      `json:"top_N"`
	MinPowerInTopN     string   `json:"min_power_in_top_N"`
	ValidatorsPowerCap int      `json:"validators_power_cap"`
	ValidatorSetCap    int      `json:"validator_set_cap"`
	Allowlist          []string `json:"allowlist"`
	Denylist           []string `json:"denylist"`
	Phase              string   `json:"phase"`
	Metadata           Metadata `json:"metadata"`
	MinStake           string   `json:"min_stake"`
	AllowInactiveVals  bool     `json:"allow_inactive_vals"`
	ConsumerID         string   `json:"consumer_id"`
}

type Pagination struct {
	NextKey interface{} `json:"next_key"`
	Total   string      `json:"total"`
}

type Metadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Metadata    string `json:"metadata"`
}
