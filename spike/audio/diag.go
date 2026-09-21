package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/jfreymuth/pulse"
)

// cmdDiag answers tasks 0.2/0.3 directly: it documents WHICH layer is
// actually reached, rather than trusting oto's silent success/failure. It
// deliberately probes PulseAudio itself with the raw jfreymuth/pulse client
// (the same library oto's v3.5.x backend uses internally, see
// driver_pulseaudio_unix.go) BEFORE letting oto pick a backend, so the
// report can say "PulseAudio reached, N sink(s) visible" instead of just
// "oto.NewContext returned no error" — which per the plan's E.4 risk can be
// true even when the real audio silently went out the fallback ALSA path
// instead of the Bluetooth sink.
func cmdDiag() error {
	fmt.Println("=== Environment ===")
	fmt.Printf("XDG_RUNTIME_DIR = %q\n", os.Getenv("XDG_RUNTIME_DIR"))
	fmt.Printf("PULSE_SERVER    = %q\n", os.Getenv("PULSE_SERVER"))
	fmt.Printf("GOOS/GOARCH     = %s/%s\n", runtime.GOOS, runtime.GOARCH)

	fmt.Println("\n=== Raw PulseAudio probe (jfreymuth/pulse, pure Go, no cgo) ===")
	if runtime.GOOS != "linux" {
		// BUGFIX: this probe and its ALSA-fallback commentary are Linux/Pi
		// specific (tasks 0.2/0.3). Printing "oto will fall back to ALSA" on
		// Windows was nonsensical noise (Windows uses WASAPI, no ALSA/Pulse
		// concept exists there) and was reported as confusing.
		fmt.Printf("N/A on %s — PulseAudio/ALSA are Linux concepts (this probe only matters for\n", runtime.GOOS)
		fmt.Println("tasks 0.2/0.3 on the Raspberry Pi). On Windows, oto talks to WASAPI directly —")
		fmt.Println("see the oto.NewContext section below for the path that actually matters here.")
	} else if client, err := pulse.NewClient(); err != nil {
		fmt.Printf("PulseAudio UNREACHABLE: %v\n", err)
		fmt.Println("  -> oto will fall back to ALSA (dlopen libasound.so.2, no cgo needed on v3.5.x).")
		fmt.Println("  -> On the Pi in production-launch conditions, this is EXACTLY the silent")
		fmt.Println("     degradation risk documented in the plan (E.4): confirm in task 0.3 whether")
		fmt.Println("     this happens under the real launch method (systemd system vs. user session).")
	} else {
		defer client.Close()
		fmt.Println("PulseAudio REACHABLE.")
		sinks, err := client.ListSinks()
		if err != nil {
			fmt.Printf("  ListSinks() failed: %v\n", err)
		} else {
			fmt.Printf("  %d sink(s) visible:\n", len(sinks))
			for _, s := range sinks {
				fmt.Printf("    - id=%q name=%q rate=%d\n", s.ID(), s.Name(), s.SampleRate())
			}
			if def, err := client.DefaultSink(); err == nil {
				fmt.Printf("  default sink: %q\n", def.ID())
			}
		}
		fmt.Println("  NOTE for task 0.2 on the Pi: after pairing the BT speaker with bluetoothctl,")
		fmt.Println("  its sink id should read \"bluez_output.<MAC>.1\" or similar in the list above.")
		fmt.Println("  If it is ABSENT, PipeWire has not created the BT sink yet (see README.md).")
	}

	fmt.Println("\n=== oto.NewContext (what the real code path actually uses) ===")
	t0 := time.Now()
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		fmt.Printf("oto.NewContext FAILED immediately: %v\n", err)
		return nil // not a spike-program error — this IS the finding
	}
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		fmt.Println("oto.NewContext: readyChan did not fire within 5s (device likely hung/absent)")
		return nil
	}
	openMs := time.Since(t0).Milliseconds()
	fmt.Printf("Context ready in %d ms (this is the \"cold open\" cost task 0.5 asks to include on the\n", openMs)
	fmt.Println("first sound of a game — measure it again on real BT hardware, it will be far higher")
	fmt.Println("there: this sandbox has no A2DP negotiation to do).")
	if err := ctx.Err(); err != nil {
		fmt.Printf("ctx.Err() after ready: %v (degraded state — see task 0.8)\n", err)
	}
	return nil
}
