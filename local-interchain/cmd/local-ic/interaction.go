package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/localinterchain/interchain/handlers"
)

const (
	FlagAPIEndpoint = "api-address"
)

// old:
//
//	curl -X POST -H "Content-Type: application/json" -d '{
//		"chain_id": "localjuno-1",
//		"action": "query",
//		"cmd": "bank balances juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl"
//	  }' http://127.0.0.1:8080/
//
// new:
// local-ic interact localjuno-1 bin 'status --node=%RPC%'
// local-ic interact localjuno-1 query bank balances juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl
var interactCmd = &cobra.Command{
	Use:     "interact [chain_id] [interaction] [arguments...]",
	Short:   "Interact with a node",
	Args:    cobra.MinimumNArgs(3),
	Aliases: []string{"i"},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return GetFiles(), cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		ah := handlers.ActionHandler{
			ChainId: args[0],
			Action:  args[1],
			Cmd:     strings.Join(args[2:], " "),
		}

		apiAddr, _ := cmd.Flags().GetString(FlagAPIAddressOverride)

		res := makeHttpReq(apiAddr, ah)
		fmt.Println(res)
	},
}

func init() {
	interactCmd.Flags().String(FlagAPIAddressOverride, "http://127.0.0.1:8080", "override the default API address")
}

func makeHttpReq(apiEndpoint string, ah handlers.ActionHandler) string {
	client := &http.Client{}

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
