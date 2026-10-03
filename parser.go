package main

import (
	"regexp"
	"strings"
)

var (
	// Regex to match [steam] log lines and extract depot ID
	steamDepotRegex = regexp.MustCompile(`\[steam\].*?/depot/(\d+)/chunk/`)
	// Regex to match [sony] log lines and extract game ID (PPSA##### format)
	sonyGameRegex = regexp.MustCompile(`\[sony\].*?/gst/prod/\d+/(PPSA\d+)_`)
	// Regex to match [epicgames] log lines and extract game name from path
	epicGamesRegex = regexp.MustCompile(`\[epicgames\].*?"GET /([^/]+)/([^/]+)/`)
	// Regex to match [blizzard] log lines and extract game name from path
	blizzardRegex = regexp.MustCompile(`\[blizzard\].*?"GET /([^/]+)/([^/]+)/`)
	// Regex to extract HIT or MISS status
	statusRegex = regexp.MustCompile(`"(HIT|MISS)"`)
)

// LogEntry represents a parsed log line
type LogEntry struct {
	Platform string // "steam", "sony", "xboxlive", "epicgames", "blizzard"
	GameID   string // For steam: depot ID, for sony: PPSA ID, for others: game name from path
	Status   string // "HIT" or "MISS"
}

// ParseLogLine parses any supported platform log line
// Supports: steam, sony, xboxlive, epicgames, blizzard
// Returns nil if the line is not a valid log line
func ParseLogLine(line string) *LogEntry {
	if len(line) == 0 || line[0] != '[' {
		return nil
	}

	// Extract HIT/MISS status (common to all)
	statusMatches := statusRegex.FindStringSubmatch(line)
	if len(statusMatches) < 2 {
		return nil
	}
	status := statusMatches[1]

	// Check for Steam
	if steamMatches := steamDepotRegex.FindStringSubmatch(line); len(steamMatches) >= 2 {
		return &LogEntry{
			Platform: "steam",
			GameID:   steamMatches[1],
			Status:   status,
		}
	}

	// Check for Sony
	if sonyMatches := sonyGameRegex.FindStringSubmatch(line); len(sonyMatches) >= 2 {
		return &LogEntry{
			Platform: "sony",
			GameID:   sonyMatches[1],
			Status:   status,
		}
	}

	// Check for Xbox Live
	if len(line) > 10 && line[1:9] == "xboxlive" {
		return &LogEntry{
			Platform: "xboxlive",
			GameID:   "xboxlive",
			Status:   status,
		}
	}

	// Check for Epic Games - extract game name from path
	if epicMatches := epicGamesRegex.FindStringSubmatch(line); len(epicMatches) >= 3 {
		// Get the second path component and normalize to lowercase
		gameName := strings.ToLower(epicMatches[2])
		return &LogEntry{
			Platform: "epicgames",
			GameID:   gameName,
			Status:   status,
		}
	}

	// Check for Blizzard - extract game name from path
	if blizzardMatches := blizzardRegex.FindStringSubmatch(line); len(blizzardMatches) >= 3 {
		// Get the second path component and normalize to lowercase
		gameName := strings.ToLower(blizzardMatches[2])
		return &LogEntry{
			Platform: "blizzard",
			GameID:   gameName,
			Status:   status,
		}
	}

	return nil
}

// ParseSteamLogLine for backward compatibility - wraps ParseLogLine
func ParseSteamLogLine(line string) *LogEntry {
	return ParseLogLine(line)
}
