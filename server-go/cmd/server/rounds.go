package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Rounds (v11.2, #245) — ROUNDS_UPDATE and the helpers shared by the play-order code.
// Contracts: models.md §Manches, websocket-actions.md §ROUNDS_UPDATE.

const (
	maxRounds        = 50
	maxRoundNameRune = 60
)

func (a *App) questionsDirPath() string {
	d := a.config.Storage.QuestionsDir
	if d == "" {
		d = "./data/files/questions"
	}
	return d
}

// reloadRounds re-reads rounds.json into GameState.ROUNDS (startup, restore).
func (a *App) reloadRounds() {
	a.engine.SetRounds(server.LoadRoundsFile(a.questionsDirPath()).Rounds)
}

// currentRounds returns the rounds as stored in rounds.json (the source of truth, read fresh:
// an edit of the file — restore, tests — is seen at once) and keeps GameState.ROUNDS in step.
func (a *App) currentRounds() []game.Round {
	rounds := server.LoadRoundsFile(a.questionsDirPath()).Rounds
	have := a.engine.GetRounds()
	same := len(have) == len(rounds)
	for i := 0; same && i < len(rounds); i++ {
		same = have[i] == rounds[i]
	}
	if !same {
		a.engine.SetRounds(rounds)
	}
	return rounds
}

// orderedQuestions returns the questions of the quiz in THE play order (rounds + ORDER).
func (a *App) orderedQuestions() []server.OrderedQuestion {
	raw := make([]map[string]interface{}, 0)
	for _, q := range a.loadQuestions() {
		raw = append(raw, q)
	}
	return server.OrderQuestions(raw, a.currentRounds())
}

// computeRoundContext derives GameState.ROUND_CONTEXT for the question currentID
// (contracts/game-state.md §ROUND_CONTEXT). All zero without round or current question.
func computeRoundContext(ordered []server.OrderedQuestion, rounds []game.Round, currentID string) game.RoundContext {
	if len(rounds) == 0 || currentID == "" {
		return game.RoundContext{}
	}
	var cur *server.OrderedQuestion
	for i := range ordered {
		if ordered[i].ID == currentID {
			cur = &ordered[i]
			break
		}
	}
	if cur == nil {
		return game.RoundContext{}
	}
	ctx := game.RoundContext{RoundID: cur.RoundID, RoundIndex: cur.Group, RoundCount: len(rounds)}
	isEntracte := cur.Type == string(game.QuestionTypeEntracte)
	pos, total := 0, 0
	firstNonEntracte := ""
	for _, q := range ordered {
		if q.Group != cur.Group || q.Type == string(game.QuestionTypeEntracte) {
			continue
		}
		total++
		if firstNonEntracte == "" {
			firstNonEntracte = q.ID
		}
		if q.ID == cur.ID {
			pos = total
		}
	}
	ctx.Total = total
	if !isEntracte {
		ctx.Position = pos
		ctx.FirstOfRound = cur.Group > 0 && cur.ID == firstNonEntracte
	}
	return ctx
}

// refreshRoundContext recomputes ROUND_CONTEXT for the engine's current question.
func (a *App) refreshRoundContext() {
	st := a.engine.GetState()
	id := ""
	if st.Question != nil {
		id = st.Question.ID
	}
	a.engine.SetRoundContext(computeRoundContext(a.orderedQuestions(), a.currentRounds(), id))
}

// questionNumericIDs lists the numeric directory names taken in the questions directory.
func takenQuestionIDs(dir string) map[int]bool {
	taken := map[int]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return taken
	}
	for _, e := range entries {
		if e.IsDir() {
			if n, err := strconv.Atoi(e.Name()); err == nil {
				taken[n] = true
			}
		}
	}
	return taken
}

// allocateQuestionDir reserves the lowest free numeric ID (1..999) with an exclusive mkdir,
// the same technique as HTTPServer.resolveQuestionDir (safe against a concurrent upload).
func allocateQuestionDir(dir string) (string, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	for i := 1; i < 1000; i++ {
		id := strconv.Itoa(i)
		p := filepath.Join(dir, id)
		if err := os.Mkdir(p, 0o755); err == nil {
			return id, p, nil
		} else if !os.IsExist(err) {
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("question ids exhausted")
}

// writeQuestionJSON writes question.json (indented, like REORDER_QUESTIONS).
func writeQuestionJSON(dir string, q map[string]interface{}) error {
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "question.json"), data, 0o644)
}

// newAutoEntracte builds a standard ENTRACTE question (#214) for a round boundary.
func (a *App) newAutoEntracte(id, title string) map[string]interface{} {
	cfg := a.engine.GetState().EntracteConfigSaved
	return map[string]interface{}{
		"ID": id, "QUESTION": title, "ANSWER": "—", "TYPE": "ENTRACTE",
		"POINTS": "0", "TIME": "0", "POINTS_TARGET": "TEAM",
		"ENTRACTE_CONFIG": map[string]interface{}{
			"TITLE": title, "SUBTITLE": "",
			"PANEL_SIZE": cfg.PanelSize, "ANIM_PERIOD": cfg.AnimPeriod,
			"ANIM_INTENSITY": cfg.AnimIntensity, "TRANSITION_MS": cfg.TransitionMs,
		},
	}
}

func roundResult(status, reason string) protocol.RoundsUpdateResultPayload {
	return protocol.RoundsUpdateResultPayload{
		Status: status, Reason: reason, Rounds: []game.Round{}, Questions: []string{},
		CreatedEntractes: []protocol.RoundsCreatedEntractes{}, DeletedQuestions: []string{},
	}
}

// applyRoundsUpdate validates the WHOLE request, then applies it (contracts §ROUNDS_UPDATE).
// It returns the RESULT payload and the journal line.
func (a *App) applyRoundsUpdate(req protocol.RoundsUpdatePayload) (protocol.RoundsUpdateResultPayload, string) {
	qdir := a.questionsDirPath()
	server.QuestionFilesMu.Lock()
	defer server.QuestionFilesMu.Unlock()

	rf := server.LoadRoundsFile(qdir)
	existingRounds := map[string]game.Round{}
	for _, r := range rf.Rounds {
		existingRounds[r.ID] = r
	}
	files := server.ReadQuestionMaps(qdir)
	byID := map[string]map[string]interface{}{}
	for _, q := range files {
		id, _ := q["ID"].(string)
		byID[id] = q
	}

	refuse := func(reason string) (protocol.RoundsUpdateResultPayload, string) {
		return roundResult("REFUSED", reason), reason
	}

	// ---- validation (nothing is written before every check passed)
	if len(req.Rounds) > maxRounds {
		return refuse("TOO_MANY_ROUNDS")
	}
	for i := range req.Rounds {
		name := strings.TrimSpace(req.Rounds[i].Name)
		if n := utf8.RuneCountInString(name); n < 1 || n > maxRoundNameRune {
			return refuse("INVALID_NAME")
		}
		req.Rounds[i].Name = name
	}
	keptIDs := map[string]bool{}
	for _, r := range req.Rounds {
		if r.ID == "" {
			continue
		}
		if _, ok := existingRounds[r.ID]; !ok {
			return refuse("UNKNOWN_ROUND")
		}
		if keptIDs[r.ID] {
			return refuse("INVALID_DECISION")
		}
		keptIDs[r.ID] = true
	}
	deleted := map[string]string{} // round ID -> KEEP | DELETE
	for _, d := range req.DeletedRounds {
		if _, ok := existingRounds[d.ID]; !ok {
			return refuse("UNKNOWN_ROUND")
		}
		if (d.Questions != "KEEP" && d.Questions != "DELETE") || keptIDs[d.ID] {
			return refuse("INVALID_DECISION")
		}
		if _, dup := deleted[d.ID]; dup {
			return refuse("INVALID_DECISION")
		}
		deleted[d.ID] = d.Questions
	}
	var missing []game.Round
	for _, r := range rf.Rounds {
		if !keptIDs[r.ID] {
			if _, ok := deleted[r.ID]; !ok {
				missing = append(missing, r)
			}
		}
	}
	if len(missing) > 0 {
		res, line := refuse("MISSING_ROUND_DECISION")
		res.Rounds = missing
		return res, line
	}
	cited := map[string]bool{}
	var duplicate []string
	cite := func(id string) {
		if cited[id] {
			duplicate = append(duplicate, id)
		}
		cited[id] = true
	}
	for _, r := range req.Rounds {
		for _, id := range r.Questions {
			cite(id)
		}
	}
	for _, id := range req.Unassigned {
		cite(id)
	}
	if len(duplicate) > 0 {
		res, line := refuse("DUPLICATE_QUESTION")
		res.Questions = duplicate
		return res, line
	}
	var unknown []string
	for id := range cited {
		if _, ok := byID[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		res, line := refuse("UNKNOWN_QUESTION")
		res.Questions = unknown
		return res, line
	}
	var incomplete []string
	for id, q := range byID {
		if cited[id] {
			continue
		}
		rid, _ := q["ROUND_ID"].(string)
		if _, ok := deleted[rid]; rid != "" && ok {
			continue
		}
		incomplete = append(incomplete, id)
	}
	if len(incomplete) > 0 {
		sort.Strings(incomplete)
		res, line := refuse("INCOMPLETE")
		res.Questions = incomplete
		return res, line
	}
	wantEntractes := 0
	for _, r := range req.Rounds {
		if r.ID == "" && (r.Entractes == nil || *r.Entractes) {
			wantEntractes += 2
		}
	}
	if wantEntractes > 0 {
		free := 0
		taken := takenQuestionIDs(qdir)
		for i := 1; i < 1000; i++ {
			if !taken[i] {
				free++
			}
		}
		if wantEntractes > free {
			return refuse("QUESTION_ID_EXHAUSTED")
		}
	}

	// ---- application
	ordered := server.OrderQuestions(files, rf.Rounds) // play order, for KEEP / DELETE sets
	var deletedQuestions []string
	var keptToUnassigned []string
	for _, oq := range ordered {
		if cited[oq.ID] {
			continue
		}
		switch deleted[oq.RoundID] {
		case "DELETE":
			if err := os.RemoveAll(filepath.Join(qdir, oq.ID)); err != nil {
				server.LogError(game.LogComponentApp, "ROUNDS_UPDATE: failed to delete question %s: %v", oq.ID, err)
			}
			delete(byID, oq.ID)
			deletedQuestions = append(deletedQuestions, oq.ID)
		case "KEEP":
			keptToUnassigned = append(keptToUnassigned, oq.ID)
		}
	}

	newRF := server.RoundsFile{Rounds: make([]game.Round, 0, len(req.Rounds)), NextID: rf.NextID}
	result := roundResult("APPLIED", "")
	type placement struct {
		roundID string
		ids     []string
	}
	var layout []placement
	created := 0
	for _, r := range req.Rounds {
		id := r.ID
		ids := append([]string{}, r.Questions...)
		if id == "" {
			id = "r" + strconv.Itoa(newRF.NextID)
			newRF.NextID++
			created++
			if r.Entractes == nil || *r.Entractes {
				startID, startDir, err1 := allocateQuestionDir(qdir)
				endID, endDir, err2 := allocateQuestionDir(qdir)
				if err1 != nil || err2 != nil {
					server.LogError(game.LogComponentApp, "ROUNDS_UPDATE: cannot allocate entracte ids: %v %v", err1, err2)
					return refuse("QUESTION_ID_EXHAUSTED")
				}
				start := a.newAutoEntracte(startID, "Début de la manche "+r.Name)
				end := a.newAutoEntracte(endID, "Fin de la manche "+r.Name)
				start["ROUND_ID"], end["ROUND_ID"] = id, id
				start["ORDER"], end["ORDER"] = 0, len(ids)+1
				if err := writeQuestionJSON(startDir, start); err != nil {
					server.LogError(game.LogComponentApp, "ROUNDS_UPDATE: write entracte %s: %v", startID, err)
				}
				if err := writeQuestionJSON(endDir, end); err != nil {
					server.LogError(game.LogComponentApp, "ROUNDS_UPDATE: write entracte %s: %v", endID, err)
				}
				byID[startID], byID[endID] = start, end
				ids = append([]string{startID}, ids...)
				ids = append(ids, endID)
				result.CreatedEntractes = append(result.CreatedEntractes, protocol.RoundsCreatedEntractes{RoundID: id, StartID: startID, EndID: endID})
			}
		}
		newRF.Rounds = append(newRF.Rounds, game.Round{ID: id, Name: r.Name})
		layout = append(layout, placement{roundID: id, ids: ids})
	}
	unassigned := append(append([]string{}, req.Unassigned...), keptToUnassigned...)
	layout = append([]placement{{roundID: "", ids: unassigned}}, layout...)

	if err := server.SaveRoundsFile(qdir, newRF); err != nil {
		server.LogError(game.LogComponentApp, "ROUNDS_UPDATE: cannot write rounds.json: %v", err)
	}
	moved := 0
	for _, pl := range layout {
		for pos, id := range pl.ids {
			q := byID[id]
			if q == nil {
				continue
			}
			curRound, _ := q["ROUND_ID"].(string)
			curOrder := server.QuestionOrderValue(q)
			_, hasOrder := q["ORDER"]
			if curRound == pl.roundID && hasOrder && curOrder == pos {
				continue // unchanged: file left alone
			}
			if pl.roundID == "" {
				delete(q, "ROUND_ID")
			} else {
				q["ROUND_ID"] = pl.roundID
			}
			q["ORDER"] = pos
			if err := writeQuestionJSON(filepath.Join(qdir, id), q); err != nil {
				server.LogError(game.LogComponentApp, "ROUNDS_UPDATE: write question %s: %v", id, err)
			}
			if curRound != pl.roundID {
				moved++
			}
		}
	}

	a.engine.SetRounds(newRF.Rounds)
	result.Rounds = append([]game.Round{}, newRF.Rounds...)
	result.DeletedQuestions = append(result.DeletedQuestions, deletedQuestions...)
	line := fmt.Sprintf("rounds=%d created=%d deleted_rounds=%d entractes=%d moved=%d deleted_questions=%d",
		len(newRF.Rounds), created, len(req.DeletedRounds), len(result.CreatedEntractes), moved, len(deletedQuestions))
	return result, line
}

// handleRoundsUpdate processes ROUNDS_UPDATE from an admin (#245).
func (a *App) handleRoundsUpdate(clientID string, msg *protocol.Message) {
	var req protocol.RoundsUpdatePayload
	reply := func(res protocol.RoundsUpdateResultPayload) {
		if rm, err := protocol.NewMessage(protocol.ActionRoundsUpdateResult, res); err == nil && clientID != "" {
			a.wsHub.SendToClient(clientID, rm)
		}
	}
	if err := json.Unmarshal(msg.Msg, &req); err != nil {
		server.LogWarn(game.LogComponentApp, "ROUNDS_UPDATE_REFUSED client=%q reason=%q", clientID, "INVALID_NAME")
		reply(roundResult("REFUSED", "INVALID_NAME"))
		return
	}
	res, line := a.applyRoundsUpdate(req)
	if res.Status == "REFUSED" {
		server.LogWarn(game.LogComponentApp, "ROUNDS_UPDATE_REFUSED client=%q reason=%q questions=%s", clientID, res.Reason, logValue(res.Questions))
		reply(res)
		return
	}
	server.LogInfo(game.LogComponentApp, "ROUNDS_UPDATE client=%q %s", clientID, line)
	a.refreshRoundContext()
	a.broadcastUpdate()
	a.broadcastQuestions()
	a.broadcastNextQuestion()
	reply(res)
}
