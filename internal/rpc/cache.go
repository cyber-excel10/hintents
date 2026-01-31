// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go/xdr"
)

// HashLedgerKey generates a deterministic SHA-256 hash of a Stellar LedgerKey.
// This is used by verification scripts and potentially conflicting with internal hashing if different.
func HashLedgerKey(key xdr.LedgerKey) (string, error) {
	xdrBytes, err := key.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal LedgerKey to XDR: %w", err)
	}
	hash := sha256.Sum256(xdrBytes)
	return hex.EncodeToString(hash[:]), nil
}

const (
	CacheDirName = ".erst/cache"
	FilePerm     = 0600
	DirPerm      = 0700
)

type CachedEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"` // XDR value
	CreatedAt time.Time `json:"created_at"`
}

// GetCachePath returns the path to the cache directory, creating it if necessary
func GetCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	path := filepath.Join(home, CacheDirName)
	if err := os.MkdirAll(path, DirPerm); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	return path, nil
}

// getCacheKey returns the unique filename for a given ledger key
func getCacheKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// Get retrieves a value from the cache
func Get(key string) (string, bool, error) {
	cachePath, err := GetCachePath()
	if err != nil {
		return "", false, err
	}

	filename := filepath.Join(cachePath, getCacheKey(key)+".json")

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return "", false, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return "", false, fmt.Errorf("failed to read cache file: %w", err)
	}

	var entry CachedEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// If corrupted, return false (miss) and log warning
		logger.Logger.Warn("Cache file corrupted", "key", key, "error", err)
		return "", false, nil
	}

	return entry.Value, true, nil
}

// Set saves a value to the cache
func Set(key string, value string) error {
	cachePath, err := GetCachePath()
	if err != nil {
		return err
	}

	entry := CachedEntry{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	filename := filepath.Join(cachePath, getCacheKey(key)+".json")
	if err := os.WriteFile(filename, data, FilePerm); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Invalidate removes a specific key from the cache
func Invalidate(key string) error {
	cachePath, err := GetCachePath()
	if err != nil {
		return err
	}

	filename := filepath.Join(cachePath, getCacheKey(key)+".json")
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Cleanup removes cache files older than maxAge
func Cleanup(maxAge time.Duration) (int, error) {
	cachePath, err := GetCachePath()
	if err != nil {
		return 0, err
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return 0, err
	}

	removedCount := 0
	cutoff := time.Now().Add(-maxAge)

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		// Check file modification time first as a quick check
		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(cachePath, e.Name())

			// Double check content creation time if needed, but modtime is sufficient for simple TTL
			if err := os.Remove(filePath); err == nil {
				removedCount++
			}
		}
	}

	return removedCount, nil
}
