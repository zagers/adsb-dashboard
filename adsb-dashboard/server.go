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
	broker       *Broker
	statsPath    string
	pollInterval time.Duration
	lastStats    atomic.Value // stores *Stats
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
	if s.pollInterval < 10*time.Second {
		return 10 * time.Second
	}
	return s.pollInterval
}

func (s *Server) pollLoop() {
	ticker := time.NewTicker(s.pollIntervalDuration())
	defer ticker.Stop()

	s.readAndBroadcast()

	for range ticker.C {
		s.readAndBroadcast()
	}
}

func (s *Server) readAndBroadcast() {
	stats, err := ReadStatsFile(s.statsPath)
	if err != nil {
		return
	}
	s.lastStats.Store(stats)

	sysStats, err := ReadSystemStats()
	if err != nil {
		return
	}
	s.lastSysStats.Store(sysStats)

	type payload struct {
		Stats  *Stats       `json:"stats"`
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
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}

func main() {
	statsPath := "/run/dump1090-fa/stats.json"
	if p := os.Getenv("STATS_PATH"); p != "" {
		statsPath = p
	}

	pollInterval := 60 * time.Second
	if s := os.Getenv("POLL_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			pollInterval = d
		}
	}

	addr := ":8080"
	if a := os.Getenv("ADDR"); a != "" {
		addr = a
	}

	broker := NewBroker()
	server := NewServer(broker, statsPath, pollInterval)

	fmt.Fprintf(os.Stderr, "ADS-B dashboard listening on %s\n", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
