package game

import "testing"

// ---------------------------------------------------------------------------
// C4 (delta #119, plan _work/reports/plan-entracte-119-fixes-20260820-155123.md)
// — "le point le plus critique" (handoff test-writer) : la configuration
// enregistrée (EntracteConfigSaved) peut être modifiée à tout moment, y
// compris pendant une pause active, mais le panneau DÉJÀ AFFICHÉ
// (EntracteConfig, le diffusé) ne doit PAS bouger tant que la pause dure —
// les nouvelles valeurs ne prennent effet qu'au PROCHAIN déclenchement.
//
// Règle exacte (C4) : « UPDATE_ENTRACTE_CONFIG écrit TOUJOURS la
// configuration enregistrée ; elle ne rafraîchit la configuration diffusée
// au panneau que si aucun entracte n'est actif. SetEntracte(true) recopie
// l'enregistré vers le diffusé AVANT de lever le drapeau, sous le même
// verrou — pour qu'aucun client ne puisse jamais observer ENTRACTE:true
// avec une configuration encore périmée. »
//
// Complémentaire de internal/game/entracte_test.go (dev-backend, B8/D4) —
// ce fichier ne re-teste PAS la garde de phase, uniquement le mécanisme de
// gel/dégel C4, à travers SetEntracteConfig/SetEntracte/GetState (aucune
// dépendance au dispatch WS — voir cmd/server/entracte_c4_dispatch_test.go
// pour la couverture bout-en-bout à travers UPDATE_ENTRACTE_CONFIG).
// ---------------------------------------------------------------------------

func sampleEntracteConfig(title string) EntracteConfig {
	return EntracteConfig{
		Title: title, Subtitle: "Retour bientôt",
		PanelSize: 70, AnimPeriod: 8, AnimIntensity: 30, TransitionMs: 1500,
	}
}

// TestEntracteConfig_SaveOutsideEntracte_RefreshesDiffusedImmediately covers
// the "hors entracte, diffusé et enregistré restent identiques" acceptance
// criterion — the common case, no freeze involved.
func TestEntracteConfig_SaveOutsideEntracte_RefreshesDiffusedImmediately(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStopped)

	cfg := sampleEntracteConfig("Nouveau titre")
	e.SetEntracteConfig(cfg)

	state := e.GetState()
	if state.EntracteConfigSaved != cfg {
		t.Fatalf("EntracteConfigSaved not updated: got %+v, want %+v", state.EntracteConfigSaved, cfg)
	}
	if state.EntracteConfig != cfg {
		t.Errorf("EntracteConfig (diffused) should refresh immediately when no entracte is active: got %+v, want %+v", state.EntracteConfig, cfg)
	}
}

// TestEntracteConfig_SaveDuringActiveEntracte_FreezesDiffused is THE central
// C4 assertion: saving while a pause is live must update ONLY the saved
// config, leaving the panel already on screen untouched.
func TestEntracteConfig_SaveDuringActiveEntracte_FreezesDiffused(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStopped)

	original := sampleEntracteConfig("Titre original")
	e.SetEntracteConfig(original)

	if !e.SetEntracte(true) {
		t.Fatal("setup: expected entracte activation to succeed from STOPPED")
	}
	beforeSave := e.GetState()
	if beforeSave.EntracteConfig != original {
		t.Fatalf("setup: diffused config not as expected before the mid-pause save: got %+v", beforeSave.EntracteConfig)
	}

	// Modification PENDANT la pause — doit être acceptée (persistée) sans
	// toucher au panneau déjà affiché.
	updated := sampleEntracteConfig("Titre modifié pendant la pause")
	e.SetEntracteConfig(updated)

	afterSave := e.GetState()
	if afterSave.EntracteConfigSaved != updated {
		t.Errorf("EntracteConfigSaved must reflect the mid-pause edit: got %+v, want %+v", afterSave.EntracteConfigSaved, updated)
	}
	if afterSave.EntracteConfig != original {
		t.Errorf("EntracteConfig (diffused, what the panel shows) must NOT change while entracte is active: got %+v, want unchanged %+v", afterSave.EntracteConfig, original)
	}
	if !e.IsEntracte() {
		t.Error("entracte must still be active — SetEntracteConfig must not itself end the pause")
	}
}

// TestEntracteConfig_MultipleSavesWhileFrozen_LatestSavedWinsOnReactivation
// verifies several saves during the SAME pause all land in Saved (only the
// last one matters), and that value — not any intermediate one — is what
// appears at the next activation.
func TestEntracteConfig_MultipleSavesWhileFrozen_LatestSavedWinsOnReactivation(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStopped)
	e.SetEntracteConfig(sampleEntracteConfig("Départ"))
	if !e.SetEntracte(true) {
		t.Fatal("setup: activation should succeed")
	}

	e.SetEntracteConfig(sampleEntracteConfig("Premier brouillon"))
	e.SetEntracteConfig(sampleEntracteConfig("Second brouillon"))
	final := sampleEntracteConfig("Version finale")
	e.SetEntracteConfig(final)

	if got := e.GetState().EntracteConfig; got.Title != "Départ" {
		t.Fatalf("diffused config drifted mid-pause: got title %q, want unchanged %q", got.Title, "Départ")
	}

	// Sortie puis relance — la valeur diffusée doit maintenant être la
	// DERNIÈRE sauvegarde, pas une des versions intermédiaires.
	if !e.SetEntracte(false) {
		t.Fatal("deactivation should always succeed")
	}
	if !e.SetEntracte(true) {
		t.Fatal("re-activation from STOPPED should succeed")
	}

	got := e.GetState().EntracteConfig
	if got != final {
		t.Errorf("re-activation must copy the LATEST saved config: got %+v, want %+v", got, final)
	}
}

// TestSetEntracte_Activation_NoWindowWithFlagTrueAndStaleConfig is the
// "aucune fenêtre où un client verrait ENTRACTE:true + ancienne config"
// acceptance criterion. GetState() takes the same RLock SetEntracte's
// critical section holds under Lock(), so the copy-then-flag sequence is
// observed as one atomic unit by any concurrent reader — this test asserts
// the OUTCOME (both fields consistent immediately after SetEntracte(true)
// returns), which is what that atomicity guarantees external observers.
func TestSetEntracte_Activation_NoWindowWithFlagTrueAndStaleConfig(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStopped)

	saved := sampleEntracteConfig("Config à jour")
	e.SetEntracteConfig(saved)

	if !e.SetEntracte(true) {
		t.Fatal("expected activation to succeed")
	}

	state := e.GetState()
	if !state.Entracte {
		t.Fatal("expected Entracte flag true immediately after a successful SetEntracte(true)")
	}
	if state.EntracteConfig != saved {
		t.Errorf("the moment the flag is observably true, EntracteConfig must already equal EntracteConfigSaved — got %+v, want %+v", state.EntracteConfig, saved)
	}
}

// TestEntracteConfig_ExitThenReenter_ShowsNewlySavedValues is the plan's own
// acceptance-criteria wording, almost verbatim: "Sortir puis relancer un
// entracte affiche les nouvelles valeurs."
func TestEntracteConfig_ExitThenReenter_ShowsNewlySavedValues(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStopped)
	e.SetEntracteConfig(sampleEntracteConfig("Avant"))
	e.SetEntracte(true)
	e.SetEntracteConfig(sampleEntracteConfig("Édité pendant la pause"))
	e.SetEntracte(false)

	updated := sampleEntracteConfig("Nouvelle configuration")
	e.SetEntracteConfig(updated)

	if !e.SetEntracte(true) {
		t.Fatal("expected re-activation to succeed from STOPPED")
	}
	if got := e.GetState().EntracteConfig; got != updated {
		t.Errorf("relaunching entracte must show the latest saved config: got %+v, want %+v", got, updated)
	}
}
