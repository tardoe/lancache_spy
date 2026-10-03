package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// blockingTransport holds every request open until its context is cancelled
type blockingTransport struct{ started chan struct{} }

func newBlockingTransport() *blockingTransport {
	return &blockingTransport{started: make(chan struct{}, 1)}
}

func (bt *blockingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	select {
	case bt.started <- struct{}{}:
	default:
	}
	<-r.Context().Done()
	return nil, r.Context().Err()
}

// TestFetcherStopAbortsInFlight checks Stop doesn't wait out a hanging HTTP request.
// Stop runs inside Update when the user presses q, so blocking freezes the UI.
func TestFetcherStopAbortsInFlight(t *testing.T) {
	fetcher := NewGameFetcher(make(chan GameResult, 100))
	bt := newBlockingTransport()
	fetcher.sonyScraper.httpClient.Transport = bt

	fetcher.GetGameName("PPSA99999", "sony")
	select {
	case <-bt.started:
	case <-time.After(5 * time.Second):
		t.Fatal("lookup request never started")
	}

	start := time.Now()
	fetcher.Stop()
	if took := time.Since(start); took > time.Second {
		t.Errorf("Stop took %v with a request in flight, want well under 1s", took)
	}

	// An aborted lookup must not be cached as a failure
	if _, ok := fetcher.sonyScraper.cache.Load("PPSA99999"); ok {
		t.Error("lookup aborted by Stop was cached")
	}
}

// TestFetcherGetGameNameAfterStop checks late lookups from the processor
// goroutine, which outlives Stop, don't panic
func TestFetcherGetGameNameAfterStop(t *testing.T) {
	fetcher := NewGameFetcher(make(chan GameResult, 100))
	fetcher.Stop()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetGameName after Stop panicked: %v", r)
		}
	}()
	fetcher.GetGameName("PPSA12345", "sony")
}

// TestFetcherQueueFullStaysResolving checks that a lookup dropped because the
// queue is full is retried later instead of showing a permanent placeholder
func TestFetcherQueueFullStaysResolving(t *testing.T) {
	m := newTestModel(t, false)
	defer m.fetcher.Stop()
	m.fetcher.sonyScraper.httpClient.Transport = newBlockingTransport()

	// More distinct games than the queue (100) plus workers (5) can hold
	numGames := 130
	for i := 0; i < numGames; i++ {
		m.processLogEntry(&LogEntry{Platform: "sony", GameID: fmt.Sprintf("PPSA%05d", i), Status: "MISS"})
	}

	for id, stats := range m.state.games {
		if stats.GameName != resolvingName {
			t.Errorf("game %s has name %q, want %q", id, stats.GameName, resolvingName)
		}
	}
}
