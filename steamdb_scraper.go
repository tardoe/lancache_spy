package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	steamDBDepotURL = "https://steamdb.info/depot/%d/"
	userAgent       = "LanCacheSpy/1.0"
)

// SteamDBScraper scrapes SteamDB to resolve depot IDs to app IDs and game names
type SteamDBScraper struct {
	cache       sync.Map // map[int]string - depotID -> gameName
	httpClient  *http.Client
	rateLimiter chan struct{}   // Rate limit scraping
	ctx         context.Context // aborts rate-limit waits and requests on shutdown
}

// NewSteamDBScraper creates a new scraper
func NewSteamDBScraper(ctx context.Context) *SteamDBScraper {
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

	return &SteamDBScraper{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: rateLimiter,
		ctx:         ctx,
	}
}

// GetGameName resolves a depot ID to a game name
func (s *SteamDBScraper) GetGameName(depotID int) string {
	// Check cache first

	// Check common mappings
	if name, ok := GetGameNameFromDepot(depotID); ok {
		s.cache.Store(depotID, name)
		return name
	}
	if name, ok := s.cache.Load(depotID); ok {
		return name.(string)
	}

	// Rate limit
	select {
	case <-s.rateLimiter:
	case <-s.ctx.Done():
		return fmt.Sprintf("Depot %d", depotID)
	}

	// Scrape SteamDB
	url := fmt.Sprintf(steamDBDepotURL, depotID)
	req, err := http.NewRequestWithContext(s.ctx, "GET", url, nil)
	if err != nil {
		fallback := fmt.Sprintf("Depot %d", depotID)
		s.cache.Store(depotID, fallback)
		return fallback
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		fallback := fmt.Sprintf("Depot %d", depotID)
		if s.ctx.Err() != nil {
			return fallback // shutting down; don't cache
		}
		s.cache.Store(depotID, fallback)
		return fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fallback := fmt.Sprintf("Depot %d", depotID)
		s.cache.Store(depotID, fallback)
		return fallback
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fallback := fmt.Sprintf("Depot %d", depotID)
		s.cache.Store(depotID, fallback)
		return fallback
	}

	// Parse the HTML to find the game name
	gameName := s.parseGameName(string(body), depotID)
	s.cache.Store(depotID, gameName)
	return gameName
}

// parseGameName extracts the game name from SteamDB HTML
func (s *SteamDBScraper) parseGameName(html string, depotID int) string {
	// Look for patterns like:
	// <a href="/app/252950/">Rocket League</a>
	// The depot page links to the parent app

	// Pattern 1: Look for app link with name
	appLinkPattern := regexp.MustCompile(`<a href="/app/\d+/">([^<]+)</a>`)
	matches := appLinkPattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Pattern 2: Look for title tag
	titlePattern := regexp.MustCompile(`<title>([^<]+) · Depot \d+ · SteamDB</title>`)
	matches = titlePattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Pattern 3: Look for og:title meta tag
	ogTitlePattern := regexp.MustCompile(`<meta property="og:title" content="([^"]+) · Depot`)
	matches = ogTitlePattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Fallback
	return fmt.Sprintf("Depot %d", depotID)
}

// GetAppID extracts the app ID from the depot page
func (s *SteamDBScraper) GetAppID(depotID int) (int, error) {
	// Rate limit
	select {
	case <-s.rateLimiter:
	case <-s.ctx.Done():
		return 0, s.ctx.Err()
	}

	url := fmt.Sprintf(steamDBDepotURL, depotID)
	req, err := http.NewRequestWithContext(s.ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Look for app link: <a href="/app/252950/">
	appIDPattern := regexp.MustCompile(`<a href="/app/(\d+)/"`)
	matches := appIDPattern.FindStringSubmatch(string(body))
	if len(matches) > 1 {
		appID, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, err
		}
		return appID, nil
	}

	return 0, fmt.Errorf("app ID not found")
}
