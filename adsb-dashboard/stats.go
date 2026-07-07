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
