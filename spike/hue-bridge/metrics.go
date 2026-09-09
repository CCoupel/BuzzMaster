package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"time"
)

// Stats summarises latencies in milliseconds.
type Stats struct {
	Count  int     `json:"count"`
	MinMs  float64 `json:"min_ms"`
	P50Ms  float64 `json:"p50_ms"`
	P95Ms  float64 `json:"p95_ms"`
	MaxMs  float64 `json:"max_ms"`
	MeanMs float64 `json:"mean_ms"`
}

func statsOf(ds []time.Duration) Stats {
	if len(ds) == 0 {
		return Stats{}
	}
	vals := make([]float64, len(ds))
	sum := 0.0
	for i, d := range ds {
		vals[i] = float64(d) / float64(time.Millisecond)
		sum += vals[i]
	}
	sort.Float64s(vals)
	pct := func(p float64) float64 {
		r := int(math.Ceil(p/100*float64(len(vals)))) - 1
		if r < 0 {
			r = 0
		}
		return vals[r]
	}
	r1 := func(v float64) float64 { return math.Round(v*10) / 10 }
	return Stats{Count: len(vals), MinMs: r1(vals[0]), P50Ms: r1(pct(50)), P95Ms: r1(pct(95)), MaxMs: r1(vals[len(vals)-1]), MeanMs: r1(sum / float64(len(vals)))}
}

func (s Stats) String() string {
	if s.Count == 0 {
		return "n=0"
	}
	return fmt.Sprintf("n=%d min=%.1f p50=%.1f p95=%.1f max=%.1f mean=%.1f ms", s.Count, s.MinMs, s.P50Ms, s.P95Ms, s.MaxMs, s.MeanMs)
}

// Report is the JSON artefact written with -out.
type Report struct {
	Tool      string         `json:"tool"`
	Command   string         `json:"command"`
	Args      []string       `json:"args"`
	StartedAt time.Time      `json:"started_at"`
	EndedAt   time.Time      `json:"ended_at"`
	OS        string         `json:"os"`
	Arch      string         `json:"arch"`
	Bridge    string         `json:"bridge"`
	Target    *Target        `json:"target,omitempty"`
	Sections  map[string]any `json:"sections"`
	Notes     []string       `json:"notes,omitempty"`
}

func newReport(cmd string, args []string) *Report {
	return &Report{Tool: "spike-hue-bridge", Command: cmd, Args: args, StartedAt: time.Now(), OS: runtime.GOOS, Arch: runtime.GOARCH, Sections: map[string]any{}}
}

func (r *Report) Save(path string) error {
	if path == "" {
		return nil
	}
	r.EndedAt = time.Now()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
