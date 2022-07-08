package dockerutil

import "fmt"

func HandleNodeJobError(exitCode int, stdout, stderr string, err error) error {
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("container returned non-zero error code: %d\nstdout: %s\nstderr: %s", exitCode, stdout, stderr)
	}
	return nil
}
