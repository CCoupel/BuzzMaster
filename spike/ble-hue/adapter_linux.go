package main

import "tinygo.org/x/bluetooth"

// newLinuxAdapter selects a BlueZ adapter other than hci0 (e.g. a USB dongle
// on a Raspberry Pi whose on-board radio is kept for WiFi coexistence tests).
func newLinuxAdapter(id string) *bluetooth.Adapter {
	return bluetooth.NewAdapter(id)
}
