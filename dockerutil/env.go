package dockerutil

import (
	"fmt"
	"os"
	"time"
)

// GetTimeFromEnv gets a time.Duration from a given environment variable key, or returns the fallback value if the key is not set.
func GetTimeFromEnv(key, fallback string) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		v, err := time.ParseDuration(fallback)
		if err != nil {
			panic(fmt.Sprintf("BUG: failed to parse fallback %s: %s", fallback, err))
		}
		return v
	}

	v, err := time.ParseDuration(value)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to parse %s: %s", value, err))
	}
	return v
}
