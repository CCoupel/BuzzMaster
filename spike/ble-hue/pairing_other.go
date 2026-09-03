//go:build !windows

package main

import (
	"fmt"

	"tinygo.org/x/bluetooth"
)

const programmaticPairingSupported = false

func cmdPairTest(_ *bluetooth.Adapter, _ []string, _ WriteMode, _ *Report) error {
	return fmt.Errorf("pairtest is Windows-only (programmatic WinRT pairing); on Linux use bluetoothctl pair + trust")
}
