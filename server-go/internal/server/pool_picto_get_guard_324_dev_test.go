package server

import (
	"net/http"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
)

// Revue LOT-2, mineur 1 : GET <base> renvoie USED, donc le dernier mot USED pendant une manche Picto est le mot
// secret en cours. La liste est refusée (409) pendant STARTED / PAUSED, permise hors manche ; le Pool Speedy
// n'est pas concerné.
func TestPoolPicto324_ListRefused409_DuringPictoRound_AllowedOutside(t *testing.T) {
	for _, phase := range []game.GamePhase{game.PhaseStarted, game.PhasePaused} {
		s, _ := setupTestHTTPServer(t)
		addWord(t, s, "chat", "HISTORY", 1)
		if w := nrDo(t, s, "GET", "/api/picto/words", ""); w.Code != http.StatusOK {
			t.Fatalf("hors manche : %d", w.Code)
		}
		putPictoRoundInProgress(s, phase)
		w := nrDo(t, s, "GET", "/api/picto/words", "")
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "picto_round_in_progress") {
			t.Errorf("phase %s : %d %q, attendu 409 picto_round_in_progress", phase, w.Code, w.Body.String())
		}
		if w := nrDo(t, s, "GET", "/api/rafale/questions", ""); w.Code != http.StatusOK {
			t.Errorf("le Pool Speedy reste lisible : %d", w.Code)
		}
	}
}
