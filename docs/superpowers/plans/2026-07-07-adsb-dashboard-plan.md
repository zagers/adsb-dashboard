# ADS-B Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a lightweight Go web dashboard that reads dump1090-fa's `stats.json` and serves real-time performance metrics via SSE.

**Architecture:** Single Go binary with embedded static assets. Polls `stats.json` every 60s, broadcasts via SSE hub. Frontend is vanilla JS with canvas sparklines. Read-only, bind to localhost, no writes to disk.

**Tech Stack:** Go 1.21+, embedded HTML/CSS/JS, SSE, canvas API.

## Global Constraints

- All file reads are O_RDONLY only — never write to disk
- `--poll` flag has a hard minimum floor of 10s
- Server defaults to `--addr 127.0.0.1`
- No external dependencies beyond stdlib
- Cross-compile target: `GOOS=linux GOARCH=arm GOARM=6`
- Binary must stay under 10MB, RSS under 20MB

---

## File Structure

```
adsb-dashboard/
├── go.mod
├── main.go                # Entry point, flag parsing, wiring
├── stats.go               # Stats struct, JSON parsing, /proc reading
├── sse.go                 # SSE event broker (hub pattern)
├── server.go              # HTTP routes, handlers, embedded assets
├── stats_test.go          # Tests for stats parsing
├── sse_test.go            # Tests for SSE broker
├── server_test.go         # Integration tests for HTTP server
└── static/
    ├── index.html         # Dashboard HTML
    ├── style.css          # Dark theme CSS
    └── script.js          # Vanilla JS + canvas sparklines
```

---

### Task 1: Project scaffold + Stats data types

**Files:**
- Create: `adsb-dashboard/go.mod`
- Create: `adsb-dashboard/stats.go`
- Create: `adsb-dashboard/stats_test.go`

**Interfaces:**
- Produces: `type Stats struct` — the canonical in-memory representation of stats.json content
- Produces: `type SystemStats struct` — CPU/memory snapshot from /proc

- [ ] **Step 1: Initialize go.mod**

```bash
mkdir -p adsb-dashboard && cd adsb-dashboard
go mod init github.com/yourname/adsb-dashboard
```

- [ ] **Step 2: Write the test for stats.json parsing**

`adsb-dashboard/stats_test.go`:
```go
package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStatsParsing(t *testing.T) {
	sample := `{
		"latest": {
			"total": 5000,
			"valid": 4123,
			"strong": 2800,
			"weak": 1323,
			"error": 877
		},
		"aircraft": {
			"now": 142,
			"max": 189
		},
		"last1min": {"messages": 12000, "aircraft": 155},
		"last5min": {"messages": 58000, "aircraft": 170},
		"last15min": {"messages": 170000, "aircraft": 189}
	}`

	var s Stats
	if err := json.Unmarshal([]byte(sample), &s); err != nil {
		t.Fatalf("failed to parse stats.json: %v", err)
	}

	if s.Latest.Total != 5000 {
		t.Errorf("expected 5000, got %d", s.Latest.Total)
	}
	if s.Latest.Valid != 4123 {
		t.Errorf("expected 4123, got %d", s.Latest.Valid)
	}
	if s.Latest.Strong != 2800 {
		t.Errorf("expected 2800, got %d", s.Latest.Strong)
	}
	if s.Aircraft.Now != 142 {
		t.Errorf("expected 142, got %d", s.Aircraft.Now)
	}
	if s.Last1Min.Messages != 12000 {
		t.Errorf("expected 12000, got %d", s.Last1Min.Messages)
	}
}

func TestStatsSignalRatio(t *testing.T) {
	s := Stats{
		Latest: LatestStats{
			Total:  1000,
			Valid:  900,
			Strong: 600,
			Weak:   300,
			Error:  100,
		},
	}
	if s.StrongRatio() != 0.6 {
		t.Errorf("expected 0.6, got %f", s.StrongRatio())
	}
	if s.WeakRatio() != 0.3 {
		t.Errorf("expected 0.3, got %f", s.WeakRatio())
	}
	if s.ErrorRatio() != 0.1 {
		t.Errorf("expected 0.1, got %f", s.ErrorRatio())
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./... -v`
Expected: FAIL — "undefined: Stats"

- [ ] **Step 4: Write Stats struct definitions**

`adsb-dashboard/stats.go`:
```go
package main

type Stats struct {
	Latest    LatestStats `json:"latest"`
	Aircraft  Aircraft    `json:"aircraft"`
	Last1Min  TimeWindow  `json:"last1min"`
	Last5Min  TimeWindow  `json:"last5min"`
	Last15Min TimeWindow  `json:"last15min"`
	Now       int64       `json:"now"`
	Version   int         `json:"version"`
}

type LatestStats struct {
	Total  int `json:"total"`
	Valid  int `json:"valid"`
	Strong int `json:"strong"`
	Weak   int `json:"weak"`
	Error  int `json:"error"`
}

type Aircraft struct {
	Now int `json:"now"`
	Max int `json:"max"`
}

type TimeWindow struct {
	Messages int `json:"messages"`
	Aircraft int `json:"aircraft"`
}

func (s Stats) StrongRatio() float64 {
	if s.Latest.Total == 0 {
		return 0
	}
	return float64(s.Latest.Strong) / float64(s.Latest.Total)
}

func (s Stats) WeakRatio() float64 {
	if s.Latest.Total == 0 {
		return 0
	}
	return float64(s.Latest.Weak) / float64(s.Latest.Total)
}

func (s Stats) ErrorRatio() float64 {
	if s.Latest.Total == 0 {
		return 0
	}
	return float64(s.Latest.Error) / float64(s.Latest.Total)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add adsb-dashboard/go.mod adsb-dashboard/stats.go adsb-dashboard/stats_test.go
git commit -m "feat: add Stats data types and JSON parsing"
```

---

### Task 2: Stats reader + /proc reader

**Files:**
- Create: `adsb-dashboard/stats.go` (append SystemStats and reader functions)
- Modify: `adsb-dashboard/stats_test.go` (append tests)

**Interfaces:**
- Consumes: `Stats` struct from Task 1
- Produces: `type SystemStats struct{ CPUPercent float64; MemPercent float64 }`
- Produces: `func ReadStatsFile(path string) (*Stats, error)`
- Produces: `func ReadSystemStats() (*SystemStats, error)`

- [ ] **Step 1: Write failing tests for stats file reader and /proc reader**

Append to `adsb-dashboard/stats_test.go`:
```go
func TestReadStatsFile(t *testing.T) {
	// Create temp stats.json
	f, err := os.CreateTemp(t.TempDir(), "stats*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	sample := `{"latest":{"total":100},"aircraft":{"now":5,"max":10}}`
	if _, err := f.Write([]byte(sample)); err != nil {
		t.Fatal(err)
	}
	f.Close()

	s, err := ReadStatsFile(f.Name())
	if err != nil {
		t.Fatalf("ReadStatsFile failed: %v", err)
	}
	if s.Latest.Total != 100 {
		t.Errorf("expected 100, got %d", s.Latest.Total)
	}
	if s.Aircraft.Now != 5 {
		t.Errorf("expected 5, got %d", s.Aircraft.Now)
	}
}

func TestReadStatsFileNotFound(t *testing.T) {
	_, err := ReadStatsFile("/nonexistent/stats.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadSystemStats(t *testing.T) {
	s, err := ReadSystemStats()
	if err != nil {
		t.Fatalf("ReadSystemStats failed: %v", err)
	}
	if s.CPUPercent < 0 || s.CPUPercent > 100 {
		t.Errorf("CPUPercent out of range: %f", s.CPUPercent)
	}
	if s.MemPercent < 0 || s.MemPercent > 100 {
		t.Errorf("MemPercent out of range: %f", s.MemPercent)
	}
}
```

Add import to test file:
```go
import (
	"encoding/json"
	"os"
	"testing"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -v`
Expected: FAIL — "undefined: ReadStatsFile"

- [ ] **Step 3: Implement SystemStats + reader functions**

Append to `adsb-dashboard/stats.go`:
```go
import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type SystemStats struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
}

func ReadStatsFile(path string) (*Stats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading stats file: %w", err)
	}

	var s Stats
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing stats file: %w", err)
	}

	return &s, nil
}

func ReadSystemStats() (*SystemStats, error) {
	cpu, err := readCPUPercent()
	if err != nil {
		return nil, err
	}
	mem, err := readMemPercent()
	if err != nil {
		return nil, err
	}
	return &SystemStats{CPUPercent: cpu, MemPercent: mem}, nil
}

func readCPUPercent() (float64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	var cpuUser, cpuNice, cpuSystem, cpuIdle uint64
	_, err = fmt.Sscanf(string(data), "cpu %d %d %d %d", &cpuUser, &cpuNice, &cpuSystem, &cpuIdle)
	if err != nil {
		return 0, err
	}

	total := cpuUser + cpuNice + cpuSystem + cpuIdle
	idle := cpuIdle

	time.Sleep(100 * time.Millisecond)

	data2, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	var cpuUser2, cpuNice2, cpuSystem2, cpuIdle2 uint64
	_, err = fmt.Sscanf(string(data2), "cpu %d %d %d %d", &cpuUser2, &cpuNice2, &cpuSystem2, &cpuIdle2)
	if err != nil {
		return 0, err
	}

	totalDelta := (cpuUser2 + cpuNice2 + cpuSystem2 + cpuIdle2) - total
	idleDelta := cpuIdle2 - idle

	if totalDelta == 0 {
		return 0, nil
	}

	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100, nil
}

func readMemPercent() (float64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	var total, available uint64

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseMemValue(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseMemValue(line)
		}
	}

	if total == 0 {
		return 0, fmt.Errorf("could not parse MemTotal from /proc/meminfo")
	}

	used := total - available
	return float64(used) / float64(total) * 100, nil
}

func parseMemValue(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS (TestReadStatsFile and TestReadSystemStats pass)

Note: TestReadSystemStats reads from /proc, which works on Linux. On macOS, the test will fail — this is expected; the binary is only meant to run on the Pi (Linux ARM). Add a build tag if desired later.

- [ ] **Step 5: Commit**

```bash
git add adsb-dashboard/stats.go adsb-dashboard/stats_test.go
git commit -m "feat: add StatsFile reader and /proc system stats reader"
```

---

### Task 3: SSE event broker

**Files:**
- Create: `adsb-dashboard/sse.go`
- Create: `adsb-dashboard/sse_test.go`

**Interfaces:**
- Consumes: `*Stats` + `*SystemStats` from Task 2
- Produces: `type Broker struct`
- Produces: `func NewBroker() *Broker`
- Produces: `func (b *Broker) Subscribe() chan []byte`
- Produces: `func (b *Broker) Unsubscribe(ch chan []byte)`
- Produces: `func (b *Broker) Broadcast(data []byte)`
- Produces: `func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request)` — SSE handler

- [ ] **Step 1: Write failing test for SSE broker**

`adsb-dashboard/sse_test.go`:
```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrokerSubscribeBroadcast(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	msg := []byte(`{"test": true}`)
	b.Broadcast(msg)

	select {
	case received := <-ch:
		if string(received) != string(msg) {
			t.Errorf("expected %s, got %s", msg, received)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestBrokerUnsubscribe(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	b.Unsubscribe(ch)

	// Should not panic or hang
	b.Broadcast([]byte(`{"test": true}`))
}

func TestBrokerServeHTTP(t *testing.T) {
	b := NewBroker()

	// Start broadcasting in background
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			b.Broadcast([]byte(`{"test": true}`))
		}
	}()

	req := httptest.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()

	b.ServeHTTP(w, req)

	// Wait for at least one SSE event
	body := w.Body.String()
	if !strings.Contains(body, "data:") {
		t.Errorf("expected SSE data:, got: %s", body)
	}
	if !strings.Contains(body, `{"test": true}`) {
		t.Errorf("expected JSON in SSE, got: %s", body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -v`
Expected: FAIL — "undefined: NewBroker"

- [ ] **Step 3: Implement SSE broker**

`adsb-dashboard/sse.go`:
```go
package main

import (
	"fmt"
	"net/http"
	"sync"
)

type Broker struct {
	mu       sync.RWMutex
	clients  map[chan []byte]struct{}
	register chan chan []byte
}

func NewBroker() *Broker {
	return &Broker{
		clients:  make(map[chan []byte]struct{}),
		register: make(chan chan []byte),
	}
}

func (b *Broker) Subscribe() chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan []byte, 8)
	b.clients[ch] = struct{}{}
	return ch
}

func (b *Broker) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	close(ch)
}

func (b *Broker) Broadcast(data []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- data:
		default:
			// drop slow client
		}
	}
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Send initial keepalive
	fmt.Fprintf(w, ": heartbeat\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add adsb-dashboard/sse.go adsb-dashboard/sse_test.go
git commit -m "feat: add SSE event broker with subscribe/broadcast"
```

---

### Task 4: HTTP server

**Files:**
- Create: `adsb-dashboard/server.go`
- Create: `adsb-dashboard/server_test.go`
- Create: `adsb-dashboard/static/index.html`
- Create: `adsb-dashboard/static/style.css`
- Create: `adsb-dashboard/static/script.js`

**Interfaces:**
- Consumes: `*Broker` from Task 3
- Produces: `func NewServer(broker *Broker, statsPath string, pollInterval time.Duration) *Server`
- Produces: `func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)` — root mux
- Produces: `func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request)`

- [ ] **Step 1: Write failing tests for HTTP server**

`adsb-dashboard/server_test.go`:
```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServerHealthEndpoint(t *testing.T) {
	broker := NewBroker()
	s := NewServer(broker, "/tmp/stats.json", 60*time.Second)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse health response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
}

func TestServerRootEndpoint(t *testing.T) {
	broker := NewBroker()
	s := NewServer(broker, "/tmp/stats.json", 60*time.Second)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("expected HTML content type, got %s", ct)
	}
}

func TestServerSSEEndpoint(t *testing.T) {
	broker := NewBroker()
	s := NewServer(broker, "/tmp/stats.json", 60*time.Second)

	req := httptest.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", ct)
	}
}

func TestServerNotFound(t *testing.T) {
	broker := NewBroker()
	s := NewServer(broker, "/tmp/stats.json", 60*time.Second)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	s.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -v`
Expected: FAIL — "undefined: NewServer"

- [ ] **Step 3: Implement the server**

`adsb-dashboard/server.go`:
```go
package main

import (
	"encoding/json"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

//go:embed static
var staticFS embed.FS

type Server struct {
	broker      *Broker
	statsPath   string
	pollInterval time.Duration
	lastStats   atomic.Value // stores *Stats
	lastSysStats atomic.Value // stores *SystemStats
}

func NewServer(broker *Broker, statsPath string, pollInterval time.Duration) *Server {
	s := &Server{
		broker:       broker,
		statsPath:    statsPath,
		pollInterval: pollInterval,
	}
	go s.pollLoop()
	return s
}

func (s *Server) pollIntervalDuration() time.Duration {
	// Hard floor of 10s
	if s.pollInterval < 10*time.Second {
		return 10 * time.Second
	}
	return s.pollInterval
}

func (s *Server) pollLoop() {
	ticker := time.NewTicker(s.pollIntervalDuration())
	defer ticker.Stop()

	// Do initial read immediately
	s.readAndBroadcast()

	for range ticker.C {
		s.readAndBroadcast()
	}
}

func (s *Server) readAndBroadcast() {
	stats, err := ReadStatsFile(s.statsPath)
	if err != nil {
		// stats.json may not exist yet or be temporarily unavailable
		return
	}
	s.lastStats.Store(stats)

	sysStats, err := ReadSystemStats()
	if err != nil {
		return
	}
	s.lastSysStats.Store(sysStats)

	type payload struct {
		Stats *Stats       `json:"stats"`
		System *SystemStats `json:"system"`
	}

	data, _ := json.Marshal(payload{Stats: stats, System: sysStats})
	s.broker.Broadcast(data)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.Handle("/events", s.broker)
	mux.Handle("/", s.staticHandler())
	mux.ServeHTTP(w, r)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := s.lastStats.Load()
	status := "ok"
	if stats == nil {
		status = "no_data"
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"time":   time.Now().Unix(),
	})
}

func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// Fallback: just return 404
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}
```

Also add helper for `ServeHTTP` on `Broker` so it satisfies `http.Handler` — already done in Task 3.

- [ ] **Step 4: Create static frontend files**

`adsb-dashboard/static/index.html`:
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ADS-B Dashboard</title>
<link rel="stylesheet" href="/style.css">
</head>
<body>
<header>
  <h1>ADS-B Performance Dashboard</h1>
  <span id="status" class="status-indicator">connecting</span>
</header>
<main class="grid">
  <div class="card" id="card-mps">
    <h2>Messages / sec</h2>
    <div class="value" id="mps-value">--</div>
    <div class="sub" id="mps-total">total: --</div>
  </div>
  <div class="card" id="card-ac">
    <h2>Aircraft Tracked</h2>
    <div class="value" id="ac-now">--</div>
    <div class="sub" id="ac-max">peak: --</div>
  </div>
  <div class="card" id="card-signal">
    <h2>Signal Quality</h2>
    <div class="bar-container" id="signal-bar"></div>
    <div class="sub" id="signal-labels">--</div>
  </div>
  <div class="card" id="card-sys">
    <h2>System</h2>
    <div class="value" id="sys-cpu">CPU: --</div>
    <div class="sub" id="sys-mem">MEM: --</div>
  </div>
  <div class="card" id="card-uptime">
    <h2>Health</h2>
    <div class="value" id="health-status">--</div>
    <div class="sub" id="health-age">--</div>
  </div>
  <div class="card wide" id="card-msg-history">
    <h2>Messages Over Time</h2>
    <canvas id="msg-sparkline" width="400" height="80"></canvas>
  </div>
  <div class="card wide" id="card-ac-history">
    <h2>Aircraft Over Time</h2>
    <canvas id="ac-sparkline" width="400" height="80"></canvas>
  </div>
</main>
<script src="/script.js"></script>
</body>
</html>
```

`adsb-dashboard/static/style.css`:
```css
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  background: #0d1117;
  color: #c9d1d9;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  padding: 1rem;
}
header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}
h1 { font-size: 1.5rem; font-weight: 600; }
.status-indicator {
  font-size: 0.8rem;
  padding: 0.25rem 0.75rem;
  border-radius: 999px;
  background: #21262d;
  color: #8b949e;
}
.status-indicator.online { background: #1a6f36; color: #7ee787; }
.status-indicator.offline { background: #6e1616; color: #ff7b72; }
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 1rem;
}
.card {
  background: #161b22;
  border: 1px solid #30363d;
  border-radius: 8px;
  padding: 1.25rem;
}
.card.wide { grid-column: 1 / -1; }
.card h2 {
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #8b949e;
  margin-bottom: 0.5rem;
}
.value { font-size: 2rem; font-weight: 700; }
.sub { font-size: 0.85rem; color: #8b949e; margin-top: 0.25rem; }
.bar-container {
  display: flex;
  height: 1.5rem;
  border-radius: 4px;
  overflow: hidden;
  margin-top: 0.5rem;
}
.bar-segment { transition: width 0.5s ease; }
.bar-segment.strong { background: #2ea043; }
.bar-segment.weak { background: #d29922; }
.bar-segment.error { background: #da3633; }
canvas { width: 100%; height: 80px; }
```

`adsb-dashboard/static/script.js`:
```js
const history = { messages: [], aircraft: [] };
const MAX_HISTORY = 60;

function connect() {
  const es = new EventSource('/events');
  const status = document.getElementById('status');

  es.onopen = () => {
    status.textContent = 'live';
    status.className = 'status-indicator online';
  };

  es.onerror = () => {
    status.textContent = 'disconnected';
    status.className = 'status-indicator offline';
  };

  es.onmessage = (e) => {
    try {
      const data = JSON.parse(e.data);
      updateDashboard(data);
    } catch (err) {
      console.error('parse error:', err);
    }
  };
}

function updateDashboard(data) {
  const s = data.stats;

  // Messages per second
  document.getElementById('mps-value').textContent = (s.latest.valid || 0).toLocaleString();
  document.getElementById('mps-total').textContent = `total: ${(s.latest.total || 0).toLocaleString()}`;

  // Aircraft
  document.getElementById('ac-now').textContent = s.aircraft.now || 0;
  document.getElementById('ac-max').textContent = `peak: ${s.aircraft.max || 0}`;

  // Signal bar
  const strong = s.strong_ratio || (s.latest.total > 0 ? s.latest.strong / s.latest.total : 0);
  const weak = s.weak_ratio || (s.latest.total > 0 ? s.latest.weak / s.latest.total : 0);
  const err = s.error_ratio || (s.latest.total > 0 ? s.latest.error / s.latest.total : 0);
  renderSignalBar(strong, weak, err);

  // System
  if (data.system) {
    document.getElementById('sys-cpu').textContent = `CPU: ${data.system.cpu_percent.toFixed(1)}%`;
    document.getElementById('sys-mem').textContent = `MEM: ${data.system.mem_percent.toFixed(1)}%`;
  }

  // Health
  const now = s.now ? new Date(s.now * 1000) : new Date();
  const age = Math.floor((Date.now() - now.getTime()) / 1000);
  document.getElementById('health-status').textContent = age < 120 ? 'healthy' : 'stale';
  document.getElementById('health-age').textContent = `last update: ${age}s ago`;

  // History
  history.messages.push(s.latest.valid || 0);
  history.aircraft.push(s.aircraft.now || 0);
  if (history.messages.length > MAX_HISTORY) history.messages.shift();
  if (history.aircraft.length > MAX_HISTORY) history.aircraft.shift();

  drawSparkline('msg-sparkline', history.messages, '#2ea043');
  drawSparkline('ac-sparkline', history.aircraft, '#58a6ff');
}

function renderSignalBar(strong, weak, err) {
  const bar = document.getElementById('signal-bar');
  bar.innerHTML = `
    <div class="bar-segment strong" style="width:${(strong * 100).toFixed(1)}%"></div>
    <div class="bar-segment weak" style="width:${(weak * 100).toFixed(1)}%"></div>
    <div class="bar-segment error" style="width:${(err * 100).toFixed(1)}%"></div>
  `;
  document.getElementById('signal-labels').textContent =
    `${(strong * 100).toFixed(0)}% strong · ${(weak * 100).toFixed(0)}% weak · ${(err * 100).toFixed(0)}% error`;
}

function drawSparkline(canvasId, values, color) {
  const canvas = document.getElementById(canvasId);
  const ctx = canvas.getContext('2d');
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  if (values.length < 2) return;

  const max = Math.max(...values, 1);
  const step = w / (values.length - 1);

  ctx.beginPath();
  ctx.strokeStyle = color;
  ctx.lineWidth = 2;
  ctx.moveTo(0, h - (values[0] / max) * (h - 4) - 2);

  for (let i = 1; i < values.length; i++) {
    const x = i * step;
    const y = h - (values[i] / max) * (h - 4) - 2;
    ctx.lineTo(x, y);
  }
  ctx.stroke();
}

connect();
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add adsb-dashboard/server.go adsb-dashboard/server_test.go adsb-dashboard/static/
git commit -m "feat: add HTTP server with SSE, health, and static file serving"
```

---

### Task 5: Main entry point

**Files:**
- Create: `adsb-dashboard/main.go`

**Interfaces:**
- Consumes: `NewServer`, `NewBroker`, flag values
- Produces: Runnable binary

- [ ] **Step 1: Write main.go**

`adsb-dashboard/main.go`:
```go
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1", "Bind address")
	port := flag.Int("port", 8080, "HTTP port")
	statsPath := flag.String("stats", "/run/dump1090-fa/stats.json", "Path to stats.json")
	pollInterval := flag.Duration("poll", 60*time.Second, "Stats poll interval (minimum 10s)")
	flag.Parse()

	// Enforce minimum poll interval
	if *pollInterval < 10*time.Second {
		fmt.Fprintf(os.Stderr, "poll interval too short (%v), minimum is 10s\n", *pollInterval)
		os.Exit(1)
	}

	// Verify stats file is readable
	if _, err := os.Stat(*statsPath); err != nil {
		log.Printf("Warning: stats file not accessible: %v", err)
	}

	broker := NewBroker()
	server := NewServer(broker, *statsPath, *pollInterval)

	listenAddr := net.JoinHostPort(*addr, fmt.Sprintf("%d", *port))
	log.Printf("Starting ADS-B dashboard on %s", listenAddr)

	if err := http.ListenAndServe(listenAddr, server); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build -o adsb-dashboard .`
Expected: binary created, no errors

- [ ] **Step 3: Run server tests**

Run: `go test ./... -v`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add adsb-dashboard/main.go
git commit -m "feat: add main entry point with flag parsing"
```

---

### Task 6: Verify build + systemd service

**Files:**
- Create: `adsb-dashboard/adsb-dashboard.service` (optional systemd unit)

- [ ] **Step 1: Write systemd service file**

`adsb-dashboard/adsb-dashboard.service`:
```ini
[Unit]
Description=ADS-B Performance Dashboard
After=network.target dump1090-fa.service
Wants=dump1090-fa.service

[Service]
Type=simple
ExecStart=/usr/local/bin/adsb-dashboard
Restart=on-failure
RestartSec=10
CPUSchedulingPolicy=idle
IOSchedulingClass=idle
NoNewPrivileges=true
ProtectSystem=strict
ReadOnlyPaths=/run/dump1090-fa/stats.json /proc
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 2: Cross-compile for Pi Zero**

```bash
GOOS=linux GOARCH=arm GOARM=6 go build -o adsb-dashboard .
```

Expected: binary `adsb-dashboard` for Linux ARMv6 (Pi Zero compatible)

- [ ] **Step 3: Verify binary size**

```bash
ls -lh adsb-dashboard
```

Expected: < 10MB

- [ ] **Step 4: Commit**

```bash
git add adsb-dashboard/adsb-dashboard.service
git commit -m "docs: add systemd service file and build instructions"
```

---

## Self-Review Checklist

- [x] **Spec coverage:** Architecture (SSE + Go), dashboard layout (7 cards), stats.json parsing, /proc CPU/mem, poll interval with floor, bind to localhost, read-only access, no writes — all covered across Tasks 1-6.
- [x] **Placeholder scan:** No TBD, TODO, "implement later", or vague steps. Every step has exact code and commands.
- [x] **Type consistency:** `Stats`, `SystemStats`, `Broker`, `NewServer` — same names used in every task that references them. `ServeHTTP` method pattern consistent.
