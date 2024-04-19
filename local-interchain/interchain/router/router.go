package router

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	ictypes "github.com/strangelove-ventures/localinterchain/interchain/types"
	"github.com/strangelove-ventures/localinterchain/interchain/util"

	"github.com/strangelove-ventures/localinterchain/interchain/handlers"
)

type Route struct {
	Path    string   `json:"path" yaml:"path"`
	Methods []string `json:"methods" yaml:"methods"`
}

func NewRouter(
	ctx context.Context,
	ic *interchaintest.Interchain,
	config *ictypes.Config,
	cosmosChains map[string]*cosmos.CosmosChain,
	vals map[string][]*cosmos.ChainNode,
	relayer ibc.Relayer,
	authKey string,
	eRep ibc.RelayerExecReporter,
	installDir string,
) *mux.Router {
	r := mux.NewRouter()

	infoH := handlers.NewInfo(config, installDir, ctx, ic, cosmosChains, vals, relayer, eRep)
	r.HandleFunc("/info", infoH.GetInfo).Methods(http.MethodGet)

	actionsH := handlers.NewActions(ctx, ic, cosmosChains, vals, relayer, eRep, authKey)
	r.HandleFunc("/", actionsH.PostActions).Methods(http.MethodPost)

	uploaderH := handlers.NewUploader(ctx, vals, authKey)
	r.HandleFunc("/upload", uploaderH.PostUpload).Methods(http.MethodPost)

	availableRoutes := getAllMethods(*r)
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		jsonRes, err := json.MarshalIndent(availableRoutes, "", "  ")
		if err != nil {
			util.WriteError(w, err)
			return
		}
		util.Write(w, jsonRes)
	}).Methods(http.MethodGet)

	return r
}

func getAllMethods(r mux.Router) []Route {
	endpoints := make([]Route, 0)

	err := r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		tpl, err1 := route.GetPathTemplate()
		met, err2 := route.GetMethods()
		if err1 != nil {
			return err1
		}
		if err2 != nil {
			return err2
		}

		// fmt.Println(tpl, met)
		endpoints = append(endpoints, Route{
			Path:    tpl,
			Methods: met,
		})
		return nil
	})

	if err != nil {
		panic(err)
	}

	return endpoints
}
