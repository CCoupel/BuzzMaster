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

// EffectsSource / EffectsDelete / EffectsImport implement server.EffectsStore (#305, §11.5 to
// §11.7). Without a manager (natives could not be prepared) nothing can be written.
func (a *App) EffectsSource(id string) ([]byte, effects.Entry, error) {
	if a.effects == nil {
		if en, ok := a.EffectsCatalogue().Get(id); ok {
			return nil, en, &effects.StoreError{Code: effects.ReasonIO}
		}
		return nil, effects.Entry{}, &effects.StoreError{Code: effects.ReasonNotFound}
	}
	return a.effects.Source(id)
}

func (a *App) EffectsDelete(id string) (effects.Catalogue, error) {
	if a.effects == nil {
		return effects.Catalogue{}, &effects.StoreError{Code: effects.ReasonNotFound}
	}
	return a.effects.Delete(id)
}

func (a *App) EffectsImport(files []effects.ImportFile, onConflict string) ([]effects.ImportResult, effects.Catalogue) {
	if a.effects == nil {
		return nil, effects.Catalogue{}
	}
	return a.effects.Import(files, onConflict)
}

// EffectExists reports whether the catalogue holds this effect for the moment
// (ROUNDS_UPDATE validation, rounds.EffectChecker).
func (a *App) EffectExists(moment, id string) bool {
	en, ok := a.EffectsCatalogue().Get(id)
	return ok && en.Effect.HasMoment(effects.Moment(moment))
}

// EffectSettings implements ambiance.EffectsSource: the RESOLVED effect id by zone then moment
// (global ← round). It runs at the reader's 10 Hz, so when the config selects nothing beyond
// the defaults and no round overrides an effect it answers from the config alone, without
// resolving the whole settings model.
func (a *App) EffectSettings() map[string]map[string]string {
	lc := config.Get().Lighting
	if !effectsConfigured(lc) && !a.anyRoundEffects() {
		out := map[string]map[string]string{}
		for _, z := range ambiancecfg.Zones {
			out[z] = map[string]string{}
			for _, m := range ambiancecfg.Moments {
				out[z][m] = lc.Effects[z][m]
			}
		}
		return out
	}
	return a.ambianceSettings().LightEffects
}

// EffectSpeeds implements ambiance.EffectSpeedSource: the RESOLVED speed multiplier by zone then
// moment (global ← round). Answered from the config alone when every speed is the default ×1
// and no round overrides one.
func (a *App) EffectSpeeds() map[string]map[string]float64 {
	lc := config.Get().Lighting
	if !speedsConfigured(lc) && !a.anyRoundSpeeds() {
		return nil // ×1 everywhere
	}
	return a.ambianceSettings().LightSpeeds
}

func speedsConfigured(lc config.LightingConfig) bool {
	for _, mm := range lc.Speeds {
		for _, v := range mm {
			if v != 1 {
				return true
			}
		}
	}
	return false
}

func (a *App) anyRoundSpeeds() bool {
	if a.engine == nil {
		return false
	}
	for _, r := range a.engine.GetRounds() {
		if len(r.Ambiance.LightSpeeds) > 0 {
			return true
		}
	}
	return false
}

// effectsConfigured reports whether the global config selects any effect beyond the defaults
// (the room's chrono "pulsation").
func effectsConfigured(lc config.LightingConfig) bool {
	for z, mm := range lc.Effects {
		for m, id := range mm {
			if id == "" || (z == ambiancecfg.ZoneAmbiance && m == ambiancecfg.MomentChrono && id == ambiancecfg.PulsationID) {
				continue
			}
			return true
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
		if len(r.Ambiance.LightEffects) > 0 {
			return true
		}
	}
	return false
}

// StartEffectTrial / StopEffectTrial implement server.EffectTrialer (#223, §7.4).
func (a *App) StartEffectTrial(id, zone string, speed float64, seconds int) (int, error) {
	return a.ambCtl().StartEffectTrial(id, zone, speed, seconds)
}

func (a *App) StopEffectTrial() { a.ambCtl().StopEffectTrial() }
