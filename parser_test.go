package main

import (
	"testing"
)

func TestParseSteamLogLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected *LogEntry
	}{
		{
			name: "Valid Steam HIT log line",
			line: `[steam] 10.10.99.5 / 10.16.10.39 - - [10/Oct/2025:10:51:05 +0100] "GET /depot/252951/chunk/d6b10a8a6b2db9e95127c1f039e591ba45f1144c HTTP/1.0" 200 1039728 "-" "Valve/Steam HTTP Client 1.0" "HIT" "cache2-syd1.steamcontent.com" "bytes=0-1048575"`,
			expected: &LogEntry{
				GameID: 252951,
				Status: "HIT",
			},
		},
		{
			name: "Valid Steam MISS log line",
			line: `[steam] 10.10.99.1 / 10.16.10.26 - - [10/Oct/2025:10:51:05 +0100] "GET /depot/553853/chunk/4fa101f66417b625c96dc779d0b3ef4d4756e835 HTTP/1.0" 206 128 "-" "Valve/Steam HTTP Client 1.0" "MISS" "steampipe.akamaized.net" "bytes=0-1048575"`,
			expected: &LogEntry{
				GameID: 553853,
				Status: "MISS",
			},
		},
		{
			name:     "Non-Steam log line (sony)",
			line:     `[sony] 10.10.99.4 / 10.17.10.18 - - [10/Oct/2025:10:51:04 +0100] "GET /gst/prod/00/PPSA15301_00/app/pkg/51/f_fcde92ca44c1072bcb599e0e44b07e9b74bd63360e3eb510110e286c37063176/UP1001-PPSA15301_00-NBA2K24000000000_29.pkg?product=0289&serverIpAddr=10.10.99.4&r=00000002 HTTP/1.0" 206 1048576 "-" "libhttp/12.02 (PlayStation 5)" "MISS" "gst.prod.dl.playstation.net" "bytes=250609664-251658239"`,
			expected: nil,
		},
		{
			name:     "Non-Steam log line (xboxlive)",
			line:     `[xboxlive] 10.10.99.5 / 10.17.10.9 - - [10/Oct/2025:10:51:04 +0100] "GET /15/bac62044-55f1-440c-8b0d-1ccd69c43a1b/0d69c812-410d-4834-925a-c9ecece0ad37/1.3528.0.0.18be6615-b8bd-4d35-90a1-b8dd94dc3f77/ACE-Asia_1.3528.0.0_x64__a43ghgr8ekz80 HTTP/1.0" 206 1048576 "-" "-" "HIT" "assets1.xboxlive.com" "bytes=83842039808-83843088383"`,
			expected: nil,
		},
		{
			name:     "Empty line",
			line:     "",
			expected: nil,
		},
		{
			name:     "Invalid log line",
			line:     "some random text",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSteamLogLine(tt.line)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected %+v, got nil", tt.expected)
				return
			}

			if result.GameID != tt.expected.GameID {
				t.Errorf("GameID mismatch: expected %d, got %d", tt.expected.GameID, result.GameID)
			}

			if result.Status != tt.expected.Status {
				t.Errorf("Status mismatch: expected %s, got %s", tt.expected.Status, result.Status)
			}
		})
	}
}

func TestGameStatsHitRate(t *testing.T) {
	tests := []struct {
		name     string
		stats    GameStats
		expected float64
	}{
		{
			name: "50% hit rate",
			stats: GameStats{
				Total:  100,
				Hits:   50,
				Misses: 50,
			},
			expected: 50.0,
		},
		{
			name: "100% hit rate",
			stats: GameStats{
				Total:  100,
				Hits:   100,
				Misses: 0,
			},
			expected: 100.0,
		},
		{
			name: "0% hit rate",
			stats: GameStats{
				Total:  100,
				Hits:   0,
				Misses: 100,
			},
			expected: 0.0,
		},
		{
			name: "No activity",
			stats: GameStats{
				Total:  0,
				Hits:   0,
				Misses: 0,
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.stats.HitRate()
			if result != tt.expected {
				t.Errorf("Expected hit rate %.2f%%, got %.2f%%", tt.expected, result)
			}
		})
	}
}
