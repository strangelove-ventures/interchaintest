package handlers

import (
	"context"
	"fmt"
	"net/http"

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
	containerID := r.URL.Query().Get("container") // TODO: get from chain ID as well? (map chain ID to container ID somehow)
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

	rr, err := cs.cli.ContainerLogs(cs.ctx, containerID, dockertypes.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
		Details:    false,
	})
	if err != nil {
		http.Error(w, "Unable to get container logs", http.StatusInternalServerError)
		return
	}
	defer rr.Close()

	// // Set headers to keep the connection open for SSE (Server-Sent Events)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// ensure ?auth_key=<authKey> is provided
	// if ls.authKey != "" && r.URL.Query().Get("auth_key") != ls.authKey {
	// 	http.Error(w, "Unauthorized, incorrect or no ?auth_key= provided", http.StatusUnauthorized)
	// 	return
	// }

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
		// w.Write(buf[:n])
		fmt.Fprintf(w, "%s", buf[:n])
		flusher.Flush() // TODO: ?
	}

	// for {
	// 	select {
	// 	// In case client closes the connection, break out of loop
	// 	case <-r.Context().Done():
	// 		return
	// 	default:
	// 		// Try to read a line
	// 		line, err := reader.ReadString('\n')
	// 		if err == nil {
	// 			// Send the log line to the client
	// 			fmt.Fprintf(w, "%s\n", line)
	// 			flusher.Flush() // Send to client immediately
	// 		} else {
	// 			// If no new log is available, wait for a short period before retrying
	// 			time.Sleep(100 * time.Millisecond)
	// 		}
	// 	}
	// }
}

// func (ls *ContainerStream) TailLogs(w http.ResponseWriter, r *http.Request) {
// 	// ensure ?auth_key=<authKey> is provided
// 	if ls.authKey != "" && r.URL.Query().Get("auth_key") != ls.authKey {
// 		http.Error(w, "Unauthorized, incorrect or no ?auth_key= provided", http.StatusUnauthorized)
// 		return
// 	}

// 	var linesToTail uint64 = defaultTailLines
// 	tailInput := r.URL.Query().Get("lines")
// 	if tailInput != "" {
// 		tailLines, err := strconv.ParseUint(tailInput, 10, 64)
// 		if err != nil {
// 			http.Error(w, "Invalid lines input", http.StatusBadRequest)
// 			return
// 		}
// 		linesToTail = tailLines
// 	}

// 	logs := TailFile(ls.logger, ls.fName, linesToTail)
// 	for _, log := range logs {
// 		fmt.Fprintf(w, "%s\n", log)
// 	}
// }
