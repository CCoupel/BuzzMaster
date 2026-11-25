package game

import (
	"fmt"
	"strings"
	"testing"
)

// LOT-2 (v13.1.0, issue fille #324) — T16 : pool Picto côté moteur (contracts/picto-pool.md §2, §3).
// Validateur (règles du mot, doublons par catégorie sur la clé repliée, plafond 5 000, refus listés jamais tronqués)
// et pioche stratifiée en PROPRIÉTÉS (le générateur global n'est pas injectable, cf. #188 : jamais un mot attendu).

func newPictoPoolEngine(t *testing.T) *Engine {
	t.Helper()
	e := NewEngine()
	dir := t.TempDir()
	e.SetPictoPath(dir + "/files/picto/reservoir.json")
	e.SetPictoUsedPath(dir + "/config/picto_used.json")
	return e
}

func TestPictoPool_PlafondEstCinqMille(t *testing.T) {
	if PictoPoolMaxEntries != 5000 || PictoWordFieldSet.MaxEntries != 5000 {
		t.Fatalf("plafond du Pool Picto = %d / %d, attendu 5000", PictoPoolMaxEntries, PictoWordFieldSet.MaxEntries)
	}
}

func TestPictoPool_AddWords_StoresWordInQuestion_AnswerEmpty_IdPrefixP(t *testing.T) {
	e := newPictoPoolEngine(t)
	created, refused, err := e.PictoPool().AddWords(PictoWordFieldSet, []string{"  Éléphant  ", "pont-levis"}, CategoryHistory, 2)
	if err != nil || len(refused) != 0 {
		t.Fatalf("err=%v refused=%v", err, refused)
	}
	if created[0].ID != "p-001" || created[1].ID != "p-002" {
		t.Errorf("IDs = %s, %s (préfixe p-NNN attendu)", created[0].ID, created[1].ID)
	}
	if created[0].Question != "Éléphant" || created[0].Answer != "" {
		t.Errorf("mot nettoyé dans QUESTION, ANSWER vide : %+v", created[0])
	}
}

func TestPictoPool_AddWords_RefusalsAreListedWithCodes_NeverTruncated(t *testing.T) {
	e := newPictoPoolEngine(t)
	long := strings.Repeat("a", 41)
	created, refused, err := e.PictoPool().AddWords(PictoWordFieldSet, []string{"chat", "CHAT", "chat<3", "", long, "Chat", "chien"}, CategoryHistory, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || created[0].Question != "chat" || created[1].Question != "chien" {
		t.Fatalf("créés = %+v", created)
	}
	codes := map[string]int{}
	for _, r := range refused {
		codes[r.Code]++
	}
	if codes[PoolErrDuplicate] != 2 || codes[PoolErrWordInvalid] != 3 || len(refused) != 5 {
		t.Errorf("refus = %v : attendu 2 DUPLICATE (2ᵉ et 3ᵉ occurrence repliées) et 3 WORD_INVALID", codes)
	}
	// le mot de 41 caractères n'est PAS tronqué à 40
	for _, c := range created {
		if len([]rune(c.Question)) > 40 {
			t.Errorf("mot tronqué accepté : %q", c.Question)
		}
	}
}

func TestPictoPool_Duplicates_AreCheckedPerCategoryOnly_FoldedKey(t *testing.T) {
	e := newPictoPoolEngine(t)
	if _, ref, _ := e.PictoPool().AddWords(PictoWordFieldSet, []string{"Éléphant"}, CategoryHistory, 1); len(ref) != 0 {
		t.Fatal(ref)
	}
	_, ref, _ := e.PictoPool().AddWords(PictoWordFieldSet, []string{"ELEPHANT"}, CategoryHistory, 3)
	if len(ref) != 1 || ref[0].Code != PoolErrDuplicate {
		t.Errorf("« ELEPHANT » vs « Éléphant » même catégorie : %v", ref)
	}
	created, ref, _ := e.PictoPool().AddWords(PictoWordFieldSet, []string{"elephant"}, CategoryScience, 1)
	if len(created) != 1 || len(ref) != 0 {
		t.Errorf("même mot dans une autre catégorie : created=%v refused=%v", created, ref)
	}
}

func TestPictoPool_PoolFull_RemainingWordsRefusedPOOL_FULL(t *testing.T) {
	e := newPictoPoolEngine(t)
	fs := PoolWordFieldSet{MaxEntries: 3} // même mécanisme que le plafond réel, borne réduite pour le test
	words := []string{"chat", "chien", "lion", "tigre", "ours"}
	created, refused, err := e.PictoPool().AddWords(fs, words, CategoryHistory, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 3 || len(refused) != 2 {
		t.Fatalf("created=%d refused=%d", len(created), len(refused))
	}
	for _, r := range refused {
		if r.Code != PoolErrPoolFull {
			t.Errorf("code = %s, attendu POOL_FULL", r.Code)
		}
	}
	// un ajout ultérieur reste refusé tant que le pool est plein
	_, ref2, _ := e.PictoPool().AddWords(fs, []string{"zèbre"}, CategoryHistory, 1)
	if len(ref2) != 1 || ref2[0].Code != PoolErrPoolFull {
		t.Errorf("ajout sur pool plein : %v", ref2)
	}
}

func TestPictoPool_UpdateWord_ExcludesItselfFromDuplicateScan_UnknownID(t *testing.T) {
	e := newPictoPoolEngine(t)
	created, _, _ := e.PictoPool().AddWords(PictoWordFieldSet, []string{"chat", "chien"}, CategoryHistory, 1)
	out, ref, err := e.PictoPool().UpdateWord(PictoWordFieldSet, created[0].ID, "CHAT", CategoryHistory, 2)
	if err != nil || ref != nil || out.Question != "CHAT" || out.Difficulty != 2 {
		t.Errorf("mise à jour de soi-même : %+v %v %v", out, ref, err)
	}
	if _, ref, _ := e.PictoPool().UpdateWord(PictoWordFieldSet, created[0].ID, "chien", CategoryHistory, 1); ref == nil || ref.Code != PoolErrDuplicate {
		t.Errorf("renommer en doublon : %v", ref)
	}
	if _, _, err := e.PictoPool().UpdateWord(PictoWordFieldSet, "p-999", "lion", CategoryHistory, 1); err == nil {
		t.Errorf("ID inconnu : erreur attendue")
	}
}

// ---- pioche stratifiée : propriétés ----

func seedPictoCouples(t *testing.T, e *Engine, perCouple int, couples [][2]interface{}) {
	t.Helper()
	for _, c := range couples {
		words := make([]string, perCouple)
		for i := range words {
			words[i] = fmt.Sprintf("mot%s%dn%d", string(c[0].(QuestionCategory))[:2], c[1].(int), i)
		}
		if _, ref, err := e.PictoPool().AddWords(PictoWordFieldSet, words, c[0].(QuestionCategory), c[1].(int)); err != nil || len(ref) != 0 {
			t.Fatalf("seed : %v %v", err, ref)
		}
	}
}

func drawAll(t *testing.T, e *Engine, cats []string, diffs []int, n int) []*PoolEntry {
	t.Helper()
	p := e.PictoPool()
	p.mu.Lock()
	defer p.mu.Unlock()
	st := newPoolDrawState(p, cats, diffs)
	var out []*PoolEntry
	for i := 0; i < n; i++ {
		d, err := st.next(p)
		if err != nil {
			return out
		}
		out = append(out, d)
	}
	return out
}

func TestPictoPool_Draw_PropertyEveryDrawWithinFilter_AndNeverTwiceUntilExhausted(t *testing.T) {
	e := newPictoPoolEngine(t)
	seedPictoCouples(t, e, 4, [][2]interface{}{{CategoryHistory, 1}, {CategoryHistory, 2}, {CategoryScience, 1}, {CategorySports, 3}})
	got := drawAll(t, e, []string{"HISTORY", "SCIENCE"}, []int{1, 2}, 100)
	// couples actifs : HISTORY×1, HISTORY×2, SCIENCE×1 (SCIENCE×2 vide, exclu) = 12 mots tirables
	if len(got) != 12 {
		t.Fatalf("tirages = %d, attendu 12 (pool filtré épuisé puis arrêt)", len(got))
	}
	seen := map[string]bool{}
	for _, d := range got {
		if d.Category == CategorySports {
			t.Errorf("tirage hors filtre : %+v", d)
		}
		if seen[d.ID] {
			t.Errorf("%s tiré deux fois", d.ID)
		}
		seen[d.ID] = true
	}
}

func TestPictoPool_Draw_StratifiedPermutation_EachActiveCoupleOncePerCycle(t *testing.T) {
	e := newPictoPoolEngine(t)
	seedPictoCouples(t, e, 6, [][2]interface{}{{CategoryHistory, 1}, {CategoryHistory, 2}, {CategoryScience, 1}})
	got := drawAll(t, e, []string{"HISTORY", "SCIENCE"}, []int{1, 2}, 9)
	if len(got) != 9 {
		t.Fatalf("tirages = %d", len(got))
	}
	for cycle := 0; cycle < 3; cycle++ {
		couples := map[string]int{}
		for _, d := range got[cycle*3 : cycle*3+3] {
			couples[fmt.Sprintf("%s/%d", d.Category, d.Difficulty)]++
		}
		if len(couples) != 3 {
			t.Errorf("cycle %d : couples %v — une permutation visite chaque couple actif exactement une fois", cycle, couples)
		}
	}
}

func TestPictoPool_Draw_ExhaustedCoupleIsRemoved_NoBlock_ThenPoolEmpty(t *testing.T) {
	e := newPictoPoolEngine(t)
	seedPictoCouples(t, e, 1, [][2]interface{}{{CategoryHistory, 1}})
	seedPictoCouples(t, e, 5, [][2]interface{}{{CategoryScience, 1}})
	got := drawAll(t, e, []string{"HISTORY", "SCIENCE"}, []int{1}, 50)
	if len(got) != 6 {
		t.Fatalf("tirages = %d, attendu 6 : HISTORY épuisé en cours de route sort de la liste active, SCIENCE continue", len(got))
	}
}

func TestPictoPool_Draw_MarksUsedAtDraw_ResetMakesAvailable(t *testing.T) {
	e := newPictoPoolEngine(t)
	seedPictoCouples(t, e, 2, [][2]interface{}{{CategoryHistory, 1}})
	if got := drawAll(t, e, []string{"HISTORY"}, []int{1}, 10); len(got) != 2 {
		t.Fatalf("tirages = %d", len(got))
	}
	avail, used, total := e.PictoPool().Count([]string{"HISTORY"}, []int{1})
	if avail != 0 || used != 2 || total != 2 {
		t.Errorf("après tirage : disponibles=%d utilisés=%d total=%d", avail, used, total)
	}
	if n, err := e.PictoPool().ResetAllUsed(); err != nil || n != 2 {
		t.Errorf("reset-all = %d, %v", n, err)
	}
	if avail, _, _ := e.PictoPool().Count([]string{"HISTORY"}, []int{1}); avail != 2 {
		t.Errorf("après remise à zéro : disponibles = %d", avail)
	}
}

func TestPictoPool_TwoInstances_DoNotShareEntriesNorUsedMarks(t *testing.T) {
	e := newPictoPoolEngine(t)
	e.SetRafalePath(t.TempDir() + "/r.json")
	e.SetRafaleUsedPath(t.TempDir() + "/ru.json")
	if _, err := e.UpsertRafaleQuestion(RafaleQuestion{Question: "Q ?", Answer: "A", Category: CategoryHistory, Difficulty: 1}); err != nil {
		t.Fatal(err)
	}
	seedPictoCouples(t, e, 1, [][2]interface{}{{CategoryHistory, 1}})
	if _, _, total := e.PictoPool().Count([]string{"HISTORY"}, []int{1}); total != 1 {
		t.Errorf("Pool Picto total = %d", total)
	}
	if _, _, total := e.RafalePool().Count([]string{"HISTORY"}, []int{1}); total != 1 {
		t.Errorf("Pool Speedy total = %d", total)
	}
	if got := drawAll(t, e, []string{"HISTORY"}, []int{1}, 5); len(got) != 1 {
		t.Fatalf("tirage Picto = %d", len(got))
	}
	if avail, _, _ := e.RafalePool().Count([]string{"HISTORY"}, []int{1}); avail != 1 {
		t.Errorf("le tirage Picto ne doit pas marquer une entrée du Pool Speedy (disponibles = %d)", avail)
	}
}

func TestPictoPool_ClearUsed_IsNewGameReset_ForBothInstances(t *testing.T) {
	e := newPictoPoolEngine(t)
	seedPictoCouples(t, e, 1, [][2]interface{}{{CategoryHistory, 1}})
	drawAll(t, e, []string{"HISTORY"}, []int{1}, 1)
	e.ClearPictoUsed()
	if avail, _, total := e.PictoPool().Count([]string{"HISTORY"}, []int{1}); avail != 1 || total != 1 {
		t.Errorf("après ClearPictoUsed : disponibles=%d total=%d (pool conservé, marques effacées)", avail, total)
	}
}

func TestPictoPool_NewGame_ResetQuestions_ClearsBothUsedMarks_KeepsBothPools(t *testing.T) {
	e := NewEngine() // en mémoire seulement : NEW_GAME persiste en arrière-plan, ce qui courrait contre le nettoyage de TempDir
	if _, err := e.UpsertRafaleQuestion(RafaleQuestion{Question: "Q ?", Answer: "A", Category: CategoryHistory, Difficulty: 1}); err != nil {
		t.Fatal(err)
	}
	seedPictoCouples(t, e, 1, [][2]interface{}{{CategoryHistory, 1}})
	drawAll(t, e, []string{"HISTORY"}, []int{1}, 1)
	drawRafaleQuestionNoAutoSave(t, e, "HISTORY", 1) // pas de sauvegarde en arrière-plan : elle courrait contre le nettoyage de TempDir
	e.InitGameWith(NewGameOptions{ResetQuestions: true})
	for name, p := range map[string]*EntryPool{"Pool Speedy": e.RafalePool(), "Pool Picto": e.PictoPool()} {
		avail, used, total := p.Count([]string{"HISTORY"}, []int{1})
		if avail != 1 || used != 0 || total != 1 {
			t.Errorf("%s après NEW_GAME : disponibles=%d utilisés=%d total=%d (marques effacées, entrées conservées)", name, avail, used, total)
		}
	}
}
