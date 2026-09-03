//go:build windows

package main

// Windows-only security diagnostics for the #204 spike, added after the first
// physical run failed every write with ATT 0x000F (Insufficient
// Authentication) and every read with E_INVALIDARG ("Paramètre incorrect.").
//
// Two things tinygo.org/x/bluetooth v0.15.0 does not do, both done here by
// reaching the raw WinRT GattCharacteristic behind tinygo's wrapper:
//
//  1. It never sets GattCharacteristic.ProtectionLevel (always Plain). With
//     Plain, Windows issues the ATT operation on the unencrypted link and just
//     surfaces the peripheral's 0x0F. With EncryptionAndAuthenticationRequired,
//     Windows brings the link up to that level FIRST — using the stored LTK if
//     the device is really bonded, or by starting a pairing itself if it is
//     not (which is exactly the runtime prompt this spike must observe).
//  2. Its Read ignores GattReadResult.Status: on ProtocolError the value buffer
//     is null and DataReaderFromBuffer(nil) fails with E_INVALIDARG — the
//     "Paramètre incorrect." seen in the log is the SAME auth failure, not a
//     second bug. readWithStatus reports the status instead.
//
// winrt-go's generated GattCharacteristic has the Get/SetProtectionLevel
// vtable slots but no Go wrapper, so the two calls are made by hand with the
// exact pattern the generator uses (QueryInterface IGattCharacteristic, then
// syscall on the vtable slot).

import (
	"fmt"
	"reflect"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	winrt "github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/genericattributeprofile"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/storage/streams"
	tb "tinygo.org/x/bluetooth"
)

const platformSecuritySupported = true

// rawGattCharacteristic extracts the WinRT object from tinygo's wrapper
// (unexported field `characteristic` of *deviceCharacteristic). Throwaway
// spike: reflect+unsafe is acceptable here, it would not be in the server.
func rawGattCharacteristic(c tb.DeviceCharacteristic) (*genericattributeprofile.GattCharacteristic, error) {
	v := reflect.ValueOf(c)
	if v.NumField() == 0 {
		return nil, fmt.Errorf("unexpected DeviceCharacteristic layout")
	}
	inner := v.Field(0) // *deviceCharacteristic
	if inner.Kind() != reflect.Ptr || inner.IsNil() {
		return nil, fmt.Errorf("DeviceCharacteristic has no inner struct")
	}
	f := inner.Elem().FieldByName("characteristic")
	if !f.IsValid() {
		return nil, fmt.Errorf("field 'characteristic' not found — tinygo layout changed")
	}
	p := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
	gc, ok := p.Interface().(*genericattributeprofile.GattCharacteristic)
	if !ok || gc == nil {
		return nil, fmt.Errorf("field 'characteristic' is not a *GattCharacteristic")
	}
	return gc, nil
}

// --- IGattCharacteristic vtable prefix (order from winrt-go's generated file) ---

const guidIGattCharacteristic = "59cb50c1-5934-4f68-a198-eb864fa44e6b"

type iGattCharacteristic struct{ ole.IInspectable }

type iGattCharacteristicVtbl struct {
	ole.IInspectableVtbl
	GetDescriptors              uintptr
	GetCharacteristicProperties uintptr
	GetProtectionLevel          uintptr
	SetProtectionLevel          uintptr
}

func (v *iGattCharacteristic) vtable() *iGattCharacteristicVtbl {
	return (*iGattCharacteristicVtbl)(unsafe.Pointer(v.RawVTable))
}

func getProtectionLevel(gc *genericattributeprofile.GattCharacteristic) (genericattributeprofile.GattProtectionLevel, error) {
	itf := gc.MustQueryInterface(ole.NewGUID(guidIGattCharacteristic))
	defer itf.Release()
	v := (*iGattCharacteristic)(unsafe.Pointer(itf))
	var out genericattributeprofile.GattProtectionLevel
	hr, _, _ := syscall.SyscallN(v.vtable().GetProtectionLevel, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(&out)))
	if hr != 0 {
		return 0, ole.NewError(hr)
	}
	return out, nil
}

func setProtectionLevel(gc *genericattributeprofile.GattCharacteristic, level genericattributeprofile.GattProtectionLevel) error {
	itf := gc.MustQueryInterface(ole.NewGUID(guidIGattCharacteristic))
	defer itf.Release()
	v := (*iGattCharacteristic)(unsafe.Pointer(itf))
	hr, _, _ := syscall.SyscallN(v.vtable().SetProtectionLevel, uintptr(unsafe.Pointer(v)), uintptr(level))
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

func protectionLevelName(l genericattributeprofile.GattProtectionLevel) string {
	switch l {
	case genericattributeprofile.GattProtectionLevelPlain:
		return "Plain"
	case genericattributeprofile.GattProtectionLevelAuthenticationRequired:
		return "AuthenticationRequired"
	case genericattributeprofile.GattProtectionLevelEncryptionRequired:
		return "EncryptionRequired"
	case genericattributeprofile.GattProtectionLevelEncryptionAndAuthenticationRequired:
		return "EncryptionAndAuthenticationRequired"
	}
	return fmt.Sprintf("Unknown(%d)", int(l))
}

func protectionLevelFromMode(mode string) (genericattributeprofile.GattProtectionLevel, bool) {
	switch mode {
	case "encrypt":
		return genericattributeprofile.GattProtectionLevelEncryptionRequired, true
	case "auth":
		return genericattributeprofile.GattProtectionLevelEncryptionAndAuthenticationRequired, true
	}
	return genericattributeprofile.GattProtectionLevelPlain, false
}

func communicationStatusName(s genericattributeprofile.GattCommunicationStatus) string {
	switch s {
	case genericattributeprofile.GattCommunicationStatusSuccess:
		return "Success"
	case genericattributeprofile.GattCommunicationStatusUnreachable:
		return "Unreachable"
	case genericattributeprofile.GattCommunicationStatusProtocolError:
		return "ProtocolError (peripheral refused — typically ATT 0x05/0x0F: link not encrypted/authenticated)"
	case genericattributeprofile.GattCommunicationStatusAccessDenied:
		return "AccessDenied (Windows refused: protection level not met / not paired)"
	}
	return fmt.Sprintf("Unknown(%d)", int(s))
}

// --- async helpers (same pattern as tinygo's adapter_windows.go) ---

func awaitAsync(op *foundation.IAsyncOperation, resultSignature string) error {
	iid := winrt.ParameterizedInstanceGUID(foundation.GUIDAsyncOperationCompletedHandler, resultSignature)
	var status foundation.AsyncStatus
	done := make(chan struct{})
	handler := foundation.NewAsyncOperationCompletedHandler(ole.NewGUID(iid), func(_ *foundation.AsyncOperationCompletedHandler, _ *foundation.IAsyncOperation, s foundation.AsyncStatus) {
		status = s
		close(done)
	})
	defer handler.Release()
	if err := op.SetCompleted(handler); err != nil {
		return err
	}
	<-done
	if status != foundation.AsyncStatusCompleted {
		if err := asyncError(op); err != nil {
			return fmt.Errorf("async operation failed with status %d: %w", status, err)
		}
		return fmt.Errorf("async operation failed with status %d", status)
	}
	return nil
}

func asyncError(op *foundation.IAsyncOperation) error {
	iid := ole.NewGUID(foundation.GUIDIAsyncInfo)
	var info *foundation.IAsyncInfo
	hr, _, _ := syscall.SyscallN(op.VTable().QueryInterface, uintptr(unsafe.Pointer(op)), uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&info)))
	if hr != 0 {
		return nil
	}
	defer info.Release()
	code, err := info.GetErrorCode()
	if err != nil {
		return err
	}
	if code.Value == 0 {
		return nil
	}
	hres := uint32(code.Value)
	if (hres>>16)&0x1FFF == 0x65 {
		return fmt.Errorf("Bluetooth ATT error 0x%04X (%s)", hres&0xFFFF, attErrorName(hres&0xFFFF))
	}
	return fmt.Errorf("HRESULT 0x%08X", hres)
}

func attErrorName(code uint32) string {
	switch code {
	case 0x01:
		return "Invalid Handle"
	case 0x02:
		return "Read Not Permitted"
	case 0x03:
		return "Write Not Permitted"
	case 0x05:
		return "Insufficient Authentication — link not authenticated/bonded"
	case 0x08:
		return "Insufficient Authorization"
	case 0x0F:
		return "Insufficient Encryption — link not encrypted (no usable LTK for this peer)"
	case 0x0E:
		return "Unlikely Error"
	}
	return "see Bluetooth Core Spec Vol 3 Part F §3.4.1.1"
}

// readWithStatus reads a characteristic and reports the WinRT communication
// status instead of tripping over a null buffer.
func readWithStatus(gc *genericattributeprofile.GattCharacteristic) ([]byte, error) {
	op, err := gc.ReadValueWithCacheModeAsync(bluetooth.BluetoothCacheModeUncached)
	if err != nil {
		return nil, err
	}
	defer op.Release()
	if err := awaitAsync(op, genericattributeprofile.SignatureGattReadResult); err != nil {
		return nil, err
	}
	res, err := op.GetResults()
	if err != nil {
		return nil, err
	}
	result := (*genericattributeprofile.GattReadResult)(res)
	defer result.Release()
	status, err := result.GetStatus()
	if err != nil {
		return nil, err
	}
	if status != genericattributeprofile.GattCommunicationStatusSuccess {
		return nil, fmt.Errorf("read refused: status=%s", communicationStatusName(status))
	}
	buffer, err := result.GetValue()
	if err != nil {
		return nil, err
	}
	if buffer == nil {
		return nil, fmt.Errorf("read returned Success but no value buffer")
	}
	defer buffer.Release()
	n, err := buffer.GetLength()
	if err != nil {
		return nil, err
	}
	reader, err := streams.DataReaderFromBuffer(buffer)
	if err != nil {
		return nil, err
	}
	defer reader.Release()
	return reader.ReadBytes(n)
}

// --- hooks used by hue.go ---

// platformRead replaces tinygo's Read on Windows (status-aware).
func platformRead(c tb.DeviceCharacteristic) ([]byte, error) {
	gc, err := rawGattCharacteristic(c)
	if err != nil {
		buf := make([]byte, 64)
		n, rerr := c.Read(buf)
		if rerr != nil {
			return nil, rerr
		}
		return buf[:n], nil
	}
	return readWithStatus(gc)
}

// platformApplyProtection sets the requested GATT protection level on one
// characteristic and returns "before → after" for the log.
func platformApplyProtection(c tb.DeviceCharacteristic, mode string) (string, error) {
	gc, err := rawGattCharacteristic(c)
	if err != nil {
		return "", err
	}
	before, err := getProtectionLevel(gc)
	if err != nil {
		return "", fmt.Errorf("GetProtectionLevel: %w", err)
	}
	level, ok := protectionLevelFromMode(mode)
	if !ok {
		return protectionLevelName(before) + " (unchanged)", nil
	}
	if err := setProtectionLevel(gc, level); err != nil {
		return protectionLevelName(before), fmt.Errorf("SetProtectionLevel(%s): %w", protectionLevelName(level), err)
	}
	after, err := getProtectionLevel(gc)
	if err != nil {
		return protectionLevelName(before) + " → ?", nil
	}
	return protectionLevelName(before) + " → " + protectionLevelName(after), nil
}

// platformDescribe returns the WinRT property bits and protection level of a
// characteristic — the security probe printed by `demo`.
func platformDescribe(c tb.DeviceCharacteristic) string {
	gc, err := rawGattCharacteristic(c)
	if err != nil {
		return "(raw access failed: " + err.Error() + ")"
	}
	props, perr := gc.GetCharacteristicProperties()
	level, lerr := getProtectionLevel(gc)
	s := ""
	if perr == nil {
		s += fmt.Sprintf("props=0x%04X[%s]", uint32(props), propsNames(props))
	} else {
		s += "props=? (" + perr.Error() + ")"
	}
	if lerr == nil {
		s += " protection=" + protectionLevelName(level)
	} else {
		s += " protection=? (" + lerr.Error() + ")"
	}
	return s
}

func propsNames(p genericattributeprofile.GattCharacteristicProperties) string {
	names := ""
	add := func(bit genericattributeprofile.GattCharacteristicProperties, n string) {
		if p&bit != 0 {
			if names != "" {
				names += "|"
			}
			names += n
		}
	}
	add(genericattributeprofile.GattCharacteristicPropertiesRead, "Read")
	add(genericattributeprofile.GattCharacteristicPropertiesWriteWithoutResponse, "WriteNoRsp")
	add(genericattributeprofile.GattCharacteristicPropertiesWrite, "Write")
	add(genericattributeprofile.GattCharacteristicPropertiesNotify, "Notify")
	add(genericattributeprofile.GattCharacteristicPropertiesIndicate, "Indicate")
	add(genericattributeprofile.GattCharacteristicPropertiesAuthenticatedSignedWrites, "SignedWrite")
	return names
}
