//go:build windows

package main

// Option B (#204): programmatic pairing the way bleak does it on Windows —
// DeviceInformationCustomPairing.PairWithProtectionLevelAsync(ConfirmOnly,
// level) with a PairingRequested handler that calls Accept(), instead of the
// Settings-app pairing whose protection level we do not control. Sequence:
//
//  1. resolve the BluetoothLEDevice (explicit LE address type; scan first if
//     Windows does not know the address any more);
//  2. read DeviceInformation.Pairing (IsPaired / CanPair / ProtectionLevel);
//  3. if paired: UnpairAsync — never start from a Settings pairing;
//  4. PairWithProtectionLevelAsync(ConfirmOnly, requested level), lowering
//     the level like bleak when ProtectionLevelCouldNotBeMet;
//  5. connect through the raw WinRT backend and run the demo writes;
//  6. UnpairAsync at the end of the session (bleak #1943 workaround), unless
//     -keep-pairing.
//
// Stop condition agreed with the CDP: if step 4 reports Paired and step 5
// still fails (0x0F / Unreachable) BEFORE any unpair, no Windows path works
// with this stack — say so, do not invent another hypothesis.

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	winrt "github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth"
	"github.com/saltosystems/winrt-go/windows/foundation"
	tb "tinygo.org/x/bluetooth"

	"buzzcontrol/spike-ble-hue/internal/winenum"
)

const programmaticPairingSupported = true

// winResolveDevice creates the BluetoothLEDevice for mac with the given
// address type. If Windows does not know the address (device removed in
// Settings and not seen since), a short BLE scan repopulates the cache.
func winResolveDevice(adapter *tb.Adapter, mac, addrType string, scanFor time.Duration) (*bluetooth.BluetoothLEDevice, error) {
	try := func() (*bluetooth.BluetoothLEDevice, error) {
		addr, err := parseAddress(mac)
		if err != nil {
			return nil, err
		}
		var winAddr uint64
		for i := range addr.MAC {
			winAddr += uint64(addr.MAC[i]) << (8 * i)
		}
		t := bluetooth.BluetoothAddressTypePublic
		if addrType == "random" {
			t = bluetooth.BluetoothAddressTypeRandom
		}
		op, err := bluetooth.BluetoothLEDeviceFromBluetoothAddressWithBluetoothAddressTypeAsync(winAddr, t)
		if err != nil {
			return nil, err
		}
		defer op.Release()
		if err := awaitAsync(op, bluetooth.SignatureBluetoothLEDevice); err != nil {
			return nil, err
		}
		res, err := op.GetResults()
		if err != nil {
			return nil, err
		}
		if uintptr(res) == 0 {
			return nil, nil
		}
		return (*bluetooth.BluetoothLEDevice)(res), nil
	}
	dev, err := try()
	if err != nil || dev != nil {
		return dev, err
	}
	logf("Windows does not know %s (%s) yet — scanning up to %s so the stack sees it advertise...", mac, addrType, scanFor)
	seen := make(chan struct{}, 1)
	stop := time.AfterFunc(scanFor, func() { _ = adapter.StopScan() })
	_ = adapter.Scan(func(a *tb.Adapter, r tb.ScanResult) {
		if strings.EqualFold(r.Address.String(), mac) {
			select {
			case seen <- struct{}{}:
			default:
			}
			_ = a.StopScan()
		}
	})
	stop.Stop()
	select {
	case <-seen:
		logf("  seen advertising")
	default:
		logf("  NOT seen advertising within %s (bulb powered? still connected to the phone?)", scanFor)
	}
	dev, err = try()
	if err != nil {
		return nil, err
	}
	if dev == nil {
		return nil, fmt.Errorf("Windows still knows no LE device %s with address type %s after scanning", mac, addrType)
	}
	return dev, nil
}

// winPairingOf returns the DeviceInformation and its Pairing object for a device.
func winPairingOf(dev *bluetooth.BluetoothLEDevice) (*winenum.DeviceInformation, *winenum.DeviceInformationPairing, error) {
	bdid, err := dev.GetBluetoothDeviceId()
	if err != nil {
		return nil, nil, fmt.Errorf("GetBluetoothDeviceId: %w", err)
	}
	defer bdid.Release()
	id, err := bdid.GetId()
	if err != nil {
		return nil, nil, fmt.Errorf("BluetoothDeviceId.Id: %w", err)
	}
	logf("  device id: %s", id)
	op, err := winenum.DeviceInformationCreateFromIdAsync(id)
	if err != nil {
		return nil, nil, fmt.Errorf("DeviceInformation.CreateFromIdAsync: %w", err)
	}
	defer op.Release()
	if err := awaitAsync(op, winenum.SignatureDeviceInformation); err != nil {
		return nil, nil, fmt.Errorf("DeviceInformation.CreateFromIdAsync: %w", err)
	}
	res, err := op.GetResults()
	if err != nil {
		return nil, nil, err
	}
	if uintptr(res) == 0 {
		return nil, nil, fmt.Errorf("DeviceInformation.CreateFromIdAsync returned null for %s", id)
	}
	di := (*winenum.DeviceInformation)(res)
	pairing, err := di.GetPairing()
	if err != nil {
		di.Release()
		return nil, nil, fmt.Errorf("DeviceInformation.Pairing: %w", err)
	}
	return di, pairing, nil
}

type pairingState struct {
	IsPaired        bool   `json:"is_paired"`
	CanPair         bool   `json:"can_pair"`
	ProtectionLevel string `json:"protection_level"`
}

func winPairingState(p *winenum.DeviceInformationPairing) pairingState {
	st := pairingState{}
	st.IsPaired, _ = p.GetIsPaired()
	st.CanPair, _ = p.GetCanPair()
	if lvl, err := p.GetProtectionLevel(); err == nil {
		st.ProtectionLevel = nameOf(devicePairingProtectionLevelNames, int(lvl))
	} else {
		st.ProtectionLevel = "? (" + err.Error() + ")"
	}
	return st
}

// winUnpair calls UnpairAsync and returns the status name.
func winUnpair(p *winenum.DeviceInformationPairing) (string, bool, error) {
	op, err := p.UnpairAsync()
	if err != nil {
		return "", false, err
	}
	defer op.Release()
	if err := awaitAsync(op, winenum.SignatureDeviceUnpairingResult); err != nil {
		return "", false, err
	}
	res, err := op.GetResults()
	if err != nil {
		return "", false, err
	}
	r := (*winenum.DeviceUnpairingResult)(res)
	defer r.Release()
	status, err := r.GetStatus()
	if err != nil {
		return "", false, err
	}
	return nameOf(deviceUnpairingResultStatusNames, int(status)), status == winenum.DeviceUnpairingResultStatusUnpaired || status == winenum.DeviceUnpairingResultStatusAlreadyUnpaired, nil
}

type pairAttempt struct {
	RequestedLevel string  `json:"requested_level"`
	Status         string  `json:"status"`
	LevelUsed      string  `json:"level_used"`
	DurationMs     float64 `json:"duration_ms"`
	HandlerCalls   int32   `json:"handler_calls"`
	Kinds          string  `json:"kinds_requested,omitempty"`
}

// winPairCustom performs the bleak-style ceremony: ConfirmOnly, auto-Accept,
// requested protection level first, then lowered on ProtectionLevelCouldNotBeMet.
func winPairCustom(p *winenum.DeviceInformationPairing, requested int) ([]pairAttempt, bool, error) {
	custom, err := p.GetCustom()
	if err != nil {
		return nil, false, fmt.Errorf("Pairing.Custom: %w", err)
	}
	defer custom.Release()

	var handlerCalls int32
	var kindsSeen atomic.Value
	kindsSeen.Store("")
	iid := winrt.ParameterizedInstanceGUID(foundation.GUIDTypedEventHandler,
		winenum.SignatureDeviceInformationCustomPairing, winenum.SignatureDevicePairingRequestedEventArgs)
	handler := foundation.NewTypedEventHandler(ole.NewGUID(iid), func(_ *foundation.TypedEventHandler, _ unsafe.Pointer, args unsafe.Pointer) {
		atomic.AddInt32(&handlerCalls, 1)
		ev := (*winenum.DevicePairingRequestedEventArgs)(args)
		kind, _ := ev.GetPairingKind()
		pin, _ := ev.GetPin()
		kindsSeen.Store(nameOf(devicePairingKindsNames, int(kind)))
		logf("  PairingRequested: kind=%s pin=%q → Accept()", nameOf(devicePairingKindsNames, int(kind)), pin)
		if err := ev.Accept(); err != nil {
			logf("  Accept() failed: %v", err)
		}
	})
	defer handler.Release()
	token, err := custom.AddPairingRequested(handler)
	if err != nil {
		return nil, false, fmt.Errorf("AddPairingRequested: %w", err)
	}
	defer custom.RemovePairingRequested(token)

	// Ladder: requested level, then Encryption (if requested was stronger),
	// then plain PairAsync(ConfirmOnly) — exactly bleak's fallback order.
	ladder := []int{requested}
	if requested == 3 {
		ladder = append(ladder, 2)
	}
	ladder = append(ladder, -1) // -1 = PairAsync without level
	var attempts []pairAttempt
	for _, lvl := range ladder {
		var op *foundation.IAsyncOperation
		label := "PairAsync(ConfirmOnly)"
		if lvl >= 0 {
			label = "PairWithProtectionLevelAsync(ConfirmOnly, " + nameOf(devicePairingProtectionLevelNames, lvl) + ")"
			op, err = custom.PairWithProtectionLevelAsync(winenum.DevicePairingKindsConfirmOnly, winenum.DevicePairingProtectionLevel(lvl))
		} else {
			op, err = custom.PairAsync(winenum.DevicePairingKindsConfirmOnly)
		}
		if err != nil {
			return attempts, false, fmt.Errorf("%s: %w", label, err)
		}
		logf("  %s ... (Windows may take up to ~60 s to answer)", label)
		start := time.Now()
		aerr := awaitAsync(op, winenum.SignatureDevicePairingResult)
		att := pairAttempt{RequestedLevel: label, DurationMs: ms(time.Since(start))}
		if aerr != nil {
			op.Release()
			att.Status = "async error: " + aerr.Error()
			attempts = append(attempts, att)
			logf("  → %s", att.Status)
			return attempts, false, nil
		}
		res, err := op.GetResults()
		op.Release()
		if err != nil {
			return attempts, false, err
		}
		r := (*winenum.DevicePairingResult)(res)
		status, _ := r.GetStatus()
		used, _ := r.GetProtectionLevelUsed()
		r.Release()
		att.Status = nameOf(devicePairingResultStatusNames, int(status))
		att.LevelUsed = nameOf(devicePairingProtectionLevelNames, int(used))
		att.HandlerCalls = atomic.LoadInt32(&handlerCalls)
		att.Kinds, _ = kindsSeen.Load().(string)
		attempts = append(attempts, att)
		logf("  → status=%s levelUsed=%s handlerCalls=%d after %.0fms", att.Status, att.LevelUsed, att.HandlerCalls, att.DurationMs)
		if pairingSucceeded(int(status)) {
			return attempts, true, nil
		}
		if !pairingLevelNotMet(int(status)) {
			return attempts, false, nil
		}
		logf("  protection level not met — lowering, like bleak")
	}
	return attempts, false, nil
}

// cmdPairTest runs the whole option-B sequence on ONE bulb.
func cmdPairTest(adapter *tb.Adapter, macs []string, mode WriteMode, report *Report) error {
	if len(macs) != 1 {
		return fmt.Errorf("pairtest works on exactly one bulb (-bulbs MAC), got %d", len(macs))
	}
	mac := strings.ToUpper(macs[0])
	addrType := resolveAddrType(mac, addrTypeMode)
	level, ok := pairLevelFromMode(*flagPairLevel)
	if !ok {
		return fmt.Errorf("unknown -pair-level %q (auth|encrypt|default)", *flagPairLevel)
	}
	section := map[string]any{"mac": mac, "addr_type": addrType, "requested_level": nameOf(devicePairingProtectionLevelNames, level), "keep_pairing": *flagKeepPairing}
	report.Sections["pairtest"] = section

	logf("===== pairtest 1/6: resolve %s (addr-type %s) =====", mac, addrType)
	dev, err := winResolveDevice(adapter, mac, addrType, 20*time.Second)
	if err != nil {
		return err
	}
	defer func() { dev.Release() }() // closure: dev may be re-resolved after the unpair

	logf("===== pairtest 2/6: DeviceInformation.Pairing =====")
	di, pairing, err := winPairingOf(dev)
	if err != nil {
		return err
	}
	defer func() { di.Release() }()
	defer func() { pairing.Release() }()
	if name, err := di.GetName(); err == nil {
		logf("  name: %q", name)
	}
	before := winPairingState(pairing)
	section["pairing_before"] = before
	logf("  IsPaired=%v CanPair=%v ProtectionLevel=%s", before.IsPaired, before.CanPair, before.ProtectionLevel)

	logf("===== pairtest 3/6: unpair first (never start from a Settings pairing) =====")
	if before.IsPaired {
		status, unpaired, err := winUnpair(pairing)
		section["unpair_before"] = status
		if err != nil {
			return fmt.Errorf("UnpairAsync: %w", err)
		}
		logf("  UnpairAsync → %s", status)
		if !unpaired {
			return fmt.Errorf("could not unpair programmatically (%s) — remove the device in Windows Settings once, then rerun", status)
		}
		// Run 1 showed the SAME DeviceInformation object still reporting
		// IsPaired=true 2 s after Unpaired(0): the pairing state is cached in
		// the object. Drop every handle, re-resolve the device (scanning if
		// Windows forgot it) and re-create DeviceInformation until it reports
		// IsPaired=false — a pairing attempted on a still-"paired" record is
		// not a clean test (bleak would have skipped it).
		pairing.Release()
		di.Release()
		dev.Release()
		var mid pairingState
		deadline := time.Now().Add(20 * time.Second)
		for {
			time.Sleep(1500 * time.Millisecond)
			d2, err := winResolveDevice(adapter, mac, addrType, 15*time.Second)
			if err != nil {
				return fmt.Errorf("re-resolve after unpair: %w", err)
			}
			di2, p2, err := winPairingOf(d2)
			if err != nil {
				d2.Release()
				return fmt.Errorf("re-read pairing after unpair: %w", err)
			}
			mid = winPairingState(p2)
			logf("  after unpair (fresh objects): IsPaired=%v CanPair=%v ProtectionLevel=%s", mid.IsPaired, mid.CanPair, mid.ProtectionLevel)
			if !mid.IsPaired || time.Now().After(deadline) {
				dev, di, pairing = d2, di2, p2
				break
			}
			p2.Release()
			di2.Release()
			d2.Release()
		}
		section["pairing_after_unpair"] = mid
		if mid.IsPaired {
			report.Note("UnpairAsync returned Unpaired but Windows still reports IsPaired=true 20 s later — the OS did not drop the record; remove the device in Settings once and rerun")
			return fmt.Errorf("device still reported paired 20 s after UnpairAsync — remove it in Windows Settings (Bluetooth → Hue go → Remove device), wait until it disappears, then rerun pairtest")
		}
	} else {
		logf("  not paired — nothing to unpair")
	}

	logf("===== pairtest 4/6: programmatic pairing (ConfirmOnly, min level %s) =====", nameOf(devicePairingProtectionLevelNames, level))
	attempts, paired, err := winPairCustom(pairing, level)
	section["pair_attempts"] = attempts
	if err != nil {
		return err
	}
	after := winPairingState(pairing)
	section["pairing_after_pair"] = after
	logf("  after pairing: IsPaired=%v ProtectionLevel=%s", after.IsPaired, after.ProtectionLevel)
	if !paired {
		report.Note("STOP CONDITION: programmatic pairing did not reach Paired — see pair_attempts; the bulb/Windows refuse the bond itself")
		logf("RESULT: programmatic pairing FAILED — %s", lastStatus(attempts))
		return fmt.Errorf("programmatic pairing failed (%s)", lastStatus(attempts))
	}

	logf("===== pairtest 5/6: connect (winrt backend) and write =====")
	backendMode = "winrt"
	bulbs, results := connectAll(adapter, macs, *flagTimeout, logf)
	report.Bulbs = results
	if len(bulbs) == 0 {
		report.Note("paired programmatically but connection failed afterwards")
		if !*flagKeepPairing {
			logf("===== pairtest 6/6: unpair =====")
			status, _, _ := winUnpair(pairing)
			logf("  UnpairAsync → %s", status)
		}
		return fmt.Errorf("connect after pairing failed")
	}
	for _, b := range bulbs {
		logf("%-20s security probe:", b.Label)
		for _, line := range b.SecurityProbe() {
			logf("    %s", line)
		}
	}
	failures, stats := runDemoSteps(bulbs, mode, report)
	section["write_failures"] = failures
	section["write_latency"] = stats
	disconnectAll(bulbs, logf)

	if *flagKeepPairing {
		logf("===== pairtest 6/6: keeping the pairing (-keep-pairing) =====")
	} else {
		logf("===== pairtest 6/6: unpair at end of session (bleak #1943) =====")
		status, _, err := winUnpair(pairing)
		section["unpair_after"] = status
		logf("  UnpairAsync → %s err=%v", status, err)
	}

	if failures == 0 && stats.Count > 0 {
		logf("RESULT: programmatic pairing + writes SUCCEEDED (%d writes, %s)", stats.Count, stats)
		return nil
	}
	report.Note("STOP CONDITION: paired programmatically (status Paired) but %d write(s) still failed before any unpair — no Windows path works with this stack", failures)
	logf("RESULT: paired programmatically but %d write(s) FAILED — stop condition reached", failures)
	return fmt.Errorf("%d write(s) failed after programmatic pairing", failures)
}

func lastStatus(a []pairAttempt) string {
	if len(a) == 0 {
		return "no attempt"
	}
	return a[len(a)-1].Status
}
