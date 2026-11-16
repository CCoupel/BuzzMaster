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
