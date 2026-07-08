# ADS-B Performance Dashboard

## Overview

A lightweight, self-contained web dashboard that reads from dump1090-fa's `stats.json` and displays real-time receiver performance metrics. Designed to run on a Raspberry Pi Zero (single-core, 512MB RAM).

## Architecture

```
dump1090-fa  -->  stats.json  -->  Go server (polls every 60s)
                                      |
                                      v
                               SSE endpoint /events
                                      |
                                      v
                              Browser dashboard
                              (vanilla JS, DOM + canvas)
```

- Single Go binary, all assets embedded via `//go:embed`
- Serves on port 8080 (configurable)
- Poll interval defaults to 60s (matches dump1090-fa write frequency), configurable via `--poll` flag

## Data Source

Reads `/run/dump1090-fa/stats.json` (configurable via `--stats` flag). The JSON structure contains:

- `latest` — current per-second message counts (total, valid, strong, weak, error)
- `last1min` / `last5min` / `last15min` — aggregated stats over time windows
- `aircraft` — currently tracked and max aircraft seen

## Dashboard Layout

Responsive CSS grid, dark theme, cards with live-updating stats:

1. **Messages/sec** — current rate, total accumulated
2. **Aircraft Tracked** — current count, peak today
3. **Signal Quality** — strong/weak/error ratios as horizontal stacked bar + percentage
4. **CPU/Memory** — system resource usage (read from `/proc/stat` and `/proc/meminfo`)
5. **Uptime & Health** — receiver uptime, last message timestamp, data age indicator
6. **Messages Over Time** — sparkline (canvas) over last N samples
7. **Aircraft Over Time** — sparkline (canvas) over last N samples

## Server Endpoints

| Path | Description |
|------|-------------|
| `GET /` | Serves the dashboard HTML page |
| `GET /events` | SSE endpoint streaming stats JSON every poll interval |
| `GET /health` | Health check returning server + data freshness status |

## Frontend

- Vanilla JS, no frameworks or libraries
- CSS Grid layout, dark theme, responsive
- Canvas-based sparklines (no chart library)
- SSE `EventSource` connection to `/events` — auto-reconnects on disconnect

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8081` | HTTP server port |
| `--stats` | `/run/dump1090-fa/stats.json` | Path to stats.json |
| `--poll` | `60s` | How often to re-read stats.json |

## Build & Deploy

```bash
# Build (on dev machine, cross-compile for linux/arm)
GOOS=linux GOARCH=arm GOARM=6 go build -o adsb-dashboard

# Deploy to Pi Zero
scp adsb-dashboard pi@raspberrypi:~
ssh pi@raspberrypi './adsb-dashboard'

# Optional: systemd service for auto-start
```

## Safety & Protections

The dashboard **must never** interfere with piaware/dump1090-fa operation:

- **Read-only access** — the Go server opens `stats.json` with `O_RDONLY` only. Never writes to the dump1090-fa directory or any of its files.
- **Minimum poll floor** — `--poll` has a hard floor of 10s (enforced in code). Even if misconfigured, it cannot hammer the filesystem.
- **No child processes** — the Go binary does not exec or shell out to any system commands for data. All data comes from reading `stats.json` and `/proc/*` files.
- **Low resource profile** — Go binary targets < 10MB RSS, no external dependencies, no background goroutines beyond the poll timer.
- **No filesystem writes** — zero write operations to disk. No logs, no temp files, no cache.
- **systemd isolation** — if deployed as a systemd service, uses `CPUSchedulingPolicy=idle` and `IOSchedulingClass=idle` to yield to piaware processes.

## Security Considerations

Designed for a trusted home network. Assumptions and mitigations:

- **Bind to localhost by default** — server defaults to `--addr 127.0.0.1`, not `0.0.0.0`. Remote access requires explicit opt-in. This prevents LAN scanning from discovering the dashboard.
- **No write endpoints exist** — the entire API surface is three read-only GET endpoints. No POST, PUT, DELETE, no file upload, no command execution.
- **No authentication on localhost** — if you need remote access, use an SSH tunnel or a reverse proxy (e.g., nginx with basic auth). This is out of scope for this project.
- **No sensitive data** — the dashboard only exposes ADS-B performance metrics (message rates, signal quality). No IPs, credentials, or personal information.
- **Minimal attack surface** — single Go binary, no dependencies, no interpreter, no dynamic code loading. SSE is a long-lived GET connection — no WebSocket upgrade complexity.
- **High port (8080)** — runs as non-root on a high port, avoiding privilege escalation risks.

## Non-Goals

- Authentication / HTTPS (assumes localhost-only or SSH tunnel for remote access)
- Historical persistence (all data is in-memory, ephemeral)
- Multiple receiver support
