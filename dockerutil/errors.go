package dockerutil

import "fmt"

func HandleNodeJobError(exitCode int, stdout, stderr string, err error) error {
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("container returned non-zero error code: %d\n", exitCode)
	}
	return nil
}
