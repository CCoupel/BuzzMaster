package main

// Human-readable names for the Windows.Devices.Enumeration pairing enums —
// kept OS-independent (ints) so they are unit-testable on Linux.

import "fmt"

var devicePairingResultStatusNames = map[int]string{
	0:  "Paired",
	1:  "NotReadyToPair",
	2:  "NotPaired",
	3:  "AlreadyPaired",
	4:  "ConnectionRejected",
	5:  "TooManyConnections",
	6:  "HardwareFailure",
	7:  "AuthenticationTimeout",
	8:  "AuthenticationNotAllowed",
	9:  "AuthenticationFailure",
	10: "NoSupportedProfiles",
	11: "ProtectionLevelCouldNotBeMet",
	12: "AccessDenied",
	13: "InvalidCeremonyData",
	14: "PairingCanceled",
	15: "OperationAlreadyInProgress",
	16: "RequiredHandlerNotRegistered",
	17: "RejectedByHandler",
	18: "RemoteDeviceHasAssociation",
	19: "Failed",
}

var deviceUnpairingResultStatusNames = map[int]string{
	0: "Unpaired",
	1: "AlreadyUnpaired",
	2: "OperationAlreadyInProgress",
	3: "AccessDenied",
	4: "Failed",
}

var devicePairingProtectionLevelNames = map[int]string{
	0: "Default",
	1: "None",
	2: "Encryption",
	3: "EncryptionAndAuthentication",
}

var devicePairingKindsNames = map[int]string{
	0:  "None",
	1:  "ConfirmOnly",
	2:  "DisplayPin",
	4:  "ProvidePin",
	8:  "ConfirmPinMatch",
	16: "ProvidePasswordCredential",
}

func nameOf(table map[int]string, v int) string {
	if n, ok := table[v]; ok {
		return fmt.Sprintf("%s(%d)", n, v)
	}
	return fmt.Sprintf("Unknown(%d)", v)
}

// pairLevelFromMode maps the -pair-level flag to the WinRT enum value.
func pairLevelFromMode(mode string) (int, bool) {
	switch mode {
	case "auth":
		return 3, true
	case "encrypt":
		return 2, true
	case "default":
		return 0, true
	}
	return 0, false
}

// pairingSucceeded tells whether a DevicePairingResultStatus means the
// device is now paired (Paired or AlreadyPaired).
func pairingSucceeded(status int) bool { return status == 0 || status == 3 }

// pairingLevelNotMet tells whether the requested protection level could not
// be met — the one status where bleak lowers the level and retries.
func pairingLevelNotMet(status int) bool { return status == 11 }
