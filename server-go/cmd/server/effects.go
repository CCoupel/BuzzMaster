package main

import (
	"path/filepath"
	"sync"

	"buzzcontrol/assets"
	"buzzcontrol/internal/ambiancecfg"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
	"io/fs"
)

// effects.go — light-effect catalogue wiring (v11.7.0, #223, contracts/
// light-effects.md §3). The catalogue is consumed by the ambiance reader
// (Batch 2.2); here it is only created, loaded and hot-reloaded.

// effectsDir is the user effects directory: <DataDir>/files/effects.
func (a *App) effectsDir() string {
	return filepath.Join(a.config.Storage.DataDir, "files", "effects")
}

// setupEffects creates the effects directory, rewrites the `natifs/` reference
// copy and loads the catalogue. A failure is logged and never stops the server:
// the catalogue then holds the embedded natives only.
func (a *App) setupEffects() {
	native, err := fs.Sub(assets.EffectsAssets, "effects")
	if err != nil {
		server.LogWarn(game.LogComponentApp, "Effets lumière : natifs embarqués illisibles : %v", err)
		return
	}
	dir := a.effectsDir()
	if err := effects.EnsureDirs(native, dir); err != nil {
		server.LogWarn(game.LogComponentApp, "Effets lumière : dossier %s non préparé : %v", dir, err)
	}
	a.effects = effects.NewManager(native, dir, func(format string, args ...any) {
		server.LogWarn(game.LogComponentApp, format, args...)
	})
}

// startEffects launches the hot-reload loop, stopped with a.ctx.
func (a *App) startEffects() {
	if a.effects != nil {
		go a.effects.Run(a.ctx)
	}
}

// EffectsCatalogue implements server.EffectsProvider. Without a manager (natives could not
// be prepared, or a bare test App) it answers with the embedded natives alone, so the
// pulsation — now a native effect — is always available to the reader.
func (a *App) EffectsCatalogue() effects.Catalogue {
	if a.effects == nil {
		nativesOnce.Do(func() {
			if sub, err := fs.Sub(assets.EffectsAssets, "effects"); err == nil {
				nativesOnly = effects.LoadDir(sub, "")
			}
		})
		return nativesOnly
	}
	return a.effects.Catalogue()
}

var (
	nativesOnce sync.Once
	nativesOnly effects.Catalogue
)

// EffectsReload implements server.EffectsProvider.
func (a *App) EffectsReload() effects.Catalogue {
	if a.effects == nil {
		return effects.Catalogue{}
	}
	return a.effects.Reload()
}

// EffectsDir implements server.EffectsProvider.
func (a *App) EffectsDir() string { return a.effectsDir() }

// EffectExists reports whether the catalogue holds this effect for the moment
// (ROUNDS_UPDATE validation, rounds.EffectChecker).
func (a *App) EffectExists(moment, id string) bool {
	en, ok := a.EffectsCatalogue().Get(id)
	return ok && en.Effect.HasMoment(effects.Moment(moment))
}

// EffectSettings implements ambiance.EffectsSource: the RESOLVED effect id per moment
// (global ← round) for the room and, by group id, for the animateur groups. It runs at the
// reader's 10 Hz, so when no effect is configured anywhere (global or in a round) it
// answers from the config alone, without resolving the whole settings model.
func (a *App) EffectSettings() (room map[string]string, host map[string]map[string]string) {
	lc := config.Get().Lighting
	if !effectsConfigured(lc) && !a.anyRoundEffects() {
		room = map[string]string{}
		for _, m := range ambiancecfg.Moments {
			room[m] = lc.Effects[m]
		}
		return room, map[string]map[string]string{}
	}
	s := a.ambianceSettings()
	return s.LightEffects, s.LightHostEffects
}

// effectsConfigured reports whether the global config selects any effect the reader plays
// (the default pulsation is answered from the config like any other global setting).
func effectsConfigured(lc config.LightingConfig) bool {
	for _, id := range lc.Effects {
		if id != "" && id != ambiancecfg.PulsationID {
			return true
		}
	}
	for _, mm := range lc.HostEffects {
		for _, id := range mm {
			if id != "" {
				return true
			}
		}
	}
	return false
}

// anyRoundEffects reports whether any round overrides an effect.
func (a *App) anyRoundEffects() bool {
	if a.engine == nil {
		return false
	}
	for _, r := range a.engine.GetRounds() {
		if len(r.Ambiance.LightEffects) > 0 || len(r.Ambiance.LightHostEffects) > 0 {
			return true
		}
	}
	return false
}
