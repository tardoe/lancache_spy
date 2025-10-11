package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Parse command line flags
	logFilePath := flag.String("log", "", "Path to nginx log file to tail")
	depotDB := flag.String("depot-db", "", "Path to depot JSON database file for Steam game name resolution")
	noResolve := flag.Bool("no-resolve", false, "Disable game name resolution (show only game IDs)")
	debug := flag.Bool("debug", false, "Enable debug logging to lancache_spy_debug.log")
	flag.Parse()

	if *debug {
		initDebugLog()
		debugf("Starting lancache_spy with log file: %s", *logFilePath)
	}

	// Load depot database if provided
	var depotDatabase *DepotDatabase
	if *depotDB != "" {
		var err error
		depotDatabase, err = LoadDepotDatabase(*depotDB)
		if err != nil {
			fmt.Printf("Error loading depot database: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Loaded depot database with %d entries\n", len(depotDatabase.depots))
	}

	// Validate log file path
	if *logFilePath == "" {
		fmt.Println("Error: -log flag is required")
		fmt.Println("Usage: lancache_spy -log /path/to/nginx/access.log [-no-resolve]")
		os.Exit(1)
	}

	// Check if file exists
	if _, err := os.Stat(*logFilePath); os.IsNotExist(err) {
		fmt.Printf("Error: Log file does not exist: %s\n", *logFilePath)
		os.Exit(1)
	}

	// Create and run the bubbletea program
	p := tea.NewProgram(
		NewModel(*logFilePath, *noResolve, depotDatabase),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
