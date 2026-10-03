package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var (
	// Color scheme
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#3C3C3C")).
			Padding(0, 1)

	tableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	activityHitStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00"))

	activityMissStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFA500"))

	activityTimeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	graphBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9D7CD8")) // Purple
)

const (
	defaultWidth = 120

	maxGameRows     = 20
	minGameRows     = 5 // shown before the graph and activity log get any space
	maxGraphHeight  = 10
	minGraphHeight  = 3
	maxActivityRows = 15
	minActivityRows = 3

	maxGraphWidth = 60
	minGraphWidth = 10
	graphLabelW   = 8 // "%5d │ " y-axis label

	maxNameWidth = 45
	minNameWidth = 8 // keeps the full table within 80 columns
	// Game table width excluding the name column: platform and ID (10 each),
	// four numeric columns (8 each), six " | " separators and row padding
	tableFixedWidth = 10 + 10 + 4*8 + 6*3 + 2

	maxActivityNameWidth = 40
	minActivityNameWidth = 10
	activityFixedWidth   = 30 // time, icon, ID brackets, status
)

// layout holds how many body rows each section gets
type layout struct {
	gameRows     int
	graphHeight  int // 0 hides the graph
	activityRows int // 0 hides the activity log
}

// computeLayout fits the sections into the terminal height. The title,
// status bar and game table always show; the graph and activity log get a
// minimum size if there's room, then everything grows to its maximum in
// priority order.
func computeLayout(height, numGames int) layout {
	wantGames := min(numGames, maxGameRows)

	// Title, blank, table header (3 lines), one table body line, blank, status
	avail := height - 8
	take := func(n int) int {
		n = max(0, min(n, avail))
		avail -= n
		return n
	}

	var l layout
	l.gameRows = min(wantGames, 1) // uses the reserved body line
	l.gameRows += take(min(wantGames, minGameRows) - l.gameRows)

	// Blank separator, header, x-axis and label, plus the plot rows
	if avail >= minGraphHeight+4 {
		avail -= minGraphHeight + 4
		l.graphHeight = minGraphHeight
	}
	// Blank separator and header, plus the rows
	if avail >= minActivityRows+2 {
		avail -= minActivityRows + 2
		l.activityRows = minActivityRows
	}

	l.gameRows += take(wantGames - l.gameRows)
	if l.graphHeight > 0 {
		l.graphHeight += take(maxGraphHeight - l.graphHeight)
	}
	if l.activityRows > 0 {
		l.activityRows += take(maxActivityRows - l.activityRows)
	}
	return l
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

// fitWidth truncates s to w terminal columns and pads it to exactly w,
// without splitting multi-byte or double-width characters
func fitWidth(s string, w int) string {
	return runewidth.FillRight(runewidth.Truncate(s, w, "…"), w)
}

// renderUI renders the complete UI, sized to the terminal
func (m *Model) renderUI() string {
	snap := m.Snapshot()

	width := m.width
	if width <= 0 {
		width = defaultWidth
	}
	height := m.height
	if height <= 0 {
		height = math.MaxInt32
	}

	l := computeLayout(height, len(snap.Games))

	sections := []string{
		titleStyle.Render("🎮 LanCache Spy - Steam Download Monitor"),
		renderGameTable(snap.Games, l.gameRows, clamp(width-tableFixedWidth, minNameWidth, maxNameWidth)),
	}
	if l.graphHeight > 0 {
		sections = append(sections, renderLogRateGraph(snap.LogRates, l.graphHeight,
			clamp(width-graphLabelW, minGraphWidth, maxGraphWidth)))
	}
	if l.activityRows > 0 {
		sections = append(sections, renderActivityLog(snap.Activities, l.activityRows,
			clamp(width-activityFixedWidth, minActivityNameWidth, maxActivityNameWidth)))
	}
	sections = append(sections, renderStatusBar(snap))

	out := strings.Join(sections, "\n\n")

	// Terminals too short even for the minimum layout: bubbletea drops
	// overflow from the top, so clip from the bottom to keep the title and games
	if lines := strings.Split(out, "\n"); len(lines) > height {
		out = strings.Join(lines[:height], "\n")
	}
	return out
}

// renderGameTable renders the game statistics table
func renderGameTable(games []GameStats, rows, nameWidth int) string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("Game Statistics"))
	b.WriteString("\n")

	// Table header
	header := fmt.Sprintf("%-10s | %-10s | %s | %-8s | %-8s | %-8s | %-8s",
		"Platform", "Game ID", fitWidth("Game Name", nameWidth), "Total", "HITs", "MISSes", "Hit Rate")
	b.WriteString(tableRowStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", runewidth.StringWidth(header)))
	b.WriteString("\n")

	if len(games) == 0 {
		b.WriteString(tableRowStyle.Render("No downloads detected yet..."))
		return b.String()
	}

	games = games[:min(len(games), rows)]
	for i, stats := range games {
		row := fmt.Sprintf("%-10s | %s | %s | %-8d | %-8d | %-8d | %-8s",
			stats.Platform,
			fitWidth(stats.GameID, 10),
			fitWidth(stats.GameName, nameWidth),
			stats.Total,
			stats.Hits,
			stats.Misses,
			fmt.Sprintf("%.1f%%", stats.HitRate()))

		b.WriteString(tableRowStyle.Render(row))
		if i < len(games)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderActivityLog renders the recent activity log
func renderActivityLog(activities []ActivityEntry, rows, nameWidth int) string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("Recent Activity"))
	b.WriteString("\n")

	if len(activities) == 0 {
		b.WriteString(tableRowStyle.Render("No recent activity..."))
		return b.String()
	}

	activities = activities[:min(len(activities), rows)]
	for i, activity := range activities {
		timestamp := activityTimeStyle.Render(FormatTimestamp(activity.Timestamp))
		gameName := runewidth.Truncate(activity.GameName, nameWidth, "…")

		var statusIcon, statusText string
		var style lipgloss.Style

		if activity.Status == "HIT" {
			statusIcon = "🟢"
			statusText = "HIT "
			style = activityHitStyle
		} else {
			statusIcon = "🟡"
			statusText = "MISS"
			style = activityMissStyle
		}

		fmt.Fprintf(&b, "%s %s [%s] %s: %s",
			timestamp,
			statusIcon,
			activity.GameID,
			gameName,
			style.Render(statusText))
		if i < len(activities)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderStatusBar renders the bottom status bar
func renderStatusBar(snap Snapshot) string {
	status := fmt.Sprintf(
		"Total Lines: %d | Active Games: %d | Pending Lookups: %d | r: reset | q: quit",
		snap.TotalLines,
		len(snap.Games),
		snap.PendingLookups,
	)

	return statusBarStyle.Render(status)
}

// renderLogRateGraph renders a bar graph of log rate over time, with the
// most recent sample on the right once the graph is full
func renderLogRateGraph(rates []LogRatePoint, height, width int) string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Log Rate (logs/sec)"))
	b.WriteString("\n")

	if len(rates) == 0 {
		b.WriteString(tableRowStyle.Render("No data yet..."))
		return b.String()
	}

	if len(rates) > width {
		rates = rates[len(rates)-width:]
	}

	// Find max rate for scaling
	maxRate := 1
	for _, r := range rates {
		maxRate = max(maxRate, r.Rate)
	}

	// Bar heights in half-rows, rounded up so any non-zero rate is visible
	halves := make([]int, len(rates))
	for i, r := range rates {
		halves[i] = (r.Rate*2*height + maxRate - 1) / maxRate
	}

	for row := height; row >= 1; row-- {
		var line strings.Builder
		for _, h := range halves {
			switch {
			case h >= 2*row:
				line.WriteString("█")
			case h == 2*row-1:
				line.WriteString("▄")
			default:
				line.WriteByte(' ')
			}
		}
		fmt.Fprintf(&b, "%5d │ %s\n", maxRate*row/height, graphBarStyle.Render(line.String()))
	}

	// X-axis
	b.WriteString("      └" + strings.Repeat("─", width) + "\n")
	left := fmt.Sprintf("%ds ago", len(rates))
	pad := max(1, width-len(left)-len("now"))
	b.WriteString("       " + left + strings.Repeat(" ", pad) + "now")

	return b.String()
}
