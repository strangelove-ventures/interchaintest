package ibctest

import (
	"fmt"
	"os"
	"path/filepath"
)

// CreateLogFile creates a file with name in dir $HOME/.ibctest/logs/
func CreateLogFile(name string) (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("UserHomeDir: %w", err)
	}
	fpath := filepath.Join(home, ".ibctest", "logs")
	err = os.MkdirAll(fpath, 0755)
	if err != nil {
		return nil, fmt.Errorf("MkdirAll: %w", err)
	}
	return os.Create(filepath.Join(fpath, name))
}
