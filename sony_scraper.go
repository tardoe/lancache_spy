package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

const (
	prosperoPatchesURL = "https://www.prosperopatches.com/%s"
)

// SonyScraper scrapes prosperopatches.com to resolve Sony title IDs to game names
type SonyScraper struct {
	cache       sync.Map // map[string]string - titleID -> gameName
	httpClient  *http.Client
	rateLimiter chan struct{}   // Rate limit scraping
	ctx         context.Context // aborts rate-limit waits and requests on shutdown
}

// NewSonyScraper creates a new Sony scraper
func NewSonyScraper(ctx context.Context) *SonyScraper {
	// Rate limiter - max 1 request per second to be respectful
	rateLimiter := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
			select {
			case rateLimiter <- struct{}{}:
			default:
			}
		}
	}()

	return &SonyScraper{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: rateLimiter,
		ctx:         ctx,
	}
}

// GetGameName resolves a Sony title ID to a game name
func (s *SonyScraper) GetGameName(titleID string) string {
	// Check cache first
	if name, ok := s.cache.Load(titleID); ok {
		return name.(string)
	}

	// Rate limit
	select {
	case <-s.rateLimiter:
	case <-s.ctx.Done():
		return fmt.Sprintf("Sony %s", titleID)
	}

	url := fmt.Sprintf(prosperoPatchesURL, titleID)
	req, err := http.NewRequestWithContext(s.ctx, "GET", url, nil)
	if err != nil {
		fallback := fmt.Sprintf("Sony %s", titleID)
		s.cache.Store(titleID, fallback)
		return fallback
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		fallback := fmt.Sprintf("Sony %s", titleID)
		if s.ctx.Err() != nil {
			return fallback // shutting down; don't cache
		}
		s.cache.Store(titleID, fallback)
		return fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fallback := fmt.Sprintf("Sony %s", titleID)
		s.cache.Store(titleID, fallback)
		return fallback
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fallback := fmt.Sprintf("Sony %s", titleID)
		s.cache.Store(titleID, fallback)
		return fallback
	}

	gameName := s.extractGameName(string(body), titleID)
	s.cache.Store(titleID, gameName)
	return gameName
}

// extractGameName extracts the game name from the HTML title tag
func (s *SonyScraper) extractGameName(html string, titleID string) string {
	// Extract from <title> tag
	titlePattern := regexp.MustCompile(`<title>([^<]+)</title>`)
	if matches := titlePattern.FindStringSubmatch(html); len(matches) >= 2 {
		gameName := matches[1]

		// Remove "PPSA#####: " prefix if present
		prefixPattern := regexp.MustCompile(`^PPSA\d+:\s*`)
		gameName = prefixPattern.ReplaceAllString(gameName, "")

		return gameName
	}

	return fmt.Sprintf("Sony %s", titleID)
}
