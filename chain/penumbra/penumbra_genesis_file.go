package penumbra

import (
	"time"
)

type PenumbraGenesisFile struct {
	GenesisTime     string                         `json:"genesis_time"`
	ChainID         string                         `json:"chain_id"`
	InitialHeight   string                         `json:"initial_height"`
	ConsensusParams PenumbraGenesisConsensusParams `json:"consensus_params"`
	Validators      []string                       `json:"validators"`
	AppHash         string                         `json:"app_hash"`
	AppState        PenumbraGenesisAppState        `json:"app_state"`
}

type PenumbraGenesisConsensusParams struct {
	Block     PenumbraGenesisConsensusParamsBlock     `json:"block"`
	Evidence  PenumbraGenesisConsensusParamsEvidence  `json:"evidence"`
	Validator PenumbraGenesisConsensusParamsValidator `json:"validator"`
}

type PenumbraGenesisConsensusParamsBlock struct {
	MaxBytes   string `json:"max_bytes"`
	MaxGas     string `json:"max_gas"`
	TimeIotaMs string `json:"time_iota_ms"`
}

type PenumbraGenesisConsensusParamsEvidence struct {
	MaxAgeNumBlocks string `json:"max_age_num_blocks"`
	MaxAgeDuration  string `json:"max_age_duration"`
	MaxBytes        string `json:"max_bytes"`
}

type PenumbraGenesisConsensusParamsValidator struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

type PenumbraGenesisAppState struct {
	ChainParams PenumbraGenesisAppStateChainParams  `json:"chain_params"`
	Validators  []PenumbraValidatorDefinition       `json:"validators"`
	Allocations []PenumbraGenesisAppStateAllocation `json:"allocations"`
}

type PenumbraGenesisAppStateChainParams struct {
	ChainID                       string `json:"chain_id"`
	EpochDuration                 int64  `json:"epoch_duration"`
	UnbondingEpochs               int64  `json:"unbonding_epochs"`
	ActiveValidatorLimit          int64  `json:"active_validator_limit"`
	BaseRewardRate                int64  `json:"base_reward_rate"`
	SlashingPenaltyMisbehaviorBPS int64  `json:"slashing_penalty_misbehavior_bps"`
	SlashingPenaltyDowntimeBPS    int64  `json:"slashing_penalty_downtime_bps"`
	SignedBlocksWindowLen         int64  `json:"signed_blocks_window_len"`
	MissedBlocksMaximum           int64  `json:"missed_blocks_maximum"`
	IBCEnabled                    bool   `json:"ibc_enabled"`
	InboundICS20TransfersEnabled  bool   `json:"inbound_ics20_transfers_enabled"`
	OutboundICS20TransfersEnabled bool   `json:"outbound_ics20_transfers_enabled"`
}

type PenumbraValidatorDefinition struct {
	IdentityKey    string                           `json:"identity_key"`
	ConsensusKey   string                           `json:"consensus_key"`
	Name           string                           `json:"name"`
	Website        string                           `json:"website"`
	Description    string                           `json:"description"`
	FundingStreams []PenumbraValidatorFundingStream `json:"funding_streams"`
	SequenceNumber int64                            `json:"sequence_number"`
}

type PenumbraValidatorFundingStream struct {
	Address string `json:"address"`
	RateBPS int64  `json:"rate_bps"`
}

type PenumbraGenesisAppStateAllocation struct {
	Amount  int64  `json:"amount"`
	Denom   string `json:"denom"`
	Address string `json:"address"`
}

var testGenesisConsensusParams = PenumbraGenesisConsensusParams{
	Block: PenumbraGenesisConsensusParamsBlock{
		MaxBytes:   "22020096",
		MaxGas:     "-1",
		TimeIotaMs: "500",
	},
	Evidence: PenumbraGenesisConsensusParamsEvidence{
		MaxAgeNumBlocks: "100000",
		MaxAgeDuration:  "86400000000000",
		MaxBytes:        "1048576",
	},
	Validator: PenumbraGenesisConsensusParamsValidator{
		PubKeyTypes: []string{"ed25519"},
	},
}

func genesisAppStateChainParams(chainID string) PenumbraGenesisAppStateChainParams {
	return PenumbraGenesisAppStateChainParams{
		ChainID:                       chainID,
		EpochDuration:                 40,
		UnbondingEpochs:               40,
		ActiveValidatorLimit:          10,
		BaseRewardRate:                30000,
		SlashingPenaltyMisbehaviorBPS: 1000,
		SlashingPenaltyDowntimeBPS:    1,
		SignedBlocksWindowLen:         10000,
		MissedBlocksMaximum:           500,
		// TODO: change these to true when ready
		IBCEnabled:                    false,
		InboundICS20TransfersEnabled:  false,
		OutboundICS20TransfersEnabled: false,
	}
}

func newPenumbraGenesisFileJSON(
	chainID string,
	validators []PenumbraValidatorDefinition,
	allocations []PenumbraGenesisAppStateAllocation,
) PenumbraGenesisFile {
	return PenumbraGenesisFile{
		GenesisTime:     time.Now().UTC().Format(time.RFC3339),
		ChainID:         chainID,
		InitialHeight:   "0",
		ConsensusParams: testGenesisConsensusParams,
		Validators:      []string{},
		AppHash:         "",
		AppState: PenumbraGenesisAppState{
			ChainParams: genesisAppStateChainParams(chainID),
			Validators:  validators,
			Allocations: allocations,
		},
	}
}
