package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"buzzcontrol/internal/game"
)

// Pool HTTP handler factory (v13.1.0, issue fille #324, contracts/picto-pool.md
// §1, §4) — the RAFALE reservoir API (v8.0.0 — #197, contracts/rafale.md §9)
// generalised: ONE implementation of list / create-update / delete / reset /
// reset-all / pool count, instantiated for `rafale` (paths, shapes and error
// messages unchanged, byte for byte) and `picto` (/api/picto/*). Two options
// differ per instance: the paste endpoint (`batch`, Pool Picto only) and the
// round guard (409 on writes during a Picto round, Pool Picto only).

// rafaleQuestionResponse is the wire shape of one reservoir entry for
// GET <base> — USED is derived at read time from the used-flags file, never
// stored in the reservoir itself (contract §3.2/§9). Identical for both
// instances (ANSWER is always "" for picto).
type rafaleQuestionResponse struct {
	ID         string `json:"ID"`
	Question   string `json:"QUESTION"`
	Answer     string `json:"ANSWER"`
	Category   string `json:"CATEGORY"`
	Difficulty int    `json:"DIFFICULTY"`
	Used       bool   `json:"USED"`
}

// poolBody is the JSON body of a create/update request.
type poolBody struct {
	ID         string `json:"ID"`
	Question   string `json:"QUESTION"`
	Answer     string `json:"ANSWER"`
	Category   string `json:"CATEGORY"`
	Difficulty int    `json:"DIFFICULTY"`
}

// poolHTTPSpec parameterises the factory for one pool instance.
type poolHTTPSpec struct {
	key      string                                                    // "rafale" | "picto" — logs
	base     string                                                    // "/api/rafale/questions" | "/api/picto/words"
	poolPath string                                                    // "/api/rafale/pool" | "/api/picto/pool"
	pool     func(h *HTTPServer) *game.EntryPool                       // the instance
	create   func(h *HTTPServer, w http.ResponseWriter, body poolBody) // validate + upsert + respond
	batch    bool                                                      // POST <base>/batch (paste) — Pool Picto only
	letters  bool                                                      // GET <pool> adds LETTERS (RAFALE alphabetical, #282)
	// guard reports a round in progress that must refuse writes (409). nil = never.
	guard func(st game.GameState) (code, message string, blocked bool)
}

// poolGuardPicto refuses writes of the `picto` pool while a PICTO round is
// STARTED or PAUSED (contracts/picto-pool.md §4, same guard as
// rafale_round_in_progress parameterised). RAFALE_PICTO joins the predicate
// with its type (LOT-3).
func poolGuardPicto(st game.GameState) (string, string, bool) {
	if st.Question != nil && game.IsPictoFamily(st.Question.Type) &&
		(st.Phase == game.PhaseStarted || st.Phase == game.PhasePaused) {
		return "picto_round_in_progress", "Une manche Picto est en cours — le Pool Picto est verrouillé jusqu'à sa fin.", true
	}
	return "", "", false
}

func (s *poolHTTPSpec) refuseIfRound(h *HTTPServer, w http.ResponseWriter) bool {
	if s.guard == nil {
		return false
	}
	if code, msg, blocked := s.guard(h.engine.GetState()); blocked {
		writeGenerateError(w, http.StatusConflict, code, msg, 0)
		return true
	}
	return false
}

// registerPoolRoutes mounts the factory's routes for one instance. The exact
// "<base>/reset-all" is registered next to the "<base>/" prefix route:
// net/http's ServeMux always prefers the longer exact match (feature #197).
func (h *HTTPServer) registerPoolRoutes(s *poolHTTPSpec) {
	h.mux.HandleFunc(s.base, func(w http.ResponseWriter, r *http.Request) { h.poolQuestions(s, w, r) })
	h.mux.HandleFunc(s.base+"/reset-all", func(w http.ResponseWriter, r *http.Request) { h.poolResetAll(s, w, r) })
	h.mux.HandleFunc(s.base+"/", func(w http.ResponseWriter, r *http.Request) { h.poolByID(s, w, r) })
	h.mux.HandleFunc(s.poolPath, func(w http.ResponseWriter, r *http.Request) { h.poolCount(s, w, r) })
}

// raflePoolSpec / pictoPoolSpec are the two instances.
func raflePoolSpec() *poolHTTPSpec {
	return &poolHTTPSpec{
		key:      "rafale",
		base:     "/api/rafale/questions",
		poolPath: "/api/rafale/pool",
		pool:     func(h *HTTPServer) *game.EntryPool { return h.engine.RafalePool() },
		create:   (*HTTPServer).createRafaleEntry,
		letters:  true,
	}
}

func pictoPoolSpec() *poolHTTPSpec {
	return &poolHTTPSpec{
		key:      "picto",
		base:     "/api/picto/words",
		poolPath: "/api/picto/pool",
		pool:     func(h *HTTPServer) *game.EntryPool { return h.engine.PictoPool() },
		create:   (*HTTPServer).createPictoEntry,
		batch:    true,
		guard:    poolGuardPicto,
	}
}

// isKnownCategory reports whether key matches a hardcoded category or a
// custom category discovered on disk (data/files/categories/) — same
// vocabulary check as handleGetCategories, reused for pool entry validation
// (contract §9, POST 400 on an unknown category). Shared by both instances.
func (h *HTTPServer) isKnownCategory(key string) bool {
	for _, c := range hardcodedCategories {
		if c.Key == key {
			return true
		}
	}
	catDir := filepath.Join(h.dataDir, "files", "categories")
	entries, _ := os.ReadDir(catDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if toUpperSnakeCase(stem) == key {
			return true
		}
	}
	return false
}

// isKnownRafaleCategory is the historical name of isKnownCategory.
func (h *HTTPServer) isKnownRafaleCategory(key string) bool { return h.isKnownCategory(key) }

// poolQuestions handles GET (list, optional filters) and POST (create/update).
func (h *HTTPServer) poolQuestions(s *poolHTTPSpec, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// The list carries USED: during a Picto round the last USED word is the
		// secret word in play (review LOT-2, minor 1) — same guard as the writes.
		if s.refuseIfRound(h, w) {
			return
		}
		h.poolList(s, w, r)
	case http.MethodPost:
		if s.refuseIfRound(h, w) {
			return
		}
		h.poolPost(s, w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// poolList lists the reservoir, optionally filtered by ?categories=A,B (OR)
// and/or ?difficulty=N — contract §9.
func (h *HTTPServer) poolList(s *poolHTTPSpec, w http.ResponseWriter, r *http.Request) {
	var catFilter map[string]struct{}
	if raw := r.URL.Query().Get("categories"); raw != "" {
		catFilter = make(map[string]struct{})
		for _, c := range strings.Split(raw, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				catFilter[c] = struct{}{}
			}
		}
	}

	hasDiffFilter := false
	diffFilter := 0
	if raw := r.URL.Query().Get("difficulty"); raw != "" {
		d, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "Invalid difficulty", http.StatusBadRequest)
			return
		}
		diffFilter = d
		hasDiffFilter = true
	}

	questions, used := s.pool(h).Snapshot()

	result := make([]rafaleQuestionResponse, 0, len(questions))
	for _, q := range questions {
		if catFilter != nil {
			if _, ok := catFilter[string(q.Category)]; !ok {
				continue
			}
		}
		if hasDiffFilter && q.Difficulty != diffFilter {
			continue
		}
		result = append(result, rafaleQuestionResponse{
			ID:         q.ID,
			Question:   q.Question,
			Answer:     q.Answer,
			Category:   string(q.Category),
			Difficulty: q.Difficulty,
			Used:       used[q.ID],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"QUESTIONS": result,
		"TOTAL":     len(result),
	})
}

// poolPost decodes the JSON body (reservoir entries carry no media, arbitrage
// D3) and hands it to the instance's own validator/writer.
func (h *HTTPServer) poolPost(s *poolHTTPSpec, w http.ResponseWriter, r *http.Request) {
	var body poolBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	s.create(h, w, body)
}

// createRafaleEntry creates (no ID) or updates (ID present) one RAFALE
// reservoir question — contract rafale.md §9. Messages and order of checks
// are those of the pre-v13.1.0 handler.
func (h *HTTPServer) createRafaleEntry(w http.ResponseWriter, body poolBody) {
	if strings.TrimSpace(body.Question) == "" {
		http.Error(w, "QUESTION must not be empty", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Answer) == "" {
		http.Error(w, "ANSWER must not be empty", http.StatusBadRequest)
		return
	}
	// Length caps (contract rafale-ai-generation.md §5.3, feature #203) —
	// [BREAKING] mineur : ces plafonds n'existaient pas avant v8.1.0. Aucune
	// question existante n'est supprimée ni tronquée ; ils s'appliquent
	// uniquement à l'écriture (create/update depuis cet éditeur), et sont la
	// MÊME source (rafaleMaxQuestionRunes/rafaleMaxAnswerRunes,
	// ai_generator_rafale.go) que celle utilisée par la génération IA — un
	// texte accepté ici est garanti ré-écrivable depuis le chemin IA et
	// inversement.
	if n := rafaleRuneLen(body.Question); n > rafaleMaxQuestionRunes {
		http.Error(w, fmt.Sprintf("QUESTION must be at most %d characters (got %d)", rafaleMaxQuestionRunes, n), http.StatusBadRequest)
		return
	}
	if n := rafaleRuneLen(body.Answer); n > rafaleMaxAnswerRunes {
		http.Error(w, fmt.Sprintf("ANSWER must be at most %d characters (got %d)", rafaleMaxAnswerRunes, n), http.StatusBadRequest)
		return
	}
	if body.Difficulty < 1 || body.Difficulty > 3 {
		http.Error(w, "DIFFICULTY must be between 1 and 3", http.StatusBadRequest)
		return
	}
	if !h.isKnownCategory(body.Category) {
		http.Error(w, "Unknown CATEGORY", http.StatusBadRequest)
		return
	}

	saved, err := h.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		ID:         body.ID,
		Question:   body.Question,
		Answer:     body.Answer,
		Category:   game.QuestionCategory(body.Category),
		Difficulty: body.Difficulty,
	})
	if err != nil {
		log.Printf("[HTTP] Rafale question upsert failed: %v", err)
		http.Error(w, "Failed to save question", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ID": saved.ID})
}

// createPictoEntry creates (no ID) or updates (ID present) one Pool Picto
// word — contracts/picto-pool.md §2, §4. The word travels in QUESTION; ANSWER
// must be empty. 400 prefixed by the contract code (WORD_INVALID,
// CATEGORY_UNKNOWN, DIFFICULTY_INVALID, DUPLICATE, POOL_FULL).
func (h *HTTPServer) createPictoEntry(w http.ResponseWriter, body poolBody) {
	if body.Difficulty < 1 || body.Difficulty > 3 {
		http.Error(w, "DIFFICULTY_INVALID: DIFFICULTY must be between 1 and 3", http.StatusBadRequest)
		return
	}
	if !h.isKnownCategory(body.Category) {
		http.Error(w, "CATEGORY_UNKNOWN: unknown CATEGORY", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Answer) != "" {
		http.Error(w, game.PoolErrWordInvalid+": ANSWER must be empty for a Picto word", http.StatusBadRequest)
		return
	}
	pool := h.engine.PictoPool()
	cat := game.QuestionCategory(body.Category)

	if body.ID != "" {
		saved, ref, err := pool.UpdateWord(game.PictoWordFieldSet, body.ID, body.Question, cat, body.Difficulty)
		if err != nil {
			if err == game.ErrPoolEntryNotFoundForUpdate {
				http.Error(w, "Word not found", http.StatusNotFound)
				return
			}
			log.Printf("[HTTP] Picto word update failed: %v", err)
			http.Error(w, "Failed to save word", http.StatusInternalServerError)
			return
		}
		if ref != nil {
			http.Error(w, ref.Code+": "+ref.Err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ID": saved.ID})
		return
	}

	created, refused, err := pool.AddWords(game.PictoWordFieldSet, []string{body.Question}, cat, body.Difficulty)
	if err != nil {
		log.Printf("[HTTP] Picto word create failed: %v", err)
		http.Error(w, "Failed to save word", http.StatusInternalServerError)
		return
	}
	if len(refused) > 0 {
		http.Error(w, refused[0].Code+": "+refused[0].Err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ID": created[0].ID})
}

// poolBatchMaxWords bounds the paste endpoint (contracts/picto-pool.md §4).
const poolBatchMaxWords = 200

// poolBatch handles POST <base>/batch — the paste of a word list (M1 A,
// Pool Picto only): valid lines are created in ONE save, refusals are listed
// (never truncated), a duplicate inside the batch is refused from its 2nd
// occurrence.
func (h *HTTPServer) poolBatch(s *poolHTTPSpec, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	var body struct {
		Category   string   `json:"CATEGORY"`
		Difficulty int      `json:"DIFFICULTY"`
		Words      []string `json:"WORDS"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(body.Words) < 1 || len(body.Words) > poolBatchMaxWords {
		http.Error(w, fmt.Sprintf("WORDS must contain 1 to %d words", poolBatchMaxWords), http.StatusBadRequest)
		return
	}
	if body.Difficulty < 1 || body.Difficulty > 3 {
		http.Error(w, "DIFFICULTY_INVALID: DIFFICULTY must be between 1 and 3", http.StatusBadRequest)
		return
	}
	if !h.isKnownCategory(body.Category) {
		http.Error(w, "CATEGORY_UNKNOWN: unknown CATEGORY", http.StatusBadRequest)
		return
	}

	created, refused, err := s.pool(h).AddWords(game.PictoWordFieldSet, body.Words, game.QuestionCategory(body.Category), body.Difficulty)
	if err != nil {
		log.Printf("[HTTP] %s batch failed: %v", s.key, err)
		http.Error(w, "Failed to save words", http.StatusInternalServerError)
		return
	}

	type createdItem struct {
		ID       string `json:"ID"`
		Question string `json:"QUESTION"`
	}
	type refusedItem struct {
		Question string `json:"QUESTION"`
		Code     string `json:"CODE"`
	}
	out := struct {
		Created []createdItem `json:"CREATED"`
		Refused []refusedItem `json:"REFUSED"`
	}{Created: make([]createdItem, 0, len(created)), Refused: make([]refusedItem, 0, len(refused))}
	for _, c := range created {
		out.Created = append(out.Created, createdItem{ID: c.ID, Question: c.Question})
	}
	for _, rf := range refused {
		out.Refused = append(out.Refused, refusedItem{Question: rf.Word, Code: rf.Code})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// poolByID handles DELETE <base>/{id}, POST <base>/{id}/reset and (when the
// instance enables it) POST <base>/batch — contract §9. The "/reset" suffix
// (feature #197) is parsed here rather than given its own mux route.
func (h *HTTPServer) poolByID(s *poolHTTPSpec, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, s.base+"/")
	path = strings.TrimSuffix(path, "/")

	if s.batch && path == "batch" {
		if r.Method == http.MethodPost && s.refuseIfRound(h, w) {
			return
		}
		h.poolBatch(s, w, r)
		return
	}

	if id, ok := strings.CutSuffix(path, "/reset"); ok {
		h.poolResetOne(s, w, r, id)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.refuseIfRound(h, w) {
		return
	}

	id := path
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if err := s.pool(h).Delete(id); err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"DELETED": id})
}

// poolResetOne handles POST <base>/{id}/reset — contract §9, feature #197:
// makes ONE entry available again (removes it from the "already used" flag)
// without touching the reservoir itself.
func (h *HTTPServer) poolResetOne(s *poolHTTPSpec, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.refuseIfRound(h, w) {
		return
	}
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if err := s.pool(h).MarkAvailable(id); err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ID": id, "AVAILABLE": true})
}

// poolResetAll handles POST <base>/reset-all — contract §9, feature #197:
// makes the ENTIRE reservoir available again without touching its content —
// unlike the destructive /reset-select, which also deletes every entry.
func (h *HTTPServer) poolResetAll(s *poolHTTPSpec, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.refuseIfRound(h, w) {
		return
	}

	n, err := s.pool(h).ResetAllUsed()
	if err != nil {
		log.Printf("[HTTP] %s reset-all-used failed: %v", s.key, err)
		http.Error(w, "Failed to reset used flags", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"RESET": n})
}

// poolCount returns the pool count (available/used/total) for a
// categories/difficulties filter — contract §9, feeding the pre-round alert
// (§7.5). ?categories=A,B&difficulties=1,2 (plural, comma-separated, #216).
func (h *HTTPServer) poolCount(s *poolHTTPSpec, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var categories []string
	for _, c := range strings.Split(r.URL.Query().Get("categories"), ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			categories = append(categories, c)
		}
	}
	if len(categories) == 0 {
		http.Error(w, "categories required", http.StatusBadRequest)
		return
	}

	var difficulties []int
	for _, d := range strings.Split(r.URL.Query().Get("difficulties"), ",") {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		parsed, err := strconv.Atoi(d)
		if err != nil {
			http.Error(w, "Invalid difficulties", http.StatusBadRequest)
			return
		}
		difficulties = append(difficulties, parsed)
	}
	if len(difficulties) == 0 {
		http.Error(w, "difficulties required", http.StatusBadRequest)
		return
	}

	available, used, total := s.pool(h).Count(categories, difficulties)

	resp := map[string]interface{}{
		"AVAILABLE": available,
		"USED":      used,
		"TOTAL":     total,
	}
	if s.letters {
		// LETTERS (#282, contract rafale.md §15.6): A–Z of the available answers,
		// through the engine's single normalisation; always an array.
		resp["LETTERS"] = h.engine.RafaleAvailableLetters(categories, difficulties)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
