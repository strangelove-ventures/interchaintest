package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/util"
	"github.com/strangelove-ventures/interchaintest/v9/chain/cosmos"
)

type upload struct {
	ctx  context.Context
	vals map[string][]*cosmos.ChainNode

	authKey string
}

type Uploader struct {
	ChainId   string `json:"chain_id"`
	NodeIndex int    `json:"node_index"`
	FilePath  string `json:"file_path"`

	// Upload-Type: cosmwasm only
	KeyName string `json:"key_name,omitempty"`
	AuthKey string `json:"auth_key,omitempty"`
}

func NewUploader(ctx context.Context, vals map[string][]*cosmos.ChainNode, authKey string) *upload {
	return &upload{
		ctx:     ctx,
		vals:    vals,
		authKey: authKey,
	}
}

func (u *upload) PostUpload(w http.ResponseWriter, r *http.Request) {
	var upload Uploader
	err := json.NewDecoder(r.Body).Decode(&upload)
	if err != nil {
		util.WriteError(w, err)
		return
	}

	if u.authKey != "" && u.authKey != upload.AuthKey {
		util.WriteError(w, fmt.Errorf("invalid `auth_key`"))
		return
	}

	srcPath := upload.FilePath
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		util.WriteError(w, fmt.Errorf("file %s does not exist on the source machine", srcPath))
		return
	}

	chainId := upload.ChainId
	if _, ok := u.vals[chainId]; !ok {
		util.Write(w, []byte(fmt.Sprintf(`{"error":"chain_id %s not found"}`, chainId)))
		return
	}

	nodeIdx := upload.NodeIndex
	if len(u.vals[chainId]) <= nodeIdx {
		util.Write(w, []byte(fmt.Sprintf(`{"error":"node_index %d not found"}`, nodeIdx)))
		return
	}

	val := u.vals[chainId][nodeIdx]

	headerType := r.Header.Get("Upload-Type")
	switch headerType {
	case "cosmwasm":
		// Upload & Store the contract on chain.
		codeId, err := val.StoreContract(u.ctx, upload.KeyName, srcPath)
		if err != nil {
			util.WriteError(w, err)
			return
		}

		util.Write(w, []byte(fmt.Sprintf(`{"code_id":%s}`, codeId)))
		return
	default:
		// Upload the file to the docker volume (val[0]).
		_, file := filepath.Split(srcPath)
		if err := val.CopyFile(u.ctx, srcPath, file); err != nil {
			util.WriteError(w, fmt.Errorf(`{"error":"writing contract file to docker volume: %w"}`, err))
			return
		}

		home := val.HomeDir()
		fileLoc := filepath.Join(home, file)
		util.Write(w, []byte(fmt.Sprintf(`{"success":"file uploaded to %s","location":"%s"}`, chainId, fileLoc)))
	}
}
