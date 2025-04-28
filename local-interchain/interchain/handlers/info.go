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

	types "github.com/strangelove-ventures/interchaintest/local-interchain/interchain/types"
	util "github.com/strangelove-ventures/interchaintest/local-interchain/interchain/util"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

type info struct {
	Config     *types.Config
	InstallDir string

	// used to get information about state of the container
	ctx     context.Context
	ic      *interchaintest.Interchain
	vals    map[string][]*cosmos.ChainNode
	relayer ibc.Relayer
	eRep    ibc.RelayerExecReporter

	cc map[string]*cosmos.CosmosChain

	chainId string
}

func NewInfo(
	cfg *types.Config,
	installDir string,
	ctx context.Context,
	ic *interchaintest.Interchain,
	cosmosChains map[string]*cosmos.CosmosChain,
	vals map[string][]*cosmos.ChainNode,
	relayer ibc.Relayer,
	eRep ibc.RelayerExecReporter,
) *info {
	return &info{
		Config:     cfg,
		InstallDir: installDir,

		ctx:     ctx,
		ic:      ic,
		vals:    vals,
		cc:      cosmosChains,
		relayer: relayer,
		eRep:    eRep,
	}
}

type GetInfo struct {
	Logs   types.MainLogs `json:"logs" yaml:"logs"`
	Chains []types.Chain  `json:"chains" yaml:"chains"`
	Relay  types.Relayer  `json:"relayer" yaml:"relayer"`
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

	nodeIdx, ok := form["node_index"]
	if !ok {
		nodeIdx = []string{"0"}
	}

	idx, err := strconv.Atoi(nodeIdx[0])
	if err != nil {
		util.WriteError(w, fmt.Errorf("failed to convert node_index to int: %w", err))
		return
	}

	if len(i.vals[i.chainId]) <= idx {
		util.WriteError(w, fmt.Errorf("node_index '%d' not found. nodes: %v", idx, len(i.vals[i.chainId])))
		return
	}

	val := i.vals[i.chainId][idx]

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
	case "build_information":
		getBuildInfo(w, r, i, val)
	case "genesis_file_content":
		v, _ := val.GenesisFileContent(i.ctx)
		util.Write(w, v)
	case "peer":
		util.Write(w, getPeer(i.ctx, val))
	default:
		util.WriteError(w, fmt.Errorf("invalid get param: %s. does not exist", res[0]))
	}
}

func config(w http.ResponseWriter, r *http.Request, val *cosmos.ChainNode) {
	cfg := val.Chain.Config()
	jsonRes, err := MarshalIBCChainConfig(cfg)
	if err != nil {
		util.WriteError(w, fmt.Errorf("failed to marshal config: %w", err))
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

	// hide mnemonics from query
	chains := i.Config.Chains
	for idx, chain := range chains {
		updatedAccounts := []types.GenesisAccount{}

		for _, acc := range chain.Genesis.Accounts {
			acc.Mnemonic = "hidden"
			updatedAccounts = append(updatedAccounts, acc)
		}

		chain.Genesis.Accounts = updatedAccounts
		chains[idx] = chain
	}

	info := GetInfo{
		Logs:   logs,
		Chains: chains,
		Relay:  *i.Config.Relayer,
	}

	jsonRes, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		util.WriteError(w, err)
		return
	}

	util.Write(w, jsonRes)
}

func getPeer(ctx context.Context, val *cosmos.ChainNode) []byte {
	peer, err := val.NodeID(ctx)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error":"%s"}`, err))
	}

	return []byte(peer + "@" + val.Chain.GetHostPeerAddress())
}
