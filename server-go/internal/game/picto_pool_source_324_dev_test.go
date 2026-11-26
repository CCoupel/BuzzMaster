package game

import "testing"

// #324 (v13.1.0, LOT-2 T8) — source RESERVOIR du type PICTO : garde READY / START (POOL_EMPTY), tirage stratifié
// avec marquage « utilisé », indice de catégorie. Propriétés uniquement (le générateur global n'est pas injectable).

func newPictoPoolRig(t *testing.T, entries ...PoolEntry) *pictoRig {
	t.Helper()
	r := newPictoRig(t)
	for _, en := range entries {
		cp := en
		r.e.pictoPool.entries[en.ID] = &cp
	}
	q := &Question{ID: "pq2", Question: "Picto", Type: QuestionTypePicto, Category: "ANIMALS", Time: "90",
		TypedContent: TypedContent{PictoSource: PictoSourceReservoir, PictoCategories: []string{"HISTORY"}, PictoDifficulties: []int{1, 2}}}
	r.e.Ready("pq2", q)
	for id := range r.e.GetTeamsAndBumpers().Bumpers {
		r.e.SetBumperReady(id)
	}
	return r
}

func TestPictoPool324_Reservoir_EmptyPool_BlocksReadyAndStart(t *testing.T) {
	r := newPictoPoolRig(t, PoolEntry{ID: "p-001", Question: "chat", Category: "SCIENCE", Difficulty: 1}) // hors filtre
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if st := r.st(); st.Phase != PhasePrepare || st.PictoPoolAvailable != 0 || st.PictoWordsCount != 0 {
		t.Fatalf("pool vide : phase=%s available=%d wordsCount=%d (attendu PREPARE, 0, 0)", st.Phase, st.PictoPoolAvailable, st.PictoWordsCount)
	}
	if st := r.st(); st.PictoWordCategory != "ANIMALS" {
		t.Errorf("indice en préparation = catégorie de la question, got %q", st.PictoWordCategory)
	}
	// un mot arrive dans le pool : la garde READY relit le compte frais
	r.e.pictoPool.entries["p-002"] = &PoolEntry{ID: "p-002", Question: "épée", Category: "HISTORY", Difficulty: 2}
	if !r.e.ParticipantsConform() {
		t.Error("avec un mot disponible dans le filtre, la question est conforme")
	}
}

func TestPictoPool324_Reservoir_StartDrawsFromFilter_MarksUsed_NoRepeat(t *testing.T) {
	r := newPictoPoolRig(t,
		PoolEntry{ID: "p-001", Question: "épée", Category: "HISTORY", Difficulty: 1},
		PoolEntry{ID: "p-002", Question: "château", Category: "HISTORY", Difficulty: 2},
		PoolEntry{ID: "p-003", Question: "chat", Category: "SCIENCE", Difficulty: 1},
	)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoDrawDrawer(); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		r.e.StartImmediate(60)
		info, ok := r.e.PictoWord()
		if !ok || (info.Word != "épée" && info.Word != "château") || info.Category != "HISTORY" {
			t.Fatalf("mot tiré hors filtre ou vide : %+v ok=%v", info, ok)
		}
		if seen[info.Word] {
			t.Fatalf("mot %q tiré deux fois alors qu'il est marqué utilisé", info.Word)
		}
		seen[info.Word] = true
		if got := r.st().PictoWordCategory; got != "HISTORY" {
			t.Errorf("PICTO_WORD_CATEGORY = %q", got)
		}
		r.e.Stop()
	}
	if r.st().PictoPoolAvailable != 0 {
		t.Errorf("tous les mots du filtre sont utilisés : PICTO_POOL_AVAILABLE = %d", r.st().PictoPoolAvailable)
	}
	if r.e.pictoPool.used["p-003"] {
		t.Error("un mot hors filtre ne doit jamais être marqué utilisé")
	}
	wantPictoErr(t, r.e.PictoStartCheck(), PictoErrPoolEmpty)
}

func TestPictoPool324_WordsSource_UnchangedByDefault(t *testing.T) {
	r := newPictoRig(t, "chat", "chien")
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if st := r.st(); st.PictoWordsCount != 2 || st.PictoPoolAvailable != 2 {
		t.Errorf("source WORDS : count=%d available=%d", st.PictoWordsCount, st.PictoPoolAvailable)
	}
	if len(r.e.pictoPool.used) != 0 {
		t.Error("la source WORDS ne marque rien dans le pool")
	}
}
