package game

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizePictoWord(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
		wantErr        bool
	}{
		{"simple", "Chat", "Chat", false},
		{"trim_and_compact", "  Pont   levis ", "Pont levis", false},
		{"accents_apostrophe_hyphen", "L'été d’Anne-Marie 2", "L'été d'Anne-Marie 2", false},
		{"empty", "   ", "", true},
		{"tab_refused", "a\tb", "", true},
		{"punctuation_refused", "chat!", "", true},
		{"slash_refused", "a/b", "", true},
		{"40_runes_ok", strings.Repeat("é", 40), strings.Repeat("é", 40), false},
		{"41_runes_refused", strings.Repeat("é", 41), "", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizePictoWord(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrPictoWord) {
				t.Errorf("err does not wrap ErrPictoWord: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPictoWordKey_FoldsCaseAndAccents(t *testing.T) {
	t.Parallel()
	if PictoWordKey("Éléphant") != PictoWordKey("elephant") {
		t.Errorf("Éléphant / elephant must share one key: %q vs %q", PictoWordKey("Éléphant"), PictoWordKey("elephant"))
	}
	// Articles are kept (unlike rafaleAlphaKey): "le chat" ≠ "chat".
	if PictoWordKey("Le chat") == PictoWordKey("chat") {
		t.Error("articles must be kept in the PICTO key")
	}
}

func TestValidatePictoWords(t *testing.T) {
	t.Parallel()
	many := make([]string, 21)
	for i := range many {
		many[i] = "mot" + strings.Repeat("a", i)
	}
	tests := []struct {
		name    string
		in      []string
		wantErr bool
	}{
		{"one", []string{"chat"}, false},
		{"twenty", many[:20], false},
		{"twentyone", many, true},
		{"empty", nil, true},
		{"duplicate_folded", []string{"Éléphant", "ELEPHANT"}, true},
		{"duplicate_spaces", []string{"pont levis", " pont   levis"}, true},
		{"invalid_word", []string{"chat", "a*b"}, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidatePictoWords(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(got) != len(tt.in) {
				t.Errorf("len %d, want %d", len(got), len(tt.in))
			}
		})
	}
	got, _ := ValidatePictoWords([]string{"  Pont   levis "})
	if got[0] != "Pont levis" {
		t.Errorf("not cleaned: %q", got[0])
	}
}

func TestValidPictoTime(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		t  int
		ok bool
	}{{29, false}, {30, true}, {90, true}, {300, true}, {301, false}, {0, false}} {
		if ValidPictoTime(c.t) != c.ok {
			t.Errorf("ValidPictoTime(%d) = %v", c.t, !c.ok)
		}
	}
}

func TestPictoRegistry(t *testing.T) {
	t.Parallel()
	d, ok := TypeDescriptorFor(QuestionTypePicto)
	if !ok {
		t.Fatal("PICTO not registered")
	}
	if len(d.OwnedFields) != 1 || d.OwnedFields[0] != "PICTO_WORDS" || d.NestableInMotionCard || !d.HasPlayerInput || d.MediaSlots != nil || d.DefaultPointsRule != "" {
		t.Errorf("unexpected descriptor: %+v", d)
	}
	if IsNestableInMotionCard(QuestionTypePicto) {
		t.Error("PICTO must not be nestable")
	}
	// An unknown type is NOT registered (no silent SPEEDY fallback).
	if _, ok := TypeDescriptorFor("PICTIONARY"); ok {
		t.Error("unknown type must not resolve")
	}
}

// RAFALE non-regression of the factored folding.
func TestRafaleAlphaKey_AfterFoldKeyRefactor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, letter, key string }{
		{"  Éléphant", "E", "ELEPHANT"},
		{"L'été", "E", "ETE"},
		{"…La maison", "M", "MAISON"},
		{"42", "#", "42"},
	} {
		l, k := rafaleAlphaKey(c.in)
		if l != c.letter || k != c.key {
			t.Errorf("rafaleAlphaKey(%q) = %q,%q want %q,%q", c.in, l, k, c.letter, c.key)
		}
	}
}
