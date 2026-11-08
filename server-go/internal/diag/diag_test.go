package diag

import "testing"

// #291 Phase B : tests unitaires du paquet extrait. Les scénarios journalisés de bout en bout
// (ardoise_instrumentation_246_test.go) passent par l'App et restent dans cmd/server.

func TestLogToken_BareWhenHarmlessQuotedOtherwise(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                   `""`,
		"abc_DEF-1.2:3":      "abc_DEF-1.2:3",
		"a b":                `"a b"`,
		"x\ny":               `"x\ny"`,
		`quo"te`:             `"quo\"te"`,
		"forged level=ERROR": `"forged level=ERROR"`,
	}
	for in, want := range cases {
		if got := LogToken(in); got != want {
			t.Errorf("LogToken(%q) = %s, attendu %s", in, got, want)
		}
	}
}

func TestLogValue_ScalarsThenQuotedJSON(t *testing.T) {
	t.Parallel()
	if got := LogValue("ok"); got != "ok" {
		t.Errorf("string : %s", got)
	}
	if got := LogValue(true); got != "true" {
		t.Errorf("bool : %s", got)
	}
	if got := LogValue(float64(3)); got != "3" {
		t.Errorf("float64 : %s", got)
	}
	if got := LogValue(nil); got != "<nil>" {
		t.Errorf("nil : %s", got)
	}
	if got := LogValue(map[string]int{"a": 1}); got != `"{\"a\":1}"` {
		t.Errorf("objet re-sérialisé et quoté : %s", got)
	}
}

func TestCheckArdoiseSeq_StaleIgnoredGapAppliedPerConnectionAndQuestion(t *testing.T) {
	t.Parallel()
	sq := map[string]*ArdoiseSeqState{}
	if !CheckArdoiseSeq(sq, "c1", "b1", "q1", 1) {
		t.Fatal("SEQ 1 est appliqué")
	}
	if CheckArdoiseSeq(sq, "c1", "b1", "q1", 1) {
		t.Error("SEQ répété = périmé, ignoré")
	}
	if !CheckArdoiseSeq(sq, "c1", "b1", "q1", 5) {
		t.Error("un trou de SEQ est seulement journalisé, le message est appliqué")
	}
	if CheckArdoiseSeq(sq, "c1", "b1", "q1", 3) {
		t.Error("SEQ inférieur au dernier appliqué = périmé")
	}
	if !CheckArdoiseSeq(sq, "c1", "b1", "q2", 1) {
		t.Error("nouvelle question : l'état de séquence repart de zéro")
	}
	if !CheckArdoiseSeq(sq, "c2", "b2", "q1", 1) {
		t.Error("autre connexion : état indépendant")
	}
}
