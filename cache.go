package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const cacheDir = "cache"

// saveToCache creates the cache directory if needed, then JSON-encodes data and
// writes it to cache/<filename>.
func saveToCache(filename string, data any) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating %s: %v\n", cacheDir, err)
		return
	}
	path := filepath.Join(cacheDir, filename)
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error encoding %s: %v\n", path, err)
		return
	}
	if err := os.WriteFile(path, encoded, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", path, err)
	}
}

// loadFromCache reads cache/<filename> and JSON-decodes it into out. Returns false
// on a cold start (missing or unparseable file) so callers can fall back to computing
// fresh data.
func loadFromCache(filename string, out any) bool {
	path := filepath.Join(cacheDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(data, out); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing %s: %v\n", path, err)
		return false
	}
	return true
}
