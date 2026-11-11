package main

import (
	"path/filepath"

	"buzzcontrol/assets"
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
