package main

import (
	"fmt"
	"sync"
)

const (
	maxWorkers = 5
)

// GameFetcher manages game name resolution with rate limiting
type GameFetcher struct {
	cache        sync.Map // map[int]string - depotID -> gameName
	pending      sync.Map // map[int]bool - depotIDs currently being fetched
	requestQueue chan int
	resultChan   chan GameResult
	stopChan     chan struct{}
	wg           sync.WaitGroup
	scraper      *SteamDBScraper
}

// GameResult represents a fetched game name
type GameResult struct {
	GameID   int
	GameName string
	Error    error
}

// NewGameFetcher creates a new GameFetcher with worker pool
func NewGameFetcher(resultChan chan GameResult) *GameFetcher {
	gf := &GameFetcher{
		requestQueue: make(chan int, 100),
		resultChan:   resultChan,
		stopChan:     make(chan struct{}),
		scraper:      NewSteamDBScraper(),
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
		case gameID := <-gf.requestQueue:
			gf.fetchGameName(gameID)
		case <-gf.stopChan:
			return
		}
	}
}

// fetchGameName fetches the game name by scraping SteamDB
func (gf *GameFetcher) fetchGameName(depotID int) {
	defer gf.pending.Delete(depotID)

	// Use the scraper to get the game name
	gameName := gf.scraper.GetGameName(depotID)

	// Cache the result
	gf.cache.Store(depotID, gameName)

	gf.sendResult(GameResult{
		GameID:   depotID,
		GameName: gameName,
		Error:    nil,
	})
}

// sendResult sends a result without blocking
func (gf *GameFetcher) sendResult(result GameResult) {
	select {
	case gf.resultChan <- result:
		// Sent successfully
	case <-gf.stopChan:
		// Shutting down, don't block
	default:
		// Channel full, drop the message to prevent deadlock
		// The game name is already cached, so UI will get it eventually
	}
}

// GetGameName returns the game name if cached, otherwise queues a fetch
func (gf *GameFetcher) GetGameName(gameID int) string {
	// Check cache first
	if name, ok := gf.cache.Load(gameID); ok {
		return name.(string)
	}

	// Check if already pending
	if _, pending := gf.pending.LoadOrStore(gameID, true); pending {
		return "Resolving..."
	}

	// Queue for fetching
	select {
	case gf.requestQueue <- gameID:
		return "Resolving..."
	default:
		// Queue full, return placeholder
		gf.pending.Delete(gameID)
		return fmt.Sprintf("Game %d", gameID)
	}
}

// Stop gracefully shuts down the worker pool
func (gf *GameFetcher) Stop() {
	close(gf.stopChan)
	gf.wg.Wait()
	close(gf.requestQueue)
	// Don't close resultChan - it's owned by the caller
}
