package game

// #249 (v11.2) — report des états indexés par nom d'équipe lors d'un renommage permis (hors verrou, REVEALED
// compris) et effacement à la suppression. Liste FERMÉE : contracts/game-state.md §TEAMS_LOCKED.
// Scénarios 7 et 8 du plan (_work/reports/planner-20261011-001500.md §A.5). Point d'entrée moteur : RenameTeamState /
// PurgeTeamState (nommage de dev-backend).

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// seed249 amorce, pour les équipes « Les Bleus » (renommée) et « Les Rouges » (témoin), TOUS les états de la liste fermée.
func seed249(e *Engine) {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := &e.state
	st.ArdoiseAnswers = map[string]ArdoiseAnswer{"Les Bleus": {Text: "PARIS"}, "Les Rouges": {Text: "LYON"}}
	st.MemoryTeamPairs = map[string]int{"Les Bleus": 3, "Les Rouges": 1}
	st.MemoryTeamErrors = map[string]int{"Les Bleus": 2, "Les Rouges": 4}
	st.MemoryPairOwners = map[int]string{1: "Les Bleus", 2: "Les Rouges", 3: "Les Bleus"}
	st.MemoryParticipatingTeams = []string{"Les Bleus", "Les Rouges"}
	st.MemoryCurrentTeam = "Les Bleus"
	st.MotionCardTeams = map[string]string{"c1": "Les Bleus", "c2": "Les Rouges"}
	st.MotionParticipatingTeams = []string{"Les Rouges", "Les Bleus"}
	st.MotionCurrentTeam = "Les Bleus"
	st.RafaleTeamCounters = map[string]int{"Les Bleus": 7, "Les Rouges": 5}
	st.RafaleTeamCategoryCounters = map[string]map[string]int{"Les Bleus": {"histoire": 2}, "Les Rouges": {"sport": 1}}
	st.RafaleTeamBest = map[string]int{"Les Bleus": 4, "Les Rouges": 2}
	st.RafaleTeamStreak = map[string]int{"Les Bleus": 3, "Les Rouges": 0}
	st.RafaleTeamErrors = map[string]int{"Les Bleus": 1, "Les Rouges": 6}
	st.RafaleParticipatingTeams = []string{"Les Bleus", "Les Rouges"}
	st.RafaleCurrentTeam = "Les Bleus"
	e.history = append(e.history,
		GameEvent{TeamName: "Les Bleus", WinnerType: "TEAM", WinnerID: "Les Bleus", WinnerName: "Les Bleus"},
		GameEvent{TeamName: "Les Rouges", WinnerType: "TEAM", WinnerID: "Les Rouges", WinnerName: "Les Rouges"},
	)
}

func TestTeamRename249_CarriesEveryStateOfTheClosedList(t *testing.T) {
	e := NewEngine()
	seed249(e)

	e.RenameTeamState("Les Bleus", "Les Azur")

	st := e.GetState()
	if st.ArdoiseAnswers["Les Azur"].Text != "PARIS" {
		t.Errorf("ARDOISE_ANSWERS : %v", st.ArdoiseAnswers)
	}
	if st.MemoryTeamPairs["Les Azur"] != 3 || st.MemoryTeamErrors["Les Azur"] != 2 {
		t.Errorf("MEMORY_TEAM_PAIRS/ERRORS : %v %v", st.MemoryTeamPairs, st.MemoryTeamErrors)
	}
	if st.MemoryPairOwners[1] != "Les Azur" || st.MemoryPairOwners[3] != "Les Azur" || st.MemoryPairOwners[2] != "Les Rouges" {
		t.Errorf("MEMORY_PAIR_OWNERS (valeur = équipe) : %v", st.MemoryPairOwners)
	}
	if st.MotionCardTeams["c1"] != "Les Azur" || st.MotionCardTeams["c2"] != "Les Rouges" {
		t.Errorf("MEMOTION_CARD_TEAMS : %v", st.MotionCardTeams)
	}
	for name, l := range map[string][]string{"MEMORY_PARTICIPATING_TEAMS": st.MemoryParticipatingTeams, "MEMOTION_PARTICIPATING_TEAMS": st.MotionParticipatingTeams, "RAFALE_PARTICIPATING_TEAMS": st.RafaleParticipatingTeams} {
		sorted := append([]string(nil), l...)
		sort.Strings(sorted)
		if strings.Join(sorted, ",") != "Les Azur,Les Rouges" {
			t.Errorf("%s : %v", name, l)
		}
	}
	if st.MemoryCurrentTeam != "Les Azur" || st.MotionCurrentTeam != "Les Azur" || st.RafaleCurrentTeam != "Les Azur" {
		t.Errorf("équipes courantes : %q %q %q", st.MemoryCurrentTeam, st.MotionCurrentTeam, st.RafaleCurrentTeam)
	}
	if st.RafaleTeamCounters["Les Azur"] != 7 || st.RafaleTeamCategoryCounters["Les Azur"]["histoire"] != 2 ||
		st.RafaleTeamBest["Les Azur"] != 4 || st.RafaleTeamStreak["Les Azur"] != 3 || st.RafaleTeamErrors["Les Azur"] != 1 {
		t.Errorf("RAFALE_* : %v %v %v %v %v", st.RafaleTeamCounters, st.RafaleTeamCategoryCounters, st.RafaleTeamBest, st.RafaleTeamStreak, st.RafaleTeamErrors)
	}
	for _, m := range []map[string]int{st.MemoryTeamPairs, st.MemoryTeamErrors, st.RafaleTeamCounters, st.RafaleTeamBest, st.RafaleTeamStreak, st.RafaleTeamErrors} {
		if _, ok := m["Les Bleus"]; ok {
			t.Errorf("plus aucune entrée sous l'ancienne clé : %v", m)
		}
	}
	if _, ok := st.ArdoiseAnswers["Les Bleus"]; ok {
		t.Errorf("ARDOISE_ANSWERS : ancienne clé encore présente")
	}
}

func TestTeamRename249_CarriesThePointsHistorySoAlreadyCreditedSurvives(t *testing.T) {
	e := NewEngine()
	seed249(e)

	e.RenameTeamState("Les Bleus", "Les Azur")

	e.mu.RLock()
	defer e.mu.RUnlock()
	got := []string{}
	for _, ev := range e.history {
		got = append(got, ev.TeamName+"|"+ev.WinnerID+"|"+ev.WinnerName)
	}
	if strings.Join(got, ";") != "Les Azur|Les Azur|Les Azur;Les Rouges|Les Rouges|Les Rouges" {
		t.Errorf("TEAM_NAME (et vainqueur de type TEAM) de toutes les entrées reportés : %v", got)
	}
}

func TestTeamRename249_OtherTeamsAreNeverTouched(t *testing.T) {
	e := NewEngine()
	seed249(e)
	before := e.GetState()
	witness := map[string]interface{}{
		"ardoise": before.ArdoiseAnswers["Les Rouges"], "pairs": before.MemoryTeamPairs["Les Rouges"], "errors": before.MemoryTeamErrors["Les Rouges"],
		"counters": before.RafaleTeamCounters["Les Rouges"], "best": before.RafaleTeamBest["Les Rouges"], "streak": before.RafaleTeamStreak["Les Rouges"],
		"rerrors": before.RafaleTeamErrors["Les Rouges"], "cat": before.RafaleTeamCategoryCounters["Les Rouges"], "card": before.MotionCardTeams["c2"], "owner": before.MemoryPairOwners[2],
	}

	e.RenameTeamState("Les Bleus", "Les Azur")

	after := e.GetState()
	now := map[string]interface{}{
		"ardoise": after.ArdoiseAnswers["Les Rouges"], "pairs": after.MemoryTeamPairs["Les Rouges"], "errors": after.MemoryTeamErrors["Les Rouges"],
		"counters": after.RafaleTeamCounters["Les Rouges"], "best": after.RafaleTeamBest["Les Rouges"], "streak": after.RafaleTeamStreak["Les Rouges"],
		"rerrors": after.RafaleTeamErrors["Les Rouges"], "cat": after.RafaleTeamCategoryCounters["Les Rouges"], "card": after.MotionCardTeams["c2"], "owner": after.MemoryPairOwners[2],
	}
	if !reflect.DeepEqual(witness, now) {
		t.Errorf("« Les Rouges » doit rester strictement identique :\navant %v\naprès %v", witness, now)
	}
}

func TestTeamRename249_RenamingATeamWithNoStateIsHarmless(t *testing.T) {
	e := NewEngine()
	e.RenameTeamState("Les Fantômes", "Les Azur") // aucune entrée nulle part : ni panique ni état créé
	st := e.GetState()
	if _, ok := st.ArdoiseAnswers["Les Azur"]; ok {
		t.Errorf("aucune entrée ne doit être créée pour une équipe sans état")
	}
}

func TestTeamPurge249_DeleteErasesTheTeamStatesAndKeepsTheOthers(t *testing.T) {
	e := NewEngine()
	seed249(e)

	e.PurgeTeamState("Les Rouges")

	st := e.GetState()
	if _, ok := st.ArdoiseAnswers["Les Rouges"]; ok {
		t.Errorf("ARDOISE_ANSWERS : %v", st.ArdoiseAnswers)
	}
	for _, m := range []map[string]int{st.MemoryTeamPairs, st.MemoryTeamErrors, st.RafaleTeamCounters, st.RafaleTeamBest, st.RafaleTeamStreak, st.RafaleTeamErrors} {
		if _, ok := m["Les Rouges"]; ok {
			t.Errorf("état de l'équipe supprimée à effacer : %v", m)
		}
	}
	if _, ok := st.RafaleTeamCategoryCounters["Les Rouges"]; ok {
		t.Errorf("RAFALE_TEAM_CATEGORY_COUNTERS à effacer")
	}
	for id, tm := range st.MemoryPairOwners {
		if tm == "Les Rouges" {
			t.Errorf("paire %d encore attribuée à l'équipe supprimée", id)
		}
	}
	for _, l := range [][]string{st.MemoryParticipatingTeams, st.MotionParticipatingTeams, st.RafaleParticipatingTeams} {
		for _, n := range l {
			if n == "Les Rouges" {
				t.Errorf("liste de participantes : %v", l)
			}
		}
	}
	if st.ArdoiseAnswers["Les Bleus"].Text != "PARIS" || st.MemoryTeamPairs["Les Bleus"] != 3 || st.RafaleTeamCounters["Les Bleus"] != 7 {
		t.Errorf("les autres équipes restent inchangées")
	}
}

// Garde-fou de la « liste fermée » : tout champ de GameState dont le nom JSON contient TEAM doit être, soit dans la
// liste reportée au renommage (contracts/game-state.md §TEAMS_LOCKED), soit une exception documentée. Un nouveau
// champ indexé par équipe fait échouer ce test : il doit rejoindre RenameTeamState/PurgeTeamState et le contrat.
func TestTeamRename249_ClosedListCoversEveryTeamIndexedGameStateField(t *testing.T) {
	carried := map[string]bool{
		"ARDOISE_ANSWERS": true, "MEMORY_TEAM_PAIRS": true, "MEMORY_TEAM_ERRORS": true, "MEMORY_PAIR_OWNERS": true,
		"RAFALE_TEAM_COUNTERS": true, "RAFALE_TEAM_CATEGORY_COUNTERS": true, "RAFALE_TEAM_BEST": true, "RAFALE_TEAM_STREAK": true, "RAFALE_TEAM_ERRORS": true,
		"MEMOTION_CARD_TEAMS": true, "MEMORY_PARTICIPATING_TEAMS": true, "MEMOTION_PARTICIPATING_TEAMS": true, "RAFALE_PARTICIPATING_TEAMS": true,
		"MEMORY_CURRENT_TEAM": true, "MEMOTION_CURRENT_TEAM": true, "RAFALE_CURRENT_TEAM": true,
	}
	exceptions := map[string]string{
		"MEMORY_CURRENT_TEAM_COLOR":   "couleur : inchangée au renommage",
		"MEMOTION_CURRENT_TEAM_COLOR": "couleur : inchangée au renommage",
		"RAFALE_CURRENT_TEAM_COLOR":   "couleur : inchangée au renommage",
		"TEAMS_LOCKED":                "drapeau, pas un index d'équipe",
	}
	typ := reflect.TypeOf(GameState{})
	for i := 0; i < typ.NumField(); i++ {
		tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if !strings.Contains(tag, "TEAM") {
			continue
		}
		if carried[tag] {
			continue
		}
		if _, ok := exceptions[tag]; ok {
			continue
		}
		t.Errorf("champ GameState %q indexé par équipe ? Il doit être reporté au renommage (RenameTeamState/PurgeTeamState + contrat game-state.md §TEAMS_LOCKED) ou ajouté aux exceptions documentées de ce test", tag)
	}
	for tag := range carried {
		found := false
		for i := 0; i < typ.NumField(); i++ {
			if strings.Split(typ.Field(i).Tag.Get("json"), ",")[0] == tag {
				found = true
			}
		}
		if !found {
			t.Errorf("la liste fermée cite %q, absent de GameState : contrat et code divergent", tag)
		}
	}
}
