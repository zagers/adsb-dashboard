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
	if s.Last1Min.Local.Modes == 0 {
		return 0.0
	}
	return (float64(s.Last1Min.Local.Bad) / float64(s.Last1Min.Local.Modes)) * 100.0
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

type CPUReader struct {
	prevIdle  uint64
	prevTotal uint64
	prevTime  time.Time
	ready     bool
}

func (r *CPUReader) Read() (float64, error) {
	idle, total, err := readCPUTicks()
	if err != nil {
		return 0, err
	}

	now := time.Now()

	if !r.ready {
		r.prevIdle = idle
		r.prevTotal = total
		r.prevTime = now
		r.ready = true
		return 0, nil
	}

	totalDelta := total - r.prevTotal
	idleDelta := idle - r.prevIdle

	r.prevIdle = idle
	r.prevTotal = total
	r.prevTime = now

	if totalDelta == 0 {
		return 0, nil
	}

	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100, nil
}

func readCPUTicks() (idle, total uint64, err error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}

	lines := strings.SplitN(string(data), "\n", 2)
	if len(lines) == 0 {
		return 0, 0, fmt.Errorf("empty /proc/stat")
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("unexpected /proc/stat format")
	}

	var vals [4]uint64
	for i := 0; i < 4; i++ {
		vals[i], _ = strconv.ParseUint(fields[i+1], 10, 64)
	}
	return vals[3], vals[0] + vals[1] + vals[2] + vals[3], nil
}

type ThrottleCache struct {
	status    string
	lastCheck time.Time
	interval  time.Duration
	checker   func() string
}

func (c *ThrottleCache) Get() string {
	if c.interval == 0 {
		c.interval = 60 * time.Second
	}
	if c.checker == nil {
		c.checker = getThrottledStatus
	}
	if time.Since(c.lastCheck) < c.interval {
		return c.status
	}
	c.lastCheck = time.Now()
	c.status = c.checker()
	return c.status
}

var (
	cpuReader     = &CPUReader{}
	throttleCache = &ThrottleCache{}
)

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
	cpu, _ := cpuReader.Read()
	mem, err := readMemPercent()
	if err != nil {
		return nil, err
	}
	return &SystemStats{
		CPUPercent:      cpu,
		MemPercent:      mem,
		ThrottledStatus: throttleCache.Get(),
	}, nil
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
			total, _ = parseMemValue(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available, _ = parseMemValue(line)
		}
	}

	if total == 0 {
		return 0, fmt.Errorf("could not parse MemTotal from /proc/meminfo")
	}

	used := total - available
	return float64(used) / float64(total) * 100, nil
}

func parseMemValue(line string) (uint64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, fmt.Errorf("unexpected format: %s", line)
	}
	v, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing memory value: %w", err)
	}
	return v, nil
}

type AircraftEntry struct {
	Hex  string   `json:"hex"`
	Seen float64  `json:"seen"`
	Lat  *float64 `json:"lat,omitempty"`
	Lon  *float64 `json:"lon,omitempty"`
}

type AircraftJSON struct {
	Now      float64          `json:"now"`
	Messages int64            `json:"messages"`
	Aircraft []AircraftEntry  `json:"aircraft"`
}

func (a AircraftJSON) TracksHeard() int {
	return len(a.Aircraft)
}

func (a AircraftJSON) PositionsCount() int {
	count := 0
	for _, ac := range a.Aircraft {
		if ac.Lat != nil && ac.Lon != nil {
			count++
		}
	}
	return count
}

func ReadAircraftFile(path string) (*AircraftJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var a AircraftJSON
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func getThrottledStatus() string {
	val := uint64(0)

	data, err := os.ReadFile("/sys/devices/platform/soc/soc:firmware/get_throttled")
	if err == nil {
		hexStr := strings.TrimSpace(string(data))
		val, _ = strconv.ParseUint(hexStr, 16, 32)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		out, err2 := exec.CommandContext(ctx, "vcgencmd", "get_throttled").Output()
		if err2 != nil {
			return "Unknown"
		}
		parts := strings.Split(strings.TrimSpace(string(out)), "=")
		if len(parts) != 2 {
			return "OK"
		}
		hexStr := strings.TrimPrefix(parts[1], "0x")
		val, _ = strconv.ParseUint(hexStr, 16, 32)
	}

	if val == 0 {
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
