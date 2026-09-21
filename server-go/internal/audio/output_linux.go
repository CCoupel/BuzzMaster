//go:build linux

package audio

// newPlatformOutput on Linux/ARM64 (the Raspberry Pi target, contract
// sound.md §1.1) delegates to the shared oto-based backend
// (output_oto.go). On this platform, oto talks PulseAudio/PipeWire-pulse
// natively (pure Go, `github.com/jfreymuth/pulse`) with a dlopen'd
// libasound.so.2 fallback (`github.com/ebitengine/purego`, no cgo either
// way) — spike #226 verdict §2.2, confirmed by real cross-compilation
// under CGO_ENABLED=0.
//
// ⚠️ Deployment note traced from the spike and carried forward here
// (contracts/sound.md §8, plan #228 §1.2): PipeWire-pulse runs in a USER
// session and needs XDG_RUNTIME_DIR + a session D-Bus. The documented Pi
// deployment (docs/ADMIN_GUIDE.md) is a systemd *system* unit, which by
// default has neither. This is why the systemd unit must export
// XDG_RUNTIME_DIR/PULSE_SERVER (docs/ADMIN_GUIDE.md, this same issue) —
// entirely a deployment/documentation concern, nothing to resolve here:
// this file talks to whatever PulseAudio socket the environment presents
// it, exactly the same way on every Linux target.
func newPlatformOutput(cfg OutputConfig) Output {
	return newOtoOutput(cfg)
}
