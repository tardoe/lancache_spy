package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// populatedModel returns a ready Model with more data than any section can show
func populatedModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel(t, true)
	m.ready = true
	for i := 0; i < 25; i++ {
		id := fmt.Sprint(i)
		m.state.games[id] = &GameStats{GameID: id, GameName: "Some Game", Platform: "steam", Total: i}
	}
	for i := 0; i < 15; i++ {
		m.state.activities = append(m.state.activities, ActivityEntry{GameID: "1", GameName: "Some Game", Status: "HIT"})
	}
	for i := 0; i < 60; i++ {
		m.state.logRates = append(m.state.logRates, LogRatePoint{Rate: i})
	}
	return m
}

// TestViewFitsTerminalHeight checks the view never exceeds the terminal
// height. bubbletea drops overflow from the top, which hid the title and
// the busiest games on anything shorter than the old fixed 62 rows.
func TestViewFitsTerminalHeight(t *testing.T) {
	m := populatedModel(t)
	for _, width := range []int{40, 80, 120, 200} {
		for height := 3; height <= 80; height++ {
			m.width, m.height = width, height
			lines := strings.Split(m.View(), "\n")
			if len(lines) > height {
				t.Fatalf("%dx%d: view is %d lines tall", width, height, len(lines))
			}
			if !strings.Contains(lines[0], "LanCache Spy") {
				t.Fatalf("%dx%d: first line is %q, want the title", width, height, lines[0])
			}
		}
	}
}

func TestComputeLayout(t *testing.T) {
	tests := []struct {
		height, games int
		want          layout
	}{
		// Plenty of room: everything at maximum
		{100, 25, layout{gameRows: 20, graphHeight: 10, activityRows: 15}},
		// Standard 24-row terminal: 5 games, minimum graph and activity log
		{24, 25, layout{gameRows: 5, graphHeight: 3, activityRows: 3}},
		// Spare rows go to games first
		{30, 25, layout{gameRows: 11, graphHeight: 3, activityRows: 3}},
		// Too short for the activity log; the leftover row goes to games
		{20, 25, layout{gameRows: 6, graphHeight: 3, activityRows: 0}},
		// No games yet: graph and activity log use the space
		{100, 0, layout{gameRows: 0, graphHeight: 10, activityRows: 15}},
	}
	for _, tt := range tests {
		if got := computeLayout(tt.height, tt.games); got != tt.want {
			t.Errorf("computeLayout(%d, %d) = %+v, want %+v", tt.height, tt.games, got, tt.want)
		}
	}
}

// TestLayoutHeightsMatchRender checks computeLayout's line accounting matches
// what renderUI actually produces when every section is full
func TestLayoutHeightsMatchRender(t *testing.T) {
	m := populatedModel(t)
	m.width = 120
	for _, height := range []int{24, 30, 40} {
		m.height = height
		if got := len(strings.Split(m.View(), "\n")); got != height {
			t.Errorf("height %d: rendered %d lines, want the layout to fill exactly %d", height, got, height)
		}
	}

	// Every section at maximum: 20 games, 10-row graph, 15 activities
	m.height = 100
	if got := len(strings.Split(m.View(), "\n")); got != 58 {
		t.Errorf("unconstrained view is %d lines, want 58", got)
	}
}

// TestTruncationKeepsValidUTF8 checks long names are cut on character
// boundaries; PlayStation titles often contain ™ and ®
func TestTruncationKeepsValidUTF8(t *testing.T) {
	m := newTestModel(t, true)
	m.ready = true
	m.width, m.height = 120, 60
	name := strings.Repeat("A", 41) + "™ Deluxe Edition Upgrade Bundle"
	m.state.games["PPSA00001"] = &GameStats{GameID: "PPSA00001", GameName: name, Platform: "sony", Total: 1}
	m.state.activities = []ActivityEntry{{GameID: "PPSA00001", GameName: name, Status: "HIT"}}

	if out := m.View(); !utf8.ValidString(out) {
		t.Error("rendered view contains invalid UTF-8")
	}
}

func TestLogRateGraph(t *testing.T) {
	t.Run("zero rates draw no bars", func(t *testing.T) {
		out := renderLogRateGraph([]LogRatePoint{{Rate: 0}, {Rate: 0}, {Rate: 0}}, 5, 60)
		if strings.ContainsAny(out, "█▄") {
			t.Errorf("graph of all-zero rates contains bars:\n%s", out)
		}
	})

	t.Run("bar heights scale to the max rate", func(t *testing.T) {
		// Height 4 = 8 half-rows: 10 fills all 8, 5 fills 4, and 1 rounds
		// up to a single half-row
		out := renderLogRateGraph([]LogRatePoint{{Rate: 10}, {Rate: 5}, {Rate: 1}}, 4, 60)
		lines := strings.Split(out, "\n")[1:5] // plot rows, top first
		want := []string{"█", "█", "██", "██▄"}
		for i, l := range lines {
			plot := strings.TrimRight(string([]rune(stripANSI(l))[8:]), " ") // drop the "%5d │ " label
			if plot != want[i] {
				t.Errorf("row %d = %q, want %q", i, plot, want[i])
			}
		}
	})
}

// stripANSI removes terminal escape sequences
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEscape = true
		case inEscape:
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				inEscape = false
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
