// Round/direct ambiance layers (v11.6.0, #290 — contracts/ambiance-settings.md).
//
// The behaviour settings of the room (sound muted, countdown, cues, light mode) are no
// longer read from config.Get() by the sites that use them: they are RESOLVED here,
// global ← round ← direct, through ONE accessor, a.ambianceSettings(). A test-guard
// (AST) forbids any other reader of config.Get().Sound.Muted / .CuesDisabled /
// .CountdownDisabled / .CountdownOnce / Lighting.DefaultMode: the only adapter from
// config.json is server.AmbianceFromConfig.
package main

import (
	"sync"

	"buzzcontrol/internal/ambiancecfg"
	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/server"
)

// ambianceLayers is the process-memory state of the two non-global layers.
type ambianceLayers struct {
	mu sync.Mutex
	// refreshMu serialises resolve → publish → effects of refreshAmbianceB (review m2):
	// two concurrent refreshes must not lose a transition (missed stop / broadcast).
	refreshMu sync.Mutex
	roundID   string              // effective round ("" = none)
	direct    ambiancecfg.Partial // gestures of the régie since the last round change
}

// ambianceGlobal is the global layer, read from config.json through the adapter.
func ambianceGlobal() ambiancecfg.Settings {
	return server.AmbianceFromConfig(config.Get())
}

// AmbianceRoundLayer returns the override of the effective round (empty outside a round
// or for an unknown round). Implements server.AmbianceProvider.
func (a *App) AmbianceRoundLayer() ambiancecfg.Partial {
	if a == nil {
		return ambiancecfg.Partial{}
	}
	a.ambLayers.mu.Lock()
	id := a.ambLayers.roundID
	a.ambLayers.mu.Unlock()
	return a.roundLayerOf(id)
}

func (a *App) roundLayerOf(id string) ambiancecfg.Partial {
	if id == "" || a == nil || a.engine == nil {
		return ambiancecfg.Partial{}
	}
	for _, r := range a.engine.GetRounds() {
		if r.ID == id {
			return r.Ambiance.Clone()
		}
	}
	return ambiancecfg.Partial{}
}

// resolveAmbiance resolves the three layers.
func (a *App) resolveAmbiance() (ambiancecfg.Settings, ambiancecfg.Sources, string) {
	if a == nil { // adapters built without an App (tests): global only
		eff, src := ambiancecfg.Resolve(ambianceGlobal())
		return eff, src, ""
	}
	a.ambLayers.mu.Lock()
	id, direct := a.ambLayers.roundID, a.ambLayers.direct.Clone()
	a.ambLayers.mu.Unlock()
	eff, src := ambiancecfg.Resolve(ambianceGlobal(), a.roundLayerOf(id), direct)
	return eff, src, id
}

// ambianceSettings is THE accessor of the resolved settings — never cached by callers.
func (a *App) ambianceSettings() ambiancecfg.Settings {
	eff, _, _ := a.resolveAmbiance()
	return eff
}

// AmbianceSettings implements server.AmbianceProvider.
func (a *App) AmbianceSettings() (ambiancecfg.Settings, ambiancecfg.Sources) {
	eff, src, _ := a.resolveAmbiance()
	return eff, src
}

// AmbianceView implements server.AmbianceProvider (GET /api/ambiance/settings).
func (a *App) AmbianceView() server.AmbianceView {
	eff, src, id := a.resolveAmbiance()
	return server.AmbianceView{
		Global: ambianceGlobal(), Effective: eff, Sources: src,
		RoundID: id, RoundOverride: a.roundLayerOf(id),
	}
}

// AmbianceGesture implements server.AmbianceProvider: a direct gesture (§4.2 point 1).
func (a *App) AmbianceGesture(changes ambiancecfg.Partial) {
	a.ambLayers.mu.Lock()
	a.ambLayers.direct = a.ambLayers.direct.Merge(changes)
	a.ambLayers.mu.Unlock()
	a.refreshAmbiance()
}

// AmbianceRoundsChanged implements server.AmbianceProvider (r3): rounds.json was rewritten
// by the per-round sound files API — reload GameState.ROUNDS, re-resolve, broadcast.
func (a *App) AmbianceRoundsChanged() {
	a.reloadRounds()
	a.refreshAmbianceB(false)
	a.broadcastUpdate()
}

// AmbianceTestFile implements server.AmbianceProvider (r3, C8).
func (a *App) AmbianceTestFile(key, rel string) bool {
	if key == ambiancecfg.DecompteKey {
		return a.countdownSound().TestAt(rel)
	}
	return a.sound().PlayCueAt(audio.Cue(key), rel)
}

// SetLightingMode implements server.LightingProvider: POST /api/lighting/mode is a
// direct gesture — it never writes the global default_mode (§4.2 point 4).
func (a *App) SetLightingMode(mode string) error {
	m := ambiancecfg.LightMode(mode)
	if !m.Valid() {
		return a.ambCtl().SetLightingMode(mode) // same error as before
	}
	a.AmbianceGesture(ambiancecfg.Partial{LightMode: &m})
	return nil
}

// setAmbianceRound makes id the effective round. A change (or force) empties the direct
// layer — a change of context chosen by the régie (§4.2) — then everything is re-applied.
func (a *App) setAmbianceRound(id string, force bool) { a.setAmbianceRoundB(id, force, true) }

// setAmbianceRoundB is setAmbianceRound with control over the broadcast (a caller that
// broadcasts right after, like NEW_GAME, must not insert an extra UPDATE).
func (a *App) setAmbianceRoundB(id string, force, broadcast bool) {
	a.ambLayers.mu.Lock()
	changed := a.ambLayers.roundID != id
	if changed || force {
		a.ambLayers.roundID = id
		a.ambLayers.direct = ambiancecfg.Partial{}
	}
	a.ambLayers.mu.Unlock()
	if changed || force {
		a.refreshAmbianceB(broadcast)
	}
}

// syncAmbianceRound follows the round of the current question (READY, RESTORE, startup).
func (a *App) syncAmbianceRound() {
	st := a.engine.GetState()
	if st.Question == nil {
		return
	}
	a.setAmbianceRound(st.RoundContext.RoundID, false)
}

// refreshAmbiance re-resolves, publishes AMBIANCE_EFFECTIVE, pushes the light mode and
// applies the side effects of a mute / countdown switch off. Broadcasts when it changed.
func (a *App) refreshAmbiance() { a.refreshAmbianceB(true) }

func (a *App) refreshAmbianceB(broadcast bool) {
	if a.engine == nil {
		return
	}
	a.ambLayers.refreshMu.Lock()
	eff, src, id := a.resolveAmbiance()
	prev := a.engine.GetState().AmbianceEffective
	next := ambiancecfg.Effective{Settings: eff, Sources: src, RoundID: id}
	a.engine.SetAmbianceEffective(next)

	// Light: the resolved mode is pushed to the controller at each resolution change.
	if ctl := a.ambCtl(); string(eff.LightMode) != ctl.LightingMode() {
		_ = ctl.SetLightingMode(string(eff.LightMode))
	}
	// Sound: mute / countdown switch-off effects follow the RESOLUTION (sound.md §11.2).
	if eff.SoundMuted && !prev.Settings.SoundMuted {
		a.questionSound().Stop()
		a.countdownSound().Stop()
	}
	if !eff.CountdownEnabled && prev.Settings.CountdownEnabled {
		a.countdownSound().Stop()
	}
	a.ambLayers.refreshMu.Unlock()
	if broadcast && !ambianceEffectiveEqual(prev, next) {
		a.broadcastUpdate()
	}
}

func ambianceEffectiveEqual(x, y ambiancecfg.Effective) bool {
	if x.RoundID != y.RoundID {
		return false
	}
	xs, ys := x.Settings, y.Settings
	if xs.SoundMuted != ys.SoundMuted || xs.CountdownEnabled != ys.CountdownEnabled ||
		xs.CountdownLoop != ys.CountdownLoop || xs.LightMode != ys.LightMode || len(xs.CuesEnabled) != len(ys.CuesEnabled) {
		return false
	}
	for k, v := range xs.CuesEnabled {
		if yv, ok := ys.CuesEnabled[k]; !ok || yv != v {
			return false
		}
		if x.Sources.CuesEnabled[k] != y.Sources.CuesEnabled[k] {
			return false
		}
	}
	for _, z := range ambiancecfg.Zones { // effects and speeds by zone, and where they come from
		for _, m := range ambiancecfg.Moments {
			if xs.LightEffects[z][m] != ys.LightEffects[z][m] || x.Sources.LightEffects[z][m] != y.Sources.LightEffects[z][m] ||
				xs.LightSpeeds[z][m] != ys.LightSpeeds[z][m] || x.Sources.LightSpeeds[z][m] != y.Sources.LightSpeeds[z][m] {
				return false
			}
		}
	}
	xo, yo := x.Sources, y.Sources
	return xo.SoundMuted == yo.SoundMuted && xo.CountdownEnabled == yo.CountdownEnabled &&
		xo.CountdownLoop == yo.CountdownLoop && xo.LightMode == yo.LightMode
}
