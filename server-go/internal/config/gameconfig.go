package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// GameSettings holds the "jeu" (game) subset of configuration — default
// game delay and neon-effect visuals — split out of Config by #150 (option
// b, user-arbitrated). Unlike Config (server root, server-go/config.json,
// system settings: API keys, WiFi, ports), GameSettings lives inside
// dataDir at data/config/game-config.json, alongside teams.json/
// bumpers.json/history.json, so it travels automatically with a game's data
// in /fs-backup and can be included in selective backup/restore/reset —
// the entire point of the split (a "system" config.json monolith made game
// settings impossible to save/restore independently of API keys and WiFi
// credentials).
//
// Deliberately mirrors Config's Load/Save/singleton shape (GetGameSettings/
// SetGameSettingsInstance/SaveGameSettings below) rather than sharing code
// with it via generics — this codebase's other persistence code (engine.go's
// SaveTeams/SaveHistory/etc.) is similarly non-generic, one concrete type
// per file, and this is the smaller, more readable option for exactly two
// call sites (Config, GameSettings).
type GameSettings struct {
	Game       GameConfig       `json:"game"`
	NeonEffect NeonEffectConfig `json:"neon_effect"`
	// NOTE: an "entracte" section briefly lived here (v6.5.2, #119, QUALIF
	// only) and was REMOVED 2026-08-20 (C1, arbitrage utilisateur): the
	// entracte panel configuration is a property of the GAME/session, not
	// of server config — it now lives in game_state.json
	// (game.PersistedGameState, internal/game/state_persistence.go),
	// edited via the WS action UPDATE_ENTRACTE_CONFIG. No migration is
	// provided (never reached production); a residual "entracte" key in an
	// old game-config.json is simply ignored by json.Unmarshal. See
	// contracts/http-endpoints.md §"Mode ENTRACTE" and
	// contracts/CHANGELOG.md [20260820-2] (BREAKING, QUALIF-only).
}

// ApplyGameSettingsDefaults fills every zero-valued field with its default
// value — the GameSettings counterpart to ApplyDefaults, holding exactly
// the defaults that used to live in ApplyDefaults for the Game/NeonEffect
// sections before the #150 split. Same rationale: single source of truth
// for defaults, applied by LoadGameSettings, by GetGameSettings's in-memory
// fallback, by the migration path (MigrateGameSettings below), and by the
// additive POST /game-config.json handler.
func ApplyGameSettingsDefaults(gs *GameSettings) {
	if gs.Game.DefaultDelay == 0 {
		gs.Game.DefaultDelay = 30
	}

	if gs.NeonEffect.Mode == "" {
		gs.NeonEffect.Mode = "bar" // Default to bar mode
	}
	if gs.NeonEffect.ArcWidth == 0 {
		gs.NeonEffect.ArcWidth = 60
	}
	if gs.NeonEffect.IntensityGap == 0 {
		gs.NeonEffect.IntensityGap = 80
	}
	if gs.NeonEffect.RotationSpeed == 0 {
		gs.NeonEffect.RotationSpeed = 4.0
	}
	if gs.NeonEffect.BarOffset == 0 {
		gs.NeonEffect.BarOffset = 20 // 20 pixels from edge
	}
	if gs.NeonEffect.BarThickness == 0 {
		gs.NeonEffect.BarThickness = 4 // 4 pixels thick
	}
	if gs.NeonEffect.ArcBlur == 0 {
		gs.NeonEffect.ArcBlur = 100 // 100% of bar thickness
	}
	if gs.NeonEffect.GlowPulseSpeed == 0 {
		gs.NeonEffect.GlowPulseSpeed = 2.0 // 2 seconds
	}
	if gs.NeonEffect.GlowPulseMin == 0 {
		gs.NeonEffect.GlowPulseMin = 30 // 30% minimum glow
	}
	if gs.NeonEffect.GlowPulseMax == 0 {
		gs.NeonEffect.GlowPulseMax = 50 // 50% maximum glow
	}
	// Enabled defaults to false (zero value)

	// NOTE: entracte defaults used to live here (v6.5.2, #119) — moved with
	// the rest of the entracte config to internal/game (NewEngine's compile-
	// time defaults + state_persistence.go's LoadState) by C1. See
	// GameSettings' own doc comment above.
}

// ValidateAndClampNeonEffect validates and clamps neon effect values to
// acceptable ranges — moved verbatim from Config by the #150 split, only
// the receiver type changed.
func (gs *GameSettings) ValidateAndClampNeonEffect() {
	// Validate mode
	if gs.NeonEffect.Mode != "halo" && gs.NeonEffect.Mode != "bar" {
		gs.NeonEffect.Mode = "bar"
	}

	// Clamp arc_width to 30-180 degrees
	if gs.NeonEffect.ArcWidth < 30 {
		gs.NeonEffect.ArcWidth = 30
	} else if gs.NeonEffect.ArcWidth > 180 {
		gs.NeonEffect.ArcWidth = 180
	}

	// Clamp intensity_gap to 0-100%
	if gs.NeonEffect.IntensityGap < 0 {
		gs.NeonEffect.IntensityGap = 0
	} else if gs.NeonEffect.IntensityGap > 100 {
		gs.NeonEffect.IntensityGap = 100
	}

	// Clamp rotation_speed to 1.0-10.0 seconds
	if gs.NeonEffect.RotationSpeed < 1.0 {
		gs.NeonEffect.RotationSpeed = 1.0
	} else if gs.NeonEffect.RotationSpeed > 10.0 {
		gs.NeonEffect.RotationSpeed = 10.0
	}

	// Clamp bar_offset to 10-100 pixels
	if gs.NeonEffect.BarOffset < 10 {
		gs.NeonEffect.BarOffset = 10
	} else if gs.NeonEffect.BarOffset > 100 {
		gs.NeonEffect.BarOffset = 100
	}

	// Clamp bar_thickness to 2-20 pixels
	if gs.NeonEffect.BarThickness < 2 {
		gs.NeonEffect.BarThickness = 2
	} else if gs.NeonEffect.BarThickness > 20 {
		gs.NeonEffect.BarThickness = 20
	}

	// Clamp arc_blur to 0-200%
	if gs.NeonEffect.ArcBlur < 0 {
		gs.NeonEffect.ArcBlur = 0
	} else if gs.NeonEffect.ArcBlur > 200 {
		gs.NeonEffect.ArcBlur = 200
	}

	// Clamp glow_pulse_speed to 0.5-5.0 seconds
	if gs.NeonEffect.GlowPulseSpeed < 0.5 {
		gs.NeonEffect.GlowPulseSpeed = 0.5
	} else if gs.NeonEffect.GlowPulseSpeed > 5.0 {
		gs.NeonEffect.GlowPulseSpeed = 5.0
	}

	// Clamp glow_pulse_min to 0-100%
	if gs.NeonEffect.GlowPulseMin < 0 {
		gs.NeonEffect.GlowPulseMin = 0
	} else if gs.NeonEffect.GlowPulseMin > 100 {
		gs.NeonEffect.GlowPulseMin = 100
	}

	// Clamp glow_pulse_max to 0-100%
	if gs.NeonEffect.GlowPulseMax < 0 {
		gs.NeonEffect.GlowPulseMax = 0
	} else if gs.NeonEffect.GlowPulseMax > 100 {
		gs.NeonEffect.GlowPulseMax = 100
	}

	// Ensure min <= max
	if gs.NeonEffect.GlowPulseMin > gs.NeonEffect.GlowPulseMax {
		gs.NeonEffect.GlowPulseMin, gs.NeonEffect.GlowPulseMax = gs.NeonEffect.GlowPulseMax, gs.NeonEffect.GlowPulseMin
	}
}

// NOTE: entracteTextMaxRunes and ValidateAndClampEntracte used to live here
// (v6.5.2, #119) — the bounds (PANEL_SIZE 20-100, ANIM_PERIOD 2-30,
// ANIM_INTENSITY 0-100, TRANSITION_MS 0-10000, text truncated to 200 runes)
// moved to cmd/server/main.go's clampEntracteConfig, alongside the new
// UPDATE_ENTRACTE_CONFIG handler that's now the only writer (C1-B5) — the
// values, not just their location, are otherwise unchanged.

// LoadGameSettings reads game-config.json from path.
func LoadGameSettings(path string) (*GameSettings, error) {
	var gs GameSettings

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &gs); err != nil {
		return nil, err
	}

	ApplyGameSettingsDefaults(&gs)

	return &gs, nil
}

var (
	gameSettingsInstance *GameSettings
	gameSettingsMu       sync.RWMutex
	gameSettingsOnce     sync.Once
)

// gameConfigPath is the filesystem path GetGameSettings()/SaveGameSettings()
// resolve against — the GameSettings counterpart to configPath (config.go).
// Defaults to "data/config/game-config.json" (relative to CWD), matching
// the production layout when Storage.DataDir is left at its own default
// ("./data", see ApplyDefaults). Callers running with a non-default DataDir
// MUST call SetGameConfigPath explicitly with the resolved path (main.go
// does this in init(), the same place it sets the four existing engine
// Set*Path calls) — unlike configPath, this path is inherently
// dataDir-relative, so no single hardcoded literal can be correct for every
// deployment.
var (
	gameConfigPath   = filepath.Join("data", "config", "game-config.json")
	gameConfigPathMu sync.RWMutex
)

// SetGameConfigPath overrides the path GetGameSettings()/SaveGameSettings()
// use. Safe to call concurrently. Same once.Do caveat as SetConfigPath: call
// this before the first GetGameSettings() of the process/test binary.
func SetGameConfigPath(path string) {
	gameConfigPathMu.Lock()
	gameConfigPath = path
	gameConfigPathMu.Unlock()
}

// GameConfigPath returns the path GetGameSettings()/SaveGameSettings()
// currently resolve against.
func GameConfigPath() string {
	gameConfigPathMu.RLock()
	defer gameConfigPathMu.RUnlock()
	return gameConfigPath
}

// GetGameSettings returns the singleton GameSettings instance — the
// GameSettings counterpart to Get(). Falls back to defaults (no error
// surfaced beyond the log line) if game-config.json cannot be read, e.g. on
// a fresh install before the file has ever been written.
func GetGameSettings() *GameSettings {
	gameSettingsMu.RLock()
	cur := gameSettingsInstance
	gameSettingsMu.RUnlock()
	if cur != nil {
		return cur
	}
	gameSettingsOnce.Do(func() {
		loaded, err := LoadGameSettings(GameConfigPath())
		if err != nil {
			log.Printf("Warning: Could not load game-config.json, using defaults: %v", err)
			loaded = &GameSettings{}
			ApplyGameSettingsDefaults(loaded)
		}
		gameSettingsMu.Lock()
		gameSettingsInstance = loaded
		gameSettingsMu.Unlock()
	})
	gameSettingsMu.RLock()
	defer gameSettingsMu.RUnlock()
	return gameSettingsInstance
}

// SetGameSettingsInstance sets the singleton GameSettings instance — the
// GameSettings counterpart to SetInstance(). Used after every
// SaveGameSettings-backed write (POST /game-config.json, migration,
// restore, reset) so subsequent GetGameSettings() calls (including the ones
// feeding the WS neon_effect payload, cmd/server/main.go) see the fresh
// value immediately, without waiting for a process restart.
func SetGameSettingsInstance(gs *GameSettings) {
	gameSettingsMu.Lock()
	gameSettingsInstance = gs
	gameSettingsMu.Unlock()
}

// SaveGameSettings persists gs to game-config.json atomically (temp file +
// rename, identical pattern to Save()) — the GameSettings counterpart to
// Save(). Creates the destination directory if it doesn't exist yet (unlike
// Save(), whose destination — the server root — always exists already;
// data/config/ may not, e.g. on a fresh install or right before the very
// first migration).
func SaveGameSettings(gs *GameSettings) error {
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		return err
	}

	path := GameConfigPath()
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".game-config.json.tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// MigrateGameSettings implements the #150 (option b) startup migration:
// splitting the "game" and "neon_effect" sections out of the system
// config.json (ConfigPath()) into the independently-pathed game-config.json
// (GameConfigPath()) — BOTH must already be set (SetConfigPath/
// SetGameConfigPath) by the caller before this runs; see cmd/server/main.go.
//
// MUST run before anything parses config.json into the (now slimmed)
// Config struct: Config no longer declares Game/NeonEffect fields at all,
// so by the time a *Config exists, any "game"/"neon_effect" keys still
// present in the JSON have already been silently dropped by
// json.Unmarshal — this function reads the RAW bytes as a generic
// map[string]json.RawMessage specifically to still be able to recover
// them.
//
// Idempotent (required — a second startup must not rewrite anything):
//   - Neither section present in config.json (never had them, or already
//     migrated) -> no-op, returns nil immediately.
//   - Section(s) present AND game-config.json does not exist yet -> first
//     migration: extract the values into a new game-config.json, then
//     strip the sections from config.json.
//   - Section(s) present AND game-config.json already exists -> stray
//     values from a partially-migrated or hand-edited config.json are
//     DISCARDED (game-config.json is authoritative, contract §2.1); a
//     warning is logged; the sections are still stripped from config.json.
//
// Either way, config.json is rewritten via the normal Load+Save round-trip
// through the slimmed Config struct (not manual JSON surgery on the raw
// map) — Save() no longer has Game/NeonEffect fields to marshal, so this
// naturally drops the two keys while re-encoding every other section
// byte-faithfully via the same MarshalIndent path any other config.json
// write already uses. One cosmetic side effect: this canonical re-encoding
// uses the Config struct's field order, which may differ from whatever key
// order a hand-edited or pre-#150 config.json happened to have — harmless,
// and a one-time event for any given installation.
func MigrateGameSettings() error {
	configJSONPath := ConfigPath()

	rawData, err := os.ReadFile(configJSONPath)
	if err != nil {
		// No config.json at all yet (fresh install) — nothing to migrate;
		// Get()/GetGameSettings() will each apply their own defaults.
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rawData, &raw); err != nil {
		return fmt.Errorf("migrate game settings: %s is not valid JSON: %w", configJSONPath, err)
	}

	gameRaw, hasGame := raw["game"]
	neonRaw, hasNeon := raw["neon_effect"]
	if !hasGame && !hasNeon {
		// Already migrated (or this config.json never had these sections in
		// the first place) — idempotent no-op, nothing to rewrite.
		return nil
	}

	gcPath := GameConfigPath()
	if _, err := os.Stat(gcPath); err == nil {
		log.Printf("Warning: %s still has \"game\"/\"neon_effect\" sections but %s already exists — discarding the stray config.json values (game-config.json is authoritative, contract http-endpoints.md)", configJSONPath, gcPath)
	} else {
		var gs GameSettings
		if hasGame {
			if err := json.Unmarshal(gameRaw, &gs.Game); err != nil {
				return fmt.Errorf("migrate game settings: invalid \"game\" section: %w", err)
			}
		}
		if hasNeon {
			if err := json.Unmarshal(neonRaw, &gs.NeonEffect); err != nil {
				return fmt.Errorf("migrate game settings: invalid \"neon_effect\" section: %w", err)
			}
		}
		ApplyGameSettingsDefaults(&gs)

		if err := SaveGameSettings(&gs); err != nil {
			return fmt.Errorf("migrate game settings: could not write %s: %w", gcPath, err)
		}
		log.Printf("Migrated game settings (default_delay, neon_effect) from %s to %s (#150)", configJSONPath, gcPath)
	}

	// Strip the two sections from config.json regardless of which branch
	// ran above — see func doc comment for why a plain Load+Save
	// round-trip is sufficient.
	cfg, err := Load(configJSONPath)
	if err != nil {
		return fmt.Errorf("migrate game settings: could not reload %s: %w", configJSONPath, err)
	}
	if err := Save(cfg); err != nil {
		return fmt.Errorf("migrate game settings: could not rewrite %s: %w", configJSONPath, err)
	}

	return nil
}
