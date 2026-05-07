package updatecheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Cache is the on-disk representation of the last update check.
type Cache struct {
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
		return Cache{}, fmt.Errorf("updatecheck: cache read: %w", err)
	}
	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		// Corrupt JSON is treated as missing — caller will refresh on next run.
		return Cache{}, nil
	}
	return c, nil
}

// WriteCacheToPath atomically writes the cache via a uniquely-named temp file
// in the destination directory, then renames into place. The unique suffix
// avoids clobbers when multiple grove processes write concurrently. Creates
// the parent directory if it doesn't exist.
func WriteCacheToPath(path string, c Cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("updatecheck: cache mkdir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("updatecheck: cache marshal: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), "update-check-*.json.tmp")
	if err != nil {
		return fmt.Errorf("updatecheck: cache create temp: %w", err)
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("updatecheck: cache write: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("updatecheck: cache close: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return fmt.Errorf("updatecheck: cache rename: %w", err)
	}
	return nil
}
