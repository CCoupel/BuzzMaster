package main

import "testing"

func TestPairingNames(t *testing.T) {
	if nameOf(devicePairingResultStatusNames, 0) != "Paired(0)" || nameOf(devicePairingResultStatusNames, 11) != "ProtectionLevelCouldNotBeMet(11)" {
		t.Error("pairing status names wrong")
	}
	if nameOf(devicePairingResultStatusNames, 99) != "Unknown(99)" {
		t.Error("unknown status must be labelled Unknown")
	}
	if nameOf(deviceUnpairingResultStatusNames, 1) != "AlreadyUnpaired(1)" || nameOf(devicePairingProtectionLevelNames, 3) != "EncryptionAndAuthentication(3)" || nameOf(devicePairingKindsNames, 1) != "ConfirmOnly(1)" {
		t.Error("enum names wrong")
	}
}

func TestPairLevelFromMode(t *testing.T) {
	for mode, want := range map[string]int{"auth": 3, "encrypt": 2, "default": 0} {
		got, ok := pairLevelFromMode(mode)
		if !ok || got != want {
			t.Errorf("pairLevelFromMode(%q) = %d,%v want %d", mode, got, ok, want)
		}
	}
	if _, ok := pairLevelFromMode("plain"); ok {
		t.Error("plain is not a pairing level")
	}
}

func TestPairingStatusPredicates(t *testing.T) {
	if !pairingSucceeded(0) || !pairingSucceeded(3) || pairingSucceeded(19) || pairingSucceeded(11) {
		t.Error("pairingSucceeded wrong")
	}
	if !pairingLevelNotMet(11) || pairingLevelNotMet(0) {
		t.Error("pairingLevelNotMet wrong")
	}
}
