# lancache_spy

A terminal UI that tails a [LanCache](https://lancache.net) monolithic access log and shows, per game, what is being served from the cache (HIT) versus filled from upstream (MISS).

Supported platforms: Steam, PlayStation, Xbox Live, Epic Games and Blizzard. Steam depot IDs and PlayStation title IDs are resolved to game names by scraping SteamDB and prosperopatches.com (rate-limited to one request per second each).

## Build

```sh
make build   # ./lancache_spy for the host
make linux   # ./lancache_spy_linux_amd64
make test
```

## Usage

```sh
./lancache_spy -log /path/to/lancache/logs/access.log
```

| Flag | Description |
| --- | --- |
| `-log` | Log file to tail (required). Starts at the end of the file, like `tail -f`. |
| `-depot-db` | JSON file mapping Steam depot IDs to `{"appid", "appname"}`, checked before scraping. |
| `-no-resolve` | Show raw game IDs without resolving names. |
| `-debug` | Write a debug log to `lancache_spy_debug.log`. |

Keys: `r` resets the stats, `q` quits.
