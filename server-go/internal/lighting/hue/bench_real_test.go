package hue

// Optional measurement against a REAL bridge (contract §8), skipped unless
// the environment names one. Writes only to the configured light names,
// through the same guarded driver as production:
//
//   HUE_BRIDGE_IP=192.168.1.101 HUE_API_KEY=… HUE_LIGHTS=BuzzHue1 \
//   HUE_BENCH_OUT=/tmp/hue-bench.json go test ./internal/lighting/hue -run TestRealBridge -v
//
// Optional: HUE_BRIDGE_ID (identity check), HUE_ITERATIONS (default 20).

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/lighting"
)

func TestRealBridgeLatencyAndSpread(t *testing.T) {
	ip, key, names := os.Getenv("HUE_BRIDGE_IP"), os.Getenv("HUE_API_KEY"), os.Getenv("HUE_LIGHTS")
	if ip == "" || key == "" || names == "" {
		t.Skip("set HUE_BRIDGE_IP, HUE_API_KEY and HUE_LIGHTS to run against a real bridge")
	}
	iterations := 20
	if v, err := strconv.Atoi(os.Getenv("HUE_ITERATIONS")); err == nil && v > 0 {
		iterations = v
	}
	var cfgs []LightSpec
	for _, n := range strings.Split(names, ",") {
		if n = strings.TrimSpace(n); n != "" {
			cfgs = append(cfgs, LightSpec{Name: n})
		}
	}
	d, err := New(Config{BridgeIP: ip, BridgeID: os.Getenv("HUE_BRIDGE_ID"), APIKey: key, Lights: cfgs, Logger: t.Logf})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	inv, err := d.Inventory(ctx)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	st := d.Status()
	if st.LightsOK != len(cfgs) {
		t.Fatalf("not all configured lights resolved/reachable: %+v (inventory %d lights)", st.Lights, len(inv))
	}

	type sample struct {
		ApplyMs float64 `json:"apply_ms"`
	}
	var samples []float64
	colours := [][3]int{{255, 26, 26}, {26, 94, 255}, {26, 255, 83}, {255, 217, 26}}
	for i := 0; i < iterations; i++ {
		start := time.Now()
		if err := d.Apply(ctx, devGeneral(colours[i%len(colours)], 255)); err != nil {
			t.Fatalf("apply %d: %v", i, err)
		}
		samples = append(samples, devMs(time.Since(start)))
		time.Sleep(RecommendedMinInterval)
	}
	_ = d.Apply(ctx, devGeneral([3]int{255, 214, 170}, 0)) // off
	sort.Float64s(samples)
	pct := func(p float64) float64 { return samples[int(float64(len(samples)-1)*p/100)] }
	out := map[string]any{
		"bridge": ip, "lights": len(cfgs), "iterations": iterations,
		"apply_ms": map[string]float64{"p50": pct(50), "p95": pct(95), "max": samples[len(samples)-1]},
		"note":     "apply_ms = one Apply writing every configured light sequentially; with N lights the spread first→last ≈ apply_ms − one write",
		"stats":    d.Status().Stats,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	t.Logf("REAL_BRIDGE_MEASUREMENT %s", b)
	if path := os.Getenv("HUE_BENCH_OUT"); path != "" {
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Error(err)
		}
	}
	if pct(95) > 150*float64(len(cfgs)) {
		t.Errorf("p95 %.0f ms exceeds 150 ms per light (contract §8)", pct(95))
	}
	_ = lighting.ZoneGeneral
}
