package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	sdkmath "cosmossdk.io/math"

	dockerapitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/util"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

type actions struct {
	ctx  context.Context
	ic   *interchaintest.Interchain
	vals map[string][]*cosmos.ChainNode
	cc   map[string]*cosmos.CosmosChain

	relayer ibc.Relayer
	eRep    ibc.RelayerExecReporter

	authKey string
}

type ActionHandler struct {
	ChainId   string `json:"chain_id"`
	NodeIndex int    `json:"node_index"`
	Action    string `json:"action"`
	Cmd       string `json:"cmd"`
	AuthKey   string `json:"auth_key,omitempty"`
}

func NewActions(
	ctx context.Context, ic *interchaintest.Interchain,
	cosmosChains map[string]*cosmos.CosmosChain, vals map[string][]*cosmos.ChainNode,
	relayer ibc.Relayer, eRep ibc.RelayerExecReporter,
	authKey string,
) *actions {
	return &actions{
		ctx:     ctx,
		ic:      ic,
		vals:    vals,
		cc:      cosmosChains,
		relayer: relayer,
		eRep:    eRep,
		authKey: authKey,
	}
}

func (a *actions) PostActions(w http.ResponseWriter, r *http.Request) {
	var ah ActionHandler
	err := json.NewDecoder(r.Body).Decode(&ah)
	if err != nil {
		util.WriteError(w, fmt.Errorf("failed to decode json: %s", err))
		return
	}

	if a.authKey != "" && ah.AuthKey != a.authKey {
		util.WriteError(w, fmt.Errorf("invalid `auth_key`"))
		return
	}

	action := ah.Action
	if action == "kill-all" {
		KillAll(a.ctx, a.ic, a.vals, a.relayer, a.eRep)
		return
	}

	chainId := ah.ChainId
	if _, ok := a.vals[chainId]; !ok {
		util.Write(w, []byte(fmt.Sprintf(`{"error":"chain_id '%s' not found. Chains %v"}`, chainId, a.vals[chainId])))
		return
	}

	if len(a.vals[chainId]) <= ah.NodeIndex {
		util.Write(w, []byte(fmt.Sprintf(`{"error":"node_index '%d' not found. nodes: %v"}`, ah.NodeIndex, len(a.vals[chainId]))))
		return
	}

	val := a.vals[chainId][ah.NodeIndex]

	ah.Cmd = strings.ReplaceAll(ah.Cmd, "%RPC%", fmt.Sprintf("tcp://%s:26657", val.HostName()))
	ah.Cmd = strings.ReplaceAll(ah.Cmd, "%CHAIN_ID%", ah.ChainId)
	ah.Cmd = strings.ReplaceAll(ah.Cmd, "%HOME%", val.HomeDir())

	cmd := strings.Split(ah.Cmd, " ")

	// Output can only ever be 1 thing. So we check which is set, then se the output to the user.
	var output []byte
	var stdout, stderr []byte

	// parse out special commands if there are any.
	cmdMap := make(map[string]string)
	if strings.Contains(ah.Cmd, "=") {
		for _, c := range strings.Split(ah.Cmd, ";") {
			s := strings.Split(c, "=")
			cmdMap[s[0]] = s[1]
		}
	}

	// Node / Docker Linux Actions
	switch action {
	case "q", "query":
		stdout, stderr, err = val.ExecQuery(a.ctx, cmd...)
	case "b", "bin", "binary":
		stdout, stderr, err = val.ExecBin(a.ctx, cmd...)
	case "e", "exec", "execute":
		stdout, stderr, err = val.Exec(a.ctx, cmd, []string{})
	case "recover-key":
		kn := cmdMap["keyname"]
		if err := val.RecoverKey(a.ctx, kn, cmdMap["mnemonic"]); err != nil {
			if !strings.Contains(err.Error(), "aborted") {
				util.WriteError(w, fmt.Errorf("failed to recover key: %s", err))
				return
			}
		}
		stdout = []byte(fmt.Sprintf(`{"recovered_key":"%s"}`, kn))
	case "overwrite-genesis-file":
		if err := val.OverwriteGenesisFile(a.ctx, []byte(cmdMap["new_genesis"])); err != nil {
			util.WriteError(w, fmt.Errorf("failed to override genesis file: %s", err))
			return
		}
		stdout = []byte(fmt.Sprintf(`{"overwrote_genesis_file":"%s"}`, val.ContainerID()))
	case "add-full-nodes":
		chain := a.cc[chainId]

		amt, err := strconv.Atoi(cmdMap["amount"])
		if err != nil {
			util.WriteError(w, fmt.Errorf("failed to convert amount to int: %s", err))
			return
		}

		if err := chain.AddFullNodes(a.ctx, nil, amt); err != nil {
			util.WriteError(w, fmt.Errorf("failed to add full nodes: %w", err))
			return
		}

		stdout = []byte(fmt.Sprintf(`{"added_full_node":"%s"}`, cmdMap["amount"]))
	case "dump-contract-state":
		stdout = dumpContractState(r, cmdMap, a, val)
	case "faucet":
		stdout = faucet(r, cmdMap, a.ctx, a, val)
	}

	// Relayer Actions if the above is not used.
	if len(stdout) == 0 && len(stderr) == 0 && err == nil {
		if err := a.relayerCheck(w, r); err != nil {
			return
		}

		switch action {
		case "stop-relayer", "stop_relayer", "stopRelayer":
			err = a.relayer.StopRelayer(a.ctx, a.eRep)

		case "start-relayer", "start_relayer", "startRelayer":
			paths := strings.FieldsFunc(ah.Cmd, func(c rune) bool {
				return c == ',' || c == ' '
			})
			err = a.relayer.StartRelayer(a.ctx, a.eRep, paths...)

		case "relayer", "relayer-exec", "relayer_exec", "relayerExec":
			if !strings.Contains(ah.Cmd, "--home") {
				// does this ever change for any other relayer?
				cmd = append(cmd, "--home", "/home/relayer")
			}

			res := a.relayer.Exec(a.ctx, a.eRep, cmd, []string{})
			stdout = []byte(res.Stdout)
			stderr = []byte(res.Stderr)
			err = res.Err

		case "get_channels", "get-channels", "getChannels":
			res, err := a.relayer.GetChannels(a.ctx, a.eRep, chainId)
			if err != nil {
				util.WriteError(w, err)
				return
			}

			r, err := json.Marshal(res)
			if err != nil {
				util.WriteError(w, err)
				return
			}
			stdout = r
		}
	}

	if len(stdout) > 0 {
		output = stdout
	} else if len(stderr) > 0 {
		output = stderr
	} else if err == nil {
		output = []byte("{}")
	} else {
		output = []byte(fmt.Sprintf(`%s`, err))
	}

	// Send the response
	util.Write(w, []byte(output))
}

func (a *actions) relayerCheck(w http.ResponseWriter, r *http.Request) error {
	var err error = nil

	if a.relayer == nil {
		util.Write(w, []byte(`{"error":"relayer not configured for this setup"}`))
		err = fmt.Errorf("relayer not configured for this setup")
	}

	return err
}

func KillAll(ctx context.Context, ic *interchaintest.Interchain, vals map[string][]*cosmos.ChainNode, relayer ibc.Relayer, eRep ibc.RelayerExecReporter) {
	fmt.Println("KillAll: Stopping all containers")
	for _, v := range vals {
		fmt.Println("KillAll: Stopping chain contains", v)
		for _, c := range v {
			fmt.Printf("KillAll: Stopping container %s\n", c.ContainerID())
			go c.StopContainer(ctx) // nolint:errcheck
		}
	}

	if relayer != nil {
		fmt.Println("KillAll: Pausing relayer")
		relayer.PauseRelayer(ctx)
		fmt.Println("KillAll: Stopping relayer")
		relayer.StopRelayer(ctx, eRep)
	}

	ic.Close()
	<-ctx.Done()
}

func KillAllLocalICContainers(ctx context.Context) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	containers, err := cli.ContainerList(ctx, dockerapitypes.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Println("Killing all local-interchain containers")
	for _, container := range containers {

		if len(container.Names) == 0 {
			continue
		}

		name := strings.ToLower(container.Names[0])
		name = strings.TrimPrefix(name, "/")

		if !(strings.HasPrefix(name, "ict-") || strings.HasPrefix(name, "interchaintest-") || strings.HasPrefix(name, "ictrelay-")) {
			// fmt.Println("Skipping container", name, "as it is not ict")
			continue
		}

		fmt.Printf("  - %s\n", name)

		inspected, err := cli.ContainerInspect(ctx, container.ID)
		if err != nil {
			panic(err)
		}

		if inspected.State.Running {
			if err := cli.ContainerKill(ctx, container.ID, "SIGKILL"); err != nil {
				panic(err)
			}
		}

	}
}

func dumpContractState(r *http.Request, cmdMap map[string]string, a *actions, val *cosmos.ChainNode) []byte {
	contract, ok1 := cmdMap["contract"]
	height, ok2 := cmdMap["height"]
	if !ok1 || !ok2 {
		return []byte(`{"error":"'contract' or 'height' not found in commands"}`)
	}

	heightInt, err := strconv.ParseInt(height, 10, 64)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error":"failed to convert height to int: %s"}`, height))
	}

	state, err := val.DumpContractState(a.ctx, contract, heightInt)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error":"%s"}`, err))
	}

	jsonRes, err := json.Marshal(state.Models)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error":"%s"}`, err))
	}

	return jsonRes
}

func faucet(r *http.Request, cmdMap map[string]string, ctx context.Context, a *actions, val *cosmos.ChainNode) []byte {
	amount, ok1 := cmdMap["amount"]
	toAddr, ok2 := cmdMap["address"]

	if !ok1 || !ok2 {
		return []byte(`{"error":"'amount' or 'address' not found in commands"}`)
	}

	amt, ok := sdkmath.NewIntFromString(amount)
	if !ok {
		return []byte(fmt.Sprintf(`{"error":"failed to convert amount to int: %s"}`, amount))
	}

	if err := val.BankSend(ctx, "faucet", ibc.WalletAmount{
		Address: toAddr,
		Amount:  amt,
		Denom:   val.Chain.Config().Denom,
	}); err != nil {
		return []byte(fmt.Sprintf(`{"error":"%s"}`, err))
	}

	return []byte(fmt.Sprintf(`{"sent_funds":"%s"}`, amount))
}
