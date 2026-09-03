// Package winenum is a vendored, trimmed projection of the WinRT namespace
// Windows.Devices.Enumeration (pairing classes only), generated with
// saltosystems/winrt-go's `winrt-go-gen` from the WinMD metadata shipped in
// that module, for the #204 spike. Only compiled on Windows.
//
// Generated: DeviceInformation (statics + vtables), DeviceInformationPairing,
// DeviceInformationCustomPairing, DevicePairingRequestedEventArgs,
// DevicePairingResult, DeviceUnpairingResult and the four enums.
// Hand-written (ext.go): DeviceInformation.GetId/GetName/GetPairing and the
// IDevicePairingSettings stub. Removed: AcceptWithPasswordCredential (its
// Windows.Security.Credentials dependency is not generated).
package winenum
