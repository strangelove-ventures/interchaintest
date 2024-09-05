package handlers

import (
	"net/http"
	"os"
)

// If the chain_registry.json file is found within the current running directory, show it as an endpoint.
// Used in: spawn

type chainRegistry struct {
	DataJSON []byte `json:"data_json"`
}

// NewChainRegistry creates a new chainRegistry with the JSON from the file at location.
func NewChainRegistry(loc string) *chainRegistry {
	dataJSON, err := os.ReadFile(loc)
	if err != nil {
		panic(err)
	}

	return &chainRegistry{
		DataJSON: dataJSON,
	}
}

func (cr chainRegistry) GetChainRegistry(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(cr.DataJSON); err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}
