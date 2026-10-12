package main

// #253 (v11.2, lot V2-4) — expiration de sécurité du verrou LAUNCH_LOCK : 45 s sans renouvellement lèvent le verrou
// (popup oubliée, onglet gelé, poste coupé), renouvellement toutes les 15 s = verrou maintenu indéfiniment.
// Contrat : contracts/websocket-actions.md §LAUNCH_LOCK « Cycle de vie », contracts/logging.md §3.2 sexies.
//
// Les durées contractuelles (45 s / 15 s) sont ramenées à quelques centaines de millisecondes par la variable exportée
// game.LaunchLockTTL (point d'injection du moteur) : le test n'attend jamais 45 s. Le rapport 45 s/15 s est conservé
// (renouvellement tous les TTL/3). La valeur contractuelle de 45 s elle-même est vérifiée dans launch_lock_253_test.go
// (EXPIRES_AT − SINCE = 45000).

import (
	"buzzcontrol/internal/game"
	"testing"
	"time"
)

// shortTTL253 règle le TTL AVANT toute activité du verrou et le restaure en fin de test (après le dernier timer).
func shortTTL253(t *testing.T, ttl time.Duration) {
	t.Helper()
	prev := game.LaunchLockTTL
	game.LaunchLockTTL = ttl
	t.Cleanup(func() {
		time.Sleep(ttl + 100*time.Millisecond) // aucun timer en vol ne lit la valeur restaurée
		game.LaunchLockTTL = prev
	})
}

func TestLaunchLockExpiry253_Critical_NoRenewalReleasesTheLockAndLogsAWarning(t *testing.T) {
	shortTTL253(t, 400*time.Millisecond)
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	if !e.waitLock(func(l lockState253) bool { return l.Active }) {
		t.Fatal("précondition : verrou posé")
	}
	// autres postes bloqués pendant la fenêtre de validité
	e.send(t, e.admin2, "admin", "TEAM_POINTS", map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 2})
	if _, ok := readRefused253(e.a2, "TEAM_POINTS"); !ok {
		t.Fatal("précondition : refus sous verrou")
	}

	time.Sleep(900 * time.Millisecond) // > TTL, sans aucun renouvellement

	l := e.lock()
	if l.Active || l.Owner != "" || l.Since != 0 || l.ExpiresAt != 0 {
		t.Errorf("sans renouvellement le verrou est levé et remis à zéro : %+v", l)
	}
	if !e.hasLog(game.LogLevelWarn, "LAUNCH_LOCK_RELEASED", "expired", "held_ms") {
		t.Errorf("WARN LAUNCH_LOCK_RELEASED reason=expired held_ms=… ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	// la partie reprend : plus aucun refus
	before := e.teamScore("Les Bleus")
	collectRaw253(e.a2, 200*time.Millisecond)
	e.send(t, e.admin2, "admin", "TEAM_POINTS", map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 2})
	if n := countAction253(collectRaw253(e.a2, 300*time.Millisecond), "ACTION_REFUSED"); n != 0 {
		t.Error("après l'expiration, les actions de jeu sont de nouveau acceptées")
	}
	if e.teamScore("Les Bleus") != before+2 {
		t.Errorf("le crédit est appliqué après l'expiration : %d → %d", before, e.teamScore("Les Bleus"))
	}
}

func TestLaunchLockExpiry253_ExpiryIsBroadcastToAdminAndAnim(t *testing.T) {
	shortTTL253(t, 300*time.Millisecond)
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	collectRaw253(e.a2, 100*time.Millisecond)
	collectRaw253(e.an, 50*time.Millisecond)

	adminFrames := collectRaw253(e.a2, 900*time.Millisecond)
	animFrames := collectRaw253(e.an, 100*time.Millisecond)
	for name, frames := range map[string][]string{"admin": adminFrames, "anim": animFrames} {
		if !anyContains253(frames, `"ACTIVE":false`) {
			t.Errorf("%s : l'état LAUNCH_LOCK {ACTIVE:false} est rediffusé à l'expiration (le propriétaire découvre l'expiration par l'état reçu)", name)
		}
	}
}

func TestLaunchLockExpiry253_Critical_RenewalEveryThirdOfTheTTLKeepsTheLockWellBeyondTheTTL(t *testing.T) {
	shortTTL253(t, 450*time.Millisecond)
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	first := e.lock()

	// renouvellement toutes les TTL/3 (15 s pour 45 s) pendant plus de 3 × TTL
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)
		e.openLock(t, e.admin1)
		if l := e.lock(); !l.Active || l.Owner != e.admin1 {
			t.Fatalf("verrou perdu malgré le renouvellement : %+v", l)
		}
	}
	last := e.lock()
	if !last.Active || last.Since != first.Since {
		t.Errorf("verrou maintenu, SINCE inchangé : %+v → %+v", first, last)
	}
	if last.ExpiresAt <= first.ExpiresAt {
		t.Errorf("EXPIRES_AT repoussé à chaque renouvellement : %d → %d", first.ExpiresAt, last.ExpiresAt)
	}
	if e.hasLog(game.LogLevelWarn, "LAUNCH_LOCK_RELEASED") {
		t.Errorf("aucune libération pendant les renouvellements ; logs=%v", e.logLines(game.LogLevelWarn))
	}

	// puis le poste se fige : l'expiration reprend
	time.Sleep(900 * time.Millisecond)
	if e.lock().Active {
		t.Error("arrêt des renouvellements → expiration")
	}
}

func TestLaunchLockExpiry253_AnExpiredLockNeverBlocksANewOpenBySomeoneElse(t *testing.T) {
	shortTTL253(t, 300*time.Millisecond)
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	time.Sleep(700 * time.Millisecond)

	e.openLock(t, e.admin2)

	if !e.waitLock(func(l lockState253) bool { return l.Active && l.Owner == e.admin2 }) {
		t.Fatalf("un verrou expiré ne bloque pas le 2ᵉ admin : %+v", e.lock())
	}
	if n := countAction253(collectRaw253(e.a2, 300*time.Millisecond), "LAUNCH_LOCK_REFUSED"); n != 0 {
		t.Error("aucun LAUNCH_LOCK_REFUSED après expiration")
	}
}

func TestLaunchLockExpiry253_NewGameOfTheOwnerBeforeTheTTLCancelsTheExpiryTimer(t *testing.T) {
	shortTTL253(t, 400*time.Millisecond)
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	e.send(t, e.admin1, "admin", "NEW_GAME", map[string]interface{}{})
	e.waitLock(func(l lockState253) bool { return !l.Active })
	e.openLock(t, e.admin2) // nouveau verrou, d'un autre poste
	e.waitLock(func(l lockState253) bool { return l.Active && l.Owner == e.admin2 })

	time.Sleep(250 * time.Millisecond) // l'ancien minuteur (400 ms après le 1er OPEN) serait échu ici
	e.openLock(t, e.admin2)
	time.Sleep(250 * time.Millisecond)

	if l := e.lock(); !l.Active || l.Owner != e.admin2 {
		t.Errorf("l'ancien minuteur d'expiration ne doit pas lever le verrou du nouveau propriétaire : %+v", l)
	}
	if e.countLog(game.LogLevelWarn, "expired") != 0 {
		t.Errorf("aucune expiration parasite ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}
