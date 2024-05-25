package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/interchaintest/localinterchain/interchain/handlers"
)

const (
	FlagAPIEndpoint = "api-endpoint"
	FlagNodeIndex   = "node-index"
)

func init() {
	interactCmd.Flags().String(FlagAPIAddressOverride, "http://127.0.0.1:8080", "override the default API address")
	interactCmd.Flags().String(FlagAuthKey, "a", "auth key to use")
	interactCmd.Flags().IntP(FlagNodeIndex, "n", 0, "node index to interact with")
}

var interactCmd = &cobra.Command{
	Use:   "interact [chain_id] [interaction] [arguments...]",
	Short: "Interact with a node",
	Example: `  local-ic interact localcosmos-1 bin 'status --node=%RPC%' --api-endpoint=http://127.0.0.1:8080
  local-ic interact localcosmos-1 query bank balances cosmos1hj5fveer5cjtn4wd6wstzugjfdxzl0xpxvjjvr
  local-ic interact localcosmos-1 get_channels
  local-ic interact localcosmos-1 relayer-exec rly q channels localcosmos-1
`,
	Args:    cobra.MinimumNArgs(2),
	Aliases: []string{"i"},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return GetFiles(), cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {

		ah := &handlers.ActionHandler{
			ChainId: args[0],
			Action:  args[1],
		}

		if len(args) > 2 {
			ah.Cmd = strings.Join(args[2:], " ")
		}

		authKey, err := cmd.Flags().GetString(FlagAuthKey)
		if err != nil {
			panic(err)
		}

		nodeIdx, err := cmd.Flags().GetInt(FlagNodeIndex)
		if err != nil {
			panic(err)
		}

		ah.AuthKey = authKey
		ah.NodeIndex = nodeIdx

		apiAddr, err := cmd.Flags().GetString(FlagAPIAddressOverride)
		if err != nil {
			panic(err)
		}

		res := makeHttpReq(apiAddr, ah)
		fmt.Println(res)
	},
}

func makeHttpReq(apiEndpoint string, ah *handlers.ActionHandler) string {
	client := &http.Client{}

	//	curl -X POST -H "Content-Type: application/json" -d '{
	//		"chain_id": "localjuno-1",
	//		"action": "query",
	//		"cmd": "bank balances juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl"
	//	  }' http://127.0.0.1:8080/
	jsonData, err := json.Marshal(ah)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return string(bodyText)
}
