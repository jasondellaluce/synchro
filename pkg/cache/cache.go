package cache

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/go-github/v56/github"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	CacheFile = "cache.db"
	cacheTTL  = 24 * time.Hour
)

func GetCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".synchro", CacheFile), nil
}

var cacheableEndpoints = []*regexp.Regexp{
	regexp.MustCompile(`/repos/[^/]+/[^/]+/commits/[^/]+/pulls`),
	regexp.MustCompile(`/repos/[^/]+/[^/]+/commits`),
	regexp.MustCompile(`/repos/[^/]+/[^/]+/pulls/\d+$`),
	regexp.MustCompile(`/repos/[^/]+/[^/]+/commits/[^/]+/comments`),
}

type CacheEntry struct {
	Key       string `gorm:"primaryKey"`
	Value     []byte
	CreatedAt time.Time
}

type CachingTransport struct {
	db        *gorm.DB
	transport http.RoundTripper
}

func (t *CachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.transport.RoundTrip(req)
	}

	isCacheable := false
	for _, re := range cacheableEndpoints {
		if re.MatchString(req.URL.Path) {
			isCacheable = true
			break
		}
	}

	if !isCacheable {
		return t.transport.RoundTrip(req)
	}

	key := req.URL.String()
	var entry CacheEntry
	if err := t.db.First(&entry, "key = ?", key).Error; err == nil {
		if time.Since(entry.CreatedAt) < cacheTTL {
			resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(entry.Value)), req)
			if err == nil {
				return resp, nil
			}
		}
	}

	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	dump, err := httputil.DumpResponse(resp, true)
	if err == nil {
		t.db.Create(&CacheEntry{Key: key, Value: dump, CreatedAt: time.Now()})
	}

	return resp, nil
}

func NewCachingClient(client *github.Client) (*github.Client, error) {
	cachePath, err := GetCachePath()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(cachePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database: %w", err)
	}

	if err := db.AutoMigrate(&CacheEntry{}); err != nil {
		return nil, fmt.Errorf("failed to migrate cache schema: %w", err)
	}

	transport := client.Client().Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	cachingTransport := &CachingTransport{
		db:        db,
		transport: transport,
	}

	httpClient := &http.Client{
		Transport: cachingTransport,
	}

	return github.NewClient(httpClient), nil
}
