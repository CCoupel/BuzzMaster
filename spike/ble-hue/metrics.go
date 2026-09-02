package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"sync"
	"time"
)

// Samples collects durations (stored in ms) and produces percentile stats.
type Samples struct {
	mu   sync.Mutex
	vals []float64
}

func (s *Samples) Add(d time.Duration) { s.AddMs(ms(d)) }

func (s *Samples) AddMs(v float64) {
	s.mu.Lock()
	s.vals = append(s.vals, v)
	s.mu.Unlock()
}

func (s *Samples) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.vals)
}

// Stats is the summary of a Samples set (all values in milliseconds).
type Stats struct {
	Count  int     `json:"count"`
	MinMs  float64 `json:"min_ms"`
	P50Ms  float64 `json:"p50_ms"`
	P95Ms  float64 `json:"p95_ms"`
	P99Ms  float64 `json:"p99_ms"`
	MaxMs  float64 `json:"max_ms"`
	MeanMs float64 `json:"mean_ms"`
}

func (s *Samples) Stats() Stats {
	s.mu.Lock()
	vals := append([]float64(nil), s.vals...)
	s.mu.Unlock()
	return computeStats(vals)
}

func computeStats(vals []float64) Stats {
	if len(vals) == 0 {
		return Stats{}
	}
	sort.Float64s(vals)
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return Stats{
		Count:  len(vals),
		MinMs:  round1(vals[0]),
		P50Ms:  round1(percentile(vals, 50)),
		P95Ms:  round1(percentile(vals, 95)),
		P99Ms:  round1(percentile(vals, 99)),
		MaxMs:  round1(vals[len(vals)-1]),
		MeanMs: round1(sum / float64(len(vals))),
	}
}

// percentile uses the nearest-rank method on a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func (st Stats) String() string {
	if st.Count == 0 {
		return "n=0"
	}
	return fmt.Sprintf("n=%d min=%.1f p50=%.1f p95=%.1f p99=%.1f max=%.1f mean=%.1f ms",
		st.Count, st.MinMs, st.P50Ms, st.P95Ms, st.P99Ms, st.MaxMs, st.MeanMs)
}

// Report is the JSON artefact written with -out. Sections are free-form so
// each command can add whatever it measured.
type Report struct {
	Tool      string          `json:"tool"`
	Command   string          `json:"command"`
	Args      []string        `json:"args"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   time.Time       `json:"ended_at"`
	OS        string          `json:"os"`
	Arch      string          `json:"arch"`
	GoVersion string          `json:"go_version"`
	Library   string          `json:"library"`
	Bulbs     []ConnectResult `json:"bulbs,omitempty"`
	Sections  map[string]any  `json:"sections"`
	Notes     []string        `json:"notes,omitempty"`
}

func newReport(command string, args []string) *Report {
	return &Report{
		Tool:      "spike-ble-hue",
		Command:   command,
		Args:      args,
		StartedAt: time.Now(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
		Library:   libraryVersion(),
		Sections:  map[string]any{},
	}
}

func (r *Report) Note(format string, a ...any) {
	r.Notes = append(r.Notes, fmt.Sprintf(format, a...))
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

func libraryVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "tinygo.org/x/bluetooth" {
				return d.Path + " " + d.Version
			}
		}
	}
	return "tinygo.org/x/bluetooth (version unknown)"
}
