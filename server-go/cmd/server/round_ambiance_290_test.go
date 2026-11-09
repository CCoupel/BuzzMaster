// #290 (v11.6.0, LOT-4) — surcharge d'ambiance par manche : résolution global ← manche ← geste direct câblée dans
// l'application (entrée/sortie de manche, D1′, ROUNDS_UPDATE, AMBIANCE_EFFECTIVE, GET /api/ambiance/settings).
// Source normative : contracts/ambiance-settings.md §2–§5, game-state.md (AMBIANCE_EFFECTIVE, ROUNDS[].AMBIANCE),
// websocket-actions.md (ROUNDS_UPDATE, INVALID_AMBIANCE), http-endpoints.md (GET /api/ambiance/settings,
// `scope` des écritures), lighting.md §12. Écrit depuis le contrat avant le code (rouge attendu).
//
// Frontière : serveur HTTP RÉEL (setupCallbacks + Start) sur un port libre, config posée par JSON, manches par
// ROUNDS_UPDATE (dispatch réel), questions sur disque, READY / NEW_GAME par dispatch, état lu par
// GET /api/ambiance/settings et par la sérialisation JSON de GameState. Aucun symbole du dev n'est référencé
// hors `a.ambianceSettings()` (nom imposé par le contrat §5.2). Préfixe tw290.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/rounds"
	"buzzcontrol/internal/server"
)

type tw290Env struct {
	app  *App
	base string
	dir  string // dossier des questions
}

func tw290New(t *testing.T) *tw290Env {
	t.Helper()
	t.Chdir(t.TempDir()) // POST /config.json écrit config.json dans le répertoire courant
	app := newAnimTestApp(t)
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	dataDir := t.TempDir()
	cfg := saved
	cfg.Storage.DataDir = dataDir
	cfg.Storage.QuestionsDir = filepath.Join(dataDir, "files", "questions")
	cfg.Storage.FilesDir = filepath.Join(dataDir, "files")
	cfg.Sound = config.SoundConfig{}
	config.SetInstance(&cfg)
	app.config = config.Get()
	if err := os.MkdirAll(app.config.Storage.QuestionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0o755); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	app.logsHub = server.NewLogsWebSocketHub(10)
	app.httpServer = server.NewHTTPServer(port, app.engine, app.wsHub, app.buzzerHub, app.logsHub)
	app.ackManager = server.NewAckManager(&app.config.Server)
	app.setupCallbacks()
	go app.wsHub.Run()
	go app.buzzerHub.Run()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := app.httpServer.Start(ctx); err != nil {
		t.Fatalf("démarrage du serveur HTTP : %v", err)
	}
	e := &tw290Env{app: app, base: fmt.Sprintf("http://127.0.0.1:%d", port), dir: app.config.Storage.QuestionsDir}
	eventually264(t, "serveur HTTP prêt", func() bool {
		r, err := http.Get(e.base + "/api/categories")
		if err != nil {
			return false
		}
		_ = r.Body.Close()
		return r.StatusCode == http.StatusOK
	})
	t.Cleanup(func() { app.engine.Stop() })
	return e
}

func (e *tw290Env) writeQ(t *testing.T, id string, order int) {
	t.Helper()
	q := map[string]interface{}{"ID": id, "TYPE": "SPEEDY", "QUESTION": "Question " + id, "ANSWER": "A", "POINTS": "1", "TIME": "30", "CATEGORY": "CULTURE", "ORDER": order}
	d := filepath.Join(e.dir, id)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(q)
	if err := os.WriteFile(filepath.Join(d, "question.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e *tw290Env) post(t *testing.T, path string, body string) (int, map[string]interface{}) {
	t.Helper()
	req, _ := http.NewRequest("POST", e.base+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s : %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m
}

// settings relit GET /api/ambiance/settings.
func (e *tw290Env) settings(t *testing.T) map[string]interface{} {
	t.Helper()
	resp, err := http.Get(e.base + "/api/ambiance/settings")
	if err != nil {
		t.Fatalf("GET /api/ambiance/settings : %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /api/ambiance/settings : %d %s", resp.StatusCode, raw)
	}
	var m map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("réponse non JSON : %v", err)
	}
	return m
}

func tw290Path(m map[string]interface{}, path ...string) interface{} {
	var cur interface{} = m
	for _, p := range path {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = mm[p]
	}
	return cur
}

// roundsUpdate envoie ROUNDS_UPDATE (dispatch réel) : r1 = questions 1-2, r2 = question 3 ; ambR1 = AMBIANCE de r1 (nil = absent).
func (e *tw290Env) roundsUpdate(t *testing.T, rs []map[string]interface{}) {
	t.Helper()
	p := map[string]interface{}{"ROUNDS": rs, "UNASSIGNED": []string{"4"}} // question 4 : toujours « Sans manche »
	dispatchAs(t, e.app, server.ClientTypeAdmin, "ROUNDS_UPDATE", p)
}

func (e *tw290Env) twoRounds(t *testing.T, ambR1 interface{}) {
	t.Helper()
	for i, id := range []string{"1", "2", "3", "4"} {
		e.writeQ(t, id, i)
	}
	r1 := map[string]interface{}{"NAME": "Blind-test", "QUESTIONS": []string{"1", "2"}, "ENTRACTES": false}
	if ambR1 != nil {
		r1["AMBIANCE"] = ambR1
	}
	e.roundsUpdate(t, []map[string]interface{}{r1, {"NAME": "Finale", "QUESTIONS": []string{"3"}, "ENTRACTES": false}})
}

func (e *tw290Env) ready(t *testing.T, id string) {
	t.Helper()
	dispatchAs(t, e.app, server.ClientTypeAdmin, protocol.ActionReady, protocol.ReadyPayload{Question: id})
	eventually264(t, "question "+id+" sélectionnée", func() bool {
		st := e.app.engine.GetState()
		return st.Question != nil && st.Question.ID == id
	})
}

func (e *tw290Env) gameJSON(t *testing.T) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(e.app.engine.GetState())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return m
}

const tw290Blind = `{"sound":{"muted":true,"countdown":{"enabled":false,"loop":false},"cues":{"qcm-indice":false,"rafale-bonne":false}},"light":{"mode":"ON"}}`

func tw290AmbBlind() interface{} {
	var v interface{}
	_ = json.Unmarshal([]byte(tw290Blind), &v)
	return v
}

// ---------------------------------------------------------------------------
// Défauts : sans manche, effectif = global ; FromConfig sans default_mode ⇒ AUTO.
// ---------------------------------------------------------------------------

func TestRoundAmbiance290_Defaults_EffectiveEqualsGlobal_AllSourcesGlobal_LightAuto(t *testing.T) {
	e := tw290New(t)
	m := e.settings(t)
	for _, k := range []string{"global", "effective", "sources", "round_id", "round_override"} {
		if _, ok := m[k]; !ok {
			t.Errorf("clé %q absente de GET /api/ambiance/settings", k)
		}
	}
	if tw290Path(m, "global", "light", "mode") != "AUTO" {
		t.Errorf("config sans lighting.default_mode ⇒ light.mode=AUTO, got %v", tw290Path(m, "global", "light", "mode"))
	}
	if tw290Path(m, "effective", "sound", "muted") != false || tw290Path(m, "effective", "sound", "countdown", "enabled") != true {
		t.Errorf("effectif attendu = défauts (non coupé, décompte actif), got %v", m["effective"])
	}
	if tw290Path(m, "sources", "sound", "muted") != "global" || tw290Path(m, "sources", "light", "mode") != "global" {
		t.Errorf("toutes les sources \"global\" attendues, got %v", m["sources"])
	}
	if m["round_id"] != "" {
		t.Errorf("round_id=\"\" hors manche, got %v", m["round_id"])
	}
	if ro, ok := m["round_override"].(map[string]interface{}); !ok || len(ro) != 0 {
		t.Errorf("round_override={} hors manche, got %v", m["round_override"])
	}
	cues, _ := tw290Path(m, "effective", "sound", "cues").(map[string]interface{})
	if len(cues) != 15 {
		t.Errorf("forme complète : 15 cues attendues, got %d", len(cues))
	}
	for c, v := range cues {
		if v != true {
			t.Errorf("cue %s : active par défaut, got %v", c, v)
		}
	}
}

func TestRoundAmbiance290_GlobalConfig_FeedsGlobalLayer(t *testing.T) {
	e := tw290New(t)
	if err := json.Unmarshal([]byte(`{"sound":{"muted":true,"cues_disabled":{"reveal":true,"rafale-fin-lot":true},"countdown_disabled":true,"countdown_once":true}}`), config.Get()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.Get().Sound = config.SoundConfig{} })
	m := e.settings(t)
	if tw290Path(m, "global", "sound", "muted") != true {
		t.Error("global.sound.muted=true attendu (sound.muted)")
	}
	if tw290Path(m, "global", "sound", "countdown", "enabled") != false || tw290Path(m, "global", "sound", "countdown", "loop") != false {
		t.Errorf("décompte : enabled=false (countdown_disabled) et loop=false (countdown_once) attendus, got %v", tw290Path(m, "global", "sound", "countdown"))
	}
	if tw290Path(m, "global", "sound", "cues", "reveal") != false || tw290Path(m, "global", "sound", "cues", "rafale-fin-lot") != false {
		t.Error("cues_disabled ⇒ cues false dans la forme positive (nouvelles cues comprises)")
	}
	if tw290Path(m, "global", "sound", "cues", "depart") != true {
		t.Error("cue non désactivée ⇒ true")
	}
}

func TestRoundAmbiance290_DefaultMode_PersistedByConfig_ValidatedAndOptional(t *testing.T) {
	e := tw290New(t)
	if code, _ := e.post(t, "/config.json", `{"lighting":{"default_mode":"DIM"}}`); code != http.StatusBadRequest {
		t.Errorf("default_mode hors {ON,AUTO,OFF} : 400 attendu, got %d", code)
	}
	if tw290Path(e.settings(t), "global", "light", "mode") != "AUTO" {
		t.Error("un refus n'écrit rien")
	}
	if code, _ := e.post(t, "/config.json", `{"lighting":{"default_mode":"ON"}}`); code != http.StatusOK {
		t.Fatalf("default_mode ON : 200 attendu, got %d", code)
	}
	m := e.settings(t)
	if tw290Path(m, "global", "light", "mode") != "ON" || tw290Path(m, "effective", "light", "mode") != "ON" {
		t.Errorf("default_mode=ON ⇒ global ET effectif ON, got %v", m)
	}
}

// ---------------------------------------------------------------------------
// ROUNDS_UPDATE : AMBIANCE validée, persistée (rounds.json), partielle, toujours sérialisée.
// ---------------------------------------------------------------------------

func (e *tw290Env) roundsFile(t *testing.T) []map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dir, "rounds.json"))
	if err != nil {
		t.Fatalf("rounds.json illisible : %v", err)
	}
	var f struct {
		Rounds []map[string]interface{} `json:"ROUNDS"`
	}
	_ = json.Unmarshal(b, &f)
	return f.Rounds
}

func TestRoundAmbiance290_RoundsUpdate_StoresAmbianceInRoundsJson_AndAlwaysSerializesInGameState(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, tw290AmbBlind())

	rs := e.roundsFile(t)
	if len(rs) != 2 {
		t.Fatalf("2 manches attendues, got %d", len(rs))
	}
	amb, _ := rs[0]["AMBIANCE"].(map[string]interface{})
	if amb == nil || tw290Path(amb, "sound", "muted") != true || tw290Path(amb, "light", "mode") != "ON" {
		t.Errorf("rounds.json : AMBIANCE de r1 attendue, got %v", rs[0]["AMBIANCE"])
	}

	game := e.gameJSON(t)
	list, _ := game["ROUNDS"].([]interface{})
	if len(list) != 2 {
		t.Fatalf("GameState.ROUNDS : 2 manches attendues, got %v", game["ROUNDS"])
	}
	for i, r := range list {
		rm, _ := r.(map[string]interface{})
		a, present := rm["AMBIANCE"]
		if !present {
			t.Errorf("ROUNDS[%d].AMBIANCE doit TOUJOURS être sérialisé (pas d'omitempty), got %v", i, rm)
			continue
		}
		if _, isMap := a.(map[string]interface{}); !isMap {
			t.Errorf("ROUNDS[%d].AMBIANCE : objet attendu ({} si hérité), got %T", i, a)
		}
	}
	r2, _ := list[1].(map[string]interface{})
	if a, _ := r2["AMBIANCE"].(map[string]interface{}); len(a) != 0 {
		t.Errorf("r2 sans surcharge ⇒ AMBIANCE={}, got %v", a)
	}
}

func TestRoundAmbiance290_RoundsJsonWithoutAmbiance_LoadsAsEmpty_V112Compatible(t *testing.T) {
	e := tw290New(t)
	for i, id := range []string{"1", "2"} {
		e.writeQ(t, id, i)
	}
	legacy := `{"ROUNDS":[{"ID":"r1","NAME":"Ancienne manche"}],"NEXT_ID":2}`
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	rounds.ReloadRounds(e.app)
	game := e.gameJSON(t)
	list, _ := game["ROUNDS"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("la manche v11.2 doit se charger, got %v", game["ROUNDS"])
	}
	rm, _ := list[0].(map[string]interface{})
	if a, present := rm["AMBIANCE"]; !present || len(a.(map[string]interface{})) != 0 {
		t.Errorf("rounds.json sans AMBIANCE ⇒ AMBIANCE={} sérialisée, got %v", rm)
	}
}

func tw290ApplyResult(t *testing.T, e *tw290Env, payload string) (status, reason string) {
	t.Helper()
	var req protocol.RoundsUpdatePayload
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatal(err)
	}
	res, _ := rounds.ApplyUpdate(e.app, req)
	return res.Status, res.Reason
}

func TestRoundAmbiance290_RoundsUpdate_InvalidAmbiance_RefusedAtomically(t *testing.T) {
	cases := map[string]string{
		"light.mode inconnu":   `{"light":{"mode":"DIM"}}`,
		"cue inconnue (faute)": `{"sound":{"cues":{"qcm-indce":false}}}`,
		"clé inconnue":         `{"sound":{"mutd":true}}`,
		"type non booléen":     `{"sound":{"muted":"oui"}}`,
		"racine inconnue":      `{"lumiere":{"mode":"ON"}}`,
	}
	for name, amb := range cases {
		t.Run(name, func(t *testing.T) {
			e := tw290New(t)
			for i, id := range []string{"1", "2"} {
				e.writeQ(t, id, i)
			}
			payload := `{"ROUNDS":[{"NAME":"R","QUESTIONS":["1","2"],"ENTRACTES":false,"AMBIANCE":` + amb + `}],"UNASSIGNED":[]}`
			status, reason := tw290ApplyResult(t, e, payload)
			if status != "REFUSED" || reason != "INVALID_AMBIANCE" {
				t.Fatalf("attendu REFUSED/INVALID_AMBIANCE, got %s/%s", status, reason)
			}
			if _, err := os.Stat(filepath.Join(e.dir, "rounds.json")); err == nil {
				t.Error("un refus ne doit rien écrire (rounds.json créé)")
			}
		})
	}
}

func TestRoundAmbiance290_RoundsUpdate_AmbianceAbsent_Keeps_Empty_Clears(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, tw290AmbBlind())

	// 1. AMBIANCE absente : la surcharge existante est CONSERVÉE
	e.roundsUpdate(t, []map[string]interface{}{
		{"ID": "r1", "NAME": "Blind-test renommée", "QUESTIONS": []string{"1", "2"}},
		{"ID": "r2", "NAME": "Finale", "QUESTIONS": []string{"3"}},
	})
	amb, _ := e.roundsFile(t)[0]["AMBIANCE"].(map[string]interface{})
	if tw290Path(amb, "sound", "muted") != true {
		t.Errorf("AMBIANCE absente ⇒ surcharge conservée, got %v", e.roundsFile(t)[0])
	}
	if e.roundsFile(t)[0]["NAME"] != "Blind-test renommée" {
		t.Error("sanity : le renommage devait être appliqué")
	}

	// 2. {} explicite : EFFACE la surcharge
	e.roundsUpdate(t, []map[string]interface{}{
		{"ID": "r1", "NAME": "Blind-test renommée", "QUESTIONS": []string{"1", "2"}, "AMBIANCE": map[string]interface{}{}},
		{"ID": "r2", "NAME": "Finale", "QUESTIONS": []string{"3"}},
	})
	if a, _ := e.roundsFile(t)[0]["AMBIANCE"].(map[string]interface{}); len(a) != 0 {
		t.Errorf("{} explicite ⇒ surcharge effacée, got %v", e.roundsFile(t)[0]["AMBIANCE"])
	}
}

func TestRoundAmbiance290_RoundsUpdate_NewRoundWithoutAmbiance_IsEmpty(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	for _, r := range e.roundsFile(t) {
		if a, _ := r["AMBIANCE"].(map[string]interface{}); len(a) != 0 {
			t.Errorf("nouvelle manche sans AMBIANCE ⇒ {} (tout hérité), got %v", r["AMBIANCE"])
		}
	}
}

// ---------------------------------------------------------------------------
// Entrée / sortie de manche (§4.1) : couche manche recalculée, effets appliqués.
// ---------------------------------------------------------------------------

func TestRoundAmbiance290_EnteringRound_AppliesEveryOverriddenField(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, tw290AmbBlind())
	e.ready(t, "1") // question de r1

	m := e.settings(t)
	if m["round_id"] != "r1" {
		t.Fatalf("round_id=\"r1\" attendu, got %v", m["round_id"])
	}
	checks := map[string]interface{}{
		"sound.muted": true, "sound.countdown.enabled": false, "sound.countdown.loop": false,
		"sound.cues.qcm-indice": false, "sound.cues.rafale-bonne": false, "light.mode": "ON",
	}
	for path, want := range checks {
		var parts []string
		cur := ""
		for _, c := range path + "." {
			if c == '.' {
				parts = append(parts, cur)
				cur = ""
			} else {
				cur += string(c)
			}
		}
		// « sound.cues.qcm-indice » : le nom de cue contient un tiret mais pas de point
		if got := tw290Path(m, append([]string{"effective"}, parts...)...); got != want {
			t.Errorf("effectif %s : attendu %v, got %v", path, want, got)
		}
		if src := tw290Path(m, append([]string{"sources"}, parts...)...); src != "round" {
			t.Errorf("source de %s : attendu \"round\", got %v", path, src)
		}
	}
	// les autres cues restent héritées du global
	if tw290Path(m, "effective", "sound", "cues", "depart") != true || tw290Path(m, "sources", "sound", "cues", "depart") != "global" {
		t.Error("une cue non surchargée reste héritée (true, \"global\")")
	}
	if ro, _ := m["round_override"].(map[string]interface{}); tw290Path(ro, "sound", "muted") != true {
		t.Errorf("round_override doit refléter l'AMBIANCE de la manche, got %v", m["round_override"])
	}
}

func TestRoundAmbiance290_AmbianceEffective_AlwaysSerialized_InGameState(t *testing.T) {
	e := tw290New(t)
	for _, step := range []string{"initial", "round", "sans manche"} {
		switch step {
		case "round":
			e.twoRounds(t, tw290AmbBlind())
			e.ready(t, "1")
		case "sans manche":
			e.ready(t, "4")
		}
		ae, present := e.gameJSON(t)["AMBIANCE_EFFECTIVE"]
		if !present {
			t.Fatalf("[%s] AMBIANCE_EFFECTIVE doit TOUJOURS être sérialisé (pas d'omitempty)", step)
		}
		m, _ := ae.(map[string]interface{})
		for _, k := range []string{"SETTINGS", "SOURCES", "ROUND_ID"} {
			if _, ok := m[k]; !ok {
				t.Errorf("[%s] AMBIANCE_EFFECTIVE.%s absent : %v", step, k, m)
			}
		}
		if step == "round" && m["ROUND_ID"] != "r1" {
			t.Errorf("ROUND_ID=\"r1\" attendu en manche, got %v", m["ROUND_ID"])
		}
		if step != "round" && m["ROUND_ID"] != "" {
			t.Errorf("[%s] ROUND_ID=\"\" attendu hors manche, got %v", step, m["ROUND_ID"])
		}
	}
}

func TestRoundAmbiance290_ReadyOutsideRound_BackToGlobal(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, tw290AmbBlind())
	e.ready(t, "1")
	if tw290Path(e.settings(t), "effective", "sound", "muted") != true {
		t.Fatal("setup invalide : muted imposé par la manche")
	}
	e.ready(t, "4") // « Sans manche »
	m := e.settings(t)
	if tw290Path(m, "effective", "sound", "muted") != false || tw290Path(m, "sources", "sound", "muted") != "global" || m["round_id"] != "" {
		t.Errorf("hors manche : retour au global, got effective=%v sources=%v round=%v", m["effective"], m["sources"], m["round_id"])
	}
}

func TestRoundAmbiance290_NewGame_ClearsRoundLayer(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, tw290AmbBlind())
	e.ready(t, "1")
	if tw290Path(e.settings(t), "effective", "light", "mode") != "ON" {
		t.Fatal("setup invalide")
	}
	dispatchAs(t, e.app, server.ClientTypeAdmin, protocol.ActionNewGame, map[string]interface{}{})
	eventually264(t, "NEW_GAME appliqué", func() bool { return e.settings(t)["round_id"] == "" })
	m := e.settings(t)
	if tw290Path(m, "effective", "light", "mode") != "AUTO" || tw290Path(m, "sources", "light", "mode") != "global" {
		t.Errorf("NEW_GAME : manche effective ← aucune, retour au global, got %v", m)
	}
}

func TestRoundAmbiance290_SameRound_NextQuestion_KeepsLayers(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, tw290AmbBlind())
	e.ready(t, "1")
	if code, _ := e.post(t, "/config.json", `{"sound":{"muted":false}}`); code != http.StatusOK {
		t.Fatalf("geste direct : %d", code)
	}
	e.ready(t, "2") // même manche r1 : la couche directe n'est PAS vidée (§4.1)
	m := e.settings(t)
	if tw290Path(m, "effective", "sound", "muted") != false || tw290Path(m, "sources", "sound", "muted") != "direct" {
		t.Errorf("question suivante de la MÊME manche : le geste direct persiste, got %v / %v", m["effective"], m["sources"])
	}
}

func TestRoundAmbiance290_UpdateDuringRound_AppliesImmediately(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.ready(t, "1")
	if tw290Path(e.settings(t), "effective", "light", "mode") != "AUTO" {
		t.Fatal("setup invalide")
	}
	e.roundsUpdate(t, []map[string]interface{}{
		{"ID": "r1", "NAME": "Blind-test", "QUESTIONS": []string{"1", "2"}, "AMBIANCE": map[string]interface{}{"light": map[string]interface{}{"mode": "OFF"}}},
		{"ID": "r2", "NAME": "Finale", "QUESTIONS": []string{"3"}},
	})
	eventually264(t, "surcharge appliquée sans attendre la manche suivante", func() bool {
		return tw290Path(e.settings(t), "effective", "light", "mode") == "OFF"
	})
	if tw290Path(e.settings(t), "sources", "light", "mode") != "round" {
		t.Error("source \"round\" attendue")
	}
}

// ---------------------------------------------------------------------------
// D1′ — précédence des gestes directs (§4.2).
// ---------------------------------------------------------------------------

// Hors surcharge : le geste est écrit dans le global (comportement v11.4.0 conservé) ET alimente la couche directe.
func TestRoundAmbiance290_D1prime_DirectGestureOutsideOverride_RewritesGlobal(t *testing.T) {
	e := tw290New(t)
	code, body := e.post(t, "/config.json", `{"sound":{"muted":true}}`)
	if code != http.StatusOK {
		t.Fatalf("attendu 200, got %d", code)
	}
	if body["scope"] != "global" {
		t.Errorf("réponse : scope=\"global\" attendu, got %v", body["scope"])
	}
	m := e.settings(t)
	if tw290Path(m, "global", "sound", "muted") != true {
		t.Error("hors surcharge, le geste direct est persisté dans le global")
	}
	if tw290Path(m, "effective", "sound", "muted") != true || tw290Path(m, "sources", "sound", "muted") != "direct" {
		t.Errorf("effectif muted=true, source \"direct\", got %v / %v", m["effective"], m["sources"])
	}
}

// Avec surcharge de CE champ par la manche : couche directe seule, global intact, scope \"round\".
func TestRoundAmbiance290_D1prime_DirectGestureOnOverriddenField_GlobalUntouched(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, map[string]interface{}{"sound": map[string]interface{}{"muted": false}}) // la manche impose « son actif »
	e.ready(t, "1")

	code, body := e.post(t, "/config.json", `{"sound":{"muted":true}}`)
	if code != http.StatusOK {
		t.Fatalf("attendu 200, got %d", code)
	}
	if body["scope"] != "round" {
		t.Errorf("réponse : scope=\"round\" attendu (global non écrit), got %v", body["scope"])
	}
	m := e.settings(t)
	if tw290Path(m, "global", "sound", "muted") != false {
		t.Error("la manche surcharge ce champ : le global NE DOIT PAS être réécrit")
	}
	if tw290Path(m, "effective", "sound", "muted") != true || tw290Path(m, "sources", "sound", "muted") != "direct" {
		t.Errorf("le geste vaut pour la manche en cours : muted=true source \"direct\", got %v / %v", m["effective"], m["sources"])
	}
}

// La règle est CHAMP PAR CHAMP : la manche surcharge muted, un geste sur le décompte écrit bien le global.
func TestRoundAmbiance290_D1prime_FieldByField_OtherFieldStillWritesGlobal(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, map[string]interface{}{"sound": map[string]interface{}{"muted": true}})
	e.ready(t, "1")
	code, body := e.post(t, "/config.json", `{"sound":{"countdown_disabled":true}}`)
	if code != http.StatusOK {
		t.Fatalf("attendu 200, got %d", code)
	}
	if body["scope"] != "global" {
		t.Errorf("la manche ne surcharge pas le décompte : scope=\"global\", got %v", body["scope"])
	}
	if tw290Path(e.settings(t), "global", "sound", "countdown", "enabled") != false {
		t.Error("le geste sur un champ NON surchargé est persisté dans le global")
	}
}

// La couche directe est vidée au changement de manche ; la manche n'écrit jamais le global.
func TestRoundAmbiance290_D1prime_DirectLayerClearedOnRoundChange_RoundNeverWritesGlobal(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, map[string]interface{}{"sound": map[string]interface{}{"muted": false}})
	e.ready(t, "1")
	e.post(t, "/config.json", `{"sound":{"muted":true}}`) // geste direct sur le champ surchargé
	if tw290Path(e.settings(t), "effective", "sound", "muted") != true {
		t.Fatal("setup invalide")
	}
	e.ready(t, "3") // r2 : changement de manche
	m := e.settings(t)
	if tw290Path(m, "effective", "sound", "muted") != false || tw290Path(m, "sources", "sound", "muted") != "global" {
		t.Errorf("changement de manche : couche directe vidée, retour au global, got %v / %v", m["effective"], m["sources"])
	}
	if tw290Path(m, "global", "sound", "muted") != false {
		t.Error("une manche ne réécrit JAMAIS le global (ni à l'entrée, ni à la sortie)")
	}
}

func TestRoundAmbiance290_EnteringAndLeavingRound_NeverRewritesGlobal(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, tw290AmbBlind())
	before := e.settings(t)["global"]
	e.ready(t, "1")
	e.ready(t, "4")
	after := e.settings(t)["global"]
	b1, _ := json.Marshal(before)
	b2, _ := json.Marshal(after)
	if !bytes.Equal(b1, b2) {
		t.Errorf("le global doit rester identique après entrée/sortie de manche :\n avant %s\n après %s", b1, b2)
	}
}

// tw290Lighting configure un éclairage RÉEL de bout en bout avec un pont Hue FACTICE (httptest, tw286Bridge) :
// config → OnConfigUpdate → pilote construit. Sans pilote, POST /api/lighting/mode répond 409 not_configured.
func tw290Lighting(t *testing.T, e *tw290Env) {
	t.Helper()
	bridge := &tw286Bridge{}
	srv := httptest.NewServer(bridge.handler())
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.app.ctx = ctx
	e.app.httpServer.Lighting = e.app // câblé par setup() en production (main.go), pas par setupCallbacks
	config.Get().Lighting = config.LightingConfig{Enabled: true, BridgeIP: srv.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{{Name: "L1", Role: "general"}}}
	e.app.httpServer.OnConfigUpdate()
	tw286WaitDriver(t, e.app, false)
}

// POST /api/lighting/mode = geste direct : jamais d'écriture de default_mode (§4.2 point 4).
func TestRoundAmbiance290_D1prime_LightingModeGesture_NeverWritesDefaultMode(t *testing.T) {
	e := tw290New(t)
	tw290Lighting(t, e)
	code, body := e.post(t, "/api/lighting/mode", `{"mode":"OFF"}`)
	if code != http.StatusOK {
		t.Fatalf("POST /api/lighting/mode : 200 attendu avec un pilote configuré, got %d %v", code, body)
	}
	if body["scope"] != "global" {
		t.Errorf("hors surcharge de la manche : scope=\"global\" attendu, got %v", body["scope"])
	}
	m := e.settings(t)
	if tw290Path(m, "effective", "light", "mode") != "OFF" || tw290Path(m, "sources", "light", "mode") != "direct" {
		t.Errorf("le geste alimente la couche directe : OFF / \"direct\", got %v / %v", m["effective"], m["sources"])
	}
	if tw290Path(m, "global", "light", "mode") != "AUTO" {
		t.Error("POST /api/lighting/mode n'écrit JAMAIS lighting.default_mode : global=AUTO attendu")
	}
}

// Avec une manche qui impose le mode : le geste vaut pour la manche (scope \"round\"), le global reste intact.
func TestRoundAmbiance290_D1prime_LightingModeGesture_OnOverriddenField_ScopeRound(t *testing.T) {
	e := tw290New(t)
	tw290Lighting(t, e)
	e.twoRounds(t, map[string]interface{}{"light": map[string]interface{}{"mode": "ON"}})
	e.ready(t, "1")
	code, body := e.post(t, "/api/lighting/mode", `{"mode":"OFF"}`)
	if code != http.StatusOK {
		t.Fatalf("200 attendu, got %d %v", code, body)
	}
	if body["scope"] != "round" {
		t.Errorf("la manche surcharge le mode : scope=\"round\" attendu, got %v", body["scope"])
	}
	m := e.settings(t)
	if tw290Path(m, "effective", "light", "mode") != "OFF" || tw290Path(m, "sources", "light", "mode") != "direct" {
		t.Errorf("OFF / \"direct\" attendus, got %v / %v", m["effective"], m["sources"])
	}
	if tw290Path(m, "global", "light", "mode") != "AUTO" {
		t.Error("le global ne doit pas bouger")
	}
	e.ready(t, "3") // autre manche : la couche directe est vidée
	if tw290Path(e.settings(t), "effective", "light", "mode") != "AUTO" {
		t.Error("changement de manche : retour au global (AUTO)")
	}
}

func TestRoundAmbiance290_D1prime_DefaultModeWrite_IsAGlobalWrite_NotADirectGesture(t *testing.T) {
	e := tw290New(t)
	e.post(t, "/config.json", `{"lighting":{"default_mode":"OFF"}}`)
	m := e.settings(t)
	if tw290Path(m, "global", "light", "mode") != "OFF" || tw290Path(m, "effective", "light", "mode") != "OFF" {
		t.Errorf("default_mode=OFF ⇒ global et effectif OFF, got %v", m)
	}
}

// ---------------------------------------------------------------------------
// Accesseur unique (§5.2) : a.ambianceSettings() reflète la résolution.
// ---------------------------------------------------------------------------

func TestRoundAmbiance290_Accessor_ReflectsRoundOverride(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, tw290AmbBlind())
	if s := e.app.ambianceSettings(); s.SoundMuted || !s.CountdownEnabled {
		t.Fatalf("hors manche : global, got %+v", s)
	}
	e.ready(t, "1")
	s := e.app.ambianceSettings()
	if !s.SoundMuted || s.CountdownEnabled || s.CountdownLoop || string(s.LightMode) != "ON" {
		t.Errorf("en manche r1 : muted, décompte coupé et sans boucle, lumière ON ; got %+v", s)
	}
	if s.CuesEnabled["qcm-indice"] || s.CuesEnabled["rafale-bonne"] || !s.CuesEnabled["depart"] {
		t.Errorf("cues : qcm-indice et rafale-bonne coupées, depart active ; got %v", s.CuesEnabled)
	}
	time.Sleep(10 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// API : GET /api/sound/status.muted = effectif + muted_source ; GET /api/ambiance/settings en lecture seule.
// ---------------------------------------------------------------------------

func (e *tw290Env) soundStatus(t *testing.T) map[string]interface{} {
	t.Helper()
	resp, err := http.Get(e.base + "/api/sound/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var m map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return m
}

func TestRoundAmbiance290_SoundStatus_MutedIsEffective_WithSource(t *testing.T) {
	e := tw290New(t)
	if st := e.soundStatus(t); st["muted"] != false || st["muted_source"] != "global" {
		t.Errorf("hors manche : muted=false, muted_source=\"global\", got %v / %v", st["muted"], st["muted_source"])
	}
	e.twoRounds(t, map[string]interface{}{"sound": map[string]interface{}{"muted": true}})
	e.ready(t, "1")
	if st := e.soundStatus(t); st["muted"] != true || st["muted_source"] != "round" {
		t.Errorf("manche « son coupé » : muted=true, muted_source=\"round\", got %v / %v", st["muted"], st["muted_source"])
	}
	e.post(t, "/config.json", `{"sound":{"muted":false}}`) // geste direct sur le champ surchargé
	if st := e.soundStatus(t); st["muted"] != false || st["muted_source"] != "direct" {
		t.Errorf("après geste direct : muted=false, muted_source=\"direct\", got %v / %v", st["muted"], st["muted_source"])
	}
}

func TestRoundAmbiance290_Settings_ReadOnlyEndpoint(t *testing.T) {
	e := tw290New(t)
	req, _ := http.NewRequest("POST", e.base+"/api/ambiance/settings", bytes.NewBufferString("{}"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET uniquement : 405 attendu pour POST, got %d", resp.StatusCode)
	}
}

// Une écriture invalide de réglages par POST /config.json est refusée (400), jamais ignorée en silence (§3.2).
func TestRoundAmbiance290_ConfigJson_InvalidDefaultMode_Refused_NothingChanged(t *testing.T) {
	e := tw290New(t)
	before := e.settings(t)["global"]
	if code, _ := e.post(t, "/config.json", `{"lighting":{"default_mode":"on"}}`); code != http.StatusBadRequest {
		t.Errorf("default_mode en minuscules : 400 attendu, got %d", code)
	}
	after := e.settings(t)["global"]
	b1, _ := json.Marshal(before)
	b2, _ := json.Marshal(after)
	if !bytes.Equal(b1, b2) {
		t.Error("un refus ne modifie rien")
	}
}
