// Suite test-writer pour l'addendum v11.1 « Média indisponible = lancement
// bloqué » (#219/#236/#237, plan _work/reports/plan-20260923-101500.md
// rév. 8 §6 tâche 10, contract sound.md §10.8) : la gate T0 dans
// participantsConform (engine.go), SetQuestionSoundUnavailable (§10.8.7),
// et le contournement ForceReady() (§10.8.6, CA22/CA25/CA27).
//
// Convention de collision : préfixe tw219s pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet (⚠️ tw219s est
// déjà pris par internal/audio/oto_singleton_219_test.go — paquet
// DIFFÉRENT, aucune collision réelle possible, mais gardé cohérent avec la
// convention "un préfixe par fichier").
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package game

import "testing"

// ---------------------------------------------------------------------------
// participantsConform — la branche son, en isolation (fonction pure).
// ---------------------------------------------------------------------------

func TestParticipantsConform_SoundGate_PureFunction(t *testing.T) {
	tests := []struct {
		name        string
		sound       string
		unavail     string
		bypassed    bool
		wantConform bool
	}{
		{"pas de son : toujours conforme, quel que soit l'état son", "", "FILE", false, true},
		{"pas de son : conforme même avec un motif ET bypass", "", "OUTPUT", true, true},
		{"son présent, disponible (motif vide)", "/x.wav", "", false, true},
		{"son présent, indisponible (FILE), pas de bypass : bloqué", "/x.wav", "FILE", false, false},
		{"son présent, indisponible (DISABLED), pas de bypass : bloqué", "/x.wav", "DISABLED", false, false},
		{"son présent, indisponible (OUTPUT), pas de bypass : bloqué", "/x.wav", "OUTPUT", false, false},
		{"son présent, indisponible, bypass=true : débloqué (CA22)", "/x.wav", "FILE", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Sound: tt.sound}
			state := &GameState{QuestionSoundUnavailable: tt.unavail, SoundGateBypassed: tt.bypassed}
			got := participantsConform(q, state)
			if got != tt.wantConform {
				t.Errorf("participantsConform(sound=%q, unavail=%q, bypassed=%v) = %v, attendu %v",
					tt.sound, tt.unavail, tt.bypassed, got, tt.wantConform)
			}
		})
	}
}

// TestParticipantsConform_SoundGate_CombinedWithParticipants_CA27 verrouille
// la limite du contournement (CA27, arbitrage #172 B5) au niveau de la
// fonction pure elle-même : le bypass son ne lève JAMAIS la branche
// participants — les deux conditions sont combinées par ET, jamais par OU
// (commentaire normatif de participantsConform).
func TestParticipantsConform_SoundGate_CombinedWithParticipants_CA27(t *testing.T) {
	memorySolo := &Question{ID: "q1", Type: QuestionTypeMemory, Sound: "/x.wav", TypedContent: TypedContent{MemoryMode: "SOLO"}}

	t.Run("participants non conformes (0 équipe) + bypass=true : reste bloqué (CA27)", func(t *testing.T) {
		state := &GameState{QuestionSoundUnavailable: "FILE", SoundGateBypassed: true, MemoryParticipatingTeams: nil}
		if participantsConform(memorySolo, state) {
			t.Fatal("CA27 violé : le bypass son ne doit JAMAIS lever la branche participants — une question MEMORY SOLO sans équipe doit rester bloquée, même avec SoundGateBypassed=true")
		}
	})

	t.Run("participants conformes (1 équipe) + bypass=true : débloqué (bypass utile pour SA branche uniquement)", func(t *testing.T) {
		state := &GameState{QuestionSoundUnavailable: "FILE", SoundGateBypassed: true, MemoryParticipatingTeams: []string{"TeamA"}}
		if !participantsConform(memorySolo, state) {
			t.Fatal("participants conformes ET bypass son levé : la question doit être conforme (la branche son ne doit plus bloquer, la branche participants est déjà satisfaite par ailleurs)")
		}
	})

	t.Run("participants conformes (1 équipe) + son indisponible + bypass=false : reste bloqué (branche son seule suffit à refuser)", func(t *testing.T) {
		state := &GameState{QuestionSoundUnavailable: "FILE", SoundGateBypassed: false, MemoryParticipatingTeams: []string{"TeamA"}}
		if participantsConform(memorySolo, state) {
			t.Fatal("participants conformes ne doit PAS suffire à lever la gate son sans le bypass explicite")
		}
	})

	t.Run("participants non conformes (0 équipe) + son disponible : reste bloqué (branche participants seule suffit à refuser)", func(t *testing.T) {
		state := &GameState{QuestionSoundUnavailable: "", SoundGateBypassed: false, MemoryParticipatingTeams: nil}
		if participantsConform(memorySolo, state) {
			t.Fatal("un son disponible ne doit PAS suffire à satisfaire la conformité si les participants ne le sont pas")
		}
	})
}

// TestParticipantsConform_SoundGate_EmptySound_NeverAffectsAnyType_CA_NonRegression172
// verrouille R22 (« sortie immédiate si Question.Sound == "" ») pour
// CHAQUE type de jeu — CA sans son ne doit JAMAIS être affectée par
// QuestionSoundUnavailable/SoundGateBypassed, quel que soit leur contenu.
func TestParticipantsConform_SoundGate_EmptySound_NeverAffectsAnyType_CA_NonRegression172(t *testing.T) {
	baseline := []struct {
		name     string
		question *Question
		state    *GameState
	}{
		{"SPEEDY, aucune contrainte", &Question{Type: QuestionTypeSpeedy}, &GameState{}},
		{"QCM, aucune contrainte", &Question{Type: QuestionTypeQCM}, &GameState{}},
		{"ARDOISE, aucune contrainte", &Question{Type: QuestionTypeArdoise}, &GameState{}},
		{"MEMORY SOLO, non conforme (0 équipe)", &Question{Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: "SOLO"}}, &GameState{}},
		{"MEMORY SOLO, conforme (1 équipe)", &Question{Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: "SOLO"}}, &GameState{MemoryParticipatingTeams: []string{"TeamA"}}},
		{"MEMOTION SOLO, non conforme", &Question{Type: QuestionTypeMemotion}, &GameState{}},
		{"RAFALE, non conforme (aucune catégorie)", &Question{Type: QuestionTypeRafale}, &GameState{}},
	}

	for _, b := range baseline {
		t.Run(b.name, func(t *testing.T) {
			// Question SANS son — Sound reste "" pour ce sous-test.
			b.question.Sound = ""
			want := participantsConform(b.question, b.state)

			for _, reason := range []string{"", "DISABLED", "OUTPUT", "FILE"} {
				for _, bypass := range []bool{false, true} {
					b.state.QuestionSoundUnavailable = reason
					b.state.SoundGateBypassed = bypass
					got := participantsConform(b.question, b.state)
					if got != want {
						t.Errorf("R22 violé (%s) : QuestionSoundUnavailable=%q/SoundGateBypassed=%v a changé le verdict d'une question SANS SON — got %v, attendu %v (inchangé)",
							b.name, reason, bypass, got, want)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SetQuestionSoundUnavailable — idempotence, et gate bidirectionnelle réelle
// via reevaluatePrepareReadyUnsafe (CA19).
// ---------------------------------------------------------------------------

func TestSetQuestionSoundUnavailable_Idempotent_NoSpuriousCallback(t *testing.T) {
	e := NewEngine()
	q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x", Sound: "/x.wav"}
	e.Ready(q.ID, q)

	calls := 0
	e.OnStateChange = func(GamePhase) { calls++ }

	e.SetQuestionSoundUnavailable("FILE")
	firstCallCount := calls
	if firstCallCount == 0 {
		// Un changement "" -> "FILE" ne provoque PAS forcément de transition
		// de phase tant qu'on est encore en PREPARE (rien à réévaluer vers
		// READY sans buzzers prêts) — c'est attendu, pas une erreur de ce
		// test : voir le test dédié ci-dessous pour la transition réelle.
		t.Log("aucun callback à la première levée du motif — attendu si aucune transition de phase n'a eu lieu")
	}

	e.SetQuestionSoundUnavailable("FILE") // même raison — doit être un pur no-op
	if calls != firstCallCount {
		t.Errorf("un second SetQuestionSoundUnavailable avec la MÊME raison ne doit déclencher AUCUN callback supplémentaire — got %d appels après le premier (%d), attendu inchangé", calls, firstCallCount)
	}
	if e.GetState().QuestionSoundUnavailable != "FILE" {
		t.Errorf("QUESTION_SOUND_UNAVAILABLE doit rester %q, got %q", "FILE", e.GetState().QuestionSoundUnavailable)
	}
}

// TestSetQuestionSoundUnavailable_ReadyToPrepare_BlocksAutomatically vérifie
// le premier sens de la gate bidirectionnelle : une question déjà en READY
// retombe en PREPARE dès que le motif d'indisponibilité apparaît.
func TestSetQuestionSoundUnavailable_ReadyToPrepare_BlocksAutomatically(t *testing.T) {
	e := NewEngine()
	q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x", Sound: "/x.wav"}
	e.Ready(q.ID, q)
	e.TransitionToReady()
	if e.GetPhase() != PhaseReady {
		t.Fatalf("setup invalide : TransitionToReady() n'a pas atteint PhaseReady, got %s", e.GetPhase())
	}

	var got GamePhase
	e.OnStateChange = func(p GamePhase) { got = p }
	e.SetQuestionSoundUnavailable("FILE")

	if e.GetPhase() != PhasePrepare {
		t.Errorf("une question à son devenue indisponible doit repasser de READY à PREPARE, got phase=%s", e.GetPhase())
	}
	if got != PhasePrepare {
		t.Errorf("le callback OnStateChange doit être invoqué avec PhasePrepare, got %s", got)
	}
	if e.GetState().QuestionSoundUnavailable != "FILE" {
		t.Errorf("QUESTION_SOUND_UNAVAILABLE doit refléter %q, got %q", "FILE", e.GetState().QuestionSoundUnavailable)
	}
}

// TestSetQuestionSoundUnavailable_PrepareToReady_CA19_ReversibilityAutomatic
// est LE test de CA19 : la question repasse automatiquement en READY dès
// que la disponibilité revient, sans le moindre geste supplémentaire
// (buzzers déjà marqués prêts une seule fois, jamais reproposés).
func TestSetQuestionSoundUnavailable_PrepareToReady_CA19_ReversibilityAutomatic(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"TeamA": {Name: "TeamA"}})
	e.UpdateBumper("b1", map[string]interface{}{"NAME": "B1", "TEAM": "TeamA"})

	q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x", Sound: "/x.wav"}
	e.Ready(q.ID, q)
	e.SetBumperReady("b1") // buzzer prêt UNE SEULE FOIS — jamais reproposé plus bas
	if !e.AreAllTeamsReady() {
		t.Fatal("setup invalide : AreAllTeamsReady() devrait être true après SetBumperReady")
	}

	// Le média devient indisponible pendant que le jeu attend déjà — reste
	// en PREPARE (branche son bloque, malgré des buzzers déjà prêts).
	e.SetQuestionSoundUnavailable("FILE")
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("setup invalide : la question devrait rester en PREPARE tant que le son est indisponible, got %s", e.GetPhase())
	}

	var got GamePhase
	e.OnStateChange = func(p GamePhase) { got = p }

	// Le média redevient disponible — AUCUN geste supplémentaire (pas de
	// nouveau SetBumperReady) : la reévaluation doit suffire (CA19).
	e.SetQuestionSoundUnavailable("")

	if e.GetPhase() != PhaseReady {
		t.Errorf("CA19 violé : la question doit repasser AUTOMATIQUEMENT en READY dès que le média redevient disponible, got phase=%s", e.GetPhase())
	}
	if got != PhaseReady {
		t.Errorf("le callback OnStateChange doit être invoqué avec PhaseReady, got %s", got)
	}
}

// ---------------------------------------------------------------------------
// ForceReady — l'échappatoire (CA22), sa limite (CA27), et la non-persistance
// du contournement (CA25).
// ---------------------------------------------------------------------------

func TestForceReady_CA22_BypassesSoundGate_ForQuestionWithSound(t *testing.T) {
	e := NewEngine()
	q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x", Sound: "/x.wav"}
	e.Ready(q.ID, q)
	e.SetQuestionSoundUnavailable("FILE")
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("setup invalide : la question devrait être bloquée en PREPARE, got %s", e.GetPhase())
	}

	e.ForceReady()

	if e.GetPhase() != PhaseReady {
		t.Errorf("CA22 violé : ForceReady() doit faire passer une question à média indisponible en READY, got phase=%s", e.GetPhase())
	}
	if !e.GetState().SoundGateBypassed {
		t.Error("SOUND_GATE_BYPASSED doit être true après ForceReady() sur une question à son indisponible")
	}
}

func TestForceReady_CA27_NeverBypassesParticipantsGate(t *testing.T) {
	e := NewEngine()
	// MEMORY SOLO, son indisponible ET aucune équipe sélectionnée — les DEUX
	// conditions de blocage sont réunies à dessein.
	q := &Question{ID: "q1", Type: QuestionTypeMemory, Answer: "x", Sound: "/x.wav",
		TypedContent: TypedContent{MemoryMode: "SOLO"}}
	e.Ready(q.ID, q)
	e.SetQuestionSoundUnavailable("FILE")
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("setup invalide : devrait être bloquée en PREPARE, got %s", e.GetPhase())
	}

	e.ForceReady()

	if e.GetPhase() != PhasePrepare {
		t.Errorf("CA27 violé (arbitrage #172 B5) : ForceReady() ne doit JAMAIS faire passer en READY une question MEMORY SOLO sans équipe, même avec un son indisponible — got phase=%s", e.GetPhase())
	}
	// Le drapeau son est bien levé en interne (ForceReady le pose
	// inconditionnellement AVANT le contrôle participants) — mais son EFFET
	// reste sans portée puisque la question n'atteint jamais READY.
	if !e.GetState().SoundGateBypassed {
		t.Error("setup invalide : SOUND_GATE_BYPASSED devrait être true (posé avant le contrôle participants), même si la question reste bloquée pour une AUTRE raison")
	}
}

func TestForceReady_CA25_BypassNeverOutlivesTheQuestion(t *testing.T) {
	t.Run("remis à zéro par Ready() (nouvelle question)", func(t *testing.T) {
		e := NewEngine()
		q1 := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x", Sound: "/x.wav"}
		e.Ready(q1.ID, q1)
		e.SetQuestionSoundUnavailable("FILE")
		e.ForceReady()
		if !e.GetState().SoundGateBypassed {
			t.Fatal("setup invalide : bypass devrait être true après ForceReady()")
		}

		q2 := &Question{ID: "q2", Type: QuestionTypeSpeedy, Answer: "y", Sound: "/y.wav"}
		e.Ready(q2.ID, q2)

		if e.GetState().SoundGateBypassed {
			t.Error("CA25 violé : SOUND_GATE_BYPASSED doit être remis à false par Ready() — le contournement ne doit jamais survivre à un changement de question")
		}
		if e.GetState().QuestionSoundUnavailable != "" {
			t.Errorf("QUESTION_SOUND_UNAVAILABLE doit être remis à \"\" par Ready(), got %q", e.GetState().QuestionSoundUnavailable)
		}
	})

	t.Run("remis à zéro par Stop()", func(t *testing.T) {
		e := NewEngine()
		q := &Question{ID: "q1", Type: QuestionTypeSpeedy, Answer: "x", Sound: "/x.wav"}
		e.Ready(q.ID, q)
		e.SetQuestionSoundUnavailable("FILE")
		e.ForceReady()
		e.StartImmediate(20)
		defer e.Stop()
		if !e.GetState().SoundGateBypassed {
			t.Fatal("setup invalide : bypass devrait être true avant Stop()")
		}

		e.Stop()

		if e.GetState().SoundGateBypassed {
			t.Error("CA25 violé : SOUND_GATE_BYPASSED doit être remis à false par Stop()")
		}
		if e.GetState().QuestionSoundUnavailable != "" {
			t.Errorf("QUESTION_SOUND_UNAVAILABLE doit être remis à \"\" par Stop(), got %q", e.GetState().QuestionSoundUnavailable)
		}
	})
}
