package query

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

const (
	cacheMaxEntries = 128
	cacheTTL        = 1 * time.Hour
)

type cacheEntry struct {
	result    QueryResult
	mtimes    map[string]time.Time // file path → mtime at query time
	createdAt time.Time
}

func (e *cacheEntry) expired() bool {
	return time.Since(e.createdAt) > cacheTTL
}

// filesChanged returns true if any file has been modified since the cache entry was created.
func (e *cacheEntry) filesChanged() bool {
	for path, cachedMtime := range e.mtimes {
		info, err := os.Stat(path)
		if err != nil {
			return true // file deleted or inaccessible
		}
		if info.ModTime().After(cachedMtime) {
			return true
		}
	}
	return false
}

var (
	queryCache   = make(map[string]*cacheEntry)
	cacheMu      sync.Mutex
	cacheHits    int64
	cacheMisses  int64
)

// cacheKey computes a deterministic hash of question + mode + sorted file paths.
// File contents are NOT included — we use mtime-based invalidation instead.
func cacheKey(question string, files []string, mode string) string {
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)

	h := sha256.New()
	fmt.Fprintf(h, "q:%s\nm:%s\n", question, mode)
	for _, f := range sorted {
		fmt.Fprintf(h, "f:%s\n", f)
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// cacheGet returns a cached result if valid, or nil.
func cacheGet(key string) *QueryResult {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	entry, ok := queryCache[key]
	if !ok {
		cacheMisses++
		return nil
	}
	if entry.expired() || entry.filesChanged() {
		delete(queryCache, key)
		cacheMisses++
		return nil
	}
	cacheHits++
	result := entry.result
	return &result
}

// cachePut stores a successful result.
func cachePut(key string, result QueryResult, mtimes map[string]time.Time) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Evict oldest entries if at capacity.
	if len(queryCache) >= cacheMaxEntries {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range queryCache {
			if oldestKey == "" || v.createdAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.createdAt
			}
		}
		delete(queryCache, oldestKey)
	}

	queryCache[key] = &cacheEntry{
		result:    result,
		mtimes:    mtimes,
		createdAt: time.Now(),
	}
}

// CacheStats returns cache hit/miss counts (for diagnostics).
func CacheStats() string {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	total := cacheHits + cacheMisses
	if total == 0 {
		return "cache: 0 queries"
	}
	hitRate := float64(cacheHits) / float64(total) * 100
	return fmt.Sprintf("cache: %d entries, %d hits, %d misses (%.0f%% hit rate)",
		len(queryCache), cacheHits, cacheMisses, hitRate)
}

// buildMtimes collects current modification times for files.
func buildMtimes(files []string) map[string]time.Time {
	mtimes := make(map[string]time.Time, len(files))
	for _, path := range files {
		info, err := os.Stat(path)
		if err == nil {
			mtimes[path] = info.ModTime()
		}
	}
	return mtimes
}

