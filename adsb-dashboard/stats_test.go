package main

import (
	"encoding/json"
	"os"
	"testing"
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

func TestReadStatsFile(t *testing.T) {
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
