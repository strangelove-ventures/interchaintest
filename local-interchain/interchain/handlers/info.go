package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	types "github.com/strangelove-ventures/localinterchain/interchain/types"
	util "github.com/strangelove-ventures/localinterchain/interchain/util"
)

type info struct {
	Config     *types.Config
	InstallDir string

	// used to get information about state of the container
	ctx     context.Context
	ic      *interchaintest.Interchain
	vals    map[string]*cosmos.ChainNode
	relayer ibc.Relayer
	eRep    ibc.RelayerExecReporter

	chainId string
}

func NewInfo(cfg *types.Config, installDir string, ctx context.Context, ic *interchaintest.Interchain, vals map[string]*cosmos.ChainNode, relayer ibc.Relayer, eRep ibc.RelayerExecReporter) *info {
	return &info{
		Config:     cfg,
		InstallDir: installDir,

		ctx:     ctx,
		ic:      ic,
		vals:    vals,
		relayer: relayer,
		eRep:    eRep,
	}
}

type GetInfo struct {
	Logs   types.MainLogs `json:"logs"`
	Chains []types.Chain  `json:"chains"`
	Relay  types.Relayer  `json:"relayer"`
}

func (i *info) GetInfo(w http.ResponseWriter, r *http.Request) {
	form := r.URL.Query()

	res, ok := form["request"]
	if !ok {
		get_logs(w, r, i)
		return
	}

	chainId, ok := form["chain_id"]
	if !ok {
		util.WriteError(w, fmt.Errorf("chain_id not found in query params"))
		return
	}
	i.chainId = chainId[0]

	val := i.vals[i.chainId]

	switch res[0] {
	case "logs":
		get_logs(w, r, i)
	case "config":
		config(w, r, val)
	case "name":
		util.Write(w, []byte(val.Name()))
	case "container_id":
		util.Write(w, []byte(val.ContainerID()))
	case "hostname":
		util.Write(w, []byte(val.HostName()))
	case "home_dir":
		util.Write(w, []byte(val.HomeDir()))
	case "is_above_sdk_47", "is_above_sdk_v47":
		util.Write(w, []byte(strconv.FormatBool(val.IsAboveSDK47(i.ctx))))
	case "has_command":
		hasCommand(w, r, form, i, val)
	case "read_file":
		readFile(w, r, form, i, val)
	case "height":
		height, _ := val.Height(i.ctx)
		util.Write(w, []byte(strconv.Itoa(int(height))))
	case "dump_contract_state":
		dumpContractState(w, r, form, i, val)
	case "query_proposal":
		queryProposal(w, r, form, i, val)
	case "build_information":
		getBuildInfo(w, r, i, val)
	case "genesis_file_content":
		v, _ := val.GenesisFileContent(i.ctx)
		util.Write(w, v)
	default:
		util.WriteError(w, fmt.Errorf("invalid get param: %s. does not exist", res[0]))
	}
}

func config(w http.ResponseWriter, r *http.Request, val *cosmos.ChainNode) {
	cfg := val.Chain.Config()

	type Alias struct {
		Type           string  `json:"type"`
		Name           string  `json:"name"`
		ChainID        string  `json:"chain_id"`
		Bin            string  `json:"bin"`
		Bech32Prefix   string  `json:"bech32_prefix"`
		Denom          string  `json:"denom"`
		CoinType       string  `json:"coin_type"`
		GasPrices      string  `json:"gas_prices"`
		GasAdjustment  float64 `json:"gas_adjustment"`
		TrustingPeriod string  `json:"trusting_period"`
	}

	alias := Alias{
		Type:           cfg.Type,
		Name:           cfg.Name,
		ChainID:        cfg.ChainID,
		Bin:            cfg.Bin,
		Bech32Prefix:   cfg.Bech32Prefix,
		Denom:          cfg.Denom,
		CoinType:       cfg.CoinType,
		GasPrices:      cfg.GasPrices,
		GasAdjustment:  cfg.GasAdjustment,
		TrustingPeriod: cfg.TrustingPeriod,
	}

	jsonRes, err := json.MarshalIndent(alias, "", "  ")
	if err != nil {
		util.WriteError(w, err)
		return
	}

	util.Write(w, []byte(jsonRes))
}

func hasCommand(w http.ResponseWriter, r *http.Request, form url.Values, i *info, val *cosmos.ChainNode) {
	cmd, ok := form["command"]
	if !ok {
		util.WriteError(w, fmt.Errorf("command not found in query params"))
		return
	}

	util.Write(w, []byte(strconv.FormatBool(val.HasCommand(i.ctx, cmd[0]))))
}

func readFile(w http.ResponseWriter, r *http.Request, form url.Values, i *info, val *cosmos.ChainNode) {
	relPath, ok := form["relative_path"]
	if !ok {
		util.WriteError(w, fmt.Errorf("relPath not found in query params"))
		return
	}

	bz, err := val.ReadFile(i.ctx, relPath[0])
	if err != nil {
		util.WriteError(w, err)
		return
	}

	util.Write(w, bz)
}

func dumpContractState(w http.ResponseWriter, r *http.Request, form url.Values, i *info, val *cosmos.ChainNode) {
	contract, ok1 := form["contract"]
	height, ok2 := form["height"]
	if !ok1 || !ok2 {
		util.WriteError(w, fmt.Errorf("contract or height not found in query params"))
		return
	}

	heightInt, err := strconv.ParseInt(height[0], 10, 64)
	if err != nil {
		util.WriteError(w, err)
		return
	}

	state, err := val.DumpContractState(i.ctx, contract[0], heightInt)
	if err != nil {
		util.WriteError(w, err)
		return
	}

	jsonRes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		util.WriteError(w, err)
		return
	}
	util.Write(w, []byte(jsonRes))
}

func queryProposal(w http.ResponseWriter, r *http.Request, form url.Values, i *info, val *cosmos.ChainNode) {
	if proposalID, ok := form["proposal_id"]; !ok {
		util.WriteError(w, fmt.Errorf("proposal not found in query params"))
		return
	} else {
		propResp, err := val.QueryProposal(i.ctx, proposalID[0])
		if err != nil {
			util.WriteError(w, err)
			return
		}

		jsonRes, err := json.MarshalIndent(propResp, "", "  ")
		if err != nil {
			util.WriteError(w, err)
			return
		}

		util.Write(w, jsonRes)
	}
}

func getBuildInfo(w http.ResponseWriter, r *http.Request, i *info, val *cosmos.ChainNode) {
	bi := val.GetBuildInformation(i.ctx)
	jsonRes, err := json.MarshalIndent(bi, "", "  ")
	if err != nil {
		util.WriteError(w, err)
		return
	}
	util.Write(w, []byte(jsonRes))
}

func get_logs(w http.ResponseWriter, r *http.Request, i *info) {
	fp := filepath.Join(i.InstallDir, "configs", "logs.json")

	bz, err := os.ReadFile(fp)
	if err != nil {
		util.WriteError(w, err)
		return
	}

	var logs types.MainLogs
	if err := json.Unmarshal(bz, &logs); err != nil {
		util.WriteError(w, err)
		return
	}

	info := GetInfo{
		Logs:   logs,
		Chains: i.Config.Chains,
		Relay:  i.Config.Relayer,
	}

	jsonRes, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		util.WriteError(w, err)
		return
	}

	util.Write(w, jsonRes)
}
