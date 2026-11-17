package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
)

// ---------------------------------------------------------------------------
// #145 (v12.0.0, LOT-1.1) — NON-FUITE de la phrase ROUE (contracts/roue.md §9) — CRITICAL.
//
// QUESTION.ROUE_PHRASE est retiré de tout message destiné à /ws/tv et /ws/player, dans toutes les
// phases et pour toutes les actions diffusant le GameState ; il reste pour /ws/admin et /ws/anim.
// Le sérialiseur TV ne retirait AUCUN champ de QUESTION avant v12.0.0 : ce fichier est le garde-fou.
// Modèle : rafale_leak_test.go / ardoise_leak_128_test.go.
// LOT-1.2 complète ce fichier : une vérification par sous-phase (ROUE_SUBPHASE) quand le moteur existera.
// ---------------------------------------------------------------------------

const roueSecretPhrase = "Combien pour ce chien dans la vitrine"

// roueLeakVariants : formes sous lesquelles la phrase pourrait fuiter (saisie, majuscules, repli).
var roueLeakVariants = []string{
	roueSecretPhrase,
	strings.ToUpper(roueSecretPhrase),
	strings.ToLower(roueSecretPhrase),
	"COMBIEN POUR",
	"CE CHIEN DANS",
	"LA VITRINE",
	"VITRINE",
}

func buildRoueLeakMsg(t *testing.T, action, phase string) *Message {
	t.Helper()
	q := game.Question{
		ID: "r1", Question: "Manche roue", Type: game.QuestionTypeRoue, Category: "ANIMALS",
		TypedContent: game.TypedContent{
			RouePhrase: roueSecretPhrase, RoueMode: "TANT_QUE_JE_GAGNE", RoueGainsRule: "WINNER_ONLY",
		},
	}
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE": phase, "TIME": int64(1234567890), "CURRENT_TIME": 12,
			"QUESTION":      q,
			"ROUE_SUBPHASE": "TURN",
		},
		"bumpers": map[string]interface{}{
			"p-1": map[string]interface{}{"NAME": "Alice", "TEAM": "TeamA", "CONNECTED": true, "FIRMWARE_VERSION": "3.1.1"},
		},
		"teams": map[string]interface{}{"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 0}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	msg, err := NewMessage(action, nil)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	msg.Msg = raw
	return msg
}

func assertNoPhrase(t *testing.T, who string, data []byte) {
	t.Helper()
	s := string(data)
	if strings.Contains(s, "ROUE_PHRASE") {
		t.Errorf("%s : la clé ROUE_PHRASE ne doit jamais être envoyée (contrat §9) : %s", who, s)
	}
	for _, v := range roueLeakVariants {
		if strings.Contains(s, v) {
			t.Errorf("%s : fragment de la phrase %q présent dans le message : %s", who, v, s)
		}
	}
}

func questionNodeOf(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	q, ok := gameNodeOf(t, data)["QUESTION"].(map[string]interface{})
	if !ok {
		t.Fatalf("GAME.QUESTION absent ou non-objet : %s", data)
	}
	return q
}

var roueLeakActions = []string{ActionUpdate, ActionStart, ActionStop, ActionPause, ActionContinue, ActionUpdateTimer}
var roueLeakPhases = []string{"PREPARE", "READY", "COUNTDOWN", "STARTED", "PAUSED", "STOPPED", "REVEALED"}

// critical — TV : la phrase est absente pour chaque action × phase, le reste de la question survit.
func TestRoueLeak145_TV_PhraseAbsent_AllActionsAndPhases(t *testing.T) {
	for _, action := range roueLeakActions {
		for _, phase := range roueLeakPhases {
			t.Run(action+"/"+phase, func(t *testing.T) {
				data, err := buildRoueLeakMsg(t, action, phase).SerializeForTV()
				if err != nil {
					t.Fatalf("SerializeForTV: %v", err)
				}
				assertNoPhrase(t, "TV", data)
				q := questionNodeOf(t, data)
				if q["TYPE"] != "ROUE" || q["ROUE_MODE"] != "TANT_QUE_JE_GAGNE" {
					t.Errorf("le reste de la question ROUE doit survivre (TYPE, ROUE_MODE), got %v", q)
				}
			})
		}
	}
}

// critical — tablette (chemin commun, joueur non identifié).
func TestRoueLeak145_VPlayerCommon_PhraseAbsent_AllActionsAndPhases(t *testing.T) {
	for _, action := range roueLeakActions {
		for _, phase := range roueLeakPhases {
			t.Run(action+"/"+phase, func(t *testing.T) {
				data, err := buildRoueLeakMsg(t, action, phase).SerializeForVPlayerCommon()
				if err != nil {
					t.Fatalf("SerializeForVPlayerCommon: %v", err)
				}
				assertNoPhrase(t, "VPlayer", data)
			})
		}
	}
}

// critical — tablette identifiée (réduction PREPARE/READY et cadre par destinataire).
func TestRoueLeak145_VPlayerIdentified_PhraseAbsent_AllPhases(t *testing.T) {
	for _, phase := range roueLeakPhases {
		t.Run(phase, func(t *testing.T) {
			data, err := buildRoueLeakMsg(t, ActionUpdate, phase).SerializeForVPlayer("p-1")
			if err != nil {
				t.Fatalf("SerializeForVPlayer: %v", err)
			}
			assertNoPhrase(t, "VPlayer(p-1)", data)
		})
	}
}

// La phrase n'est pas non plus transmise par un autre champ de la question (ANSWER, etc.).
func TestRoueLeak145_NoOtherQuestionFieldCarriesThePhrase(t *testing.T) {
	data, _ := buildRoueLeakMsg(t, ActionUpdate, "STARTED").SerializeForTV()
	q := questionNodeOf(t, data)
	for k, v := range q {
		if s, ok := v.(string); ok {
			for _, frag := range roueLeakVariants {
				if strings.Contains(s, frag) {
					t.Errorf("champ %s = %q contient la phrase", k, s)
				}
			}
		}
	}
	if a, _ := q["ANSWER"].(string); a != "" {
		t.Errorf("ANSWER doit rester vide pour ROUE, got %q", a)
	}
}

// Admin : il reçoit la phrase (éditeur, conduite de manche).
func TestRoueLeak145_Admin_KeepsPhrase(t *testing.T) {
	data, err := buildRoueLeakMsg(t, ActionUpdate, "STARTED").SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin: %v", err)
	}
	if q := questionNodeOf(t, data); q["ROUE_PHRASE"] != roueSecretPhrase {
		t.Errorf("l'admin doit recevoir ROUE_PHRASE, got %v", q["ROUE_PHRASE"])
	}
}

// Animateur : SerializeForWebClient (chemin /ws/anim) conserve la phrase — le retrait est propre à TV et tablettes.
func TestRoueLeak145_AnimPath_KeepsPhrase(t *testing.T) {
	data, err := buildRoueLeakMsg(t, ActionUpdate, "STARTED").SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient: %v", err)
	}
	if q := questionNodeOf(t, data); q["ROUE_PHRASE"] != roueSecretPhrase {
		t.Errorf("l'animateur doit recevoir ROUE_PHRASE (§9), got %v", q["ROUE_PHRASE"])
	}
}

// Non-régression : les autres types de question gardent leurs champs côté TV (aucun retrait collatéral).
func TestRoueLeak145_NonRegression_OtherQuestionsUnchangedOnTV(t *testing.T) {
	q := game.Question{ID: "s1", Question: "Capitale ?", Answer: "Paris", Type: game.QuestionTypeSpeedy, Points: "10", Time: "30"}
	raw, _ := json.Marshal(map[string]interface{}{
		"GAME":    map[string]interface{}{"PHASE": "STARTED", "QUESTION": q},
		"bumpers": map[string]interface{}{},
	})
	msg, _ := NewMessage(ActionUpdate, nil)
	msg.Msg = raw
	data, err := msg.SerializeForTV()
	if err != nil {
		t.Fatalf("SerializeForTV: %v", err)
	}
	got := questionNodeOf(t, data)
	if got["QUESTION"] != "Capitale ?" || got["TYPE"] != "SPEEDY" {
		t.Errorf("la question SPEEDY doit rester intacte côté TV, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// #144 (v12.0.0, LOT-1.2) — non-fuite À CHAQUE SOUS-PHASE (contracts/roue.md §9) — CRITICAL.
// « Pour chaque sous-phase avant SOLVED, la phrase et chacune de ses lettres non révélées sont absentes
// des messages sérialisés TV et tablette. » Le plateau diffusé (ROUE_BOARD) ne porte que `_` / `*` pour les
// lettres cachées : seule la phrase de QUESTION pourrait la trahir.
// ---------------------------------------------------------------------------

var roueSubphases = []string{"TURN", "SPINNING", "STOPPING", "LETTER", "LIT", "REVEALING", "ANSWER", "TURN_END"}

// buildRoueSubphaseMsg : GameState ROUE réaliste pour une sous-phase, phrase « COMBIEN POUR CE CHIEN DANS LA
// VITRINE » dont seules les lettres T, R, N sont révélées (les autres sont `_`), comme le moteur les diffuse.
func buildRoueSubphaseMsg(t *testing.T, subphase string) *Message {
	t.Helper()
	q := game.Question{
		ID: "r1", Question: "Manche roue", Type: game.QuestionTypeRoue,
		TypedContent: game.TypedContent{RouePhrase: roueSecretPhrase},
	}
	board := map[string]interface{}{"COLS": 14, "ROWS": []string{"_____N ___R", "__ _____N __N_", "__ __TR_N_"}}
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE": "STARTED", "QUESTION": q,
			"ROUE_SUBPHASE": subphase, "ROUE_STEP": 3, "ROUE_TEAMS": []string{"TeamA", "TeamB"},
			"ROUE_CURRENT_TEAM": "TeamA", "ROUE_BOARD": board, "ROUE_TEAM_BANK": map[string]int{"TeamA": 100, "TeamB": 0},
			"ROUE_LETTERS":         []map[string]interface{}{{"LETTER": "T", "TEAM": "TeamA", "HIT": true}, {"LETTER": "R", "TEAM": "TeamB", "HIT": true}, {"LETTER": "N", "TEAM": "TeamA", "HIT": true}},
			"ROUE_CONSONANTS_LEFT": true,
			"ROUE_WHEEL":           map[string]interface{}{"LAYOUT": "strip", "CURRENT": 2, "STOP": nil},
			"ROUE_LAST_EVENT":      map[string]interface{}{"SEQ": 4, "KIND": "LETTER_HIT", "TEAM": "TeamA", "LETTER": "N", "COUNT": 4},
		},
		"bumpers": map[string]interface{}{"p-1": map[string]interface{}{"NAME": "Alice", "TEAM": "TeamA", "CONNECTED": true}},
		"teams":   map[string]interface{}{"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 0}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	msg, _ := NewMessage(ActionUpdate, nil)
	msg.Msg = raw
	return msg
}

func TestRoueLeak144_EverySubphase_TVAndTablet(t *testing.T) {
	for _, sp := range roueSubphases {
		t.Run(sp, func(t *testing.T) {
			m := buildRoueSubphaseMsg(t, sp)
			tv, err := m.SerializeForTV()
			if err != nil {
				t.Fatalf("SerializeForTV: %v", err)
			}
			assertNoPhrase(t, "TV/"+sp, tv)
			vc, _ := m.SerializeForVPlayerCommon()
			assertNoPhrase(t, "VPlayer/"+sp, vc)
			vi, _ := m.SerializeForVPlayer("p-1")
			assertNoPhrase(t, "VPlayer(p-1)/"+sp, vi)
		})
	}
}

// Le plateau diffusé garde les champs de jeu publics : le retrait de la phrase ne doit pas les amputer.
func TestRoueLeak144_PublicRoueFieldsSurviveOnTVAndTablet(t *testing.T) {
	m := buildRoueSubphaseMsg(t, "LETTER")
	for name, ser := range map[string]func() ([]byte, error){
		"TV": m.SerializeForTV, "VPlayer": m.SerializeForVPlayerCommon,
	} {
		data, err := ser()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		g := gameNodeOf(t, data)
		for _, k := range []string{"ROUE_SUBPHASE", "ROUE_BOARD", "ROUE_TEAM_BANK", "ROUE_LETTERS", "ROUE_WHEEL", "ROUE_LAST_EVENT", "ROUE_CURRENT_TEAM"} {
			if _, ok := g[k]; !ok {
				t.Errorf("%s : %s doit rester diffusé (public)", name, k)
			}
		}
	}
}

// Les lettres cachées du plateau diffusé sont `_` : aucune lettre non révélée n'apparaît dans ROUE_BOARD.
func TestRoueLeak144_BoardCarriesNoUnrevealedLetter(t *testing.T) {
	data, _ := buildRoueSubphaseMsg(t, "LIT").SerializeForTV()
	board, _ := gameNodeOf(t, data)["ROUE_BOARD"].(map[string]interface{})
	rows, _ := board["ROWS"].([]interface{})
	revealed := "TRN"
	for _, r := range rows {
		for _, ch := range r.(string) {
			if strings.ContainsRune("_* ", ch) || strings.ContainsRune(revealed, ch) {
				continue
			}
			t.Errorf("lettre non révélée %q dans ROUE_BOARD : %v", ch, rows)
		}
	}
}
