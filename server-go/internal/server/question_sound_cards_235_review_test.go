// #235 (v11.6.0, LOT-2) — tests correctifs de la revue LOT-2 (_work/reports/code-reviewer-lot2-v11.6.0.md) :
// M2 (collision de nom au remplacement d'un son de carte), m2 (aucun fichier de carte orphelin si l'enregistrement
// est refusé après coup), m3 (avertissement de durée de carte : `card_warnings` avec `card_id`).
// Réutilise les helpers de question_sound_cards_235_test.go. Préfixe tw235r.
package server

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// M2 — « tirage forcé » : math/rand global réamorcé avec la même graine avant chaque envoi ⇒ le nom aléatoire du
// nouveau fichier est IDENTIQUE à celui de l'ancien. Le remplacement ne doit alors ni détruire le fichier fraîchement
// écrit (suppression différée de l'ancien nom), ni laisser question.json pointer vers un fichier absent.
func TestCardSound235_Replace_ForcedSameRandomName_FileKeptAndReferenced(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	const seed = 235235

	rand.Seed(seed) //nolint:staticcheck // tirage forcé voulu : reproduit le même nom de fichier
	tw235OK(t, tw235Post(t, s, tw235Form{cards: tw235Cards("c1"), files: map[string][]byte{"motion_card_c1_sound": tw219hCanonicalWAV(1)}}))
	first, _ := tw235Card(t, tw235ReadQuestion(t, dataDir), "c1")["SOUND"].(string)
	if first == "" {
		t.Fatal("setup invalide : premier son absent")
	}

	rand.Seed(seed) //nolint:staticcheck
	tw235OK(t, tw235Post(t, s, tw235Form{cards: tw235Cards("c1"), files: map[string][]byte{"motion_card_c1_sound": tw219hCanonicalWAV(3)}}))
	second, _ := tw235Card(t, tw235ReadQuestion(t, dataDir), "c1")["SOUND"].(string)
	if second == "" {
		t.Fatal("le second envoi doit référencer un son")
	}
	if _, err := os.Stat(filepath.Join(tw235QDir(dataDir), filepath.Base(second))); err != nil {
		t.Fatalf("le fichier référencé %q est absent du disque (détruit par la suppression de l'ancien nom ?) : %v", second, err)
	}
	// Le fichier référencé doit être le NOUVEAU contenu (3 s), pas un reste de l'ancien (1 s) ni un fichier vide.
	raw, err := os.ReadFile(filepath.Join(tw235QDir(dataDir), filepath.Base(second)))
	if err != nil || len(raw) < 3*44100*4 {
		t.Errorf("le fichier référencé doit contenir le nouveau son de 3 s (len=%d, err=%v)", len(raw), err)
	}
	if second == first {
		t.Log("collision de nom effectivement forcée (même URL) — c'est le cas à protéger")
	}
}

// Quel que soit le tirage, un remplacement répété ne laisse jamais plus d'un fichier de son pour la carte.
func TestCardSound235_Replace_NoStaleFilesLeftBehind(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	for i := 0; i < 3; i++ {
		tw235OK(t, tw235Post(t, s, tw235Form{cards: tw235Cards("c1"), files: map[string][]byte{"motion_card_c1_sound": tw219hCanonicalWAV(1)}}))
	}
	matches, _ := filepath.Glob(filepath.Join(tw235QDir(dataDir), "motion_c1_sound_*.wav"))
	if len(matches) != 1 {
		t.Errorf("un seul fichier de son attendu pour la carte c1 après 3 remplacements, got %v", matches)
	}
	url, _ := tw235Card(t, tw235ReadQuestion(t, dataDir), "c1")["SOUND"].(string)
	if len(matches) == 1 && filepath.Base(url) != filepath.Base(matches[0]) {
		t.Errorf("le fichier restant (%s) doit être celui que question.json référence (%s)", matches[0], url)
	}
}

// m2 — un enregistrement refusé APRÈS l'écriture du fichier d'une première carte ne laisse aucun fichier orphelin.
func TestCardSound235_RefusedAfterFirstCardWritten_NoOrphanFile(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	w := tw235Post(t, s, tw235Form{
		cards: tw235Cards("c1", "c2"),
		files: map[string][]byte{
			"motion_card_c1_sound": tw219hCanonicalWAV(1),                                  // valide, traitée en premier
			"motion_card_c2_sound": tw232uAssemble(0x55, 2, 44100, 16, make([]byte, 4000)), // refusée ensuite
		},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("attendu 400 (son de c2 compressé), got %d : %s", w.Code, w.Body.String())
	}
	if matches, _ := filepath.Glob(filepath.Join(tw235QDir(dataDir), "motion_*_sound_*.wav")); len(matches) != 0 {
		t.Errorf("aucun fichier son ne doit rester après un refus global, got %v", matches)
	}
}

// m2 (variante) — refus sur un champ ultérieur : le son neuf de la carte n'est pas laissé sur le disque et le son
// existant d'une carte conservée reste intact.
func TestCardSound235_RefusedUpload_ExistingSoundsUntouched(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	url1, url2 := tw235SeedTwoSoundCards(t, s, dataDir)
	w := tw235Post(t, s, tw235Form{
		cards: tw235Cards("c1", "c2"),
		files: map[string][]byte{
			"motion_card_c1_sound": tw219hCanonicalWAV(2),
			"motion_card_c2_sound": tw219hCanonicalWAV(31), // > 30 s : refusé
		},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("attendu 400, got %d : %s", w.Code, w.Body.String())
	}
	q := tw235ReadQuestion(t, dataDir)
	if tw235Card(t, q, "c1")["SOUND"] != url1 || tw235Card(t, q, "c2")["SOUND"] != url2 {
		t.Error("un enregistrement refusé ne modifie aucune référence existante")
	}
	for _, u := range []string{url1, url2} {
		if _, err := os.Stat(filepath.Join(tw235QDir(dataDir), filepath.Base(u))); err != nil {
			t.Errorf("fichier existant %s détruit par un envoi refusé : %v", u, err)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(tw235QDir(dataDir), "motion_*_sound_*.wav"))
	if len(matches) != 2 {
		t.Errorf("exactement 2 fichiers de son (les deux existants) attendus, got %v", matches)
	}
}

// m3 — `card_warnings` : une carte dont le son dépasse le temps de réponse (mode simultané) est signalée.
func tw235Warnings(t *testing.T, body []byte) []map[string]interface{} {
	t.Helper()
	var resp struct {
		CardWarnings []map[string]interface{} `json:"card_warnings"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("réponse non JSON : %v", err)
	}
	return resp.CardWarnings
}

func TestCardSound235_Warning_SoundLongerThanAnswerTime_ReportsCardID(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := tw235Post(t, s, tw235Form{
		time:  "3",
		cards: tw235Cards("c1", "c2"),
		files: map[string][]byte{"motion_card_c1_sound": tw219hCanonicalWAV(5)}, // 5 s > 3 s, simultané
	})
	tw235OK(t, w)
	got := tw235Warnings(t, w.Body.Bytes())
	if len(got) != 1 || got[0]["card_id"] != "c1" {
		t.Fatalf("card_warnings attendu [{card_id:\"c1\", warning:…}], got %v", got)
	}
	if msg, _ := got[0]["warning"].(string); msg == "" {
		t.Error("le champ `warning` doit contenir le message lisible")
	}
}

func TestCardSound235_Warning_NotRaisedWhenDelayedOrShort(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	// différé (« À la fin ») : le chrono de la carte attend la fin du son ⇒ pas de décalage possible
	w := tw235Post(t, s, tw235Form{
		time:   "3",
		cards:  tw235Cards("c1", "c2"),
		fields: map[string]string{"motion_card_c1_sound_timer_delayed": "true"},
		files: map[string][]byte{
			"motion_card_c1_sound": tw219hCanonicalWAV(5),
			"motion_card_c2_sound": tw219hCanonicalWAV(1), // plus court que 3 s
		},
	})
	tw235OK(t, w)
	if got := tw235Warnings(t, w.Body.Bytes()); len(got) != 0 {
		t.Errorf("aucun avertissement attendu (différé / plus court), got %v", got)
	}
}
