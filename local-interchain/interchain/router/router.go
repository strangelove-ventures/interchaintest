package router

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	ictypes "github.com/strangelove-ventures/interchaintest/local-interchain/interchain/types"
	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/util"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/handlers"
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

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	chainRegistryFile := filepath.Join(wd, "chain_registry.json")
	if _, err := os.Stat(chainRegistryFile); err == nil {
		crH := handlers.NewChainRegistry(chainRegistryFile)
		r.HandleFunc("/chain_registry", crH.GetChainRegistry).Methods(http.MethodGet)
	} else {
		log.Printf("chain_registry.json not found in %s, not exposing endpoint.", wd)
	}

	chainRegistryAssetsFile := filepath.Join(wd, "chain_registry_assets.json")
	if _, err := os.Stat(chainRegistryAssetsFile); err == nil {
		crH := handlers.NewChainRegistry(chainRegistryAssetsFile)
		r.HandleFunc("/chain_registry_assets", crH.GetChainRegistry).Methods(http.MethodGet)
	} else {
		log.Printf("chain_registry_assets.json not found in %s, not exposing endpoint.", wd)
	}

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
