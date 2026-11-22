package game

// #312 (v12.1.0) — raccord Go → hook : la fixture `web/src/hooks/fixtures/roue-gamestate.312.json` (champs ROUE_* du
// GameState sérialisé par le VRAI moteur, manche voyelles démarrée) est relue par `useWebSocket.roueVowels.312.test.js`.
// Ce test garde la fixture fidèle au moteur : si le JSON Go change (champ ajouté, renommé), il échoue tant que la fixture
// n'est pas régénérée (`UPDATE_ROUE_FIXTURE=1 go test ./internal/game -run RoueGameStateFixture312`).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const roueFixture312 = "../../web/src/hooks/fixtures/roue-gamestate.312.json"

func TestRoueGameStateFixture312_MatchesEngine(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "BOTH", "ROUE_VOWELS_BEFORE": "ONE", "ROUE_VOWEL_PRICE": 300, "ROUE_VOWEL_GAIN": "REFUND", "ROUE_BONUS_COUNT": 1})
	full := vState147(t, e)
	roue := map[string]interface{}{}
	for k, v := range full {
		if strings.HasPrefix(k, "ROUE_") {
			roue[k] = v
		}
	}
	got, err := json.MarshalIndent(map[string]interface{}{"GAME": roue}, "", "  ") // clés triées : sortie déterministe
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.FromSlash(roueFixture312)
	if os.Getenv("UPDATE_ROUE_FIXTURE") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture absente (%v) : UPDATE_ROUE_FIXTURE=1 go test ./internal/game -run RoueGameStateFixture312", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("la fixture ne correspond plus au GameState du moteur : régénérer avec UPDATE_ROUE_FIXTURE=1 (champs ROUE_* ajoutés / renommés ?)")
	}
}
