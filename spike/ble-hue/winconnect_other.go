//go:build !windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const winrtBackendSupported = false

func winConnectWithTimeout(mac, _ string, _ time.Duration, _ func(string, ...any)) (bulbLink, map[string]gattChar, int, int, error) {
	return nil, nil, 0, 0, fmt.Errorf("-backend winrt is Windows-only (bulb %s)", mac)
}

func resolveAddrType(mac, mode string) string {
	if mode != "auto" {
		return mode
	}
	if addressLooksRandom(mac) {
		return "random"
	}
	return "public"
}

// addressLooksRandom reports whether the first byte has both MSBs set
// (0b11xxxxxx = static random device address).
func addressLooksRandom(mac string) bool {
	mac = strings.TrimSpace(mac)
	if len(mac) < 2 {
		return false
	}
	b, err := strconv.ParseUint(mac[:2], 16, 8)
	if err != nil {
		return false
	}
	return b&0xC0 == 0xC0
}
