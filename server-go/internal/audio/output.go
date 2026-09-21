package audio

import (
	"context"
	"io"
)

// noopOutput is the shared silent-degradation Output — used both as the
// entire backend on a platform this milestone doesn't target
// (output_other.go) and as what the real oto backend falls back to when
// construction fails on linux/windows (output_oto.go, contract §5.5).
// Declared here, build-tag-free, so both call sites share one type.
type noopOutput struct{}

func (noopOutput) Play(ctx context.Context, pcm io.Reader) error {
	// Draining pcm costs nothing and keeps this a well-behaved io.Reader
	// consumer, matching what a real Output would do before playing.
	_, _ = io.Copy(io.Discard, pcm)
	return nil
}

func (noopOutput) Close() error { return nil }

// OutputConfig is the platform-agnostic configuration for NewOutput.
// Device (contract sound.md §9) is reserved for a future sink/device
// selector — #228 does not implement selection yet (the library it uses,
// `oto`, exposes no such option; a Linux-only bypass via jfreymuth/pulse
// directly was identified as a viable fallback by the spike, #226 verdict
// §2.4, but is not built here). A non-empty Device is logged once as
// "not yet honoured" rather than silently ignored or rejected.
type OutputConfig struct {
	Device string
}

// NewOutput builds the platform's real Output (contracts/sound.md §4),
// or a harmless no-op on any platform this milestone does not target
// (contract §1.1: "Windows + Raspberry Pi/Linux" only — see
// output_other.go). NEVER returns a hard error: any failure to reach real
// hardware (no audio subsystem, permission denied, context never
// becomes ready) degrades to a silent no-op Output instead — contract
// §5.5's "dégradation silencieuse" applies to construction too, not only
// to a Play call after the fact. This is also what keeps this package
// safe to construct on a CI runner or a test machine with no audio
// hardware at all: NewOutput itself never fails, only Play/logs
// reflect the degraded state.
func NewOutput(cfg OutputConfig) Output {
	return newPlatformOutput(cfg)
}

// IsNeutral reports whether o is the silent-degradation Output — the one
// NewOutput returns when it could not reach real hardware (contract §4
// amendment, 2026-09-21, #230) — or nil. A real pilote (otoOutput, or any
// other future backend) is never neutral. This is the ONLY way to tell a
// working Output from a degraded one from outside this package: NewOutput
// itself never errors, and neither Engine.Enabled() nor Stats.PlayErrors
// can make the distinction (a noopOutput never fails either — dégradation
// silencieuse jusqu'au bout, contract §5.5). #230's `GET /api/sound/status`
// and `POST /api/sounds/{cue}/test` both need it — see contracts/sound.md
// §4 and contracts/http-endpoints.md §Sound.
func IsNeutral(o Output) bool {
	if o == nil {
		return true
	}
	_, ok := o.(noopOutput)
	return ok
}
