package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	case "name":
		util.Write(w, []byte(val.Name()))
	case "container_id":
		util.Write(w, []byte(val.ContainerID()))
	case "hostname":
		util.Write(w, []byte(val.HostName()))
	case "home_dir":
		util.Write(w, []byte(val.HomeDir()))
	case "read_file":
		if relPath, ok := form["relative_path"]; !ok {
			util.WriteError(w, fmt.Errorf("relPath not found in query params"))
			return
		} else {
			bz, err := val.ReadFile(i.ctx, relPath[0])
			if err != nil {
				util.WriteError(w, err)
				return
			}

			util.Write(w, bz)
		}
	case "height":
		height, _ := val.Height(i.ctx)
		util.Write(w, []byte(strconv.Itoa(int(height))))
	case "genesis_file_content":
		v, _ := val.GenesisFileContent(i.ctx)
		util.Write(w, v)
	default:
		util.WriteError(w, fmt.Errorf("invalid get param %s", res[0]))
	}
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
