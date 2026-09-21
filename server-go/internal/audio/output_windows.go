//go:build windows

package audio

// newPlatformOutput on Windows/AMD64 delegates to the shared oto-based
// backend (output_oto.go). On this platform, oto talks WASAPI directly
// via syscalls (no cgo) — spike #226 verdict §2.2, confirmed by real
// cross-compilation under CGO_ENABLED=0. No PulseAudio/ALSA concept
// applies here, and no systemd-style deployment caveat exists either: a
// Windows service/console process reaches the default audio device the
// same way an interactive process does.
func newPlatformOutput(cfg OutputConfig) Output {
	return newOtoOutput(cfg)
}
