package game

import "testing"

func TestRafaleAlphaKey282_Table(t *testing.T) {
	tests := []struct {
		in, letter, key string
	}{
		{"Canberra", "C", "CANBERRA"},
		{"canberra", "C", "CANBERRA"},
		{"  Paris ", "P", "PARIS"},
		{"Élan", "E", "ELAN"},
		{"écureuil", "E", "ECUREUIL"},
		{"Çà", "C", "CA"},
		{"Ñandú", "N", "NANDU"},
		{"Œuf", "O", "OEUF"},
		{"Æsir", "A", "AESIR"},
		{"Straße", "S", "STRASSE"},
		{"Øre", "O", "ORE"},
		{"La Seine", "S", "SEINE"},
		{"L'Égypte", "E", "EGYPTE"},
		{"L’Égypte", "E", "EGYPTE"},
		{"Les Misérables", "M", "MISERABLES"},
		{"Un chien", "C", "CHIEN"},
		{"Une pomme", "P", "POMME"},
		{"Des pommes", "P", "POMMES"},
		{"Du pain", "P", "PAIN"},
		{"De la farine", "F", "FARINE"},
		{"De l'eau", "E", "EAU"},
		{"Lapin", "L", "LAPIN"},
		{"Lune", "L", "LUNE"},
		{"Unique", "U", "UNIQUE"},
		{"Le", "L", "LE"},
		{"\"Hello\"", "H", "HELLO\""},
		{"(Test)", "T", "TEST)"},
		{"- tiret", "T", "TIRET"},
		{"42", "#", "42"},
		{"7 Wonders", "#", "7 WONDERS"},
		{"", "#", ""},
		{"   ", "#", ""},
		{"Ωmega", "#", "ΩMEGA"},
		{"Zèbre", "Z", "ZEBRE"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			l, k := rafaleAlphaKey(tt.in)
			if l != tt.letter || k != tt.key {
				t.Errorf("rafaleAlphaKey(%q) = (%q,%q), want (%q,%q)", tt.in, l, k, tt.letter, tt.key)
			}
		})
	}
}

func TestRafaleAlphaKey282_ExportedIsSameFunction(t *testing.T) {
	l1, k1 := RafaleAlphaKey("L'Île")
	l2, k2 := rafaleAlphaKey("L'Île")
	if l1 != l2 || k1 != k2 || l1 != "I" {
		t.Fatalf("exported wrapper diverges: (%q,%q) vs (%q,%q)", l1, k1, l2, k2)
	}
}

func TestRafaleWheelLayout282_Valid(t *testing.T) {
	for s, want := range map[string]bool{"strip": true, "arc": true, "": false, "ARC": false, "circle": false} {
		if got := ValidRafaleWheelLayout(s); got != want {
			t.Errorf("ValidRafaleWheelLayout(%q)=%v want %v", s, got, want)
		}
	}
}
