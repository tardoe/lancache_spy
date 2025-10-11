package main

import (
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hpcloud/tail"
)

// GameStats tracks statistics for a single game
type GameStats struct {
	GameID   string
	GameName string
	Platform string
	Total    int
	Hits     int
	Misses   int
}

// HitRate calculates the cache hit rate as a percentage
func (gs *GameStats) HitRate() float64 {
	if gs.Total == 0 {
		return 0
	}
	return float64(gs.Hits) / float64(gs.Total) * 100
}

// ActivityEntry represents a recent log event
type ActivityEntry struct {
	Timestamp time.Time
	GameID   string
	GameName  string
	Status    string // "HIT" or "MISS"
}

// LogRatePoint represents log rate at a point in time
type LogRatePoint struct {
	Timestamp time.Time
	Rate      int // logs per second
}

// SharedState contains mutable state updated by background goroutines
type SharedState struct {
	games      map[string]*GameStats
	gamesMutex sync.RWMutex
	activities []ActivityEntry
	totalLines int
	logRates   []LogRatePoint // Time series of log rates
	lastRate   int            // Last calculated rate
}

// Model is the bubbletea model for the application
type Model struct {
	state        *SharedState // Pointer to shared state
	fetcher      *GameFetcher
	tailer       *tail.Tail
	logFilePath  string
	noResolve    bool
	depotDB      *DepotDatabase
	ready        bool
	quitting     bool
	width        int
	height       int
	gameResultCh chan GameResult
	lineCh       chan string
}

// GameNameMsg is sent when a game name is resolved
type GameNameMsg GameResult

// InitialMsg is sent when the TUI is ready
type InitialMsg struct{}

// TailerReadyMsg is sent when the tailer is initialized
type TailerReadyMsg struct {
	tailer *tail.Tail
}

// TickMsg is sent periodically to trigger UI refresh
type TickMsg time.Time

// LogRateMsg is sent periodically to update log rate
type LogRateMsg time.Time

// NewModel creates a new Model
func NewModel(logFilePath string, noResolve bool, depotDB *DepotDatabase) Model {
	var fetcher *GameFetcher
	var gameResultCh chan GameResult

	if !noResolve {
		gameResultCh = make(chan GameResult, 100)
		fetcher = NewGameFetcher(gameResultCh)
	}

	// Create shared state that will be accessed by background goroutines
	sharedState := &SharedState{
		games:      make(map[string]*GameStats),
		activities: make([]ActivityEntry, 0),
		totalLines: 0,
		logRates:   make([]LogRatePoint, 0),
		lastRate:   0,
	}

	return Model{
		state:        sharedState,
		logFilePath:  logFilePath,
		noResolve:    noResolve,
		gameResultCh: gameResultCh,
		fetcher:      fetcher,
		depotDB:      depotDB,
		lineCh:       make(chan string, 50000),
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.startTailing(),
		m.tick(),          // Start UI refresh timer
		m.updateLogRate(), // Start log rate tracking
	}

	if !m.noResolve {
		cmds = append(cmds, m.waitForGameNames())
	}

	return tea.Batch(cmds...)
}

// tick returns a command that waits and sends a TickMsg
func (m Model) tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// updateLogRate returns a command that calculates log rate every second
func (m Model) updateLogRate() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return LogRateMsg(t)
	})
}

// startTailing begins tailing the log file and starts a background goroutine
func (m Model) startTailing() tea.Cmd {
	return func() tea.Msg {
		t, err := tail.TailFile(m.logFilePath, tail.Config{
			Follow:    true,  // Keep following the file as it grows
			ReOpen:    true,  // Reopen if rotated
			Poll:      true,  // Use polling for compatibility
			MustExist: true,
			Location:  &tail.SeekInfo{Offset: 0, Whence: 2}, // Start from END of file (tail -f behavior)
		})
		if err != nil {
			return tea.Quit()
		}

		// Start background goroutine to read from tailer and feed lineCh
		go func() {
			debugf("tailer goroutine: started")
			lineCount := 0
			for line := range t.Lines {
				lineCount++
				if line.Err != nil {
					debugf("tailer goroutine: error reading line %d: %v", lineCount, line.Err)
					continue
				}

				// Blocking send - we have a huge buffer, should be fine
				m.lineCh <- line.Text

				if lineCount%10000 == 0 {
					debugf("tailer goroutine: sent line %d, lineCh %d/%d", lineCount, len(m.lineCh), cap(m.lineCh))
				}
			}
			debugf("tailer goroutine: exiting after %d lines, t.Lines channel closed", lineCount)
			close(m.lineCh)
		}()

		// Start background processor goroutine that processes lines independently
		go func() {
			debugf("processor goroutine: started")
			processedCount := 0
			steamLineCount := 0
			gameIDsSeen := make(map[string]int) // Track how many lines per game ID

			for line := range m.lineCh {
				processedCount++
				entry := ParseSteamLogLine(line)
				if entry == nil {
					continue
				}

				steamLineCount++
				gameIDsSeen[entry.GameID]++
				m.processLogEntry(entry)

				if processedCount%10000 == 0 {
					debugf("processor goroutine: processed %d lines (%d steam, %d unique games)",
						processedCount, steamLineCount, len(gameIDsSeen))
				}
			}
			debugf("processor goroutine: FINAL - processed %d lines, %d steam lines, %d unique games",
				processedCount, steamLineCount, len(gameIDsSeen))
			debugf("processor goroutine: totalLines in state=%d, games in state=%d",
				m.state.totalLines, len(m.state.games))
		}()

		return TailerReadyMsg{tailer: t}
	}
}

// processLogEntry processes a single log entry (called by background goroutine)
func (m *Model) processLogEntry(entry *LogEntry) {
	m.state.gamesMutex.Lock()
	defer m.state.gamesMutex.Unlock()

	m.state.totalLines++

	// Update or create game stats
	stats, exists := m.state.games[entry.GameID]
	if !exists {
		gameName := fmt.Sprintf("%s %s", entry.Platform, entry.GameID)

		// For Steam, look up game name in depot database or fetcher
		if entry.Platform == "steam" {
			if m.depotDB != nil {
				if name, ok := m.depotDB.GetGameName(entry.GameID); ok {
					gameName = name
				} else if !m.noResolve && m.fetcher != nil {
					// Depot DB didn't have it, try the fetcher
					gameName = m.fetcher.GetGameName(entry.GameID, entry.Platform)
				}
			} else if !m.noResolve && m.fetcher != nil {
				gameName = m.fetcher.GetGameName(entry.GameID, entry.Platform)
			}
		} else if !m.noResolve && m.fetcher != nil {
			// For non-Steam platforms, try the fetcher
			gameName = m.fetcher.GetGameName(entry.GameID, entry.Platform)
		}
		stats = &GameStats{
			GameID:   entry.GameID,
			GameName: gameName,
			Platform: entry.Platform,
			Total:    0,
			Hits:     0,
			Misses:   0,
		}
		m.state.games[entry.GameID] = stats
	} else {
		// Refresh game name from cache if it's still "Resolving..."
		if !m.noResolve && m.fetcher != nil && stats.GameName == "Resolving..." {
			stats.GameName = m.fetcher.GetGameName(entry.GameID, stats.Platform)
		}
	}

	stats.Total++
	if entry.Status == "HIT" {
		stats.Hits++
	} else {
		stats.Misses++
	}

	// Add to activity log
	activity := ActivityEntry{
		Timestamp: time.Now(),
		GameID:    entry.GameID,
		GameName:  stats.GameName,
		Status:    entry.Status,
	}

	m.state.activities = append([]ActivityEntry{activity}, m.state.activities...)
	if len(m.state.activities) > 15 {
		m.state.activities = m.state.activities[:15]
	}
}

// waitForGameNames listens for resolved game names
func (m Model) waitForGameNames() tea.Cmd {
	if m.noResolve || m.gameResultCh == nil {
		return nil
	}

	return func() tea.Msg {
		gameResultChanLen := len(m.gameResultCh)
		gameResultChanCap := cap(m.gameResultCh)
		debugf("waitForGameNames: gameResultCh status: %d/%d", gameResultChanLen, gameResultChanCap)

		select {
		case result := <-m.gameResultCh:
			debugf("waitForGameNames: received result for game %d", result.GameID)
			return GameNameMsg(result)
		case <-time.After(100 * time.Millisecond):
			// Timeout to prevent blocking forever
			debugf("waitForGameNames: timeout")
			return nil
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer func() {
		if r := recover(); r != nil {
			debugf("PANIC in Update: %v", r)
			panic(r)
		}
	}()

	debugf("Update: received message type: %T", msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			if m.fetcher != nil {
				m.fetcher.Stop()
			}
			if m.tailer != nil {
				m.tailer.Stop()
			}
			return m, tea.Quit
		case "r":
			m.resetStats()
		}
		return m, nil

	case TailerReadyMsg:
		m.tailer = msg.tailer
		return m, nil

	case TickMsg:
		// UI refresh tick - just trigger re-render and queue next tick
		debugf("Update: TickMsg - totalLines=%d, games=%d", m.state.totalLines, len(m.state.games))
		return m, m.tick()

	case LogRateMsg:
		m.calculateLogRate(time.Time(msg))
		return m, m.updateLogRate()

	case GameNameMsg:
		m.updateGameName(GameResult(msg))
		return m, m.waitForGameNames()
	}

	// Keep game name resolution running if enabled
	if m.ready && !m.quitting && !m.noResolve {
		return m, m.waitForGameNames()
	}

	return m, nil
}

// updateGameName updates the game name when resolved
func (m *Model) updateGameName(result GameResult) {
	m.state.gamesMutex.Lock()
	defer m.state.gamesMutex.Unlock()

	if stats, exists := m.state.games[result.GameID]; exists {
		stats.GameName = result.GameName
	}

	// Update activity entries
	for i := range m.state.activities {
		if m.state.activities[i].GameID == result.GameID {
			m.state.activities[i].GameName = result.GameName
		}
	}
}

func (m Model) View() string {
	defer func() {
		if r := recover(); r != nil {
			debugf("PANIC in View: %v", r)
		}
	}()

	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		return "Shutting down...\n"
	}

	// Render current state (called at 10 FPS via TickMsg)
	return m.renderUI()
}

// GetGameList returns a sorted list of games for rendering
func (m *Model) GetGameList() []*GameStats {
	m.state.gamesMutex.RLock()
	defer m.state.gamesMutex.RUnlock()

	games := make([]*GameStats, 0, len(m.state.games))
	for _, stats := range m.state.games {
		games = append(games, stats)
	}

	// Sort by total activity (descending)
	for i := 0; i < len(games)-1; i++ {
		for j := i + 1; j < len(games); j++ {
			if games[i].Total < games[j].Total {
				games[i], games[j] = games[j], games[i]
			}
		}
	}

	return games
}

// GetStats returns overall statistics
func (m *Model) GetStats() (totalGames, pendingLookups int) {
	m.state.gamesMutex.RLock()
	defer m.state.gamesMutex.RUnlock()

	totalGames = len(m.state.games)

	// Count pending lookups
	for _, stats := range m.state.games {
		if stats.GameName == "Resolving..." {
			pendingLookups++
		}
	}

	return
}

// GetActivities returns recent activities
func (m *Model) GetActivities() []ActivityEntry {
	m.state.gamesMutex.RLock()
	defer m.state.gamesMutex.RUnlock()

	// Return a copy to avoid race conditions
	activities := make([]ActivityEntry, len(m.state.activities))
	copy(activities, m.state.activities)
	return activities
}

// resetStats clears all statistics
func (m *Model) resetStats() {
	m.state.gamesMutex.Lock()
	defer m.state.gamesMutex.Unlock()

	m.state.games = make(map[string]*GameStats)
	m.state.activities = make([]ActivityEntry, 0)
	m.state.totalLines = 0
	m.state.logRates = make([]LogRatePoint, 0)
	m.state.lastRate = 0
}

// calculateLogRate calculates the current log rate and adds it to history
func (m *Model) calculateLogRate(t time.Time) {
	m.state.gamesMutex.Lock()
	defer m.state.gamesMutex.Unlock()

	// Calculate rate based on totalLines change
	currentTotal := m.state.totalLines

	// Calculate rate (lines per second since last check - we check every second)
	rate := currentTotal - m.state.lastRate
	m.state.lastRate = currentTotal

	// Add to history
	m.state.logRates = append(m.state.logRates, LogRatePoint{
		Timestamp: t,
		Rate:      rate,
	})

	// Keep only last 60 seconds of data
	if len(m.state.logRates) > 60 {
		m.state.logRates = m.state.logRates[len(m.state.logRates)-60:]
	}
}

// GetLogRates returns the log rate history
func (m *Model) GetLogRates() []LogRatePoint {
	m.state.gamesMutex.RLock()
	defer m.state.gamesMutex.RUnlock()

	rates := make([]LogRatePoint, len(m.state.logRates))
	copy(rates, m.state.logRates)
	return rates
}

// FormatTimestamp formats a timestamp for display
func FormatTimestamp(t time.Time) string {
	return t.Format("15:04:05")
}
