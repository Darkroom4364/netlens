package measure

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cache is a simple file-based cache that stores JSON blobs keyed by
// a SHA-256 hash. Files are stored as {Dir}/{key}.json.
type Cache struct {
	Dir string
}

// NewCache creates a Cache. If dir is empty, ~/.cache/netlens/ is used.
func NewCache(dir string) *Cache {
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache", "netlens")
	}
	return &Cache{Dir: dir}
}

// Key returns a hex-encoded SHA-256 hash of the joined parts.
func (c *Cache) Key(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, ":")))
	return fmt.Sprintf("%x", h)
}

// Has reports whether a cached entry exists for the given key.
func (c *Cache) Has(key string) bool {
	_, err := os.Stat(filepath.Join(c.Dir, key+".json"))
	return err == nil
}

// Load reads the cached JSON for the given key.
func (c *Cache) Load(key string) ([]byte, error) {
	return os.ReadFile(filepath.Join(c.Dir, key+".json"))
}

// Store writes data to the cache, creating the directory if needed.
func (c *Cache) Store(key string, data []byte) error {
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.Dir, key+".json"), data, 0o644)
}
