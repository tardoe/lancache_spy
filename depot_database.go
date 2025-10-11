package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// DepotInfo represents depot information from the JSON database
type DepotInfo struct {
	AppID   int         `json:"appid"`
	AppName string      `json:"appname"`
	Size    interface{} `json:"size"` // Can be string or number
}

// DepotDatabase manages depot ID lookups from a JSON file
type DepotDatabase struct {
	depots map[string]DepotInfo
	mutex  sync.RWMutex
}

// LoadDepotDatabase loads a depot JSON file
func LoadDepotDatabase(filepath string) (*DepotDatabase, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read depot database: %w", err)
	}

	depots := make(map[string]DepotInfo)
	if err := json.Unmarshal(data, &depots); err != nil {
		return nil, fmt.Errorf("failed to parse depot database: %w", err)
	}

	return &DepotDatabase{
		depots: depots,
	}, nil
}

// GetGameName looks up a depot ID and returns the game name
func (db *DepotDatabase) GetGameName(depotID string) (string, bool) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	if info, ok := db.depots[depotID]; ok {
		return info.AppName, true
	}
	return "", false
}

// GetAppID looks up a depot ID and returns the app ID
func (db *DepotDatabase) GetAppID(depotID string) (int, bool) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	if info, ok := db.depots[depotID]; ok {
		return info.AppID, true
	}
	return 0, false
}
