package game

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// persistedGameStateFormatVersion is game_state.json's format version — the
// FIRST versioned envelope in this project (#141, plan task 20). None of
// the four pre-existing persisted files (history.json, teams.json,
// bumpers.json, question_statuses.json) has one: each is a bare array/map,
// and there is no migration code anywhere in the repo to handle a shape
// change. Since game_state.json is created ex nihilo by this change, adding
// a version field costs nothing today and avoids a silent future breakage —
// bump this constant (and add a migration branch in LoadState) the next
// time PersistedGameState's shape changes incompatibly.
const persistedGameStateFormatVersion = 1

// PersistedGameState is the on-disk envelope for game_state.json — the
// subset of GameState (models.go) that survives a server restart (#141).
//
// Deliberately narrow: only quiz metadata and the virtual-player-count cap,
// none of it derived or reset by NEW_GAME/InitGame (contract game-state.md
// rule H5 — QUIZ_HIDDEN_FIELDS and its siblings persist across games within
// a session; this file makes that survive a process restart too, the same
// semantics, just durable). Field names mirror GameState's own JSON tags
// (UPPER_SNAKE) for the game fields, so this file reads the same way
// GameState does on the wire — only FormatVersion (envelope metadata, not a
// game field) breaks that convention on purpose, to stay visually distinct.
//
// Explicitly NOT included, and why:
//   - Phase, Question, Memory*, Motion* (including MotionActive — #184,
//     B-B4, contract §5.2's "Non persisté"), ArdoiseAnswers,
//     EnrollmentActive, ShowQRCode, Entracte (v6.5.2, #119): ephemeral,
//     per-game-in-progress
//     state — restoring it after a restart would resurrect a game that no
//     longer has connected clients or a live timer behind it. A server that
//     restarts mid-pause simply comes back outside ENTRACTE (contract
//     game-state.md §"Persistance"). EntracteConfig (the SAVED variant,
//     GameState.EntracteConfigSaved — see below) is NOT in this exclusion
//     list: as of C1 (2026-08-20) it IS a stored property of the game,
//     alongside the Quiz* fields — a correction of the original design,
//     which persisted it independently in game-config.json
//     (config.GameSettings) instead. Two neighboring fields, two opposite
//     lifecycles: don't "fix" this apparent inconsistency by adding
//     Entracte here or removing EntracteConfig.
//   - Delay: looks like a setting but isn't — set by Start()/Ready()/the
//     MEMOTION memorize countdown on every round (engine.go), so it's
//     current-round timer state, not a stored preference. The actual
//     stored preference (GameSettings.Game.DefaultDelay, #150) already
//     lives in game-config.json.
//   - NetworkOnlyLocalhost: recalculated at startup, never a stored value.
//   - VirtualPlayerCount: derived from actually-enrolled bumpers, not a
//     setting.
//   - Backgrounds/NewGameBackgrounds: already persisted independently
//     (data/files/backgrounds/backgrounds.json) and loaded in main.go's
//     init() BEFORE LoadState runs (see cmd/server/main.go's LoadState call
//     site comment) — restoring them here would overwrite what init()
//     already populated.
type PersistedGameState struct {
	FormatVersion int `json:"format_version"`

	QuizName         string   `json:"QUIZ_NAME"`
	QuizTheme        string   `json:"QUIZ_THEME"`
	QuizNotes        string   `json:"QUIZ_NOTES"`
	QuizPopulations  []string `json:"QUIZ_POPULATIONS"`
	QuizDifficulties []string `json:"QUIZ_DIFFICULTIES"`
	QuizLanguage     string   `json:"QUIZ_LANGUAGE"`
	QuizObjectives   string   `json:"QUIZ_OBJECTIVES"`
	QuizHiddenFields []string `json:"QUIZ_HIDDEN_FIELDS"`

	// VirtualPlayerLimit (candidate field, plan task 18): an admin setting
	// (SET_VIRTUAL_PLAYER_LIMIT, default 20), not derived and not reset by
	// InitGame — same "survives NEW_GAME already, should survive a restart
	// too" reasoning as the QUIZ_* fields above.
	VirtualPlayerLimit int `json:"VIRTUAL_PLAYER_LIMIT"`

	// EntracteConfig (v6.5.2, #119, C1): the SAVED entracte panel
	// configuration (GameState.EntracteConfigSaved, not the possibly-frozen
	// GameState.EntracteConfig — C4). A *pointer*, unlike every field above,
	// and deliberately: a nil value (JSON key entirely absent) means "this
	// game_state.json predates the field, or was never configured" — apply
	// NewEngine()'s compile-time defaults (LoadState below leaves them
	// untouched in that case). A non-nil value means "configured", and is
	// used exactly as stored, WITHOUT re-defaulting any of its own
	// sub-fields — by the time a value reaches here it has already been
	// validated/clamped by the WS handler (cmd/server/main.go,
	// handleUpdateEntracteConfig) before SetEntracteConfig wrote it. This is
	// what protects ANIM_INTENSITY=0 ("animation disabled") from ever being
	// silently re-defaulted back to 20 on a reload — the exact pitfall this
	// project already hit once building the original (pre-C1) design; see
	// EntracteConfig's own doc comment (models.go) and
	// internal/game/entracte_test.go.
	//
	// IMAGE_IS_CUSTOM is written as false here regardless of the in-memory
	// value (SaveState zeroes a local copy before marshaling) — it is never
	// a stored value, always recomputed from disk (see EntracteConfig's doc
	// comment); LoadState mirrors this by leaving it false on read, for the
	// caller (cmd/server/main.go, after LoadState returns) to recompute via
	// Engine.RefreshEntracteImageIsCustom.
	EntracteConfig *EntracteConfig `json:"ENTRACTE_CONFIG,omitempty"`
}

// ClearQuizMeta resets the #141 persisted subset (quiz metadata + virtual
// player limit) to its zero/default values, in memory only — mirrors
// ClearHistory/ClearStatuses/ClearBackgrounds (engine.go): the caller
// (handleResetSelect, http.go) removes game_state.json from disk
// afterward, the same division of labor already established for
// history.json/question_statuses.json in this codebase.
func (e *Engine) ClearQuizMeta() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.QuizName = ""
	e.state.QuizTheme = ""
	e.state.QuizNotes = ""
	e.state.QuizPopulations = []string{}
	e.state.QuizDifficulties = []string{}
	e.state.QuizLanguage = ""
	e.state.QuizObjectives = ""
	e.state.QuizHiddenFields = []string{}
	e.state.VirtualPlayerLimit = 20 // matches NewEngine()'s own default
	log.Printf("[Engine] Quiz metadata cleared")
}

// SetStatePath sets the path for game state persistence (game_state.json).
func (e *Engine) SetStatePath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.statePath = path
	log.Printf("[Engine] Game state path set to: %s", path)
}

// SaveState persists the quiz-metadata subset of GameState to disk,
// atomically (temp file + chmod + rename — same pattern as SaveTeams/
// SaveBumpers, NOT SaveHistory/SaveStatuses' direct os.WriteFile, per plan
// task 19: this file, like teams.json/bumpers.json, can be written from a
// user-facing action while something else might be reading it).
func (e *Engine) SaveState() error {
	e.mu.RLock()
	path := e.statePath
	// EntracteConfig (v6.5.2, #119, C1): persist the SAVED variant
	// (EntracteConfigSaved), never the possibly-frozen diffused one — the
	// file on disk must always reflect what the Quiz page's form last
	// enregistered, not whatever happens to be showing on a panel right
	// now. IMAGE_IS_CUSTOM is explicitly zeroed in this local copy — it is
	// never a stored value (see PersistedGameState's doc comment).
	entracteCfg := e.state.EntracteConfigSaved
	entracteCfg.ImageIsCustom = false
	persisted := PersistedGameState{
		FormatVersion:      persistedGameStateFormatVersion,
		QuizName:           e.state.QuizName,
		QuizTheme:          e.state.QuizTheme,
		QuizNotes:          e.state.QuizNotes,
		QuizPopulations:    e.state.QuizPopulations,
		QuizDifficulties:   e.state.QuizDifficulties,
		QuizLanguage:       e.state.QuizLanguage,
		QuizObjectives:     e.state.QuizObjectives,
		QuizHiddenFields:   e.state.QuizHiddenFields,
		VirtualPlayerLimit: e.state.VirtualPlayerLimit,
		EntracteConfig:     &entracteCfg,
	}
	e.mu.RUnlock()

	if path == "" {
		return nil // No path configured, skip (same convention as SaveHistory/SaveTeams/etc.)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create game state directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal game state: %v", err)
		return err
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		log.Printf("[Engine] Failed to create temp file for game state save: %v", err)
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to write temp game state file: %v", err)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to close temp game state file: %v", err)
		return err
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to chmod temp game state file: %v", err)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		log.Printf("[Engine] Failed to save game state: %v", err)
		return err
	}

	log.Printf("[Engine] Game state saved to %s (quiz=%q)", path, persisted.QuizName)
	return nil
}

// LoadState restores the quiz-metadata subset of GameState from disk
// (#141). Must run AFTER loadBackgrounds()/loadNewGameBackgrounds() (called
// from main.go's init()) — see cmd/server/main.go's call site for why: this
// subset deliberately excludes Backgrounds/NewGameBackgrounds precisely so
// this ordering constraint doesn't matter, but the comment lives at both
// ends since a future field addition could reintroduce it.
//
// Applies the same nil-safety normalization SetQuizMeta/SetQuizDisplay use
// (never leave a QUIZ_* slice nil — contract game-state.md rule H1) inline
// rather than by calling those setters: they lock e.mu themselves (this
// method already holds it) and each would trigger its own SaveState() —
// harmless but pointless work while loading.
func (e *Engine) LoadState() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.statePath == "" {
		return nil
	}

	data, err := os.ReadFile(e.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No game state file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read game state: %v", err)
		return err
	}

	var persisted PersistedGameState
	if err := json.Unmarshal(data, &persisted); err != nil {
		log.Printf("[Engine] Failed to parse game state: %v", err)
		return err
	}

	if persisted.FormatVersion > persistedGameStateFormatVersion {
		log.Printf("[Engine] Warning: game_state.json format_version=%d is newer than this build supports (%d) — loading known fields only", persisted.FormatVersion, persistedGameStateFormatVersion)
	}

	if persisted.QuizPopulations == nil {
		persisted.QuizPopulations = []string{}
	}
	if persisted.QuizDifficulties == nil {
		persisted.QuizDifficulties = []string{}
	}
	if persisted.QuizHiddenFields == nil {
		persisted.QuizHiddenFields = []string{}
	}

	e.state.QuizName = persisted.QuizName
	e.state.QuizTheme = persisted.QuizTheme
	e.state.QuizNotes = persisted.QuizNotes
	e.state.QuizPopulations = persisted.QuizPopulations
	e.state.QuizDifficulties = persisted.QuizDifficulties
	e.state.QuizLanguage = persisted.QuizLanguage
	e.state.QuizObjectives = persisted.QuizObjectives
	e.state.QuizHiddenFields = persisted.QuizHiddenFields
	// Never regress to 0: a pre-#141 or corrupted file omitting this field
	// must not clobber NewEngine()'s own 20-default (GetVirtualPlayerLimit
	// self-heals a 0 back to 20, but several call sites read e.state.
	// VirtualPlayerLimit directly rather than through the getter, so 0
	// would leak through in the meantime).
	if persisted.VirtualPlayerLimit > 0 {
		e.state.VirtualPlayerLimit = persisted.VirtualPlayerLimit
	}

	// EntracteConfig (v6.5.2, #119, C1/C4): nil means "never configured" —
	// leave NewEngine()'s compile-time defaults in place for BOTH fields
	// (untouched here). Non-nil means "configured" — apply AS STORED (no
	// re-defaulting of individual sub-fields: values were already
	// validated/clamped before being saved, see PersistedGameState's doc
	// comment) to BOTH EntracteConfig and EntracteConfigSaved identically
	// (C4-B3 — a freshly restarted server is definitionally outside any
	// entracte, so "diffused" and "saved" start out equal, same as any other
	// out-of-pause moment). IMAGE_IS_CUSTOM is left at the stored false —
	// the caller (cmd/server/main.go, right after LoadState returns) must
	// call Engine.RefreshEntracteImageIsCustom to recompute it from disk.
	if persisted.EntracteConfig != nil {
		cfg := *persisted.EntracteConfig
		cfg.ImageIsCustom = false
		e.state.EntracteConfig = cfg
		e.state.EntracteConfigSaved = cfg
	}

	log.Printf("[Engine] Game state loaded from %s (quiz=%q, hidden_fields=%v, vplayer_limit=%d)",
		e.statePath, persisted.QuizName, e.state.QuizHiddenFields, e.state.VirtualPlayerLimit)
	return nil
}
