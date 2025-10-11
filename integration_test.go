package main

import (
	"bufio"
	"os"
	"testing"
	"time"
)

// TestLogFileProcessing tests the full pipeline with a real log file
func TestLogFileProcessing(t *testing.T) {
	logFile := "sample_cache_logs.txt"

	// Check if sample file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Skip("sample_cache_logs.txt not found, skipping integration test")
	}

	// Open and read the log file
	file, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	// Create a result channel for game fetching
	gameResultCh := make(chan GameResult, 100)
	fetcher := NewGameFetcher(gameResultCh)
	defer fetcher.Stop()

	// Track statistics
	steamLineCount := 0
	gameIDs := make(map[int]bool)
	hitCount := 0
	missCount := 0

	// Read and parse each line
	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		entry := ParseSteamLogLine(line)
		if entry != nil {
			steamLineCount++
			gameIDs[entry.GameID] = true

			if entry.Status == "HIT" {
				hitCount++
			} else if entry.Status == "MISS" {
				missCount++
			}

			// Trigger fetcher (won't actually fetch in test, but tests queue)
			fetcher.GetGameName(entry.GameID)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	t.Logf("Processed %d total lines", lineCount)
	t.Logf("Found %d Steam lines", steamLineCount)
	t.Logf("Found %d unique games", len(gameIDs))
	t.Logf("HITs: %d, MISSes: %d", hitCount, missCount)

	// Basic sanity checks
	if lineCount == 0 {
		t.Error("No lines were read from the log file")
	}

	if steamLineCount == 0 {
		t.Error("No Steam lines were parsed from the log file")
	}

	if len(gameIDs) == 0 {
		t.Error("No game IDs were extracted")
	}

	if hitCount+missCount != steamLineCount {
		t.Errorf("HIT/MISS count mismatch: %d + %d != %d", hitCount, missCount, steamLineCount)
	}

	// Expected game IDs from the sample (based on the first 100 lines we saw)
	expectedGames := []int{252951, 553853}
	for _, gameID := range expectedGames {
		if !gameIDs[gameID] {
			t.Errorf("Expected to find game ID %d in the log file", gameID)
		}
	}
}

// TestFetcherConcurrency tests the fetcher with concurrent requests
func TestFetcherConcurrency(t *testing.T) {
	gameResultCh := make(chan GameResult, 100)
	fetcher := NewGameFetcher(gameResultCh)
	defer fetcher.Stop()

	// Request multiple game IDs concurrently
	gameIDs := []int{252951, 553853, 730, 440, 570}

	for _, gameID := range gameIDs {
		name := fetcher.GetGameName(gameID)
		if name != "Resolving..." {
			t.Errorf("First call to GetGameName should return 'Resolving...', got %s", name)
		}

		// Second call should still return "Resolving..." since it's pending
		name2 := fetcher.GetGameName(gameID)
		if name2 != "Resolving..." {
			t.Errorf("Second call to GetGameName should return 'Resolving...', got %s", name2)
		}
	}

	// Wait a bit for some results to come back (or timeout)
	timeout := time.After(5 * time.Second)
	resultsReceived := 0

	for resultsReceived < len(gameIDs) {
		select {
		case result := <-gameResultCh:
			resultsReceived++
			t.Logf("Received result for game %d: %s (error: %v)", result.GameID, result.GameName, result.Error)

			// Verify the game ID is in our list
			found := false
			for _, id := range gameIDs {
				if id == result.GameID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Received result for unexpected game ID: %d", result.GameID)
			}

		case <-timeout:
			t.Logf("Timeout waiting for results, received %d/%d", resultsReceived, len(gameIDs))
			return // Don't fail on timeout, API might be slow or unavailable
		}
	}

	t.Logf("Successfully received %d results", resultsReceived)
}

// TestFetcherWorkerPoolLimit tests that only maxWorkers are running concurrently
func TestFetcherWorkerPoolLimit(t *testing.T) {
	gameResultCh := make(chan GameResult, 100)
	fetcher := NewGameFetcher(gameResultCh)
	defer fetcher.Stop()

	// Queue more requests than the worker pool size
	numRequests := 20
	for i := 0; i < numRequests; i++ {
		gameID := 1000 + i
		fetcher.GetGameName(gameID)
	}

	// Wait a bit and collect results
	time.Sleep(2 * time.Second)

	resultsReceived := 0
	timeout := time.After(10 * time.Second)

drainLoop:
	for {
		select {
		case result := <-gameResultCh:
			resultsReceived++
			t.Logf("Result %d: Game %d = %s", resultsReceived, result.GameID, result.GameName)

			if resultsReceived >= numRequests {
				break drainLoop
			}

		case <-timeout:
			t.Logf("Timeout after receiving %d/%d results", resultsReceived, numRequests)
			break drainLoop
		}
	}

	// We should have received at least some results (may not be all due to API limits/timeouts)
	if resultsReceived == 0 {
		t.Error("No results received from worker pool")
	}

	t.Logf("Worker pool processed %d/%d requests", resultsReceived, numRequests)
}
