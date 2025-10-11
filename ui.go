package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Color scheme
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#3C3C3C")).
			Padding(0, 1)

	hitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	missStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true)

	tableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	activityHitStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00"))

	activityMissStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFA500"))

	activityTimeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))
)

// renderUI renders the complete UI
func (m *Model) renderUI() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("🎮 LanCache Spy - Steam Download Monitor"))
	b.WriteString("\n\n")

	// Game statistics table
	b.WriteString(m.renderGameTable())
	b.WriteString("\n\n")

	// Activity log
	b.WriteString(m.renderActivityLog())
	b.WriteString("\n\n")

	// Status bar
	b.WriteString(m.renderStatusBar())

	return b.String()
}

// renderGameTable renders the game statistics table
func (m *Model) renderGameTable() string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("Game Statistics"))
	b.WriteString("\n")

	// Table header
	header := fmt.Sprintf("%-10s | %-10s | %-45s | %-8s | %-8s | %-8s | %-8s",
		"Platform", "Game ID", "Game Name", "Total", "HITs", "MISSes", "Hit Rate")
	b.WriteString(tableRowStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", len(header)))
	b.WriteString("\n")

	// Game rows
	games := m.GetGameList()
	if len(games) == 0 {
		b.WriteString(tableRowStyle.Render("No downloads detected yet..."))
		return b.String()
	}

	// Limit to top 20 games
	maxGames := 20
	if len(games) > maxGames {
		games = games[:maxGames]
	}

	for _, stats := range games {
		gameName := stats.GameName
		if len(gameName) > 45 {
			gameName = gameName[:42] + "..."
		}

		hitRate := fmt.Sprintf("%.1f%%", stats.HitRate())

		row := fmt.Sprintf("%-10s | %-10s | %-45s | %-8d | %-8d | %-8d | %-8s",
			stats.Platform,
			stats.GameID,
			gameName,
			stats.Total,
			stats.Hits,
			stats.Misses,
			hitRate)

		b.WriteString(tableRowStyle.Render(row))
		b.WriteString("\n")
	}

	return b.String()
}

// renderActivityLog renders the recent activity log
func (m *Model) renderActivityLog() string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("Recent Activity"))
	b.WriteString("\n")

	activities := m.GetActivities()
	if len(activities) == 0 {
		b.WriteString(tableRowStyle.Render("No recent activity..."))
		return b.String()
	}

	for _, activity := range activities {
		timestamp := activityTimeStyle.Render(FormatTimestamp(activity.Timestamp))

		gameName := activity.GameName
		if len(gameName) > 40 {
			gameName = gameName[:37] + "..."
		}

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

		line := fmt.Sprintf("%s %s [%s] %s: %s",
			timestamp,
			statusIcon,
			activity.GameID,
			gameName,
			style.Render(statusText))

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderStatusBar renders the bottom status bar
func (m *Model) renderStatusBar() string {
	totalGames, pendingLookups := m.GetStats()

	status := fmt.Sprintf(
		"Total Lines: %d | Active Games: %d | Pending Lookups: %d | Press 'r' to reset | 'q' to quit",
		m.state.totalLines,
		totalGames,
		pendingLookups,
	)

	return statusBarStyle.Render(status)
}
