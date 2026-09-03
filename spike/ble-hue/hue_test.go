package main

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestEncodePower(t *testing.T) {
	if !bytes.Equal(encodePower(true), []byte{1}) || !bytes.Equal(encodePower(false), []byte{0}) {
		t.Fatalf("power encoding wrong: %v / %v", encodePower(true), encodePower(false))
	}
}

func TestEncodeBrightnessClamps(t *testing.T) {
	tests := []struct {
		in   int
		want byte
	}{
		{-10, 1}, {0, 1}, {1, 1}, {127, 127}, {254, 254}, {255, 254}, {1000, 254},
	}
	for _, tt := range tests {
		if got := encodeBrightness(tt.in); len(got) != 1 || got[0] != tt.want {
			t.Errorf("encodeBrightness(%d) = %v, want [%d]", tt.in, got, tt.want)
		}
	}
}

func TestEncodeTemperatureClampsLittleEndian(t *testing.T) {
	tests := []struct {
		in   int
		want []byte
	}{
		{100, []byte{153, 0}},      // clamp low
		{153, []byte{153, 0}},      // min
		{370, []byte{0x72, 0x01}},  // 370 = 0x0172 LE
		{500, []byte{0xF4, 0x01}},  // max
		{9999, []byte{0xF4, 0x01}}, // clamp high
	}
	for _, tt := range tests {
		if got := encodeTemperature(tt.in); !bytes.Equal(got, tt.want) {
			t.Errorf("encodeTemperature(%d) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestEncodeDecodeXYRoundTrip(t *testing.T) {
	for _, c := range []XY{{0, 0}, {0.3127, 0.3290}, {0.675, 0.322}, {1, 1}} {
		enc := encodeXY(c)
		if len(enc) != 4 {
			t.Fatalf("xy payload must be 4 bytes, got %d", len(enc))
		}
		dec, err := decodeXY(enc)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(dec.X-c.X) > 1e-4 || math.Abs(dec.Y-c.Y) > 1e-4 {
			t.Errorf("round trip %v -> %v", c, dec)
		}
	}
	// Reference from HueBLE: x=0.5 -> 0x7FFF/0x8000 little endian.
	enc := encodeXY(XY{0.5, 0.5})
	if enc[1] != 0x80 || enc[3] != 0x80 {
		t.Errorf("0.5 should encode to ~0x8000 LE per axis, got %x", enc)
	}
	if _, err := decodeXY([]byte{1, 2}); err == nil {
		t.Error("short xy payload must error")
	}
}

func TestEncodeXYClamps(t *testing.T) {
	enc := encodeXY(XY{-1, 2})
	if !bytes.Equal(enc, []byte{0, 0, 0xFF, 0xFF}) {
		t.Errorf("out-of-range xy must clamp, got %x", enc)
	}
}

func TestRGBToXYPlausible(t *testing.T) {
	red := rgbToXY(255, 0, 0)
	green := rgbToXY(0, 255, 0)
	blue := rgbToXY(0, 0, 255)
	white := rgbToXY(255, 255, 255)
	black := rgbToXY(0, 0, 0)
	if !(red.X > 0.6 && red.Y < 0.35) {
		t.Errorf("red xy implausible: %+v", red)
	}
	if !(green.Y > 0.55 && green.X < 0.35) {
		t.Errorf("green xy implausible: %+v", green)
	}
	if !(blue.X < 0.2 && blue.Y < 0.1) {
		t.Errorf("blue xy implausible: %+v", blue)
	}
	if math.Abs(white.X-0.3127) > 0.01 || math.Abs(white.Y-0.3290) > 0.01 {
		t.Errorf("white should be ~D65, got %+v", white)
	}
	if math.Abs(black.X-0.3127) > 0.01 {
		t.Errorf("black must fall back to D65, got %+v", black)
	}
}

func TestParseColor(t *testing.T) {
	if _, err := parseColor("red"); err != nil {
		t.Errorf("named colour: %v", err)
	}
	if c, err := parseColor("#FF0000"); err != nil || c.X < 0.6 {
		t.Errorf("hex colour: %+v %v", c, err)
	}
	if c, err := parseColor("0.4, 0.5"); err != nil || c.X != 0.4 || c.Y != 0.5 {
		t.Errorf("x,y colour: %+v %v", c, err)
	}
	for _, bad := range []string{"", "nope", "#12", "1,2,3"} {
		if _, err := parseColor(bad); err == nil {
			t.Errorf("parseColor(%q) should fail", bad)
		}
	}
}

func TestParseWriteMode(t *testing.T) {
	for in, want := range map[string]WriteMode{"": WriteAuto, "auto": WriteAuto, "request": WriteRequest, "REQ": WriteRequest, "command": WriteCommand, "cmd": WriteCommand} {
		got, err := parseWriteMode(in)
		if err != nil || got != want {
			t.Errorf("parseWriteMode(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseWriteMode("bogus"); err == nil {
		t.Error("bogus mode must fail")
	}
}

func TestValidProtectionMode(t *testing.T) {
	for _, ok := range []string{"plain", "encrypt", "auth"} {
		if !validProtectionMode(ok) {
			t.Errorf("%q must be valid", ok)
		}
	}
	for _, bad := range []string{"", "AUTH", "none", "encryption"} {
		if validProtectionMode(bad) {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

func TestApplyProtectionPlainIsNoOp(t *testing.T) {
	b := &Bulb{MAC: "AA:BB:CC:DD:EE:FF", Label: "x"}
	b.applyProtection("plain")
	if len(b.SecurityNotes) != 0 {
		t.Fatalf("plain must not touch anything: %v", b.SecurityNotes)
	}
	// No characteristics resolved → probe reports every Hue UUID as absent.
	probe := b.SecurityProbe()
	if len(probe) != len(hueCharacteristics) {
		t.Fatalf("probe lines = %d, want %d", len(probe), len(hueCharacteristics))
	}
	for _, l := range probe {
		if !strings.HasSuffix(l, ": not exposed") {
			t.Errorf("unexpected probe line %q", l)
		}
	}
}

func TestParseAddress(t *testing.T) {
	addr, err := parseAddress("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatal(err)
	}
	if got := addr.MAC.String(); got != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MAC round trip = %q", got)
	}
	if _, err := parseAddress("not-a-mac"); err == nil {
		t.Error("bad MAC must fail")
	}
}

func TestSplitList(t *testing.T) {
	got := splitList(" a, b ;c  d,,")
	if len(got) != 4 || got[0] != "a" || got[3] != "d" {
		t.Errorf("splitList = %v", got)
	}
	if len(splitList("")) != 0 {
		t.Error("empty list must be empty")
	}
}

func TestShortUUID(t *testing.T) {
	if s := shortUUID(uuidHueColorXY); s != "hue-0005" {
		t.Errorf("shortUUID hue = %q", s)
	}
	if s := shortUUID(uuidHueName); s != "sig-0003" {
		t.Errorf("shortUUID sig = %q", s)
	}
	if s := shortUUID("1234"); s != "1234" {
		t.Errorf("shortUUID passthrough = %q", s)
	}
}
