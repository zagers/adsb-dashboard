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

type ServerSnapshot struct {
	Stats  *Stats
	System *SystemStats
}

type Server struct {
	broker        *Broker
	statsPath     string
	pollInterval  time.Duration
	lastSnapshot  atomic.Value
	acBroker      *Broker
	aircraftPath  string
}

func NewServer(broker *Broker, statsPath string, pollInterval time.Duration, acBroker *Broker, aircraftPath string) *Server {
	s := &Server{
		broker:       broker,
		statsPath:    statsPath,
		pollInterval: pollInterval,
		acBroker:     acBroker,
		aircraftPath: aircraftPath,
	}
	go s.pollLoop()
	if s.aircraftPath != "" && s.acBroker != nil {
		go s.aircraftLoop()
	}
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

	var prevGain float64

	s.readAndBroadcast(&prevGain)

	for range ticker.C {
		s.readAndBroadcast(&prevGain)
	}
}

func (s *Server) readAndBroadcast(prevGain *float64) {
	stats, err := ReadStatsFile(s.statsPath)
	if err != nil {
		return
	}

	currentGain := stats.GainDB()
	gainChanged := *prevGain != 0.0 && *prevGain != currentGain
	*prevGain = currentGain

	sysStats, _ := ReadSystemStats()

	s.lastSnapshot.Store(&ServerSnapshot{Stats: stats, System: sysStats})

	type payload struct {
		Stats              *Stats       `json:"stats"`
		System             *SystemStats `json:"system"`
		MessagesPerSec     float64      `json:"messages_per_sec"`
		AircraftNow        int          `json:"aircraft_now"`
		AircraftPeak       int          `json:"aircraft_peak"`
		StrongSignalRatio  float64      `json:"strong_signal_ratio"`
		SNR                float64      `json:"snr"`
		SignalStrength     float64      `json:"signal_strength"`
		NoiseFloor         float64      `json:"noise_floor"`
		TotalMessages      int64        `json:"total_messages"`
		GainDB             float64      `json:"gain_db"`
		GainChanged        bool         `json:"gain_changed"`
		TracksHeard        int          `json:"tracks_heard"`
		PositionsCount     int          `json:"positions_count"`
		PositioningRatio   float64      `json:"positioning_ratio"`
		BadMessagesPerSec  float64      `json:"bad_messages_per_sec"`
		ErrorRate          float64      `json:"error_rate"`
		StrongSignalsCount int          `json:"strong_signals_count"`
	}

	data, err := json.Marshal(payload{
		Stats:              stats,
		System:             sysStats,
		MessagesPerSec:     stats.MessagesPerSec(),
		AircraftNow:        stats.AircraftNow(),
		AircraftPeak:       stats.AircraftPeak(),
		StrongSignalRatio:  stats.StrongSignalRatio(),
		SNR:                stats.SNR(),
		SignalStrength:     stats.SignalStrength(),
		NoiseFloor:         stats.NoiseFloor(),
		TotalMessages:      stats.TotalMessages(),
		GainDB:             currentGain,
		GainChanged:        gainChanged,
		TracksHeard:        stats.TracksHeard(),
		PositionsCount:     stats.PositionsCount(),
		PositioningRatio:   stats.PositioningRatio(),
		BadMessagesPerSec:  stats.BadMessagesPerSec(),
		ErrorRate:          stats.ErrorRate(),
		StrongSignalsCount: stats.StrongSignalsCount(),
	})
	if err != nil {
		return
	}
	s.broker.Broadcast(data)
}

func (s *Server) aircraftLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ac, err := ReadAircraftFile(s.aircraftPath)
		if err != nil {
			continue
		}

		data, err := json.Marshal(map[string]interface{}{
			"tracks_heard":     ac.TracksHeard(),
			"positions_count":  ac.PositionsCount(),
		})
		if err != nil {
			continue
		}
		s.acBroker.Broadcast(data)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.Handle("/events", s.broker)
	if s.acBroker != nil {
		mux.Handle("/events/aircraft", s.acBroker)
	}
	mux.Handle("/", s.staticHandler())
	mux.ServeHTTP(w, r)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	snap := s.lastSnapshot.Load()
	status := "ok"
	if snap == nil {
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
