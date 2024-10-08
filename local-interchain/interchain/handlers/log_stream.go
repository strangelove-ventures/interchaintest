package handlers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

const tailLines = 25

type LogStream struct {
	fName   string
	authKey string
	logger  *zap.Logger
}

func NewLogSteam(logger *zap.Logger, file string, authKey string) *LogStream {
	return &LogStream{
		fName:   file,
		authKey: authKey,
		logger:  logger,
	}
}

func (ls *LogStream) StreamLogs(w http.ResponseWriter, r *http.Request) {
	// Set headers to keep the connection open for SSE (Server-Sent Events)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// ensure ?auth_key=<authKey> is provided
	if r.URL.Query().Get("auth_key") != ls.authKey {
		http.Error(w, "Unauthorized, incorrect or no ?auth_key= provided", http.StatusUnauthorized)
		return
	}

	// Flush ensures data is sent to the client immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Open the log file
	file, err := os.Open(ls.fName)
	if err != nil {
		http.Error(w, "Unable to open log file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Seek to the end of the file to read only new log entries
	file.Seek(0, io.SeekEnd)

	// Read new lines from the log file
	reader := bufio.NewReader(file)

	// print last out to the user on request (i.e. new connections)
	tail := TailFile(ls.logger, ls.fName, tailLines)
	for _, line := range tail {
		fmt.Fprintf(w, "%s\n", line)
	}
	flusher.Flush()

	for {
		select {
		// In case client closes the connection, break out of loop
		case <-r.Context().Done():
			return
		default:
			// Try to read a line
			line, err := reader.ReadString('\n')
			if err == nil {
				// Send the log line to the client
				fmt.Fprintf(w, "%s\n", line)
				flusher.Flush() // Send to client immediately
			} else {
				// If no new log is available, wait for a short period before retrying
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func TailFile(logger *zap.Logger, logFile string, lines uint64) []string {
	// read the last n lines of a file
	file, err := os.Open(logFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	totalLines, err := lineCounter(file)
	if err != nil {
		log.Fatal(err)
	}

	if lines > uint64(totalLines) {
		lines = uint64(totalLines)
	}

	file.Seek(0, io.SeekStart)
	reader := bufio.NewReader(file)

	var logs []string
	for i := 0; i < int(totalLines)-int(lines); i++ {
		_, _, err := reader.ReadLine()
		if err != nil {
			logger.Fatal("error reading log file", zap.Error(err))
		}
	}

	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		logs = append(logs, string(line))
	}

	return logs
}

func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}
