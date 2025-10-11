package main

// commonDepotMappings is a manually maintained map of depot IDs to game names
// This is a fallback for when we can't scrape SteamDB
var commonDepotMappings = map[int]string{
	// Rocket League (252950)
	252951: "Rocket League",
	252952: "Rocket League",
	252953: "Rocket League",

	// Counter-Strike 2 (730)
	731: "Counter-Strike 2",
	732: "Counter-Strike 2",
	733: "Counter-Strike 2",
	2347770: "Counter-Strike 2",
	2347771: "Counter-Strike 2",

	// Dota 2 (570)
	571: "Dota 2",
	572: "Dota 2",
	373301: "Dota 2",
	381451: "Dota 2",

	// Team Fortress 2 (440)
	441: "Team Fortress 2",
	442: "Team Fortress 2",

	// Rust (252490)
	252491: "Rust",
	252492: "Rust",

	// GTA V (271590)
	271591: "Grand Theft Auto V",
	271592: "Grand Theft Auto V",

	// Apex Legends (1172470)
	1172471: "Apex Legends",
	1172472: "Apex Legends",

	// PUBG (578080)
	578081: "PUBG: BATTLEGROUNDS",
	578082: "PUBG: BATTLEGROUNDS",

	// Farming Simulator 25 (2300320)
	2300321: "Farming Simulator 25",
	2300322: "Farming Simulator 25",

	// Add more as you discover them in your logs
}

// GetGameNameFromDepot looks up a depot ID in the common mappings
func GetGameNameFromDepot(depotID int) (string, bool) {
	name, ok := commonDepotMappings[depotID]
	return name, ok
}
