# Dev Journal

## 2026-07-07 — ADS-B Performance Dashboard

### Session Summary

Built a lightweight Go web dashboard that reads from dump1090-fa's `stats.json` and serves real-time ADS-B receiver performance stats via Server-Sent Events. Targets Raspberry Pi Zero (linux/arm, GOARM=6).

### What We Built

- **Go backend** — single binary, zero external dependencies, embedded static assets via `//go:embed`
  - `stats.go` — Stats data types matching real dump1090 stats.json format, /proc CPU/memory reader
  - `sse.go` — SSE broker (hub pattern) with slow-client protection
  - `server.go` — HTTP server with poll loop, health endpoint, static file serving
  - `main.go` — entry point with `--addr`, `--port`, `--stats`, `--poll` flags
- **Frontend** — vanilla JS, dark theme CSS grid, canvas sparklines, EventSource
  - Cards: Messages/sec, Aircraft Tracked, Signal Quality (SNR), System CPU/Mem, Health, sparkline charts
- **Systemd service** — idle scheduling policy, strict read-only paths, no privilege escalation

### Key Decisions

- **SSE over polling** — Server-Sent Events for real-time push with minimal Pi Zero resource usage
- **Localhost-only by default** — `--addr 127.0.0.1` to prevent LAN discovery; override with `--addr 0.0.0.0` for network access
- **60s poll interval** — matches dump1090-fa stats.json write frequency; enforces 10s minimum floor
- **Stats format** — discovered real stats.json differs from assumed format; updated structs to match nested `last1min/local/tracks` structure with `accepted` array for aircraft counts
- **Signal quality** — uses SNR (signal - noise dB) and strong signal ratio instead of invalid strong/weak/error breakdown

### Deployment

Deployed to Pi at 192.168.x.x. Port changed from 8080 → 8081 due to conflict with piaware web interface. Systemd service configured with `CPUSchedulingPolicy=idle` and `IOSchedulingClass=idle`.

### Build

```bash
GOOS=linux GOARCH=arm GOARM=6 go build -o adsb-dashboard .
```
Binary size: 8.4MB compressed, ~20MB RSS at runtime.

### Commits

```
e8d0476 docs: add dashboard design spec and implementation plan
fa33c15 feat: add Stats data types and JSON parsing
7edf542 feat: add StatsFile reader and /proc system stats reader
d52a707 feat: add SSE event broker with subscribe/broadcast
e345d49 feat: add HTTP server with SSE, health, and static file serving
8b2cbe3 fix: remove main() from server.go (belongs in Task 5 main.go)
2294cc5 feat: add main entry point with flag parsing
6ad9239 docs: add systemd service file and build instructions
7331804 fix: address code review findings
```

### Open Items

- [ ] Authentication/HTTPS if exposed beyond local network
- [ ] Historical persistence (all data currently in-memory)
- [ ] Multiple receiver support
