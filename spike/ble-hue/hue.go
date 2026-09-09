package main

// Hue BLE GATT layer — reverse-engineered protocol (no official Philips/Signify
// documentation). UUIDs and encodings cross-checked against the HueBLE project
// (github.com/flip-dots/HueBLE, HueBLE.py) on 2026-09-02.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	// Hue light control service and its characteristics.
	uuidHueLightService = "932c32bd-0000-47a2-835a-a8d455b859dd"
	uuidHuePower        = "932c32bd-0002-47a2-835a-a8d455b859dd" // 1 byte: 0x00 off, 0x01 on
	uuidHueBrightness   = "932c32bd-0003-47a2-835a-a8d455b859dd" // 1 byte: 1..254
	uuidHueTemperature  = "932c32bd-0004-47a2-835a-a8d455b859dd" // uint16 LE mirek 153..500 (0xFFFF when in colour mode)
	uuidHueColorXY      = "932c32bd-0005-47a2-835a-a8d455b859dd" // 2 x uint16 LE, CIE xy scaled by 0xFFFF
	// Light name lives in a separate (Signify) service.
	uuidHueName = "97fe6561-0003-4f62-86e9-b71ee2da3d22"
	// Service UUID present in Hue advertisements (used by `scan` to spot bulbs).
	uuidHueAdvService = "0000fe0f-0000-1000-8000-00805f9b34fb"
)

// XY is a CIE 1931 chromaticity.
type XY struct{ X, Y float64 }

// WriteMode selects the ATT write procedure.
type WriteMode int

const (
	// WriteAuto tries a Write Request first, falls back to Write Command if the
	// characteristic does not advertise the "write" property.
	WriteAuto WriteMode = iota
	// WriteRequest = ATT Write Request (bulb answers, latency = real round trip).
	WriteRequest
	// WriteCommand = ATT Write Command (fire and forget, no bulb acknowledgment).
	WriteCommand
)

func parseWriteMode(s string) (WriteMode, error) {
	switch strings.ToLower(s) {
	case "", "auto":
		return WriteAuto, nil
	case "request", "req":
		return WriteRequest, nil
	case "command", "cmd", "noresp":
		return WriteCommand, nil
	}
	return WriteAuto, fmt.Errorf("unknown write mode %q (auto|request|command)", s)
}

func (m WriteMode) String() string {
	switch m {
	case WriteRequest:
		return "request"
	case WriteCommand:
		return "command"
	}
	return "auto"
}

// ---------------------------------------------------------------------------
// Encodings
// ---------------------------------------------------------------------------

func encodePower(on bool) []byte {
	if on {
		return []byte{0x01}
	}
	return []byte{0x00}
}

// encodeBrightness clamps to the Hue range 1..254.
func encodeBrightness(level int) []byte {
	if level < 1 {
		level = 1
	}
	if level > 254 {
		level = 254
	}
	return []byte{byte(level)}
}

// encodeTemperature clamps mirek to 153..500.
func encodeTemperature(mirek int) []byte {
	if mirek < 153 {
		mirek = 153
	}
	if mirek > 500 {
		mirek = 500
	}
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(mirek))
	return b
}

func encodeXY(c XY) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint16(b[0:2], uint16(math.Round(clamp01(c.X)*0xFFFF)))
	binary.LittleEndian.PutUint16(b[2:4], uint16(math.Round(clamp01(c.Y)*0xFFFF)))
	return b
}

func decodeXY(b []byte) (XY, error) {
	if len(b) < 4 {
		return XY{}, fmt.Errorf("xy payload too short (%d bytes)", len(b))
	}
	return XY{
		X: float64(binary.LittleEndian.Uint16(b[0:2])) / 0xFFFF,
		Y: float64(binary.LittleEndian.Uint16(b[2:4])) / 0xFFFF,
	}, nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// rgbToXY converts an sRGB colour to CIE xy (D65). Good enough for a demo; the
// bulb clips to its own gamut.
func rgbToXY(r, g, b uint8) XY {
	lin := func(c uint8) float64 {
		v := float64(c) / 255.0
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	rl, gl, bl := lin(r), lin(g), lin(b)
	X := 0.4124*rl + 0.3576*gl + 0.1805*bl
	Y := 0.2126*rl + 0.7152*gl + 0.0722*bl
	Z := 0.0193*rl + 0.1192*gl + 0.9505*bl
	sum := X + Y + Z
	if sum == 0 {
		return XY{X: 0.3127, Y: 0.3290} // D65 white
	}
	return XY{X: X / sum, Y: Y / sum}
}

var namedColors = map[string][3]uint8{
	"red":     {255, 0, 0},
	"green":   {0, 255, 0},
	"blue":    {0, 0, 255},
	"yellow":  {255, 220, 0},
	"orange":  {255, 120, 0},
	"magenta": {255, 0, 255},
	"cyan":    {0, 255, 255},
	"white":   {255, 255, 255},
	"purple":  {128, 0, 255},
}

// parseColor accepts a colour name, "#RRGGBB" or "x,y".
func parseColor(s string) (XY, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if rgb, ok := namedColors[s]; ok {
		return rgbToXY(rgb[0], rgb[1], rgb[2]), nil
	}
	if strings.HasPrefix(s, "#") && len(s) == 7 {
		v, err := strconv.ParseUint(s[1:], 16, 32)
		if err != nil {
			return XY{}, fmt.Errorf("bad hex colour %q", s)
		}
		return rgbToXY(uint8(v>>16), uint8(v>>8), uint8(v)), nil
	}
	if parts := strings.Split(s, ","); len(parts) == 2 {
		x, errX := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		y, errY := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if errX == nil && errY == nil {
			return XY{X: x, Y: y}, nil
		}
	}
	return XY{}, fmt.Errorf("unknown colour %q (name, #RRGGBB or x,y)", s)
}

// ---------------------------------------------------------------------------
// Address helpers
// ---------------------------------------------------------------------------

func parseAddress(mac string) (bluetooth.Address, error) {
	m, err := bluetooth.ParseMAC(strings.ToUpper(strings.TrimSpace(mac)))
	if err != nil {
		return bluetooth.Address{}, fmt.Errorf("bad MAC %q: %w", mac, err)
	}
	return bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: m}}, nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Bulb
// ---------------------------------------------------------------------------

// gattChar is what Bulb needs from a characteristic, whichever backend
// resolved it (tinygo wrapper, or raw WinRT on Windows — winconnect_windows.go).
type gattChar interface {
	UUID() string
	Read() ([]byte, error)
	// Write performs an ATT Write Request (mode != WriteCommand) or Write
	// Command. Must return an error containing "write not supported" when the
	// characteristic lacks the requested property, so WriteAuto can fall back.
	Write(data []byte, mode WriteMode) error
	Describe() string
	ApplyProtection(mode string) (string, error)
}

// bulbLink is the connection handle behind a Bulb.
type bulbLink interface {
	Connected() (bool, error)
	Disconnect() error
}

// backendMode / addrTypeMode are the -backend / -addr-type flags.
var (
	backendMode  = "tinygo"
	addrTypeMode = "auto"
)

// --- tinygo backend adapters ---

type tgChar struct {
	c bluetooth.DeviceCharacteristic
}

func (t tgChar) UUID() string          { return strings.ToLower(t.c.UUID().String()) }
func (t tgChar) Read() ([]byte, error) { return platformRead(t.c) }
func (t tgChar) Describe() string      { return platformDescribe(t.c) }
func (t tgChar) ApplyProtection(m string) (string, error) {
	return platformApplyProtection(t.c, m)
}
func (t tgChar) Write(data []byte, mode WriteMode) error {
	var err error
	if mode == WriteCommand {
		_, err = t.c.WriteWithoutResponse(data)
	} else {
		_, err = t.c.Write(data)
	}
	return err
}

type tgLink struct{ dev bluetooth.Device }

func (l tgLink) Connected() (bool, error) { return l.dev.Connected() }
func (l tgLink) Disconnect() error        { return l.dev.Disconnect() }

// Bulb is one connected Hue bulb with its resolved characteristics.
type Bulb struct {
	MAC   string
	Label string

	link  bulbLink
	chars map[string]gattChar

	ConnectDuration  time.Duration
	DiscoverDuration time.Duration
	Services         int
	Characteristics  int
	// SecurityNotes collects what applyProtection and the first protected
	// read reported — printed by connectAll, kept in the JSON report.
	SecurityNotes []string

	mu             sync.Mutex
	resolvedMode   map[string]WriteMode // per characteristic, after WriteAuto fallback
	connectedState bool
}

// ConnectResult is the JSON-friendly outcome of one connection attempt.
type ConnectResult struct {
	MAC             string   `json:"mac"`
	OK              bool     `json:"ok"`
	Error           string   `json:"error,omitempty"`
	Name            string   `json:"name,omitempty"`
	ConnectMs       float64  `json:"connect_ms"`
	DiscoverMs      float64  `json:"discover_ms"`
	Services        int      `json:"services"`
	Characteristics int      `json:"characteristics"`
	HasColor        bool     `json:"has_color"`
	HasTemperature  bool     `json:"has_temperature"`
	SecurityNotes   []string `json:"security_notes,omitempty"`
	SecurityProbe   []string `json:"security_probe,omitempty"`
}

var errNoCharacteristic = errors.New("characteristic not exposed by this bulb")

// connectBulb connects to an already-paired bulb and resolves its GATT table.
// The tinygo Connect call has no timeout of its own on Linux (it waits for the
// BlueZ "Connected" property forever), so we bound it here. On timeout the
// underlying goroutine is leaked on purpose — this is a throwaway spike.
func connectBulb(adapter *bluetooth.Adapter, mac string, timeout time.Duration) (*Bulb, error) {
	addr, err := parseAddress(mac)
	if err != nil {
		return nil, err
	}
	b := &Bulb{MAC: strings.ToUpper(mac), Label: strings.ToUpper(mac), chars: map[string]gattChar{}, resolvedMode: map[string]WriteMode{}}

	if backendMode == "winrt" {
		// Raw WinRT path: connect + discover in one go, with an explicit LE
		// address type (the thing tinygo's Connect leaves unspecified).
		addrType := resolveAddrType(b.MAC, addrTypeMode)
		start := time.Now()
		link, chars, nSvc, nChars, err := winConnectWithTimeout(b.MAC, addrType, timeout, func(f string, a ...any) {
			b.SecurityNotes = append(b.SecurityNotes, fmt.Sprintf(f, a...))
		})
		if err != nil {
			return nil, fmt.Errorf("connect %s (winrt, addr-type %s): %w", mac, addrType, err)
		}
		b.link, b.chars, b.Services, b.Characteristics = link, chars, nSvc, nChars
		b.ConnectDuration = time.Since(start) // connect+discover are one sequence here
		b.SecurityNotes = append(b.SecurityNotes, "backend winrt, address type "+addrType)
		if !b.Has(uuidHuePower) {
			_ = b.link.Disconnect()
			return nil, fmt.Errorf("%s: Hue light service %s not found (%d services / %d characteristics)", b.MAC, uuidHueLightService, b.Services, b.Characteristics)
		}
	} else {
		type res struct {
			dev bluetooth.Device
			err error
		}
		ch := make(chan res, 1)
		start := time.Now()
		go func() {
			d, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
			ch <- res{d, err}
		}()
		var dev bluetooth.Device
		select {
		case r := <-ch:
			if r.err != nil {
				return nil, fmt.Errorf("connect %s: %w", mac, r.err)
			}
			dev = r.dev
		case <-time.After(timeout):
			return nil, fmt.Errorf("connect %s: timeout after %s (stack call still pending — bulb off, out of range, or not paired?)", mac, timeout)
		}
		b.link = tgLink{dev: dev}
		b.ConnectDuration = time.Since(start)

		start = time.Now()
		if err := b.discover(dev, timeout); err != nil {
			_ = b.link.Disconnect()
			return nil, err
		}
		b.DiscoverDuration = time.Since(start)
	}
	b.connectedState = true

	// Raise the link security level before the first protected access
	// (no-op with -protection plain, and outside Windows).
	b.applyProtection(protectionMode)

	if name, err := b.ReadName(); err == nil && name != "" {
		b.Label = name
	} else if err != nil {
		b.SecurityNotes = append(b.SecurityNotes, "name read failed: "+err.Error())
	}
	return b, nil
}

// discover walks every service/characteristic (nil filter) so that bulbs with
// a partial table — e.g. white-only bulbs without 0005 — still resolve.
func (b *Bulb) discover(dev bluetooth.Device, timeout time.Duration) error {
	type res struct {
		svcs []bluetooth.DeviceService
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		s, err := dev.DiscoverServices(nil)
		ch <- res{s, err}
	}()
	var svcs []bluetooth.DeviceService
	select {
	case r := <-ch:
		if r.err != nil {
			return fmt.Errorf("discover services %s: %w", b.MAC, r.err)
		}
		svcs = r.svcs
	case <-time.After(timeout):
		return fmt.Errorf("discover services %s: timeout after %s", b.MAC, timeout)
	}
	b.Services = len(svcs)
	for _, svc := range svcs {
		chars, err := svc.DiscoverCharacteristics(nil)
		if err != nil {
			// A service without readable characteristics is not fatal.
			continue
		}
		for _, c := range chars {
			b.chars[strings.ToLower(c.UUID().String())] = tgChar{c: c}
			b.Characteristics++
		}
	}
	if !b.Has(uuidHuePower) {
		return fmt.Errorf("%s: Hue light service %s not found (%d services / %d characteristics) — not a Hue BLE bulb, or GATT hidden until paired", b.MAC, uuidHueLightService, b.Services, b.Characteristics)
	}
	return nil
}

// Has reports whether the bulb exposes the given characteristic UUID.
func (b *Bulb) Has(uuid string) bool {
	_, ok := b.chars[strings.ToLower(uuid)]
	return ok
}

// Result converts the connection outcome to its JSON form.
func (b *Bulb) Result() ConnectResult {
	return ConnectResult{
		MAC:             b.MAC,
		OK:              true,
		Name:            b.Label,
		ConnectMs:       ms(b.ConnectDuration),
		DiscoverMs:      ms(b.DiscoverDuration),
		Services:        b.Services,
		Characteristics: b.Characteristics,
		HasColor:        b.Has(uuidHueColorXY),
		HasTemperature:  b.Has(uuidHueTemperature),
		SecurityNotes:   append([]string(nil), b.SecurityNotes...),
		SecurityProbe:   b.SecurityProbe(),
	}
}

func (b *Bulb) write(uuid string, data []byte, mode WriteMode) (time.Duration, error) {
	c, ok := b.chars[uuid]
	if !ok {
		return 0, fmt.Errorf("%s %s: %w", b.MAC, uuid, errNoCharacteristic)
	}
	b.mu.Lock()
	if mode == WriteAuto {
		if m, ok := b.resolvedMode[uuid]; ok {
			mode = m
		} else {
			mode = WriteRequest
		}
	}
	b.mu.Unlock()

	start := time.Now()
	err := c.Write(data, mode)
	d := time.Since(start)

	// WriteAuto fallback: Windows backend refuses Write when the "write"
	// property bit is absent; try Write Command once and remember it.
	if err != nil && mode == WriteRequest && strings.Contains(err.Error(), "write not supported") {
		start = time.Now()
		err = c.Write(data, WriteCommand)
		d = time.Since(start)
		if err == nil {
			b.mu.Lock()
			b.resolvedMode[uuid] = WriteCommand
			b.mu.Unlock()
		}
	} else if err == nil {
		b.mu.Lock()
		if _, ok := b.resolvedMode[uuid]; !ok {
			b.resolvedMode[uuid] = mode
		}
		b.mu.Unlock()
	}
	if err != nil {
		return d, fmt.Errorf("%s write %s: %w", b.Label, shortUUID(uuid), err)
	}
	return d, nil
}

// SetPower turns the bulb on or off.
func (b *Bulb) SetPower(on bool, mode WriteMode) (time.Duration, error) {
	return b.write(uuidHuePower, encodePower(on), mode)
}

// SetBrightness sets brightness 1..254.
func (b *Bulb) SetBrightness(level int, mode WriteMode) (time.Duration, error) {
	return b.write(uuidHueBrightness, encodeBrightness(level), mode)
}

// SetColor sets the CIE xy colour.
func (b *Bulb) SetColor(c XY, mode WriteMode) (time.Duration, error) {
	return b.write(uuidHueColorXY, encodeXY(c), mode)
}

// SetTemperature sets the colour temperature in mirek.
func (b *Bulb) SetTemperature(mirek int, mode WriteMode) (time.Duration, error) {
	return b.write(uuidHueTemperature, encodeTemperature(mirek), mode)
}

func (b *Bulb) read(uuid string) ([]byte, time.Duration, error) {
	c, ok := b.chars[uuid]
	if !ok {
		return nil, 0, fmt.Errorf("%s %s: %w", b.MAC, uuid, errNoCharacteristic)
	}
	start := time.Now()
	v, err := c.Read() // status-aware on Windows (security_windows.go)
	d := time.Since(start)
	if err != nil {
		return nil, d, fmt.Errorf("%s read %s: %w", b.Label, shortUUID(uuid), err)
	}
	return v, d, nil
}

// protectionMode is the -protection flag: "plain" (tinygo default, no
// change), "encrypt" or "auth" (Windows only: GattCharacteristic.ProtectionLevel
// set on the Hue characteristics right after discovery, so the OS raises the
// link security BEFORE the first read/write — see security_windows.go).
var protectionMode = "plain"

func validProtectionMode(s string) bool {
	switch s {
	case "plain", "encrypt", "auth":
		return true
	}
	return false
}

// hueCharacteristics lists the protected Hue/Signify characteristics the
// security probe and the protection level apply to.
var hueCharacteristics = []string{uuidHuePower, uuidHueBrightness, uuidHueTemperature, uuidHueColorXY, uuidHueName}

// applyProtection sets the requested protection level on every Hue
// characteristic the bulb exposes and records what happened.
func (b *Bulb) applyProtection(mode string) {
	if mode == "plain" || !platformSecuritySupported {
		return
	}
	for _, uuid := range hueCharacteristics {
		c, ok := b.chars[uuid]
		if !ok {
			continue
		}
		res, err := c.ApplyProtection(mode)
		if err != nil {
			b.SecurityNotes = append(b.SecurityNotes, fmt.Sprintf("%s protection: %s FAILED: %v", shortUUID(uuid), res, err))
			continue
		}
		b.SecurityNotes = append(b.SecurityNotes, fmt.Sprintf("%s protection: %s", shortUUID(uuid), res))
	}
}

// SecurityProbe describes each Hue characteristic (properties, protection
// level) as seen by the OS stack — the first thing to read when writes fail
// with ATT 0x05/0x0F.
func (b *Bulb) SecurityProbe() []string {
	var out []string
	for _, uuid := range hueCharacteristics {
		c, ok := b.chars[uuid]
		if !ok {
			out = append(out, shortUUID(uuid)+": not exposed")
			continue
		}
		out = append(out, shortUUID(uuid)+": "+c.Describe())
	}
	return out
}

// ReadPower reads the on/off state.
func (b *Bulb) ReadPower() (bool, time.Duration, error) {
	v, d, err := b.read(uuidHuePower)
	if err != nil {
		return false, d, err
	}
	return len(v) > 0 && v[0] != 0, d, nil
}

// ReadBrightness reads the brightness level.
func (b *Bulb) ReadBrightness() (int, time.Duration, error) {
	v, d, err := b.read(uuidHueBrightness)
	if err != nil {
		return 0, d, err
	}
	if len(v) == 0 {
		return 0, d, errors.New("empty brightness value")
	}
	return int(v[0]), d, nil
}

// ReadName reads the user-visible light name.
func (b *Bulb) ReadName() (string, error) {
	v, _, err := b.read(uuidHueName)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(v), "\x00"), nil
}

// Connected asks the stack whether the link is still up.
func (b *Bulb) Connected() (bool, error) {
	return b.link.Connected()
}

// Disconnect drops the link.
func (b *Bulb) Disconnect() error {
	b.mu.Lock()
	b.connectedState = false
	b.mu.Unlock()
	return b.link.Disconnect()
}

// connectAll connects sequentially (one at a time is the realistic server
// pattern and keeps failures attributable). Returns connected bulbs and one
// result per requested MAC, in order.
func connectAll(adapter *bluetooth.Adapter, macs []string, timeout time.Duration, log func(string, ...any)) ([]*Bulb, []ConnectResult) {
	var bulbs []*Bulb
	var results []ConnectResult
	for i, mac := range macs {
		log("[%d/%d] connecting %s ...", i+1, len(macs), mac)
		b, err := connectBulb(adapter, mac, timeout)
		if err != nil {
			log("        FAILED: %v", err)
			results = append(results, ConnectResult{MAC: strings.ToUpper(mac), Error: err.Error()})
			continue
		}
		r := b.Result()
		log("        OK  name=%q connect=%.0fms discover=%.0fms services=%d chars=%d color=%v",
			b.Label, r.ConnectMs, r.DiscoverMs, r.Services, r.Characteristics, r.HasColor)
		for _, n := range b.SecurityNotes {
			log("        security: %s", n)
		}
		bulbs = append(bulbs, b)
		results = append(results, r)
	}
	return bulbs, results
}

func disconnectAll(bulbs []*Bulb, log func(string, ...any)) {
	for _, b := range bulbs {
		if err := b.Disconnect(); err != nil {
			log("disconnect %s: %v", b.Label, err)
		}
	}
}

func shortUUID(u string) string {
	if len(u) >= 13 && strings.HasPrefix(u, "932c32bd-") {
		return "hue-" + u[9:13]
	}
	if len(u) >= 13 && strings.HasPrefix(u, "97fe6561-") {
		return "sig-" + u[9:13]
	}
	return u
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
