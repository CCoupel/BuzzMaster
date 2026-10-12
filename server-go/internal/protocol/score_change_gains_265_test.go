package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

// #265 — SCORE_CHANGE.GAINS doit traverser TOUS les sérialiseurs de diffusion sans être amputé
// (contrat game-state.md §SCORE_CHANGE.GAINS ; plan : « vérifier que les sérialiseurs transmettent
// SCORE_CHANGE entier »). Contrat pour la tablette : elle reçoit GAINS COMPLET (toutes les équipes) —
// sans cela elle ne pourrait pas calculer le lot de la TV ; seules les réponses d'autres équipes sont
// neutralisées (#258), pas GAINS. Ancien format (sans GAINS) : transmis tel quel, sans en inventer.
// Ces tests passent peut-être déjà (GAME est transmis intact) : ce sont des gardes de non-régression.

var gains265 = []map[string]interface{}{
	{"TEAM": "TeamA", "POINTS": 4},
	{"TEAM": "TeamB", "POINTS": -2},
	{"TEAM": "TeamC", "POINTS": 0},
}

func gainsMsg265(t *testing.T, phase string, withGains bool) *Message {
	t.Helper()
	sc := map[string]interface{}{"SEQ": 15, "DISPLAY": true, "TEAMS": []string{"TeamA", "TeamB", "TeamC"}, "PLAYERS": []string{}}
	if withGains {
		sc["GAINS"] = gains265
	}
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{"PHASE": phase, "SCORE_CHANGE": sc, "QUIZ_OBJECTIVES": "admin-only"},
		"bumpers": map[string]interface{}{
			"p1": map[string]interface{}{"NAME": "Paul", "TEAM": "TeamA", "IS_VPLAYER": true, "CONNECTED": true, "SCORE": 30},
			"p2": map[string]interface{}{"NAME": "Jeanne", "TEAM": "TeamB", "IS_VPLAYER": true, "CONNECTED": true, "SCORE": 10},
		},
		"teams": map[string]interface{}{
			"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 34},
			"TeamB": map[string]interface{}{"NAME": "TeamB", "SCORE": 8},
			"TeamC": map[string]interface{}{"NAME": "TeamC", "SCORE": 0},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("payload : %v", err)
	}
	msg, err := NewMessage(ActionUpdate, nil)
	if err != nil {
		t.Fatalf("message : %v", err)
	}
	msg.Msg = raw
	msg.Version = "test"
	return msg
}

func scoreChangeOut265(t *testing.T, data []byte) map[string]json.RawMessage {
	t.Helper()
	var env struct {
		Msg struct {
			Game struct {
				ScoreChange map[string]json.RawMessage `json:"SCORE_CHANGE"`
			} `json:"GAME"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("sortie illisible : %v\n%s", err, data)
	}
	return env.Msg.Game.ScoreChange
}

func gainsOut265(t *testing.T, data []byte) []map[string]interface{} {
	t.Helper()
	sc := scoreChangeOut265(t, data)
	raw, ok := sc["GAINS"]
	if !ok {
		t.Fatalf("GAINS absent de la sortie : %s", data)
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("GAINS illisible : %v", err)
	}
	return out
}

func wantGainsIntact265(t *testing.T, label string, data []byte) {
	t.Helper()
	got := gainsOut265(t, data)
	want := make([]map[string]interface{}, 0, len(gains265))
	for _, g := range gains265 { // JSON : les nombres redeviennent float64
		want = append(want, map[string]interface{}{"TEAM": g["TEAM"], "POINTS": float64(g["POINTS"].(int))})
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s : GAINS altéré\n got:  %v\n want: %v", label, got, want)
	}
}

func TestGains265_AllSerializersKeepGainsWhole(t *testing.T) {
	cases := map[string]func(*Message) ([]byte, error){
		"SerializeForAdmin":               func(m *Message) ([]byte, error) { return m.SerializeForAdmin() },
		"SerializeForTV":                  func(m *Message) ([]byte, error) { return m.SerializeForTV() },
		"SerializeForWebClient":           func(m *Message) ([]byte, error) { return m.SerializeForWebClient() },
		"SerializeForVPlayerCommon":       func(m *Message) ([]byte, error) { return m.SerializeForVPlayerCommon() },
		"SerializeForVPlayer(p1) PREPARE": func(m *Message) ([]byte, error) { return m.SerializeForVPlayer("p1") },
	}
	for name, ser := range cases {
		out, err := ser(gainsMsg265(t, "PREPARE", true))
		if err != nil {
			t.Fatalf("%s : %v", name, err)
		}
		wantGainsIntact265(t, name, out)
	}
}

func TestGains265_VPlayerKeepsEveryTeamsGainsOnAllPhases(t *testing.T) {
	// la tablette calcule le lot de la TV à partir de GAINS COMPLET : aucune équipe retirée, quelle que soit la phase
	for _, phase := range []string{"PREPARE", "READY", "STARTED", "PAUSED", "REVEALED", "STOPPED"} {
		out, err := gainsMsg265(t, phase, true).SerializeForVPlayer("p1")
		if err != nil {
			t.Fatalf("phase %s : %v", phase, err)
		}
		got := gainsOut265(t, out)
		if len(got) != 3 {
			t.Errorf("phase %s : la tablette doit recevoir les 3 entrées de GAINS (autres équipes comprises), a %d", phase, len(got))
		}
	}
}

func TestGains265_OtherScoreChangeFieldsUntouched(t *testing.T) {
	out, err := gainsMsg265(t, "STARTED", true).SerializeForTV()
	if err != nil {
		t.Fatal(err)
	}
	sc := scoreChangeOut265(t, out)
	for _, k := range []string{"SEQ", "DISPLAY", "TEAMS", "PLAYERS", "GAINS"} {
		if _, ok := sc[k]; !ok {
			t.Errorf("SCORE_CHANGE.%s manquant en sortie TV", k)
		}
	}
}

func TestGains265_OldFormatWithoutGainsStaysValidAndIsNotInvented(t *testing.T) {
	// rétrocompatibilité : un état sans GAINS (ancien serveur / ancien fichier) passe tel quel, sans champ ajouté
	for name, ser := range map[string]func(*Message) ([]byte, error){
		"TV":      func(m *Message) ([]byte, error) { return m.SerializeForTV() },
		"VPlayer": func(m *Message) ([]byte, error) { return m.SerializeForVPlayer("p1") },
		"Admin":   func(m *Message) ([]byte, error) { return m.SerializeForAdmin() },
	} {
		out, err := ser(gainsMsg265(t, "STARTED", false))
		if err != nil {
			t.Fatalf("%s : %v", name, err)
		}
		sc := scoreChangeOut265(t, out)
		if _, ok := sc["GAINS"]; ok {
			t.Errorf("%s : GAINS ne doit pas être inventé pour un ancien format", name)
		}
		if _, ok := sc["SEQ"]; !ok {
			t.Errorf("%s : SEQ doit rester présent", name)
		}
	}
}
