package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Stats struct {
	Latest   TimeWindowStats `json:"latest"`
	Last1Min  TimeWindowStats `json:"last1min"`
	Last5Min  TimeWindowStats `json:"last5min"`
	Last15Min TimeWindowStats `json:"last15min"`
	Total    TimeWindowStats `json:"total"`
}

type TimeWindowStats struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Local LocalStats `json:"local"`
	CPR   CPRStats  `json:"cpr"`
	CPU   CPUStats  `json:"cpu"`
	Tracks TrackStats `json:"tracks"`
	Messages int `json:"messages"`
}

type LocalStats struct {
	SamplesProcessed int64   `json:"samples_processed"`
	SamplesDropped   int64   `json:"samples_dropped"`
	ModeAC           int     `json:"modeac"`
	Modes            int     `json:"modes"`
	Bad              int     `json:"bad"`
	UnknownICAO      int     `json:"unknown_icao"`
	Accepted         []int   `json:"accepted"`
	Signal           float64 `json:"signal,omitempty"`
	Noise            float64 `json:"noise,omitempty"`
	PeakSignal       float64 `json:"peak_signal,omitempty"`
	StrongSignals    int     `json:"strong_signals,omitempty"`
	GainDB           float64 `json:"gain_db,omitempty"`
}

type CPRStats struct {
	Surface     int `json:"surface"`
	Airborne    int `json:"airborne"`
	GlobalOK    int `json:"global_ok"`
	GlobalBad   int `json:"global_bad"`
	GlobalRange int `json:"global_range"`
}

type CPUStats struct {
	Demod      int `json:"demod"`
	Reader     int `json:"reader"`
	Background int `json:"background"`
}

type TrackStats struct {
	All           int `json:"all"`
	SingleMessage int `json:"single_message"`
	Unreliable    int `json:"unreliable"`
}

func (s Stats) StrongSignalRatio() float64 {
	m := s.Last1Min.Local.Modes
	if m == 0 {
		return 0
	}
	return float64(s.Last1Min.Local.StrongSignals) / float64(m)
}

func (s Stats) SNR() float64 {
	return s.Last1Min.Local.Signal - s.Last1Min.Local.Noise
}

func (s Stats) SignalStrength() float64 {
	return s.Last1Min.Local.Signal
}

func (s Stats) NoiseFloor() float64 {
	return s.Last1Min.Local.Noise
}

func (s Stats) GainDB() float64 {
	return s.Last1Min.Local.GainDB
}

func (s Stats) TracksHeard() int {
	return s.Last1Min.Tracks.All
}

func (s Stats) PositionsCount() int {
	return s.Last1Min.CPR.Airborne
}

func (s Stats) BadMessagesPerSec() float64 {
	return float64(s.Last1Min.Local.Bad) / 60.0
}

func (s Stats) StrongSignalsCount() int {
	return s.Last1Min.Local.StrongSignals
}

func (s Stats) ErrorRate() float64 {
	total := s.Last1Min.Local.Modes + s.Last1Min.Local.Bad
	if total == 0 {
		return 0.0
	}
	return float64(s.Last1Min.Local.Bad) / float64(total)
}

func (s Stats) PositioningRatio() float64 {
	heard := s.Last1Min.Tracks.All
	if heard <= 0 {
		return 0.0
	}
	ratio := (float64(s.Last1Min.CPR.Airborne) / float64(heard)) * 100.0
	if ratio > 100.0 {
		return 100.0
	}
	return ratio
}

func (s Stats) MessagesPerSec() float64 {
	return float64(s.Last1Min.Messages) / 60.0
}

func (s Stats) AircraftNow() int {
	return s.Last1Min.Tracks.All
}

func (s Stats) AircraftPeak() int {
	peaks := []int{s.Last1Min.Tracks.All, s.Last5Min.Tracks.All, s.Last15Min.Tracks.All}
	max := 0
	for _, p := range peaks {
		if p > max {
			max = p
		}
	}
	return max
}

func (s Stats) LastUpdateTime() time.Time {
	return time.Unix(0, int64(s.Last1Min.End*1e9))
}

func (s Stats) TotalMessages() int64 {
	return int64(s.Total.Messages)
}

type SystemStats struct {
	CPUPercent      float64 `json:"cpu_percent"`
	MemPercent      float64 `json:"mem_percent"`
	ThrottledStatus string  `json:"throttled"`
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

func getThrottledStatus() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "vcgencmd", "get_throttled").Output()
	if err != nil {
		return "Unknown"
	}

	parts := strings.Split(strings.TrimSpace(string(out)), "=")
	if len(parts) != 2 {
		return "OK"
	}
	hexStr := strings.TrimPrefix(parts[1], "0x")

	val, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil || val == 0 {
		return "OK"
	}

	var nowFlags []string
	var pastFlags []string

	if val&0x1 != 0 {
		nowFlags = append(nowFlags, "Under-voltage")
	}
	if val&0x2 != 0 {
		nowFlags = append(nowFlags, "Freq Capped")
	}
	if val&0x4 != 0 {
		nowFlags = append(nowFlags, "Throttled")
	}
	if val&0x8 != 0 {
		nowFlags = append(nowFlags, "Soft Temp Limit")
	}

	if val&0x10000 != 0 {
		pastFlags = append(pastFlags, "Under-voltage")
	}
	if val&0x20000 != 0 {
		pastFlags = append(pastFlags, "Freq Capped")
	}
	if val&0x40000 != 0 {
		pastFlags = append(pastFlags, "Throttled")
	}
	if val&0x80000 != 0 {
		pastFlags = append(pastFlags, "Soft Temp Limit")
	}

	var finalStatus []string
	if len(nowFlags) > 0 {
		finalStatus = append(finalStatus, fmt.Sprintf("⚠ %s NOW", strings.Join(nowFlags, "+")))
	}
	if len(pastFlags) > 0 {
		finalStatus = append(finalStatus, fmt.Sprintf("%s (past)", strings.Join(pastFlags, "+")))
	}

	return strings.Join(finalStatus, " · ")
}
