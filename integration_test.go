package main

import (
	"bufio"
	"os"
	"testing"
	"time"
)

// TestLogFileProcessing parses the sample log files end to end
func TestLogFileProcessing(t *testing.T) {
	tests := []struct {
		file          string
		expectedGames map[string][]string // platform -> game IDs that must appear
	}{
		{
			file: "sample_cache_logs.txt",
			expectedGames: map[string][]string{
				"steam": {"252951", "553853"},
				"sony":  {"PPSA15301"},
			},
		},
		{
			file: "sony_xboxlive_epicgames_logs.txt",
			expectedGames: map[string][]string{
				"sony":      {"PPSA07805"},
				"xboxlive":  {"xboxlive"},
				"epicgames": {"unrealenginelauncher"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			file, err := os.Open(tt.file)
			if err != nil {
				t.Fatalf("Failed to open log file: %v", err)
			}
			defer file.Close()

			seen := make(map[string]map[string]bool)
			lineCount, parsedCount, hitCount, missCount := 0, 0, 0, 0

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				lineCount++
				entry := ParseLogLine(scanner.Text())
				if entry == nil {
					continue
				}

				parsedCount++
				if seen[entry.Platform] == nil {
					seen[entry.Platform] = make(map[string]bool)
				}
				seen[entry.Platform][entry.GameID] = true

				switch entry.Status {
				case "HIT":
					hitCount++
				case "MISS":
					missCount++
				}
			}
			if err := scanner.Err(); err != nil {
				t.Fatalf("Error reading log file: %v", err)
			}

			t.Logf("Processed %d lines, parsed %d (HITs: %d, MISSes: %d)", lineCount, parsedCount, hitCount, missCount)

			if parsedCount == 0 {
				t.Fatal("No lines were parsed from the log file")
			}
			if hitCount+missCount != parsedCount {
				t.Errorf("HIT/MISS count mismatch: %d + %d != %d", hitCount, missCount, parsedCount)
			}

			for platform, gameIDs := range tt.expectedGames {
				for _, gameID := range gameIDs {
					if !seen[platform][gameID] {
						t.Errorf("Expected to find %s game ID %q", platform, gameID)
					}
				}
			}
		})
	}
}

// The fetcher tests use the "xboxlive" platform, which resolves locally
// without scraping, so they exercise the worker pool without network access.

// TestFetcherDeduplicatesPending checks that repeat lookups don't queue duplicate fetches
func TestFetcherDeduplicatesPending(t *testing.T) {
	gameResultCh := make(chan GameResult, 100)
	fetcher := NewGameFetcher(gameResultCh)
	defer fetcher.Stop()

	gameIDs := []string{"a", "b", "c", "d", "e"}
	for _, gameID := range gameIDs {
		fetcher.GetGameName(gameID, "xboxlive")
		fetcher.GetGameName(gameID, "xboxlive")
	}

	received := make(map[string]int)
	timeout := time.After(5 * time.Second)
	for len(received) < len(gameIDs) {
		select {
		case result := <-gameResultCh:
			received[result.GameID]++
		case <-timeout:
			t.Fatalf("Timed out waiting for results, received %d/%d", len(received), len(gameIDs))
		}
	}

	// Give any duplicate fetches a chance to arrive
	select {
	case result := <-gameResultCh:
		t.Errorf("Received unexpected duplicate result for %q", result.GameID)
	case <-time.After(200 * time.Millisecond):
	}

	for _, gameID := range gameIDs {
		if received[gameID] != 1 {
			t.Errorf("Expected 1 result for %q, got %d", gameID, received[gameID])
		}
	}
}

// TestFetcherCachesResults checks that resolved names are served from cache
func TestFetcherCachesResults(t *testing.T) {
	gameResultCh := make(chan GameResult, 100)
	fetcher := NewGameFetcher(gameResultCh)
	defer fetcher.Stop()

	if name := fetcher.GetGameName("halo", "xboxlive"); name != "Resolving..." {
		t.Fatalf("First call should return 'Resolving...', got %q", name)
	}

	select {
	case result := <-gameResultCh:
		if result.GameName != "xboxlive halo" {
			t.Errorf("Expected resolved name 'xboxlive halo', got %q", result.GameName)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for result")
	}

	if name := fetcher.GetGameName("halo", "xboxlive"); name != "xboxlive halo" {
		t.Errorf("Expected cached name 'xboxlive halo', got %q", name)
	}
}

// TestFetcherWorkerPool queues more requests than workers and expects them all to complete
func TestFetcherWorkerPool(t *testing.T) {
	gameResultCh := make(chan GameResult, 100)
	fetcher := NewGameFetcher(gameResultCh)
	defer fetcher.Stop()

	numRequests := maxWorkers * 4
	for i := 0; i < numRequests; i++ {
		fetcher.GetGameName(string(rune('A'+i)), "xboxlive")
	}

	timeout := time.After(5 * time.Second)
	for received := 0; received < numRequests; received++ {
		select {
		case <-gameResultCh:
		case <-timeout:
			t.Fatalf("Timed out after receiving %d/%d results", received, numRequests)
		}
	}
}
