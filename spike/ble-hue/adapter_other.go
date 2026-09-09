//go:build !linux

package main

import "tinygo.org/x/bluetooth"

// newLinuxAdapter is a no-op outside Linux: WinRT has a single default adapter.
func newLinuxAdapter(_ string) *bluetooth.Adapter {
	return bluetooth.DefaultAdapter
}
