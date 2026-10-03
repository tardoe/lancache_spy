package main

import (
	"context"
	"fmt"
	"sync"
)

const (
	maxWorkers = 5

	// resolvingName is shown while a game name lookup is queued or in flight
	resolvingName = "Resolving..."
)

// GameFetcher manages game name resolution with rate limiting
type GameFetcher struct {
	cache        sync.Map // map[string]string - gameID -> gameName
	pending      sync.Map // map[string]bool - gameIDs currently being fetched
	requestQueue chan GameRequest
	resultChan   chan GameResult
	ctx          context.Context // cancelled by Stop to abort in-flight lookups
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	steamScraper *SteamDBScraper
	sonyScraper  *SonyScraper
}

// GameRequest represents a game name lookup request
type GameRequest struct {
	GameID   string
	Platform string
}

// GameResult represents a fetched game name
type GameResult struct {
	GameID   string
	GameName string
	Error    error
}

// NewGameFetcher creates a new GameFetcher with worker pool
func NewGameFetcher(resultChan chan GameResult) *GameFetcher {
	ctx, cancel := context.WithCancel(context.Background())
	gf := &GameFetcher{
		requestQueue: make(chan GameRequest, 100),
		resultChan:   resultChan,
		ctx:          ctx,
		cancel:       cancel,
		steamScraper: NewSteamDBScraper(ctx),
		sonyScraper:  NewSonyScraper(ctx),
	}

	// Start worker pool
	for i := 0; i < maxWorkers; i++ {
		gf.wg.Add(1)
		go gf.worker()
	}

	return gf
}

// worker processes game ID lookup requests
func (gf *GameFetcher) worker() {
	defer gf.wg.Done()

	for {
		select {
		case req := <-gf.requestQueue:
			gf.fetchGameName(req)
		case <-gf.ctx.Done():
			return
		}
	}
}

// fetchGameName fetches the game name by scraping the appropriate service
func (gf *GameFetcher) fetchGameName(req GameRequest) {
	defer gf.pending.Delete(req.GameID)

	var gameName string

	switch req.Platform {
	case "steam":
		// Convert string depot ID to int for Steam scraper
		depotIDInt := 0
		fmt.Sscanf(req.GameID, "%d", &depotIDInt)
		gameName = gf.steamScraper.GetGameName(depotIDInt)
	case "sony":
		gameName = gf.sonyScraper.GetGameName(req.GameID)
	default:
		gameName = fmt.Sprintf("%s %s", req.Platform, req.GameID)
	}

	// Cache the result
	gf.cache.Store(req.GameID, gameName)

	gf.sendResult(GameResult{
		GameID:   req.GameID,
		GameName: gameName,
		Error:    nil,
	})
}

// sendResult sends a result without blocking
func (gf *GameFetcher) sendResult(result GameResult) {
	select {
	case gf.resultChan <- result:
		// Sent successfully
	case <-gf.ctx.Done():
		// Shutting down, don't block
	default:
		// Channel full, drop the message to prevent deadlock
		// The game name is already cached, so UI will get it eventually
	}
}

// GetGameName returns the game name if cached, otherwise queues a fetch
func (gf *GameFetcher) GetGameName(gameID string, platform string) string {
	// Check cache first
	if name, ok := gf.cache.Load(gameID); ok {
		return name.(string)
	}

	// Check if already pending
	if _, pending := gf.pending.LoadOrStore(gameID, true); pending {
		return resolvingName
	}

	// Queue for fetching
	select {
	case gf.requestQueue <- GameRequest{GameID: gameID, Platform: platform}:
	default:
		// Queue full: report as still resolving so the next log line
		// for this game retries the lookup
		gf.pending.Delete(gameID)
	}
	return resolvingName
}

// Stop shuts down the worker pool, aborting any in-flight lookups.
// The request queue is left open so late GetGameName calls can't panic.
func (gf *GameFetcher) Stop() {
	gf.cancel()
	gf.wg.Wait()
	// Don't close resultChan - it's owned by the caller
}
