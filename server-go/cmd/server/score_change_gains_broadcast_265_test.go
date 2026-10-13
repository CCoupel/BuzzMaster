package main

import (
	"encoding/json"
	"testing"

	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// #265 — bout en bout : après une attribution de points, l'UPDATE reçu par la TV ET par un VJoueur porte des scores et
// un SCORE_CHANGE.GAINS COHÉRENTS (même UPDATE, même SEQ). Contrat game-state.md §SCORE_CHANGE.GAINS.
// Deux chemins de score réels (handlers de production) : TEAM_POINTS et BUMPER_POINTS, plus la correction cachée
// (DISPLAY = false : GAINS renseigné, c'est DISPLAY qui interdit l'animation). Le backend est livré (0aaf607e) : ces tests
// sont des GARDES de non-régression et doivent passer d'emblée. Harnais : room247 (speedy_refused_echo_247_test.go).

type scoreUpdate265 struct {
	Seq     int
	Display bool
	Gains   []map[string]interface{}
	HasGain bool
	Scores  map[string]float64
}

// lastScoreUpdate265 lit le DERNIER UPDATE de la fenêtre pour un client : SCORE_CHANGE et scores d'équipe.
func lastScoreUpdate265(t *testing.T, fs []frame247) scoreUpdate265 {
	t.Helper()
	ups := framesOf247(fs, "UPDATE")
	if len(ups) == 0 {
		t.Fatalf("aucun UPDATE reçu : %v", fs)
	}
	var env struct {
		Msg struct {
			Game struct {
				SC map[string]json.RawMessage `json:"SCORE_CHANGE"`
			} `json:"GAME"`
			Teams map[string]struct {
				Score float64 `json:"SCORE"`
			} `json:"teams"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal(ups[len(ups)-1].Raw, &env); err != nil {
		t.Fatalf("UPDATE illisible : %v", err)
	}
	out := scoreUpdate265{Scores: map[string]float64{}}
	_ = json.Unmarshal(env.Msg.Game.SC["SEQ"], &out.Seq)
	_ = json.Unmarshal(env.Msg.Game.SC["DISPLAY"], &out.Display)
	if raw, ok := env.Msg.Game.SC["GAINS"]; ok {
		out.HasGain = true
		_ = json.Unmarshal(raw, &out.Gains)
	}
	for n, tm := range env.Msg.Teams {
		out.Scores[n] = tm.Score
	}
	return out
}

func (r *room247) teamPoints265(t *testing.T, team string, points int, display *bool, sender ...updateSender) {
	t.Helper()
	p := map[string]interface{}{"TEAM": team, "POINTS": points}
	if display != nil {
		p["DISPLAY"] = *display
	}
	msg, err := protocol.NewMessage(protocol.ActionTeamPoints, p)
	if err != nil {
		t.Fatal(err)
	}
	r.app.handleTeamPoints(msg, sender...)
}

func (r *room247) bumperPoints265(t *testing.T, id string, points int) {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionBumperPoints, map[string]interface{}{"ID": id, "POINTS": points})
	if err != nil {
		t.Fatal(err)
	}
	r.app.handleBumperPoints(msg)
}

func wantGains265(t *testing.T, label string, u scoreUpdate265, want map[string]float64) {
	t.Helper()
	if !u.HasGain {
		t.Fatalf("%s : SCORE_CHANGE.GAINS absent de l'UPDATE", label)
	}
	if len(u.Gains) != len(want) {
		t.Errorf("%s : GAINS = %v, attendu %v", label, u.Gains, want)
		return
	}
	for _, g := range u.Gains {
		team, _ := g["TEAM"].(string)
		pts, _ := g["POINTS"].(float64)
		if w, ok := want[team]; !ok || w != pts {
			t.Errorf("%s : entrée inattendue %v (attendu %v)", label, g, want)
		}
	}
}

func TestGainsBroadcast265_TeamPoints_TvAndVPlayerSeeScoresAndGainsTogether(t *testing.T) {
	r := newRoom247(t)
	r.teamPoints265(t, "TeamA", 4, nil)
	got := r.observe(t)
	for _, name := range []string{"tv", "p1", "p3"} {
		u := lastScoreUpdate265(t, got[name])
		if u.Seq != 1 || !u.Display {
			t.Errorf("%s : SEQ=%d DISPLAY=%v, attendu 1/true", name, u.Seq, u.Display)
		}
		wantGains265(t, name, u, map[string]float64{"TeamA": 4})
		if u.Scores["TeamA"] != 4 {
			t.Errorf("%s : score TeamA = %v, attendu 4 (cohérent avec GAINS dans le même UPDATE)", name, u.Scores["TeamA"])
		}
	}
}

func TestGainsBroadcast265_BumperPoints_GainGoesToTheBumpersTeamForEveryScreen(t *testing.T) {
	r := newRoom247(t)
	r.bumperPoints265(t, r.ids["p3"], 3) // p3 : TeamB
	got := r.observe(t)
	for _, name := range []string{"tv", "p1", "p3"} {
		u := lastScoreUpdate265(t, got[name])
		wantGains265(t, name, u, map[string]float64{"TeamB": 3})
		if u.Scores["TeamB"] != 3 {
			t.Errorf("%s : score TeamB = %v, attendu 3", name, u.Scores["TeamB"])
		}
	}
}

func TestGainsBroadcast265_TwoSuccessiveAwards_SeqAndGainsAdvanceTogether(t *testing.T) {
	r := newRoom247(t)
	r.teamPoints265(t, "TeamA", 4, nil)
	r.observe(t)
	r.bumperPoints265(t, r.ids["p3"], 3)
	got := r.observe(t)
	for _, name := range []string{"tv", "p1"} {
		u := lastScoreUpdate265(t, got[name])
		if u.Seq != 2 {
			t.Errorf("%s : SEQ=%d, attendu 2", name, u.Seq)
		}
		wantGains265(t, name, u, map[string]float64{"TeamB": 3}) // GAINS décrit la DERNIÈRE modification, pas le cumul
		if u.Scores["TeamA"] != 4 || u.Scores["TeamB"] != 3 {
			t.Errorf("%s : scores %v, attendu TeamA=4 TeamB=3", name, u.Scores)
		}
	}
}

func TestGainsBroadcast265_HiddenCorrection_GainsPresentDisplayFalse(t *testing.T) {
	r := newRoom247(t)
	hidden := false
	admin := updateSender{ClientID: "admin-1", ClientType: server.ClientTypeAdmin}
	r.teamPoints265(t, "TeamA", 2, &hidden, admin)
	got := r.observe(t)
	for _, name := range []string{"tv", "p1"} {
		u := lastScoreUpdate265(t, got[name])
		if u.Display {
			t.Errorf("%s : DISPLAY doit valoir false (correction cachée)", name)
		}
		wantGains265(t, name, u, map[string]float64{"TeamA": 2}) // GAINS renseigné : DISPLAY seul interdit l'animation
		if u.Scores["TeamA"] != 2 {
			t.Errorf("%s : le score doit être à jour : %v", name, u.Scores["TeamA"])
		}
	}
}
