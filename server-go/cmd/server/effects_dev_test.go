package main

import (
	"testing"

	"buzzcontrol/internal/config"
)

func TestEffectsConfigured_OnlyNonDefaultEffectsCount_Dev(t *testing.T) {
	t.Parallel()
	def := config.LightingConfig{Effects: config.ZoneEffects{"ambiance": {"chrono": "pulsation", "calm": "", "score": ""}, "animateur": {"calm": ""}}}
	if effectsConfigured(def) {
		t.Error("défauts : rien à résoudre (pulsation du chronomètre de la salle seulement)")
	}
	if !effectsConfigured(config.LightingConfig{Effects: config.ZoneEffects{"ambiance": {"calm": "degrade"}}}) {
		t.Error("un effet de salle configuré")
	}
	if !effectsConfigured(config.LightingConfig{Effects: config.ZoneEffects{"animateur": {"score": "couleur-equipe"}}}) {
		t.Error("un effet de zone animateur configuré")
	}
	if !effectsConfigured(config.LightingConfig{Effects: config.ZoneEffects{"public": {"chrono": "pulsation"}}}) {
		t.Error("la pulsation hors du chronomètre de la salle compte (zone public)")
	}
}

func TestSpeedsConfigured_Dev(t *testing.T) {
	t.Parallel()
	if speedsConfigured(config.LightingConfig{Speeds: config.ZoneSpeeds{"ambiance": {"chrono": 1}}}) {
		t.Error("×1 partout : rien à résoudre")
	}
	if !speedsConfigured(config.LightingConfig{Speeds: config.ZoneSpeeds{"public": {"calm": 0.5}}}) {
		t.Error("une vitesse ≠ ×1 dans une zone")
	}
}
