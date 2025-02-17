package router

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/moby/moby/client"
	ictypes "github.com/strangelove-ventures/interchaintest/local-interchain/interchain/types"
	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/util"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/handlers"
)

type Route struct {
	Path    string   `json:"path" yaml:"path"`
	Methods []string `json:"methods" yaml:"methods"`
}

type RouterConfig struct {
	ibc.RelayerExecReporter

	Config       *ictypes.Config
	CosmosChains map[string]*cosmos.CosmosChain
	Vals         map[string][]*cosmos.ChainNode
	Relayer      ibc.Relayer
	AuthKey      string
	InstallDir   string
	LogFile      string
	TestName     string
	Logger       *zap.Logger
	DockerClient *client.Client
}

func NewRouter(
	ctx context.Context,
	ic *interchaintest.Interchain,
	rc *RouterConfig,
) *mux.Router {
	r := mux.NewRouter()

	infoH := handlers.NewInfo(rc.Config, rc.InstallDir, ctx, ic, rc.CosmosChains, rc.Vals, rc.Relayer, rc.RelayerExecReporter)
	r.HandleFunc("/info", infoH.GetInfo).Methods(http.MethodGet)

	// interaction logs
	logStream := handlers.NewLogSteam(rc.Logger, rc.LogFile, rc.AuthKey)
	r.HandleFunc("/logs", logStream.StreamLogs).Methods(http.MethodGet)
	r.HandleFunc("/logs_tail", logStream.TailLogs).Methods(http.MethodGet) // ?lines=

	containerStream := handlers.NewContainerSteam(ctx, rc.Logger, rc.DockerClient, rc.AuthKey, rc.TestName, rc.Vals)
	r.HandleFunc("/container_logs", containerStream.StreamContainer).Methods(http.MethodGet) // ?container=<id>&colored=true&lines=10000

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

	actionsH := handlers.NewActions(ctx, ic, rc.CosmosChains, rc.Vals, rc.Relayer, rc.RelayerExecReporter, rc.AuthKey)
	r.HandleFunc("/", actionsH.PostActions).Methods(http.MethodPost)

	uploaderH := handlers.NewUploader(ctx, rc.Vals, rc.AuthKey)
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
