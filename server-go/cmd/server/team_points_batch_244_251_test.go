package main

// #244 + #251 (v11.2, lot V2-2) — attribution groupée TEAM_POINTS_BATCH après le REVEAL.
// Contrats : contracts/websocket-actions.md §TEAM_POINTS_BATCH, §TEAM_POINTS_BATCH_RESULT, §AWARDED_TEAMS ;
// contracts/logging.md §3.2 quater. Scénarios serveur 1 à 10 du plan (_work/reports/planner-20261011-174500.md §4).
// Statut `regression` ; `critical` : validation globale, atomicité, double crédit.

import (
	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/protocol"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type env244 struct {
	*env246
	adminConn *websocket.Conn
	admin     string
	animConn  *websocket.Conn
	otherConn *websocket.Conn // second admin : ne doit rien recevoir de ciblé
	sound     *audio.FakeOutput
	ambiance  *lighting.FakeDriver
	buzzers   map[string]string // équipe → mac du buzzer physique
}

const qid244 = "q244"

// newEnv244 : trois équipes (Bleus, Rouges, Verts) avec un VJoueur et un buzzer physique chacune ; question ARDOISE
// jouée puis RÉVÉLÉE (REVEALED) ; son et ambiance instrumentés (lecteur/driver factices).
func newEnv244(t *testing.T) *env244 {
	t.Helper()
	// Même câblage son que twa227WireSound, SANS setupCallbacks : celui-ci démarre la boucle qui consomme hub.Incoming et
	// empêcherait learnClientID (les messages sont ici injectés par handleWebMessage).
	app := newBroadcast127TestApp(t)
	app.config.Sound.Enabled = true
	t.Cleanup(func() { app.config.Sound.Enabled = false })
	fake := audio.NewFakeOutput()
	app.soundEngine.Store(app.newSoundEngine(fake))
	soundCtx, cancelSound := context.WithCancel(context.Background())
	t.Cleanup(cancelSound)
	go app.sound().Start(soundCtx)
	setGlobalLogger246(t, app)
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{
		"Les Bleus": {Name: "Les Bleus", Color: []int{0, 0, 255}}, "Les Rouges": {Name: "Les Rouges", Color: []int{255, 0, 0}}, "Les Verts": {Name: "Les Verts", Color: []int{0, 255, 0}},
	})
	paul := setupVirtualPlayer(t, app, "Paul", "Les Bleus")
	mamie := setupVirtualPlayer(t, app, "Mamie", "Les Rouges")
	e := &env246{app: app, base: startAnimAllowlistTestServer(t, app), paul: paul, mamie: mamie}
	setupVirtualPlayer(t, app, "Vera", "Les Verts")

	buzzers := map[string]string{"Les Bleus": "AA:00:00:00:00:01", "Les Rouges": "AA:00:00:00:00:02", "Les Verts": "AA:00:00:00:00:03"}
	bumpers := map[string]*game.Bumper{}
	for id := range e.state249().Bumpers {
		cp := *app.engine.GetBumper(id)
		bumpers[id] = &cp
	}
	for team, mac := range buzzers {
		bumpers[mac] = &game.Bumper{Name: "Buzzer " + team, Team: team, Connected: true}
	}
	app.engine.SetBumpers(bumpers)

	ambiance := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(ambiance))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go app.ambiance().Start(ctx)

	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("", &game.Question{ID: qid244, Type: game.QuestionTypeArdoise, Points: "3"})
	app.engine.SetPhase(game.PhaseStarted)
	app.engine.Stop()
	app.engine.Reveal()
	if app.engine.GetPhase() != game.PhaseRevealed {
		t.Fatalf("précondition : REVEALED, obtenu %s", app.engine.GetPhase())
	}

	adminConn, admin := e.dial(t, "/ws/admin")
	animConn, _ := e.dial(t, "/ws/anim")
	otherConn, _ := e.dial(t, "/ws/admin")
	env := &env244{env246: e, adminConn: adminConn, admin: admin, animConn: animConn, otherConn: otherConn, sound: fake, ambiance: ambiance, buzzers: buzzers}
	env.settleSound(t) // #271 : sons d'amorçage traités avant les instantanés « Before » (plus de pause fixe)
	return env
}

type entry244 struct {
	Team   string      `json:"TEAM"`
	Points interface{} `json:"POINTS"`
}

func (e *env244) batch(t *testing.T, questionID string, entries []entry244, extra map[string]interface{}) {
	t.Helper()
	p := map[string]interface{}{"QUESTION_ID": questionID, "ENTRIES": entries}
	for k, v := range extra {
		p[k] = v
	}
	e.send(t, e.admin, "admin", "TEAM_POINTS_BATCH", p)
}

type result244 struct {
	Status     string `json:"STATUS"`
	Reason     string `json:"REASON"`
	QuestionID string `json:"QUESTION_ID"`
	Applied    []struct {
		Team   string `json:"TEAM"`
		Points int    `json:"POINTS"`
	} `json:"APPLIED"`
	Skipped []struct {
		Team   string `json:"TEAM"`
		Reason string `json:"REASON"`
	} `json:"SKIPPED"`
	Teams []string `json:"TEAMS"`
}

func (e *env244) result(t *testing.T) result244 {
	t.Helper()
	raw := readActionMatchingWithin(t, e.adminConn, "TEAM_POINTS_BATCH_RESULT", 2*time.Second)
	var r result244
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("TEAM_POINTS_BATCH_RESULT illisible : %v (%s)", err, raw)
	}
	return r
}

// scores244 : score total et points d'équipe par équipe.
func (e *env244) scores() map[string][2]int {
	var raw struct {
		Teams map[string]struct {
			Score      int `json:"SCORE"`
			TeamPoints int `json:"TEAM_POINTS"`
		} `json:"teams"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &raw)
	out := map[string][2]int{}
	for k, v := range raw.Teams {
		out[k] = [2]int{v.Score, v.TeamPoints}
	}
	return out
}

func (e *env244) awardedEvents(team string) (n, sum int) {
	for _, ev := range e.app.engine.GetHistory() {
		if ev.EventType == "POINTS_AWARDED" && ev.TeamName == team {
			n++
			sum += ev.Points
		}
	}
	return
}

func (e *env244) historyLen() int { return len(e.app.engine.GetHistory()) }

func awardedTeamsOf244(t *testing.T, conn *websocket.Conn) map[string]int {
	t.Helper()
	raw := readActionMatchingWithin(t, conn, "AWARDED_TEAMS", 2*time.Second)
	var p struct {
		Teams []struct {
			Team   string `json:"TEAM"`
			Points int    `json:"POINTS"`
		} `json:"TEAMS"`
	}
	_ = json.Unmarshal(raw, &p)
	out := map[string]int{}
	for _, x := range p.Teams {
		out[x.Team] = x.Points
	}
	return out
}

func threeEntries244() []entry244 {
	return []entry244{{"Les Bleus", 3}, {"Les Rouges", 2}, {"Les Verts", 5}}
}

// --- 1. Lot valide ---------------------------------------------------------------------------------

func TestTeamPointsBatch244_Critical_ValidBatchCreditsEveryTeamAndAnswersTheSenderOnly(t *testing.T) {
	e := newEnv244(t)

	e.batch(t, qid244, threeEntries244(), nil)

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Applied) != 3 || len(r.Skipped) != 0 || r.QuestionID != qid244 {
		t.Fatalf("RESULT = %+v", r)
	}
	sc := e.scores()
	for team, want := range map[string]int{"Les Bleus": 3, "Les Rouges": 2, "Les Verts": 5} {
		if sc[team][1] != want || sc[team][0] != want {
			t.Errorf("%s : score/TEAM_POINTS = %v, attendu %d", team, sc[team], want)
		}
		if n, sum := e.awardedEvents(team); n != 1 || sum != want {
			t.Errorf("%s : %d événement(s) POINTS_AWARDED (somme %d), attendu 1 × %d", team, n, sum, want)
		}
	}
	for name, conn := range map[string]*websocket.Conn{"admin": e.adminConn, "anim": e.animConn} {
		got := awardedTeamsOf244(t, conn)
		if len(got) != 3 || got["Les Verts"] != 5 {
			t.Errorf("AWARDED_TEAMS reçu par %s doit contenir les 3 équipes : %v", name, got)
		}
	}
	for _, a := range collectAfterSentinel264(t, e.app, e.otherConn)[0] { // #266 : témoin, pas de fenêtre fixe
		if a == "TEAM_POINTS_BATCH_RESULT" {
			t.Fatal("RESULT est ciblé sur l'émetteur seul")
		}
	}
	if !e.hasLog(game.LogLevelInfo, "TEAM_POINTS_BATCH") {
		t.Errorf("INFO TEAM_POINTS_BATCH attendu ; logs=%v", e.logLines(""))
	}
}

// --- 2. Refus global : état strictement inchangé ----------------------------------------------------

func TestTeamPointsBatch244_Critical_AnyValidationErrorRefusesTheWholeBatchAndChangesNothing(t *testing.T) {
	cases := []struct {
		name    string
		qid     string
		entries []entry244
		extra   map[string]interface{}
		reason  string
		prepare func(e *env244)
	}{
		{"mauvais QUESTION_ID", "q-autre", threeEntries244(), nil, "QUESTION_MISMATCH", nil},
		{"ENTRIES vide", qid244, []entry244{}, nil, "INVALID_ENTRIES", nil},
		{"équipe en double", qid244, []entry244{{"Les Bleus", 3}, {"Les Bleus", 2}}, nil, "INVALID_ENTRIES", nil},
		{"plus de 64 entrées", qid244, many244(65), nil, "INVALID_ENTRIES", nil},
		{"montant nul", qid244, []entry244{{"Les Bleus", 3}, {"Les Rouges", 0}}, nil, "INVALID_POINTS", nil},
		{"montant décimal", qid244, []entry244{{"Les Bleus", 2.5}}, nil, "INVALID_POINTS", nil},
		{"montant 1000", qid244, []entry244{{"Les Bleus", 1000}}, nil, "INVALID_POINTS", nil},
		{"montant -1000", qid244, []entry244{{"Les Bleus", -1000}}, nil, "INVALID_POINTS", nil},
		{"équipe inconnue", qid244, []entry244{{"Les Bleus", 3}, {"Les Fantômes", 2}}, nil, "UNKNOWN_TEAM", nil},
		{"phase STOPPED", qid244, threeEntries244(), nil, "NOT_REVEALED", func(e *env244) { e.app.engine.SetPhase(game.PhaseStopped) }},
		{"phase STARTED", qid244, threeEntries244(), nil, "NOT_REVEALED", func(e *env244) { e.app.engine.SetPhase(game.PhaseStarted) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv244(t)
			if tc.prepare != nil {
				tc.prepare(e)
			}
			before, hist := e.scores(), e.historyLen()

			e.batch(t, tc.qid, tc.entries, tc.extra)

			r := e.result(t)
			if r.Status != "REFUSED" || r.Reason != tc.reason {
				t.Fatalf("RESULT = %+v, attendu REFUSED/%s", r, tc.reason)
			}
			if len(r.Applied) != 0 {
				t.Errorf("rien n'est appliqué : %+v", r.Applied)
			}
			after := e.scores()
			for k, v := range before {
				if after[k] != v {
					t.Errorf("score de %s inchangé : %v → %v", k, v, after[k])
				}
			}
			if e.historyLen() != hist {
				t.Errorf("historique inchangé : %d → %d", hist, e.historyLen())
			}
			if !e.hasLog(game.LogLevelWarn, "TEAM_POINTS_BATCH_REFUSED", tc.reason) {
				t.Errorf("WARN TEAM_POINTS_BATCH_REFUSED reason=%s ; logs=%v", tc.reason, e.logLines(game.LogLevelWarn))
			}
			if got := e.cues(); len(got) != 0 {
				t.Errorf("aucun effet public sur un refus : %v", got)
			}
		})
	}
}

func many244(n int) []entry244 {
	out := make([]entry244, n)
	for i := range out {
		out[i] = entry244{"Les Bleus" + strings.Repeat("x", i), 1}
	}
	return out
}

func TestTeamPointsBatch244_UnknownTeamReportsTheUnknownKeys(t *testing.T) {
	e := newEnv244(t)
	e.batch(t, qid244, []entry244{{"Les Bleus", 3}, {"Les Fantômes", 2}, {"Les Spectres", 1}}, nil)
	r := e.result(t)
	if r.Reason != "UNKNOWN_TEAM" || len(r.Teams) != 2 {
		t.Errorf("TEAMS = clés inconnues : %+v", r)
	}
}

func TestTeamPointsBatch244_Critical_NonAdminClientCannotSendABatch(t *testing.T) {
	e := newEnv244(t)
	_, anim := e.dial(t, "/ws/anim")
	_, tablet := e.dial(t, "/ws/player")
	_, tv := e.dial(t, "/ws/tv")
	before := e.scores()

	for id, ct := range map[string]string{anim: "anim", tablet: "vplayer", tv: "tv"} {
		e.send(t, id, ct, "TEAM_POINTS_BATCH", map[string]interface{}{"QUESTION_ID": qid244, "ENTRIES": threeEntries244()})
	}

	after := e.scores()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("seul l'admin peut envoyer un lot (allow-list) : score de %s %v → %v", k, v, after[k])
		}
	}
	if !server_isRejected244(e) {
		t.Errorf("le rejet doit être journalisé ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

func server_isRejected244(e *env244) bool {
	return e.hasLog(game.LogLevelWarn, "Rejected action TEAM_POINTS_BATCH")
}

// --- 3. Équipe déjà créditée → ignorée -----------------------------------------------------------------

func TestTeamPointsBatch244_Critical_AlreadyAwardedTeamIsSkippedAndTheOthersAreCredited(t *testing.T) {
	e := newEnv244(t)
	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Rouges", Points: 4}) // créditée par le clic unitaire

	e.batch(t, qid244, threeEntries244(), nil)

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Applied) != 2 || len(r.Skipped) != 1 || r.Skipped[0].Team != "Les Rouges" || r.Skipped[0].Reason != "ALREADY_AWARDED" {
		t.Fatalf("RESULT = %+v", r)
	}
	sc := e.scores()
	if sc["Les Rouges"][1] != 4 {
		t.Errorf("Les Rouges garde son seul crédit unitaire (4), obtenu %d", sc["Les Rouges"][1])
	}
	if sc["Les Bleus"][1] != 3 || sc["Les Verts"][1] != 5 {
		t.Errorf("les autres équipes sont créditées : %v", sc)
	}
	if n, _ := e.awardedEvents("Les Rouges"); n != 1 {
		t.Errorf("aucun second POINTS_AWARDED pour une équipe ignorée : %d", n)
	}
}

// --- 4. Double envoi : aucun double crédit -----------------------------------------------------------------

func TestTeamPointsBatch244_Critical_SecondIdenticalBatchIsEntirelySkippedNoDoubleCredit(t *testing.T) {
	e := newEnv244(t)
	e.batch(t, qid244, threeEntries244(), nil)
	first := e.result(t)
	after1 := e.scores()
	hist1 := e.historyLen()

	e.batch(t, qid244, threeEntries244(), nil)

	second := e.result(t)
	if first.Status != "APPLIED" || len(first.Applied) != 3 {
		t.Fatalf("premier lot : %+v", first)
	}
	if second.Status != "APPLIED" || len(second.Applied) != 0 || len(second.Skipped) != 3 {
		t.Fatalf("second lot entièrement ignoré : %+v", second)
	}
	for _, s := range second.Skipped {
		if s.Reason != "ALREADY_AWARDED" {
			t.Errorf("motif %q", s.Reason)
		}
	}
	after2 := e.scores()
	for k, v := range after1 {
		if after2[k] != v {
			t.Errorf("aucun double crédit : %s %v → %v", k, v, after2[k])
		}
	}
	if e.historyLen() != hist1 {
		t.Errorf("historique inchangé par le second lot : %d → %d", hist1, e.historyLen())
	}
}

// --- 5. #249 : renommage / suppression ----------------------------------------------------------------------

func TestTeamPointsBatch244_RenamedTeamUnderTheOldKeyIsUnknownTeam(t *testing.T) {
	e := newEnv244(t)
	e.send(t, e.admin, "admin", protocol.ActionUpdate, e.renamePayload249("Les Bleus", "Les Azur", "Les Azur")) // permis en REVEALED

	e.batch(t, qid244, []entry244{{"Les Bleus", 3}}, nil) // sélection faite avant le renommage

	r := e.result(t)
	if r.Status != "REFUSED" || r.Reason != "UNKNOWN_TEAM" || len(r.Teams) != 1 || r.Teams[0] != "Les Bleus" {
		t.Errorf("RESULT = %+v", r)
	}
}

func TestTeamPointsBatch244_DeletedThenRecreatedTeamIsNotSkipped(t *testing.T) {
	e := newEnv244(t)
	e.batch(t, qid244, []entry244{{"Les Bleus", 3}}, nil)
	_ = e.result(t)
	e.send(t, e.admin, "admin", protocol.ActionUpdate, e.deletePayload249("Les Bleus"))
	e.recreate249(t, e.admin, "Les Bleus")
	e.app.engine.SetPhase(game.PhaseRevealed)

	e.batch(t, qid244, []entry244{{"Les Bleus", 2}}, nil)

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Applied) != 1 || len(r.Skipped) != 0 {
		t.Errorf("une équipe recréée n'hérite pas du verrou « déjà créditée » (TEAM_DELETED) : %+v", r)
	}
}

// --- 6. Montants négatifs ------------------------------------------------------------------------------------

func TestTeamPointsBatch244_NegativeAmountsAreCreditedWithNoPublicEffectAndLockTheTeam(t *testing.T) {
	e := newEnv244(t)
	soundBefore := len(e.cues())
	ambBefore := e.ambSnapshot()

	e.batch(t, qid244, []entry244{{"Les Bleus", -1}, {"Les Rouges", -1}}, nil)

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Applied) != 2 {
		t.Fatalf("RESULT = %+v", r)
	}
	if sc := e.scores(); sc["Les Bleus"][1] != -1 || sc["Les Rouges"][1] != -1 {
		t.Errorf("pénalités appliquées : %v", sc)
	}
	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	if got := len(e.cues()); got != soundBefore {
		t.Errorf("aucun son pour des montants négatifs (%d → %d)", soundBefore, got)
	}
	if e.ambChanged(ambBefore) {
		t.Errorf("aucune pulsation d'ambiance pour des montants négatifs (%d → %d)", ambBefore.notif, e.ambianceNotifies())
	}
	for team, mac := range e.buzzers {
		if p, ok := e.app.bumperLEDState[mac]; ok && p.Effect == "COMET" {
			t.Errorf("aucune comète LED pour %s (négatif)", team)
		}
	}

	e.batch(t, qid244, []entry244{{"Les Bleus", 3}, {"Les Verts", 2}}, nil)
	r2 := e.result(t)
	if len(r2.Skipped) != 1 || r2.Skipped[0].Team != "Les Bleus" {
		t.Errorf("l'équipe pénalisée est ensuite « déjà créditée » : %+v", r2)
	}
}

// --- 7. Effets publics groupés ---------------------------------------------------------------------------------

func TestTeamPointsBatch244_PublicEffectsAreGroupedOneSoundOneCometPerPositiveTeam(t *testing.T) {
	e := newEnv244(t)

	e.batch(t, qid244, []entry244{{"Les Bleus", 3}, {"Les Rouges", -1}, {"Les Verts", 5}}, nil)

	_ = e.result(t)
	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	played := e.cues()
	gagne := 0
	for _, c := range played {
		if c == audio.CueGagne {
			gagne++
		}
	}
	if gagne != 1 {
		t.Errorf("un SEUL son « gagné » pour tout le lot (3 équipes), obtenu %d : %v", gagne, played)
	}
	comets := map[string]bool{}
	for team, mac := range e.buzzers {
		if p, ok := e.app.bumperLEDState[mac]; ok && p.Effect == "COMET" {
			comets[team] = true
		}
	}
	if !comets["Les Bleus"] || !comets["Les Verts"] || comets["Les Rouges"] {
		t.Errorf("une comète LED par équipe créditée positivement (Bleus, Verts), pas pour la négative : %v", comets)
	}
}

func TestTeamPointsBatch244_DisplayFalseOrTrueIsAcceptedWithoutError(t *testing.T) {
	for _, d := range []bool{true, false} {
		e := newEnv244(t)
		e.batch(t, qid244, threeEntries244(), map[string]interface{}{"DISPLAY": d})
		if r := e.result(t); r.Status != "APPLIED" || len(r.Applied) != 3 {
			t.Errorf("DISPLAY=%v : RESULT = %+v (champ réservé à #252, accepté)", d, r)
		}
	}
}

// --- 8. Atomicité vue par un abonné ---------------------------------------------------------------------------

func TestTeamPointsBatch244_Critical_SubscribersNeverSeeAnIntermediateState(t *testing.T) {
	e := newEnv244(t)

	e.batch(t, qid244, threeEntries244(), nil)

	_ = e.result(t)
	e.adminConn.SetReadDeadline(time.Now().Add(800 * time.Millisecond))
	seenFull := false
	for {
		_, data, err := e.otherConn.ReadMessage() // le second admin reçoit les UPDATE diffusés
		if err != nil {
			break
		}
		var env struct {
			Action string `json:"ACTION"`
			Msg    struct {
				Teams map[string]struct {
					TeamPoints int `json:"TEAM_POINTS"`
				} `json:"teams"`
			} `json:"MSG"`
		}
		if json.Unmarshal(data, &env) != nil || env.Action != "UPDATE" || len(env.Msg.Teams) == 0 {
			continue
		}
		changed := 0
		for _, tm := range env.Msg.Teams {
			if tm.TeamPoints != 0 {
				changed++
			}
		}
		if changed != 0 && changed != 3 {
			t.Fatalf("un UPDATE ne doit refléter ni une partie du lot ni un état intermédiaire (%d équipes sur 3 créditées) : %s", changed, data)
		}
		if changed == 3 {
			seenFull = true
			break
		}
		e.otherConn.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	}
	if !seenFull {
		t.Error("l'abonné doit recevoir un UPDATE reflétant tout le lot")
	}
}

// --- 9. Non-régression : TEAM_POINTS unitaire reste libre ---------------------------------------------------

func TestTeamPoints244_UnitaryClickStaysFreeAndDoesNotNeedABatch(t *testing.T) {
	e := newEnv244(t)
	e.batch(t, qid244, []entry244{{"Les Bleus", 3}}, nil)
	_ = e.result(t)

	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Bleus", Points: 2}) // recrédit libre

	if sc := e.scores(); sc["Les Bleus"][1] != 5 {
		t.Errorf("la régie n'est pas contrainte : TEAM_POINTS recrédite une équipe déjà créditée (3 + 2), obtenu %d", sc["Les Bleus"][1])
	}
	if n, _ := e.awardedEvents("Les Bleus"); n != 2 {
		t.Errorf("deux POINTS_AWARDED : %d", n)
	}
}

func TestTeamPoints244_UnitaryTeamPointsStillBroadcastsAwardedTeamsToAdminAndAnim(t *testing.T) {
	e := newEnv244(t)
	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Verts", Points: 1})
	for name, conn := range map[string]*websocket.Conn{"admin": e.adminConn, "anim": e.animConn} {
		if got := awardedTeamsOf244(t, conn); got["Les Verts"] != 1 {
			t.Errorf("AWARDED_TEAMS (aussi vers l'admin) après un crédit unitaire, %s : %v", name, got)
		}
	}
}

func TestAwardedTeams244_AdminReceivesTheCurrentLocksAtHello(t *testing.T) {
	e := newEnv244(t)
	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Rouges", Points: 2})
	conn, id := e.dial(t, "/ws/admin") // nouvel admin : doit apprendre les équipes déjà créditées
	e.send(t, id, "admin", protocol.ActionHello, map[string]interface{}{})
	if got := awardedTeamsOf244(t, conn); got["Les Rouges"] != 2 {
		t.Errorf("AWARDED_TEAMS envoyé à l'admin au HELLO : %v", got)
	}
}
