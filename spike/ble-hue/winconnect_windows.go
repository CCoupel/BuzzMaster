//go:build windows

package main

// Raw WinRT backend (-backend winrt), added after T2 failed: tinygo's Windows
// Connect creates the device with FromBluetoothAddressAsync(addr) — the
// overload WITHOUT an address type — while Hue bulbs advertise a STATIC RANDOM
// address (FD:1B:… has its two MSBs set). bleak, which drives Hue bulbs on
// Windows, uses FromBluetoothAddressAsync(addr, BluetoothAddressType.Random).
// If Windows keys the bond on (address, type), a device object created with
// the wrong type reaches the peer over the existing link (discovery works,
// Plain ops go on air) but never finds the LTK — exactly the two failure
// shapes observed (0x0F with Plain, instant Unreachable with a protection
// level). This backend bypasses tinygo for connect/discover/read/write and
// passes the address type explicitly.

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/saltosystems/winrt-go/windows/devices/bluetooth"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/genericattributeprofile"
	"github.com/saltosystems/winrt-go/windows/storage/streams"
	tb "tinygo.org/x/bluetooth"
)

const winrtBackendSupported = true

// winLink is the raw-WinRT bulbLink.
type winLink struct {
	dev     *bluetooth.BluetoothLEDevice
	session *genericattributeprofile.GattSession
}

func (l *winLink) Connected() (bool, error) {
	st, err := l.dev.GetConnectionStatus()
	if err != nil {
		return false, err
	}
	return st == bluetooth.BluetoothConnectionStatusConnected, nil
}

func (l *winLink) Disconnect() error {
	var first error
	if l.session != nil {
		if err := l.session.Close(); err != nil && first == nil {
			first = err
		}
		l.session.Release()
	}
	if l.dev != nil {
		if err := l.dev.Close(); err != nil && first == nil {
			first = err
		}
		l.dev.Release()
	}
	return first
}

// winChar is the raw-WinRT gattChar.
type winChar struct {
	uuid  string
	gc    *genericattributeprofile.GattCharacteristic
	props genericattributeprofile.GattCharacteristicProperties
}

func (c *winChar) UUID() string { return c.uuid }

func (c *winChar) Read() ([]byte, error) { return readWithStatus(c.gc) }

func (c *winChar) Write(data []byte, mode WriteMode) error {
	option := genericattributeprofile.GattWriteOptionWriteWithResponse
	if mode == WriteCommand {
		if c.props&genericattributeprofile.GattCharacteristicPropertiesWriteWithoutResponse == 0 {
			return fmt.Errorf("bluetooth: write without response not supported")
		}
		option = genericattributeprofile.GattWriteOptionWriteWithoutResponse
	} else if c.props&genericattributeprofile.GattCharacteristicPropertiesWrite == 0 {
		return fmt.Errorf("bluetooth: write not supported")
	}
	writer, err := streams.NewDataWriter()
	if err != nil {
		return err
	}
	defer writer.Release()
	if err := writer.WriteBytes(uint32(len(data)), data); err != nil {
		return err
	}
	buf, err := writer.DetachBuffer()
	if err != nil {
		return err
	}
	defer buf.Release()
	op, err := c.gc.WriteValueWithOptionAsync(buf, option)
	if err != nil {
		return err
	}
	defer op.Release()
	if err := awaitAsync(op, genericattributeprofile.SignatureGattCommunicationStatus); err != nil {
		return err
	}
	res, err := op.GetResults()
	if err != nil {
		return err
	}
	status := genericattributeprofile.GattCommunicationStatus(uintptr(res))
	if status != genericattributeprofile.GattCommunicationStatusSuccess {
		return fmt.Errorf("write refused: status=%s", communicationStatusName(status))
	}
	return nil
}

func (c *winChar) Describe() string { return describeGatt(c.gc) }

func (c *winChar) ApplyProtection(mode string) (string, error) {
	return applyProtectionGatt(c.gc, mode)
}

// winConnect connects with an explicit address type and resolves the whole
// GATT table. addrType: "public" | "random".
func winConnect(mac string, addrType string, log func(string, ...any)) (bulbLink, map[string]gattChar, int, int, error) {
	addr, err := parseAddress(mac)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	var winAddr uint64
	for i := range addr.MAC {
		winAddr += uint64(addr.MAC[i]) << (8 * i)
	}
	t := bluetooth.BluetoothAddressTypePublic
	if addrType == "random" {
		t = bluetooth.BluetoothAddressTypeRandom
	}

	devOp, err := bluetooth.BluetoothLEDeviceFromBluetoothAddressWithBluetoothAddressTypeAsync(winAddr, t)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("FromBluetoothAddressAsync(%s, %s): %w", mac, addrType, err)
	}
	defer devOp.Release()
	if err := awaitAsync(devOp, bluetooth.SignatureBluetoothLEDevice); err != nil {
		return nil, nil, 0, 0, fmt.Errorf("FromBluetoothAddressAsync(%s, %s): %w", mac, addrType, err)
	}
	res, err := devOp.GetResults()
	if err != nil {
		return nil, nil, 0, 0, err
	}
	if uintptr(res) == 0 {
		return nil, nil, 0, 0, fmt.Errorf("Windows knows no LE device %s with address type %s (not paired/seen with that type)", mac, addrType)
	}
	dev := (*bluetooth.BluetoothLEDevice)(res)
	link := &winLink{dev: dev}

	// Own the connection through a GattSession (same as tinygo).
	dID, err := dev.GetBluetoothDeviceId()
	if err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, err
	}
	defer dID.Release()
	sessOp, err := genericattributeprofile.GattSessionFromDeviceIdAsync(dID)
	if err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, err
	}
	defer sessOp.Release()
	if err := awaitAsync(sessOp, genericattributeprofile.SignatureGattSession); err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, fmt.Errorf("GattSession: %w", err)
	}
	sres, err := sessOp.GetResults()
	if err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, err
	}
	link.session = (*genericattributeprofile.GattSession)(sres)
	if err := link.session.SetMaintainConnection(true); err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, fmt.Errorf("MaintainConnection: %w", err)
	}

	// Services.
	svcOp, err := dev.GetGattServicesWithCacheModeAsync(bluetooth.BluetoothCacheModeUncached)
	if err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, err
	}
	defer svcOp.Release()
	if err := awaitAsync(svcOp, genericattributeprofile.SignatureGattDeviceServicesResult); err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, fmt.Errorf("GetGattServices: %w", err)
	}
	svcRes, err := svcOp.GetResults()
	if err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, err
	}
	servicesResult := (*genericattributeprofile.GattDeviceServicesResult)(svcRes)
	defer servicesResult.Release()
	if st, err := servicesResult.GetStatus(); err != nil || st != genericattributeprofile.GattCommunicationStatusSuccess {
		link.Disconnect()
		return nil, nil, 0, 0, fmt.Errorf("GetGattServices status=%s err=%v", communicationStatusName(st), err)
	}
	svcVector, err := servicesResult.GetServices()
	if err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, err
	}
	nSvc, err := svcVector.GetSize()
	if err != nil {
		link.Disconnect()
		return nil, nil, 0, 0, err
	}

	chars := map[string]gattChar{}
	nChars := 0
	for i := uint32(0); i < nSvc; i++ {
		sp, err := svcVector.GetAt(i)
		if err != nil {
			continue
		}
		svc := (*genericattributeprofile.GattDeviceService)(sp)
		guid, err := svc.GetUuid()
		svcUUID := "?"
		if err == nil {
			svcUUID = strings.ToLower(tb.GUIDToUUID(guid).String())
		}
		chOp, err := svc.GetCharacteristicsWithCacheModeAsync(bluetooth.BluetoothCacheModeUncached)
		if err != nil {
			log("        service %s: GetCharacteristics: %v", svcUUID, err)
			continue
		}
		if err := awaitAsync(chOp, genericattributeprofile.SignatureGattCharacteristicsResult); err != nil {
			log("        service %s: GetCharacteristics: %v", svcUUID, err)
			chOp.Release()
			continue
		}
		chRes, err := chOp.GetResults()
		chOp.Release()
		if err != nil {
			continue
		}
		charsResult := (*genericattributeprofile.GattCharacteristicsResult)(chRes)
		if st, err := charsResult.GetStatus(); err != nil || st != genericattributeprofile.GattCommunicationStatusSuccess {
			log("        service %s: characteristics status=%s err=%v", svcUUID, communicationStatusName(st), err)
			charsResult.Release()
			continue
		}
		chVector, err := charsResult.GetCharacteristics()
		if err != nil {
			charsResult.Release()
			continue
		}
		n, _ := chVector.GetSize()
		for j := uint32(0); j < n; j++ {
			cp, err := chVector.GetAt(j)
			if err != nil {
				continue
			}
			gc := (*genericattributeprofile.GattCharacteristic)(cp)
			cguid, err := gc.GetUuid()
			if err != nil {
				continue
			}
			props, _ := gc.GetCharacteristicProperties()
			uuid := strings.ToLower(tb.GUIDToUUID(cguid).String())
			chars[uuid] = &winChar{uuid: uuid, gc: gc, props: props}
			nChars++
		}
		charsResult.Release()
	}
	return link, chars, int(nSvc), nChars, nil
}

// winConnectWithTimeout bounds winConnect (WinRT async calls can hang on an
// unreachable peer). The goroutine is leaked on timeout — throwaway spike.
func winConnectWithTimeout(mac, addrType string, timeout time.Duration, log func(string, ...any)) (bulbLink, map[string]gattChar, int, int, error) {
	type res struct {
		link  bulbLink
		chars map[string]gattChar
		s, c  int
		err   error
	}
	ch := make(chan res, 1)
	go func() {
		l, c, s, n, err := winConnect(mac, addrType, log)
		ch <- res{l, c, s, n, err}
	}()
	select {
	case r := <-ch:
		return r.link, r.chars, r.s, r.c, r.err
	case <-time.After(timeout):
		return nil, nil, 0, 0, fmt.Errorf("winrt connect %s: timeout after %s", mac, timeout)
	}
}

// resolveAddrType turns "auto" into "random" for static-random addresses (two
// MSBs of the first byte set — the pattern of every Hue bulb seen so far).
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

var _ = unsafe.Pointer(nil) // keep unsafe imported for pointer casts above
