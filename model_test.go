package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel returns a Model tailing an empty temporary log file
func newTestModel(t *testing.T, noResolve bool) Model {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "access.log")
	if err := os.WriteFile(logFile, nil, 0644); err != nil {
		t.Fatal(err)
	}
	return NewModel(logFile, noResolve, nil)
}

// TestGameNamesDeliveredAfterIdle runs the real bubbletea program and checks
// a name resolved after a quiet period still reaches the model. Previously
// the result listener timed out after 100ms and was never restarted.
func TestGameNamesDeliveredAfterIdle(t *testing.T) {
	m := newTestModel(t, false)
	defer m.fetcher.Stop()
	m.state.games["PPSA00001"] = &GameStats{GameID: "PPSA00001", GameName: resolvingName, Platform: "sony"}

	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard))
	done := make(chan struct{})
	go func() {
		p.Run()
		close(done)
	}()
	defer func() {
		p.Kill()
		<-done
	}()

	time.Sleep(500 * time.Millisecond)
	m.gameResultCh <- GameResult{GameID: "PPSA00001", GameName: "Resolved Game"}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap := m.Snapshot()
		if len(snap.Games) == 1 && snap.Games[0].GameName == "Resolved Game" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("resolved name was not delivered to the model; got %q", m.Snapshot().Games[0].GameName)
}

// TestRenderConcurrentWithProcessing renders while the processor goroutine
// updates stats. It only fails under -race (make race).
func TestRenderConcurrentWithProcessing(t *testing.T) {
	m := newTestModel(t, true)
	m.ready = true
	m.width, m.height = 120, 60

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			m.processLogEntry(&LogEntry{Platform: "steam", GameID: fmt.Sprint(i % 5), Status: "HIT"})
		}
	}()

	for i := 0; i < 200; i++ {
		m.calculateLogRate(time.Now())
		_ = m.View()
	}
	close(stop)
	wg.Wait()
}

func TestSnapshotSortsStably(t *testing.T) {
	m := newTestModel(t, true)
	for _, id := range []string{"c", "a", "b"} {
		m.processLogEntry(&LogEntry{Platform: "steam", GameID: id, Status: "HIT"})
	}
	m.processLogEntry(&LogEntry{Platform: "steam", GameID: "b", Status: "MISS"})

	snap := m.Snapshot()
	var got []string
	for _, g := range snap.Games {
		got = append(got, g.GameID)
	}
	if want := "[b a c]"; fmt.Sprint(got) != want {
		t.Errorf("order = %v, want %s (by total, then ID)", got, want)
	}
	if snap.TotalLines != 4 {
		t.Errorf("TotalLines = %d, want 4", snap.TotalLines)
	}
}
