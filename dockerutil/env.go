package dockerutil

import (
	"fmt"
	"os"
	"time"
)

// GetTimeFromEnv gets a time.Duration from a given environment variable key, or returns the fallback value if the key is not set.
func GetTimeFromEnv(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	v, err := time.ParseDuration(value)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to parse %s: %s", value, err))
	}
	return v
}
