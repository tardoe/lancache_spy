package main

import (
	"regexp"
	"strconv"
)

var (
	// Regex to match [steam] log lines and extract depot ID
	steamDepotRegex = regexp.MustCompile(`\[steam\].*?/depot/(\d+)/chunk/`)
	// Regex to extract HIT or MISS status
	statusRegex = regexp.MustCompile(`"(HIT|MISS)"`)
)

// LogEntry represents a parsed Steam log line
type LogEntry struct {
	GameID int
	Status string // "HIT" or "MISS"
}

// ParseSteamLogLine parses a Steam log line and extracts game ID and HIT/MISS status
// Returns nil if the line is not a valid Steam depot log line
func ParseSteamLogLine(line string) *LogEntry {
	// Check if this is a Steam log line
	if len(line) == 0 || line[0] != '[' {
		return nil
	}

	// Extract depot ID (game ID)
	depotMatches := steamDepotRegex.FindStringSubmatch(line)
	if len(depotMatches) < 2 {
		return nil
	}

	gameID, err := strconv.Atoi(depotMatches[1])
	if err != nil {
		return nil
	}

	// Extract HIT/MISS status
	statusMatches := statusRegex.FindStringSubmatch(line)
	if len(statusMatches) < 2 {
		return nil
	}

	return &LogEntry{
		GameID: gameID,
		Status: statusMatches[1],
	}
}
