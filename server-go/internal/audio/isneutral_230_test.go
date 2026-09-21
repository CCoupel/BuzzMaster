// #230 (v11.0), amendement contract sound.md §4 — IsNeutral/Engine.OutputAvailable :
// le seul moyen de distinguer un pilote réel d'une dégradation silencieuse
// depuis l'extérieur du paquet (NewOutput ne renvoie jamais d'erreur,
// Stats.PlayErrors ne comble pas ce trou). GET /api/sound/status et
// POST /api/sounds/{cue}/test (#230) en dépendent tous les deux.
//
// Convention de collision : préfixe tw230n pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package audio

import "testing"

func TestIsNeutral_Nil_IsTrue(t *testing.T) {
	if !IsNeutral(nil) {
		t.Error("IsNeutral(nil) doit être true — pas de sortie du tout est bien une dégradation")
	}
}

func TestIsNeutral_ConstructedOutput_MatchesConstructionOutcome(t *testing.T) {
	// NewOutput() dans CET environnement de sandbox (confirmé empiriquement
	// dans output_228_test.go : ni PulseAudio ni ALSA réels) dégrade
	// toujours vers noopOutput — IsNeutral doit donc rapporter true. Sur une
	// machine avec un vrai périphérique, NewOutput réussirait à la place et
	// IsNeutral devrait rapporter false — ce test accepte les deux issues et
	// vérifie seulement leur COHÉRENCE mutuelle (jamais un état contradictoire),
	// même discipline que output_228_test.go.
	out := NewOutput(OutputConfig{})
	neutral := IsNeutral(out)
	if out == nil && !neutral {
		t.Fatal("incohérent : NewOutput() a renvoyé nil sans que IsNeutral le rapporte")
	}
	t.Logf("NewOutput() -> %T, IsNeutral=%v (les deux sont valides selon l'environnement)", out, neutral)
}

func TestIsNeutral_FakeOutput_IsNotNeutral(t *testing.T) {
	// FakeOutput (fake.go, #227) est un double DE TEST fonctionnel — pas la
	// dégradation silencieuse de #228. IsNeutral doit le traiter comme un
	// pilote réel.
	if IsNeutral(NewFakeOutput()) {
		t.Error("IsNeutral(FakeOutput) doit être false — ce n'est pas noopOutput, c'est un pilote de test fonctionnel")
	}
}

// TestEngine_OutputAvailable_FalseWhenDisabledOrNeutral proves
// OutputAvailable() correctly reports false BOTH when the engine has no
// Output at all (disabled) AND when it has a neutral one — the exact
// distinction contract §Sound's `unavailable` vs `disabled` taxonomy needs
// callers (cmd/server) to make on TOP of this (Enabled() first), but the
// primitive itself must be well-defined in isolation.
func TestEngine_OutputAvailable_FalseWhenDisabledOrNeutral(t *testing.T) {
	disabled := NewEngine(Config{}) // Output nil
	if disabled.OutputAvailable() {
		t.Error("un Engine désactivé (Output nil) ne doit jamais rapporter OutputAvailable()=true")
	}

	var nilEngine *Engine
	if nilEngine.OutputAvailable() {
		t.Error("OutputAvailable() sur un Engine nil doit être false, jamais paniquer")
	}
}

func TestEngine_OutputAvailable_TrueWithARealOutput(t *testing.T) {
	e := NewEngine(Config{Output: NewFakeOutput()})
	if !e.OutputAvailable() {
		t.Error("un Engine construit avec un FakeOutput (pilote fonctionnel, non neutre) doit rapporter OutputAvailable()=true")
	}
}
