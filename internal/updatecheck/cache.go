package updatecheck

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Cache is the on-disk representation of the last update check.
type Cache struct {
	Version       int       `json:"version"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	LatestVersion string    `json:"latest_version"`
	LatestURL     string    `json:"latest_url"`
}

// DefaultCachePath returns ~/.grove/update-check.json.
func DefaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".grove", "update-check.json")
}

// ReadCacheFromPath reads the cache file. Missing or corrupt files yield a
// zero-value Cache and a nil error — callers treat zero Cache as "no data".
func ReadCacheFromPath(path string) (Cache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Cache{}, nil
		}
		return Cache{}, err
	}
	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		// Corrupt JSON is treated as missing — caller will refresh on next run.
		return Cache{}, nil
	}
	return c, nil
}

// WriteCacheToPath atomically writes the cache: write to path+".tmp", then rename.
// Creates the parent directory if it doesn't exist.
func WriteCacheToPath(path string, c Cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
