package env

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

// LoadDotEnvIfPresent reads BANSHEE_* variables from a local .env file without overwriting non-empty process env vars.
func LoadDotEnvIfPresent(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !strings.HasPrefix(key, "BANSHEE_") {
			continue
		}
		val = stripMatchingQuotes(strings.TrimSpace(val))
		if val == "" {
			continue
		}
		if existing := os.Getenv(key); existing != "" {
			continue
		}
		_ = os.Setenv(key, val)
	}
	return scanner.Err()
}

func stripMatchingQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
