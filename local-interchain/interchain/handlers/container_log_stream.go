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
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"go.uber.org/zap"
)

type ContainerStream struct {
	ctx      context.Context
	logger   *zap.Logger
	cli      *dockerclient.Client
	authKey  string
	testName string
}

func NewContainerSteam(ctx context.Context, logger *zap.Logger, cli *dockerclient.Client, authKey, testName string) *ContainerStream {
	return &ContainerStream{
		ctx:      ctx,
		authKey:  authKey,
		cli:      cli,
		logger:   logger,
		testName: testName,
	}
}

func (cs *ContainerStream) StreamContainer(w http.ResponseWriter, r *http.Request) {
	// ensure ?auth_key=<authKey> is provided
	if cs.authKey != "" && r.URL.Query().Get("auth_key") != cs.authKey {
		http.Error(w, "Unauthorized, incorrect or no ?auth_key= provided", http.StatusUnauthorized)
		return
	}

	containerID := r.URL.Query().Get("id") // TODO: get from chain ID as well? (map chain ID to container ID somehow)
	if containerID == "" {
		// returns containers only for this testnet. other containers are not shown on this endpoint
		c, err := cs.cli.ContainerList(cs.ctx, dockertypes.ContainerListOptions{
			Filters: filters.NewArgs(filters.Arg("label", dockerutil.CleanupLabel+"="+cs.testName)),
		})
		if err != nil {
			http.Error(w, "Unable to get container list", http.StatusInternalServerError)
			return
		}

		availableContainers := []string{}
		for _, container := range c {
			availableContainers = append(availableContainers, container.ID)
		}

		output := "No container ID provided. Available containers:\n"
		for _, container := range availableContainers {
			output += fmt.Sprintf("- %s\n", container)
		}

		fmt.Fprint(w, output)
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
		buf := make([]byte, 1024)
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
	r, err := regexp.Compile("\x1b\\[[0-9;]*m")
	if err != nil {
		return "", err
	}

	return r.ReplaceAllString(text, ""), nil
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
