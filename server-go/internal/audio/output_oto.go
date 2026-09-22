//go:build linux || windows

// Shared oto-based Output, used identically by output_linux.go and
// output_windows.go (B.2/B.3 of the #228 plan): `oto`'s Go API does not
// differ across these two platforms — cross-platform differences live
// INSIDE the oto package itself (its own build tags), never in ours. This
// file exists to avoid duplicating a non-trivial amount of concurrency-
// sensitive code between two otherwise-identical backends; each platform
// file stays a thin, documented entry point.
package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// otoContextReadyTimeout bounds how long NewOutput waits for oto's
// readiness channel before giving up and degrading to silence (contract
// §5.5, plan #228 risk R.2 — "canal de disponibilité non attendu ⇒ premier
// son muet"). Generous: the spike measured this cold-open cost at up to
// ~1s in an unreliable sandbox; real Bluetooth A2DP negotiation can be
// slower still.
const otoContextReadyTimeout = 5 * time.Second

// otoPlayMaxWait bounds a single Play call as a last-resort safety net —
// contract §5.3 lets Play block, but never FOREVER: a truly wedged device
// must not freeze the engine's single playback goroutine permanently.
// Generous relative to #229's "< 1 s" sound-length target.
const otoPlayMaxWait = 10 * time.Second

// otoDeviceDrainGrace is slept AFTER IsPlaying() first reports false,
// before releasing the player — IsPlaying()==false means OUR source
// buffer is drained, but the OS/driver's own output buffer may still hold
// the last chunk (oto's PulseAudio backend defaults to ~100ms of its own
// latency when no explicit buffer size is set — see oto's
// driver_pulseaudio_unix.go). Closing immediately here would risk
// clipping that tail — the exact class of bug the spike found and fixed
// (contract §4's amendment note).
const otoDeviceDrainGrace = 150 * time.Millisecond

// otoPrimeSilenceDuration is how much digital silence NewOutput plays
// once, right after construction, to pre-arm the stream (contract §228
// plan §1.2 "pré-armement") — absorbing the A2DP/stream-open cost at
// startup instead of on the very first real cue.
const otoPrimeSilenceDuration = 200 * time.Millisecond

// otoOutput is the real Output backend shared by Linux and Windows.
type otoOutput struct {
	ctx *oto.Context

	// closeOnce guards Close's no-op body — oto.Context exposes no Close
	// of its own (the process-wide context is designed to live for the
	// application's lifetime; only one is ever allowed to exist at all,
	// per-process — spike #226 verdict §2). Kept as a struct so a future
	// oto version that DOES expose teardown has a single place to wire it.
	closeOnce sync.Once
}

// sharedOtoOnce/sharedOtoCtx/sharedOtoErr back sharedOtoContext — the
// process-wide oto.Context singleton required by contracts/sound.md §10.3
// (v11.1, #219): `oto` allows exactly ONE context per process, so both the
// cues backend below (newOtoOutput) AND the question-sound MediaPlayer's
// oto backend (media_oto.go) must obtain the SAME instance, never each
// construct their own. Package-level rather than passed as a parameter:
// neither newOtoOutput's nor newPlatformMediaPlayer's public entry points
// (NewOutput/NewMediaPlayer) take a shared handle, and threading one
// through would leak this internal detail across package boundaries.
var (
	sharedOtoOnce sync.Once
	sharedOtoCtx  *oto.Context
	sharedOtoErr  error
)

// sharedOtoContext builds the process-wide oto.Context AT MOST ONCE
// (sync.Once) and hands every caller the same *oto.Context, or the same
// cached error forever after. This is a pure re-organisation of the
// construction sequence that used to live inline in newOtoOutput — the
// wait on the readiness channel, the otoContextReadyTimeout bound, and the
// post-ready ctx.Err() check all remain EXACTLY as they were, in the same
// order (contract §10.3's explicit requirement: "aucune signature publique
// n'est modifiée"). Only the degradation LOGGING moves to each call site
// (newOtoOutput below, newPlatformMediaPlayer in media_oto.go), since the
// two paths log a different, path-specific message for the same
// underlying cause.
func sharedOtoContext() (*oto.Context, error) {
	sharedOtoOnce.Do(func() {
		ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
			SampleRate:   SampleRate,
			ChannelCount: ChannelCount,
			Format:       oto.FormatSignedInt16LE,
		})
		if err != nil {
			sharedOtoErr = fmt.Errorf("oto.NewContext failed: %w", err)
			return
		}

		select {
		case <-ready:
		case <-time.After(otoContextReadyTimeout):
			sharedOtoErr = fmt.Errorf("oto context did not become ready within %s", otoContextReadyTimeout)
			return
		}
		if err := ctx.Err(); err != nil {
			sharedOtoErr = fmt.Errorf("oto context degraded before first use: %w", err)
			return
		}

		sharedOtoCtx = ctx
	})
	return sharedOtoCtx, sharedOtoErr
}

// newOtoOutput builds the real backend, or degrades to noopOutput on ANY
// failure (contract §5.5 — construction-time degradation, never a hard
// error out of NewOutput). cfg.Device is not yet honoured (see
// OutputConfig's own doc comment) — oto exposes no sink-selection option;
// the identified fallback (jfreymuth/pulse directly, Linux only) is not
// built in #228.
func newOtoOutput(cfg OutputConfig) Output {
	if cfg.Device != "" {
		log.Printf("audio: device selection (%q) is not yet implemented — using the system default output (contracts/sound.md §9)", cfg.Device)
	}

	ctx, err := sharedOtoContext()
	if err != nil {
		log.Printf("audio: oto context unavailable — sound bruitage disabled (silent degradation, contracts/sound.md §5.5): %v", err)
		return noopOutput{}
	}

	out := &otoOutput{ctx: ctx}
	out.prime()
	return out
}

// prime pre-arms the stream (contracts/sound.md, plan #228 §1.2) by
// playing a short burst of digital silence through the exact same
// blocking-play path a real cue uses — so any first-sound stream-opening
// cost (A2DP negotiation in particular) is paid once at construction,
// never on the first real game event. Errors are logged, never fatal:
// pre-arming is an optimisation, not a precondition for the driver to
// work.
func (o *otoOutput) prime() {
	frames := int(otoPrimeSilenceDuration.Seconds() * float64(SampleRate))
	silence := make([]byte, frames*FrameSize) // all-zero = digital silence, no envelope needed
	ctx, cancel := context.WithTimeout(context.Background(), otoPlayMaxWait)
	defer cancel()
	if err := o.Play(ctx, bytes.NewReader(silence)); err != nil {
		log.Printf("audio: stream pre-arming failed (non-fatal, first real cue will pay this cost instead): %v", err)
	}
}

// Play implements Output (contracts/sound.md §4 amendment, 2026-09-21):
// blocks until the sound has ACTUALLY finished rendering, or ctx is
// cancelled — never returns the instant oto.Player.Play() is called
// (that call is documented async: it only marks the player as playing,
// the mux fills its buffer on a LATER cycle — the exact bug the spike
// found and fixed, contract §4's amendment note explains it in full).
func (o *otoOutput) Play(ctx context.Context, pcm io.Reader) error {
	player := o.ctx.NewPlayer(pcm)
	defer player.Close()

	player.Play()

	deadline := time.Now().Add(otoPlayMaxWait)
	for player.IsPlaying() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if time.Now().After(deadline) {
			log.Printf("audio: Play exceeded the %s safety deadline — returning anyway to avoid freezing the engine permanently", otoPlayMaxWait)
			return nil
		}
		time.Sleep(2 * time.Millisecond)
	}

	// Device-drain grace period — see otoDeviceDrainGrace's doc comment.
	// Still ctx-aware: shutdown must never wait out this grace period.
	select {
	case <-time.After(otoDeviceDrainGrace):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// Close is a no-op: oto's process-wide Context has no teardown of its own
// (see otoOutput's doc comment) and is designed to live for the server's
// entire lifetime. Idempotent trivially, since it does nothing.
func (o *otoOutput) Close() error {
	o.closeOnce.Do(func() {})
	return nil
}
