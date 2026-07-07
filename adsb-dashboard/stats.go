package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

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
