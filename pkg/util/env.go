package util

import "os"

// GetEnvOrDefault returns the environment variable value if set, otherwise the default value
func GetEnvOrDefault(env, def string) string {
	if val := os.Getenv(env); val != "" {
		return val
	}
	return def
}
