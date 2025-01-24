package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	dockertypes "github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v9/chain/cosmos"
	"go.uber.org/zap"
)

var removeColorRegex = regexp.MustCompile("\x1b\\[[0-9;]*m")

type ContainerStream struct {
	ctx      context.Context
	logger   *zap.Logger
	cli      *dockerclient.Client
	authKey  string
	testName string

	nameToID map[string]string
}

func NewContainerSteam(ctx context.Context, logger *zap.Logger, cli *dockerclient.Client, authKey, testName string, vals map[string][]*cosmos.ChainNode) *ContainerStream {
	nameToID := make(map[string]string)
	for _, nodes := range vals {
		for _, node := range nodes {
			nameToID[node.Name()] = node.ContainerID()
		}
	}

	return &ContainerStream{
		ctx:      ctx,
		authKey:  authKey,
		cli:      cli,
		logger:   logger,
		testName: testName,
		nameToID: nameToID,
	}
}

func (cs *ContainerStream) StreamContainer(w http.ResponseWriter, r *http.Request) {
	if err := VerifyAuthKey(cs.authKey, r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	containerID := r.URL.Query().Get("id")
	if containerID == "" {
		output := "No container ID provided. Available containers:\n"
		for name, id := range cs.nameToID {
			output += fmt.Sprintf("- %s: %s\n", name, id)
		}

		fmt.Fprint(w, output)
		fmt.Fprint(w, "Provide a container ID with ?id=<containerID>")
		return
	}

	// if container id is in the cs.nameToID map, use the mapped container ID
	if id, ok := cs.nameToID[containerID]; ok {
		containerID = id
	} else {
		fmt.Fprintf(w, "Container ID %s not found\n", containerID)
		return
	}

	// http://127.0.0.1:8080/container_logs?id=<ID>&colored=true
	isColored := strings.HasPrefix(strings.ToLower(r.URL.Query().Get("colored")), "t")
	tailLines := tailLinesParam(r.URL.Query().Get("lines"))

	rr, err := cs.cli.ContainerLogs(cs.ctx, containerID, dockertypes.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Details:    true,
		Tail:       strconv.FormatUint(tailLines, 10),
	})
	if err != nil {
		http.Error(w, "Unable to get container logs", http.StatusInternalServerError)
		return
	}
	defer rr.Close()

	// Set headers to keep the connection open for SSE (Server-Sent Events)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Flush ensures data is sent to the client immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	for {
		buf := make([]byte, 8*1024)
		n, err := rr.Read(buf)
		if err != nil {
			break
		}

		text := string(buf[:n])
		if !isColored {
			text, err = removeAnsiColorCodesFromText(string(buf[:n]))
			if err != nil {
				http.Error(w, "Unable to remove ANSI color codes", http.StatusInternalServerError)
				return
			}
		}

		fmt.Fprint(w, cleanSpecialChars(text))
		flusher.Flush()
	}
}

func tailLinesParam(tailInput string) uint64 {
	if tailInput == "" {
		return defaultTailLines
	}

	tailLines, err := strconv.ParseUint(tailInput, 10, 64)
	if err != nil {
		return defaultTailLines
	}

	return tailLines
}

func removeAnsiColorCodesFromText(text string) (string, error) {
	return removeColorRegex.ReplaceAllString(text, ""), nil
}

func cleanSpecialChars(text string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}

		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, text)
}
