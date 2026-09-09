//go:build !windows

package main

import tb "tinygo.org/x/bluetooth"

// On Linux, BlueZ negotiates security itself from the bond created by
// `bluetoothctl pair`; there is no per-characteristic protection level to set.
const platformSecuritySupported = false

func platformRead(c tb.DeviceCharacteristic) ([]byte, error) {
	buf := make([]byte, 64)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func platformApplyProtection(_ tb.DeviceCharacteristic, _ string) (string, error) {
	return "n/a on this OS (BlueZ handles security from the bond)", nil
}

func platformDescribe(_ tb.DeviceCharacteristic) string {
	return "(no WinRT metadata on this OS)"
}
