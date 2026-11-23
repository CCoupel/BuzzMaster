package game

import (
	"slices"
	"sync/atomic"
	"testing"
)

// #192 (v13.0.0, LOT-2.1) — préparation de la manche PICTO, mot secret, remise (contracts/picto.md §3.2, §6.1, §6.2).
// Les tirages sont testés par propriétés (le générateur global n'est pas injectable, #188).

type pictoRig struct {
	t       *testing.T
	e       *Engine
	words   []PictoWordInfo
	resets  []int
	changes atomic.Int64
}

func newPictoRig(t *testing.T, words ...string) *pictoRig {
	t.Helper()
	r := &pictoRig{t: t, e: NewEngine()}
	r.e.SetTeams(map[string]*Team{
		"red":  {Name: "red", Color: []int{255, 0, 0}, Ready: true},
		"blue": {Name: "blue", Color: []int{0, 0, 255}, Ready: true},
	})
	r.e.SetBumpers(map[string]*Bumper{
		"v-alice": {Name: "Alice", Team: "red", IsVPlayer: true, IsVirtual: true, Connected: true, Ready: true},
		"v-bob":   {Name: "Bob", Team: "red", IsVPlayer: true, IsVirtual: true, Connected: false, Ready: true},
		"b-red":   {Name: "Buzz", Team: "red", Connected: true, Ready: true}, // physical buzzer: never a candidate
		"v-carl":  {Name: "Carl", Team: "blue", IsVPlayer: true, IsVirtual: true, Connected: true, Ready: true},
	})
	r.e.OnPictoWord = func(w PictoWordInfo) { r.words = append(r.words, w) }
	r.e.OnPictoCanvasReset = func(ep int) { r.resets = append(r.resets, ep) }
	r.e.OnPictoChange = func() { r.changes.Add(1) }
	if len(words) == 0 {
		words = []string{"chat"}
	}
	q := &Question{ID: "pq", Question: "Picto", Type: QuestionTypePicto, Category: "ANIMALS", Time: "90", TypedContent: TypedContent{PictoWords: words}}
	r.e.Ready("pq", q)
	for id := range r.e.GetTeamsAndBumpers().Bumpers {
		r.e.SetBumperReady(id) // every buzzer answered the PING: only the participants gate remains
	}
	t.Cleanup(func() { r.e.Stop() })
	return r
}

func (r *pictoRig) st() GameState { return r.e.GetState() }

func wantPictoErr(t *testing.T, err error, code string) {
	t.Helper()
	pe, ok := err.(*PictoError)
	if !ok || pe.Code != code {
		t.Fatalf("erreur attendue %s, got %v", code, err)
	}
}

func TestPicto192_Prepare_TeamCandidatesAndReadyGate(t *testing.T) {
	r := newPictoRig(t)
	if st := r.st(); st.Phase != PhasePrepare || st.PictoWordsCount != 1 || st.PictoTeam != "" {
		t.Fatalf("préparation initiale inattendue : phase=%s count=%d team=%q", st.Phase, st.PictoWordsCount, st.PictoTeam)
	}
	wantPictoErr(t, r.e.PictoSetTeam("nobody"), PictoErrInvalidTeam)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	st := r.st()
	if st.PictoTeam != "red" || len(st.PictoTeamColor) != 3 {
		t.Errorf("équipe/couleur : %q %v", st.PictoTeam, st.PictoTeamColor)
	}
	// Candidats : VJoueurs de l'équipe seulement (pas le buzzer physique, pas l'autre équipe), par nom.
	want := []PictoCandidate{{"v-alice", "Alice", true}, {"v-bob", "Bob", false}}
	if !slices.Equal(st.PictoCandidates, want) {
		t.Errorf("candidats = %v, want %v", st.PictoCandidates, want)
	}
	// READY sans dessinateur (révision 6) : équipe + durée + mots suffisent.
	if st.Phase != PhaseReady {
		t.Errorf("équipe choisie, dessinateur non exigé : READY attendu, phase=%s", st.Phase)
	}
	// Effacer l'équipe → retour en PREPARE.
	if err := r.e.PictoSetTeam(""); err != nil {
		t.Fatal(err)
	}
	if st := r.st(); st.Phase != PhasePrepare || len(st.PictoCandidates) != 0 {
		t.Errorf("équipe effacée : PREPARE et aucun candidat attendus, phase=%s cand=%v", st.Phase, st.PictoCandidates)
	}
}

func TestPicto192_ReadyGate_DurationAndWords(t *testing.T) {
	for _, tm := range []string{"29", "301", "", "abc"} {
		if participantsConform(&Question{Type: QuestionTypePicto, Time: tm, TypedContent: TypedContent{PictoWords: []string{"x"}}}, &GameState{PictoTeam: "red"}) {
			t.Errorf("durée %q : non conforme attendu", tm)
		}
	}
	if participantsConform(&Question{Type: QuestionTypePicto, Time: "90"}, &GameState{PictoTeam: "red"}) {
		t.Error("aucun mot : non conforme attendu")
	}
	if participantsConform(&Question{Type: QuestionTypePicto, Time: "90", TypedContent: TypedContent{PictoWords: []string{"x"}}}, &GameState{}) {
		t.Error("aucune équipe : non conforme attendu")
	}
	if !participantsConform(&Question{Type: QuestionTypePicto, Time: " 90 ", TypedContent: TypedContent{PictoWords: []string{"x"}}}, &GameState{PictoTeam: "red"}) {
		t.Error("équipe + 90 s + un mot : conforme attendu")
	}
}

func TestPicto192_SetDrawer(t *testing.T) {
	r := newPictoRig(t)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	wantPictoErr(t, r.e.PictoSetDrawer("v-carl"), PictoErrNotACandidate) // autre équipe
	wantPictoErr(t, r.e.PictoSetDrawer("b-red"), PictoErrNotACandidate)  // buzzer physique
	wantPictoErr(t, r.e.PictoSetDrawer("v-bob"), PictoErrDrawerOffline)  // déconnecté
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	if st := r.st(); st.PictoDrawer != "v-alice" || st.PictoDrawerName != "Alice" || !st.PictoDrawerConnected {
		t.Errorf("dessinateur : %q %q %v", st.PictoDrawer, st.PictoDrawerName, st.PictoDrawerConnected)
	}
	// Changer d'équipe efface un dessinateur qui n'en est pas.
	if err := r.e.PictoSetTeam("blue"); err != nil {
		t.Fatal(err)
	}
	if st := r.st(); st.PictoDrawer != "" || st.PictoDrawerName != "" {
		t.Errorf("le dessinateur doit être effacé au changement d'équipe : %q", st.PictoDrawer)
	}
	if err := r.e.PictoSetDrawer("v-carl"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer(""); err != nil || r.st().PictoDrawer != "" {
		t.Errorf("effacement du dessinateur refusé : %v", err)
	}
}

func TestPicto192_DrawDrawer_PropertiesAndRefusals(t *testing.T) {
	r := newPictoRig(t)
	wantPictoErr(t, r.e.PictoDrawDrawer(), PictoErrInvalidTeam) // sans équipe
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ { // rejouable ; propriété : toujours un joueur de l'équipe ET connecté
		if err := r.e.PictoDrawDrawer(); err != nil {
			t.Fatal(err)
		}
		if d := r.st().PictoDrawer; d != "v-alice" {
			t.Fatalf("seule Alice est connectée dans l'équipe red, tiré : %q", d)
		}
	}
	// Plus personne de connecté → NO_CONNECTED_PLAYER, aucune désignation silencieuse.
	r.e.UpdateBumper("v-alice", map[string]interface{}{"CONNECTED": false})
	wantPictoErr(t, r.e.PictoDrawDrawer(), PictoErrNoConnectedPlayer)
	if st := r.st(); st.PictoDrawerConnected {
		t.Error("le dessinateur déconnecté doit l'être aussi dans l'état")
	}
}

func TestPicto192_DrawDrawer_CoversEveryConnectedCandidate(t *testing.T) {
	r := newPictoRig(t)
	r.e.UpdateBumper("v-bob", map[string]interface{}{"CONNECTED": true})
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i := 0; i < 400 && len(seen) < 2; i++ {
		if err := r.e.PictoDrawDrawer(); err != nil {
			t.Fatal(err)
		}
		seen[r.st().PictoDrawer] = true
	}
	if len(seen) != 2 {
		t.Errorf("sur de nombreux tirages, les deux candidats connectés doivent sortir : %v", seen)
	}
}

func TestPicto192_PrepActionsRefusedOutsidePrepare(t *testing.T) {
	r := newPictoRig(t)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	r.e.StartImmediate(60)
	wantPictoErr(t, r.e.PictoSetTeam("blue"), PictoErrWrongPhase)
	wantPictoErr(t, r.e.PictoSetDrawer("v-alice"), PictoErrWrongPhase)
	wantPictoErr(t, r.e.PictoDrawDrawer(), PictoErrWrongPhase)
}

func TestPicto192_StartRefusedWithoutConnectedDrawer(t *testing.T) {
	r := newPictoRig(t)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if r.st().Phase != PhaseReady {
		t.Fatalf("READY attendu, phase=%s", r.st().Phase)
	}
	if pe := r.e.PictoStartCheck(); pe == nil || pe.Code != PictoErrNoDrawer {
		t.Fatalf("NO_DRAWER attendu sans dessinateur, got %v", pe)
	}
	r.e.Start(60)
	if r.st().Phase != PhaseReady {
		t.Errorf("START doit être refusé, état inchangé : phase=%s", r.st().Phase)
	}
	// Dessinateur désigné puis déconnecté : START refusé de nouveau.
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	r.e.UpdateBumper("v-alice", map[string]interface{}{"CONNECTED": false})
	if pe := r.e.PictoStartCheck(); pe == nil || pe.Code != PictoErrNoDrawer {
		t.Fatalf("NO_DRAWER attendu pour un dessinateur déconnecté, got %v", pe)
	}
	r.e.Start(60)
	if r.st().Phase != PhaseReady {
		t.Errorf("START refusé attendu, phase=%s", r.st().Phase)
	}
	// Reconnecté : OK.
	r.e.UpdateBumper("v-alice", map[string]interface{}{"CONNECTED": true})
	if pe := r.e.PictoStartCheck(); pe != nil {
		t.Fatalf("dessinateur connecté : START permis, got %v", pe)
	}
	r.e.Start(60)
	if r.st().Phase != PhaseCountdown {
		t.Errorf("COUNTDOWN attendu, phase=%s", r.st().Phase)
	}
}

func TestPicto192_StartDrawsWordAmongQuestionWords_SecretStaysOutOfState(t *testing.T) {
	words := []string{"chat", "chien", "pont-levis"}
	seen := map[string]bool{}
	for i := 0; i < 300 && len(seen) < len(words); i++ {
		r := newPictoRig(t, words...)
		if err := r.e.PictoSetTeam("red"); err != nil {
			t.Fatal(err)
		}
		if err := r.e.PictoSetDrawer("v-alice"); err != nil {
			t.Fatal(err)
		}
		r.e.StartImmediate(60)
		if len(r.words) != 1 {
			t.Fatalf("un PICTO_WORD attendu au START, got %d", len(r.words))
		}
		w := r.words[0]
		if !slices.Contains(words, w.Word) || w.Category != "ANIMALS" || w.DrawerID != "v-alice" || w.Epoch != 1 {
			t.Fatalf("mot de la manche inattendu : %+v", w)
		}
		if len(r.resets) != 2 || r.resets[1] != 1 { // Ready (époque 0) puis START (époque 1)
			t.Errorf("remises de la feuille : %v", r.resets)
		}
		seen[w.Word] = true
		st := r.st()
		if st.PictoCanvasEpoch != 1 || st.PictoRevealedWord != "" || st.PictoOutcome != "" {
			t.Errorf("état à la manche : epoch=%d revealed=%q outcome=%q", st.PictoCanvasEpoch, st.PictoRevealedWord, st.PictoOutcome)
		}
		if got, ok := r.e.PictoWord(); !ok || got.Word != w.Word {
			t.Errorf("PictoWord() = %+v %v", got, ok)
		}
		r.e.Stop()
	}
	if len(seen) != len(words) {
		t.Errorf("sur de nombreux tirages, chaque mot doit sortir au moins une fois : %v", seen)
	}
}

func TestPicto192_SingleWordAlwaysDrawn_AndRelaunch(t *testing.T) {
	r := newPictoRig(t, "chat")
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	r.e.StartImmediate(60)
	r.e.Stop()
	// Relance de la même question (§6.6) : préparation conservée, mot secret et issue vidés, nouvelle époque au START.
	q := r.e.GetState().Question
	r.e.Ready("pq", q)
	st := r.st()
	if st.PictoTeam != "red" || st.PictoDrawer != "v-alice" {
		t.Errorf("la relance conserve l'équipe et le dessinateur : %q %q", st.PictoTeam, st.PictoDrawer)
	}
	if _, ok := r.e.PictoWord(); ok {
		t.Error("le mot secret doit être vidé à la relance")
	}
	r.e.UpdateBumper("v-alice", map[string]interface{}{"CONNECTED": true})
	for id := range r.e.GetTeamsAndBumpers().Bumpers {
		r.e.SetBumperReady(id)
	}
	r.e.StartImmediate(60)
	if len(r.words) != 2 || r.words[1].Word != "chat" || r.words[1].Epoch != 2 {
		t.Errorf("relance : un seul mot → toujours « chat », époque 2 : %+v", r.words)
	}
}

func TestPicto192_NewQuestionClearsPreparation(t *testing.T) {
	r := newPictoRig(t)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	r.e.Ready("other", &Question{ID: "other", Type: QuestionTypeSpeedy, Question: "?", Answer: "!"})
	st := r.st()
	if st.PictoTeam != "" || st.PictoDrawer != "" || len(st.PictoCandidates) != 0 || st.PictoWordsCount != 0 {
		t.Errorf("préparation à vider au changement de question : %+v", st.PictoCandidates)
	}
}

func TestPicto192_ButtonsIgnored(t *testing.T) {
	r := newPictoRig(t)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	r.e.StartImmediate(60)
	if r.e.ProcessButtonPress("b-red", 1, "A") {
		t.Error("un appui buzzer doit être ignoré pendant une manche PICTO")
	}
	if r.st().Phase != PhaseStarted {
		t.Errorf("la manche doit continuer, phase=%s", r.st().Phase)
	}
}

func TestPicto192_TeamRenameAndPurge(t *testing.T) {
	r := newPictoRig(t)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	r.e.RenameTeamState("red", "crimson")
	if r.st().PictoTeam != "crimson" {
		t.Errorf("PICTO_TEAM doit suivre le renommage : %q", r.st().PictoTeam)
	}
	r.e.PurgeTeamState("crimson")
	if st := r.st(); st.PictoTeam != "" || st.PictoDrawer != "" {
		t.Errorf("purge d'équipe : %q %q", st.PictoTeam, st.PictoDrawer)
	}
}

func TestPicto192_DeletedDrawerIsPurged(t *testing.T) {
	r := newPictoRig(t)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	r.e.DeleteBumper("v-alice")
	if r.st().PictoDrawer != "" {
		t.Error("le dessinateur supprimé doit être purgé")
	}
}

// Revue M3 : NEW_GAME vide le journal de feuille (callback de remise à zéro, époque courante).
func TestPicto192_NewGameResetsSheetJournal(t *testing.T) {
	r := newPictoRig(t)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	r.e.StartImmediate(60)
	before := len(r.resets)
	r.e.InitGameWith(NewGameOptions{})
	if len(r.resets) != before+1 || r.resets[len(r.resets)-1] != 1 {
		t.Errorf("NEW_GAME doit déclencher une remise à zéro du journal à l'époque 1 : %v", r.resets)
	}
	if st := r.st(); st.PictoTeam != "" || st.PictoDrawer != "" {
		t.Errorf("NEW_GAME vide aussi la préparation : %q %q", st.PictoTeam, st.PictoDrawer)
	}
}
