package notifications

import "os"

// getenvDefault returns the environment variable value or a fallback string.
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
