//go:build linux

package main

import (
	"encoding/json"
	"os"
	"testing"
)

const sampleStats = `{
"latest": {"start":1783472409.4,"end":1783472409.4,"local":{"samples_processed":0,"samples_dropped":0,"modeac":0,"modes":0,"bad":0,"unknown_icao":0,"accepted":[0,0],"strong_signals":0,"gain_db":44.5},"remote":{"modeac":0,"modes":0,"bad":0,"unknown_icao":0,"accepted":[0,0]},"cpr":{"surface":0,"airborne":0,"global_ok":0,"global_bad":0,"global_range":0,"global_speed":0,"global_skipped":0,"local_ok":0,"local_aircraft_relative":0,"local_receiver_relative":0,"local_skipped":0,"local_range":0,"local_speed":0,"filtered":0},"altitude_suppressed":0,"cpu":{"demod":0,"reader":0,"background":0},"tracks":{"all":0,"single_message":0,"unreliable":0},"messages":0,"messages_by_df":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]},
"last1min":{"start":1783472349.4,"end":1783472409.4,"local":{"samples_processed":143917056,"samples_dropped":0,"modeac":0,"modes":1608613,"bad":3769081,"unknown_icao":653520,"accepted":[784,63],"signal":-4.1,"noise":-17.6,"peak_signal":-1.0,"strong_signals":284,"gain_db":44.5},"remote":{"modeac":0,"modes":6,"bad":0,"unknown_icao":0,"accepted":[6,0]},"cpr":{"surface":0,"airborne":65,"global_ok":56,"global_bad":0,"global_range":0,"global_speed":0,"global_skipped":0,"local_ok":5,"local_aircraft_relative":0,"local_receiver_relative":0,"local_skipped":4,"local_range":0,"local_speed":1,"filtered":0},"altitude_suppressed":0,"cpu":{"demod":14885,"reader":3305,"background":387},"tracks":{"all":7,"single_message":3,"unreliable":3},"messages":853,"messages_by_df":[458,0,0,0,46,0,0,0,0,0,0,98,0,0,0,0,52,188,8,0,2,1,0,0,0,0,0,0,0,0,0,0]},
"last5min":{"start":1783472109.4,"end":1783472409.4,"local":{"samples_processed":719978496,"samples_dropped":0,"modeac":0,"modes":8066832,"bad":18892768,"unknown_icao":3281736,"accepted":[2493,249],"signal":-4.6,"noise":-17.7,"peak_signal":-1.0,"strong_signals":770,"gain_db":44.5},"remote":{"modeac":0,"modes":24,"bad":0,"unknown_icao":0,"accepted":[24,0]},"cpr":{"surface":0,"airborne":242,"global_ok":193,"global_bad":0,"global_range":0,"global_speed":0,"global_skipped":0,"local_ok":26,"local_aircraft_relative":0,"local_receiver_relative":0,"local_skipped":23,"local_range":0,"local_speed":1,"filtered":0},"altitude_suppressed":0,"cpu":{"demod":74464,"reader":16664,"background":1887},"tracks":{"all":35,"single_message":22,"unreliable":22},"messages":2766,"messages_by_df":[1501,0,0,0,151,4,0,0,0,0,0,320,0,0,0,0,120,617,30,0,22,1,0,0,0,0,0,0,0,0,0,0]},
"last15min":{"start":1783471509.4,"end":1783472409.4,"local":{"samples_processed":2159935488,"samples_dropped":0,"modeac":0,"modes":24177888,"bad":56614609,"unknown_icao":9834368,"accepted":[8259,784],"signal":-5.2,"noise":-17.7,"peak_signal":-1.0,"strong_signals":1583,"gain_db":44.5},"remote":{"modeac":0,"modes":83,"bad":0,"unknown_icao":0,"accepted":[83,0]},"cpr":{"surface":0,"airborne":674,"global_ok":503,"global_bad":0,"global_range":0,"global_speed":0,"global_skipped":0,"local_ok":82,"local_aircraft_relative":0,"local_receiver_relative":0,"local_skipped":89,"local_range":0,"local_speed":1,"filtered":0},"altitude_suppressed":0,"cpu":{"demod":222702,"reader":50063,"background":5687},"tracks":{"all":90,"single_message":54,"unreliable":54},"messages":9126,"messages_by_df":[5300,0,0,0,436,14,0,0,0,0,0,1138,0,0,0,0,339,1751,103,0,43,2,0,0,0,0,0,0,0,0,0,0]},
"total":{"start":1783471029.3,"end":1783472409.4,"local":{"samples_processed":3312058368,"samples_dropped":0,"modeac":0,"modes":37072749,"bad":86822596,"unknown_icao":15075210,"accepted":[14108,1371],"signal":-5.5,"noise":-17.7,"peak_signal":-1.0,"strong_signals":2128,"gain_db":44.5},"remote":{"modeac":0,"modes":83,"bad":0,"unknown_icao":0,"accepted":[83,0]},"cpr":{"surface":0,"airborne":1057,"global_ok":762,"global_bad":0,"global_range":0,"global_speed":0,"global_skipped":0,"local_ok":144,"local_aircraft_relative":0,"local_receiver_relative":0,"local_skipped":151,"local_range":0,"local_speed":1,"filtered":0},"altitude_suppressed":0,"cpu":{"demod":341913,"reader":76784,"background":8682},"tracks":{"all":143,"single_message":83,"unreliable":83},"messages":15562,"messages_by_df":[9027,0,0,0,724,18,0,0,0,0,0,2342,0,0,0,0,475,2806,118,0,45,7,0,0,0,0,0,0,0,0,0,0]}
}`

func TestStatsParsing(t *testing.T) {
	var s Stats
	if err := json.Unmarshal([]byte(sampleStats), &s); err != nil {
		t.Fatalf("failed to parse stats.json: %v", err)
	}

	if s.Last1Min.Messages != 853 {
		t.Errorf("expected 853, got %d", s.Last1Min.Messages)
	}
	if len(s.Last1Min.Local.Accepted) < 1 || s.Last1Min.Local.Accepted[0] != 784 {
		t.Errorf("expected 784 aircraft, got %v", s.Last1Min.Local.Accepted)
	}
	if s.Last1Min.Local.Modes != 1608613 {
		t.Errorf("expected 1608613, got %d", s.Last1Min.Local.Modes)
	}
	if s.Last1Min.Local.StrongSignals != 284 {
		t.Errorf("expected 284, got %d", s.Last1Min.Local.StrongSignals)
	}
}

func TestStatsDerived(t *testing.T) {
	var s Stats
	if err := json.Unmarshal([]byte(sampleStats), &s); err != nil {
		t.Fatal(err)
	}

	mps := s.MessagesPerSec()
	if mps < 14 || mps > 15 {
		t.Errorf("messages_per_sec expected ~14.2, got %f", mps)
	}

	if s.AircraftNow() != 7 {
		t.Errorf("expected 7 (tracks.all), got %d", s.AircraftNow())
	}

	if s.AircraftPeak() != 90 {
		t.Errorf("expected peak 90 (max tracks.all), got %d", s.AircraftPeak())
	}

	sr := s.StrongSignalRatio()
	if sr <= 0 || sr >= 1 {
		t.Errorf("strong_signal_ratio out of range: %f", sr)
	}
	snr := s.SNR()
	if snr <= 0 || snr > 30 {
		t.Errorf("SNR out of range: %f", snr)
	}
	if s.TotalMessages() != 15562 {
		t.Errorf("expected 15562, got %d", s.TotalMessages())
	}

	if s.GainDB() != 44.5 {
		t.Errorf("expected gain 44.5, got %f", s.GainDB())
	}
	if s.TracksHeard() != 7 {
		t.Errorf("expected tracks.heard=7, got %d", s.TracksHeard())
	}
	if s.PositionsCount() != 65 {
		t.Errorf("expected positions 65, got %d", s.PositionsCount())
	}
	badPS := s.BadMessagesPerSec()
	if badPS < 62000 || badPS > 64000 {
		t.Errorf("bad_messages_per_sec expected ~62818, got %f", badPS)
	}
	er := s.ErrorRate()
	if er < 230 || er > 240 {
		t.Errorf("error_rate expected ~234 (bad/modes*100), got %f", er)
	}
	if s.StrongSignalsCount() != 284 {
		t.Errorf("expected strong_signals 284, got %d", s.StrongSignalsCount())
	}
	pr := s.PositioningRatio()
	if pr != 100.0 {
		t.Errorf("positioning_ratio expected 100.0 (clamped), got %f", pr)
	}
}

func TestReadStatsFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "stats*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := f.Write([]byte(sampleStats)); err != nil {
		t.Fatal(err)
	}
	f.Close()

	s, err := ReadStatsFile(f.Name())
	if err != nil {
		t.Fatalf("ReadStatsFile failed: %v", err)
	}
	if s.Last1Min.Messages != 853 {
		t.Errorf("expected 853, got %d", s.Last1Min.Messages)
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
