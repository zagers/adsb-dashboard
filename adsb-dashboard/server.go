package main

import (
	"encoding/json"
	"embed"
	"io/fs"
	"net/http"
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
