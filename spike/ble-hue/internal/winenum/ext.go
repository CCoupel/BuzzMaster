//go:build windows

package winenum

// Hand-written complements to the generated code (winrt-go-gen produced the
// vtables of IDeviceInformation/IDeviceInformation2 but, with the method
// filter used, no instance wrappers). Same call pattern as the generator.

import (
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

// IDevicePairingSettings is referenced by PairWithProtectionLevelAndSettingsAsync
// but never used by the spike; stub so the generated code compiles.
type IDevicePairingSettings struct {
	ole.IInspectable
}

// GetId returns the DeviceInformation.Id string (e.g. "BluetoothLE#BluetoothLE…").
func (impl *DeviceInformation) GetId() (string, error) {
	itf := impl.MustQueryInterface(ole.NewGUID(GUIDiDeviceInformation))
	defer itf.Release()
	v := (*iDeviceInformation)(unsafe.Pointer(itf))
	var out ole.HString
	hr, _, _ := syscall.SyscallN(v.VTable().GetId, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(&out)))
	if hr != 0 {
		return "", ole.NewError(hr)
	}
	s := out.String()
	ole.DeleteHString(out)
	return s, nil
}

// GetName returns the DeviceInformation.Name string.
func (impl *DeviceInformation) GetName() (string, error) {
	itf := impl.MustQueryInterface(ole.NewGUID(GUIDiDeviceInformation))
	defer itf.Release()
	v := (*iDeviceInformation)(unsafe.Pointer(itf))
	var out ole.HString
	hr, _, _ := syscall.SyscallN(v.VTable().GetName, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(&out)))
	if hr != 0 {
		return "", ole.NewError(hr)
	}
	s := out.String()
	ole.DeleteHString(out)
	return s, nil
}

// GetPairing returns DeviceInformation.Pairing (IDeviceInformation2).
func (impl *DeviceInformation) GetPairing() (*DeviceInformationPairing, error) {
	itf := impl.MustQueryInterface(ole.NewGUID(GUIDiDeviceInformation2))
	defer itf.Release()
	v := (*iDeviceInformation2)(unsafe.Pointer(itf))
	var out *DeviceInformationPairing
	hr, _, _ := syscall.SyscallN(v.VTable().GetPairing, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(&out)))
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return out, nil
}
