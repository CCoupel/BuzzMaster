#pragma once

// Differentiated error LED patterns (issue #49).
//
// Rationale: plain solid red is used both for short boot phases and for
// permanent error states — the two were indistinguishable visually. This
// module provides animated patterns so a user can tell at a glance which
// subsystem is failing.
//
// Patterns:
//   WIFI_FAILED     — red blinking 1 Hz        (WiFi association / fallback exhausted)
//   WS_DISCONNECTED — red blinking 4 Hz        (WS disconnect or reconnect cycle in progress)
//   WS_TIMEOUT      — red pulsing ~0.5 Hz      (connection timeout after full reconnect window)
//   OTA_ERROR       — red solid + brief white flash every 2 s (OTA download/flash failure)
//
// Usage:
//   setLedError(LedErrorPattern::WIFI_FAILED);  // enter error state
//   clearLedError();                            // leave error state on success
//   manageLedError();                           // call every loop() tick to animate
//
// Plain solid red via setLedColor(255,0,0,...) remains reserved for short
// boot-phase transitions — do not use it for permanent error indication.

#include <Arduino.h>
#include "Common/led.h"

enum class LedErrorPattern : uint8_t {
    NONE = 0,
    WIFI_FAILED,
    WS_DISCONNECTED,
    WS_TIMEOUT,
    OTA_ERROR
};

static volatile LedErrorPattern g_ledErrorPattern = LedErrorPattern::NONE;
static unsigned long g_ledErrorLastStep = 0;
static uint8_t g_ledErrorStepIdx = 0;

// Enter a specific error pattern (or NONE to clear). Applies the initial
// visual state immediately so there is no lag before manageLedError() runs.
inline void setLedError(LedErrorPattern p) {
    if (g_ledErrorPattern == p) return;
    g_ledErrorPattern = p;
    g_ledErrorStepIdx = 0;
    g_ledErrorLastStep = millis();

    switch (p) {
        case LedErrorPattern::NONE:
            // Caller is responsible for setting a non-error color next.
            break;
        case LedErrorPattern::WIFI_FAILED:
        case LedErrorPattern::WS_DISCONNECTED:
        case LedErrorPattern::OTA_ERROR:
            setLedColor(255, 0, 0, true);
            break;
        case LedErrorPattern::WS_TIMEOUT:
            setLedIntensity(255);
            setLedColor(255, 0, 0, true);
            break;
    }
}

inline void clearLedError() {
    setLedError(LedErrorPattern::NONE);
}

inline bool isLedErrorActive() {
    return g_ledErrorPattern != LedErrorPattern::NONE;
}

// Animate the current error pattern. Must be called from loop().
// Place AFTER other LED animations (gray rotation, server BLINK) so that an
// active error pattern visually overrides them — clearing the error via
// clearLedError() lets the next tick restore the regular game LED state.
inline void manageLedError() {
    if (g_ledErrorPattern == LedErrorPattern::NONE) return;

    const unsigned long now = millis();

    switch (g_ledErrorPattern) {
        case LedErrorPattern::WIFI_FAILED: {
            // 1 Hz: 500 ms ON, 500 ms OFF
            if (now - g_ledErrorLastStep >= 500) {
                g_ledErrorStepIdx ^= 1;
                setLedColor(g_ledErrorStepIdx ? 255 : 0, 0, 0, true);
                g_ledErrorLastStep = now;
            }
            break;
        }
        case LedErrorPattern::WS_DISCONNECTED: {
            // 4 Hz: 125 ms ON, 125 ms OFF
            if (now - g_ledErrorLastStep >= 125) {
                g_ledErrorStepIdx ^= 1;
                setLedColor(g_ledErrorStepIdx ? 255 : 0, 0, 0, true);
                g_ledErrorLastStep = now;
            }
            break;
        }
        case LedErrorPattern::WS_TIMEOUT: {
            // Slow pulse: triangle wave between intensity 32 and 255 over ~2 s
            // (40 steps × 50 ms — half ramp up, half ramp down).
            if (now - g_ledErrorLastStep >= 50) {
                g_ledErrorStepIdx = (g_ledErrorStepIdx + 1) % 40;
                const uint8_t idx = g_ledErrorStepIdx;
                const uint8_t halfRange = 223;  // 255 - 32
                uint8_t intensity;
                if (idx < 20) {
                    intensity = 32 + (halfRange * idx / 19);
                } else {
                    intensity = 32 + (halfRange * (39 - idx) / 19);
                }
                // Keep color red; only intensity changes.
                setLedColor(255, 0, 0, false);
                setLedIntensity(intensity);
                g_ledErrorLastStep = now;
            }
            break;
        }
        case LedErrorPattern::OTA_ERROR: {
            // Red solid 1800 ms, white flash 100 ms, repeat.
            // stepIdx 0 = red phase, 1 = flash phase.
            if (g_ledErrorStepIdx == 0) {
                if (now - g_ledErrorLastStep >= 1800) {
                    setLedColor(255, 255, 255, true);
                    g_ledErrorStepIdx = 1;
                    g_ledErrorLastStep = now;
                }
            } else {
                if (now - g_ledErrorLastStep >= 100) {
                    setLedColor(255, 0, 0, true);
                    g_ledErrorStepIdx = 0;
                    g_ledErrorLastStep = now;
                }
            }
            break;
        }
        default:
            break;
    }
}
