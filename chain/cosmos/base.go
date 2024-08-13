package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/strangelove-ventures/interchaintest/v8/examples/cosmwasm/external_contracts/daodaocore"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
)

func (c *CosmosChain) SetupDAODAO(ctx context.Context,
	ibcPath string,
	keyName string) (any, error) {

	dao_proposal_single_code_id, err := c.StoreContract(ctx, keyName, "../../../external_contracts/daodao/dao_proposal_single.wasm")
	if err != nil {
		return nil, err
	}
	// Convert the proposal code id to an int
	proposal_code_id, err := strconv.Atoi(dao_proposal_single_code_id)
	if err != nil {
		return nil, err
	}

	dao_voting_token_staked_code_id, err := c.StoreContract(ctx, keyName, "../../../external_contracts/daodao/dao_voting_token_staked.wasm")
	if err != nil {
		return nil, err
	}

	user_address_bytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, err
	}
	user_address := string(user_address_bytes)

	core_instantiation_msg := daodaocore.InstantiateMsg{
		ImageUrl:     nil,
		InitialItems: []daodaocore.InitialItem{},
		Name:         "V2_DAO",
		ProposalModulesInstantiateInfo: []daodaocore.ModuleInstantiateInfo{
			{
				Admin: &daodaocore.Admin{
					Address:    nil,
					CoreModule: &daodaocore.Admin_CoreModule{},
				},
				CodeId: proposal_code_id,
				Funds:  []daodaocore.Coin{},
				Label:  "v2_dao",
				Msg:    "",
			},
		},
		VotingModuleInstantiateInfo: daodaocore.ModuleInstantiateInfo{},
		AutomaticallyAddCw721S:      true,
		AutomaticallyAddCw20S:       true,
		DaoUri:                      nil,
		Description:                 "V2_DAO",
		Admin:                       &user_address,
	}

	instantiate_msg, err := json.Marshal(core_instantiation_msg)

	if err != nil {
		return nil, err
	}

	dao_core, err := c.UploadAndInstantiateContract(ctx, keyName, "../../../external_contracts/daodao/dao_dao_core.wasm",
		string(instantiate_msg), "daodao_core", true,
	)
	if err != nil {
		return nil, err
	}

	log.Println(dao_proposal_single_code_id, dao_voting_token_staked_code_id, dao_core)

	return nil, nil
}

func (c *CosmosChain) SetupPolytone(
	ctx context.Context,
	r ibc.Relayer,
	eRep *testreporter.RelayerExecReporter,
	ibcPath string,
	keyName string,
	destinationChain *CosmosChain,
	destinationKeyName string,
) (*PolytoneInstantiation, error) {

	note, listener, err := c.SetupPolytoneSourceChain(ctx, keyName, destinationChain.Config().ChainID)

	if err != nil || note == nil || listener == nil {
		return nil, err
	}

	voice, err := destinationChain.SetupPolytoneDestinationChain(ctx, destinationKeyName, c.Config().ChainID)

	if err != nil || voice == nil {
		return nil, err
	}

	channelId, err := c.FinishPolytoneSetup(ctx, r, eRep, ibcPath, note.ContractInfo.IbcPortID, voice.ContractInfo.IbcPortID, destinationChain.Config().ChainID)
	if err != nil {
		return nil, err
	}

	return &PolytoneInstantiation{
		Note:      *note,
		Listener:  *listener,
		Voice:     *voice,
		ChannelID: channelId,
	}, nil
}

func (c *CosmosChain) SetupPolytoneDestinationChain(ctx context.Context, keyName string, sourceChainId string) (*ContractInfoResponse, error) {

	var block_gas_limit uint64
	queried_limit, err := c.GetBlockGasLimit(ctx)
	if err != nil {
		return nil, err
	}

	if queried_limit == nil {
		// Default to 100M gas limit
		block_gas_limit = uint64(100_000_000)
	} else {
		block_gas_limit = *queried_limit
	}

	proxy_code_id, err := c.StoreContract(
		ctx,
		keyName,
		"../../../external_contracts/polytone/v1.0.0/polytone_proxy.wasm")

	if err != nil {
		return nil, err
	}

	voice, err := c.UploadAndInstantiateContract(ctx, keyName,
		"../../../external_contracts/polytone/v1.0.0/polytone_voice.wasm",
		fmt.Sprintf("{\"proxy_code_id\":\"%s\", \"block_max_gas\":\"%d\"}", proxy_code_id, block_gas_limit),
		fmt.Sprintf("polytone_voice_from_%s", sourceChainId),
		true)

	if err != nil {
		return nil, err
	}

	return voice, nil

}

func (c *CosmosChain) SetupPolytoneSourceChain(ctx context.Context, keyName string, destinationChainId string) (*ContractInfoResponse, *ContractInfoResponse, error) {
	var block_gas_limit uint64
	queried_limit, err := c.GetBlockGasLimit(ctx)
	if err != nil {
		return nil, nil, err
	}

	if queried_limit == nil {
		// Default to 100M gas limit
		block_gas_limit = uint64(100_000_000)
	} else {
		block_gas_limit = *queried_limit
	}

	// Upload the note contract- it emits the ibc messages
	note, err := c.UploadAndInstantiateContract(ctx, keyName,
		"../../../external_contracts/polytone/v1.0.0/polytone_note.wasm",
		fmt.Sprintf(`{"block_max_gas":"%d"}`, block_gas_limit),
		fmt.Sprintf("polytone_note_to_%v", destinationChainId),
		true)

	if err != nil {
		return nil, nil, err
	}

	// Upload the listener contract- it listens for the ibc messages
	listener, err := c.UploadAndInstantiateContract(ctx, keyName,
		"../../../external_contracts/polytone/v1.0.0/polytone_listener.wasm",
		fmt.Sprintf("{\"note\":\"%s\"}", note.Address),
		fmt.Sprintf("polytone_listener_from_%v", destinationChainId),
		true)

	if err != nil {
		return nil, nil, err
	}

	return note, listener, nil
}

func (c *CosmosChain) FinishPolytoneSetup(ctx context.Context, r ibc.Relayer, eRep *testreporter.RelayerExecReporter, ibcPath string, notePortId string, voicePortId string, destChainId string) (string, error) {

	// Create the channel between the two contracts
	err := r.CreateChannel(ctx, eRep, ibcPath, ibc.CreateChannelOptions{
		SourcePortName: notePortId,
		DestPortName:   voicePortId,
		Order:          ibc.Unordered,
		Version:        "polytone-1",
	})
	if err != nil {
		return "", err
	}

	err = r.StopRelayer(ctx, eRep)
	if err != nil {
		return "", err
	}

	err = r.StartRelayer(ctx, eRep)
	if err != nil {
		return "", err
	}

	// Get the new channel
	channelsInfo, err := r.GetChannels(ctx, eRep, c.Config().ChainID)
	if err != nil {
		return "", err
	}

	channelId := channelsInfo[len(channelsInfo)-1].ChannelID

	return channelId, nil

}

type PolytoneInstantiation struct {
	Note      ContractInfoResponse
	Listener  ContractInfoResponse
	Voice     ContractInfoResponse
	ChannelID string
}
