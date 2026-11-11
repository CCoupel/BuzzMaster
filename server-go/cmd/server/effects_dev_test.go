package main

import (
	"testing"

	"buzzcontrol/internal/config"
)

func TestEffectsConfigured_OnlyNonPulsationEffectsCount_Dev(t *testing.T) {
	t.Parallel()
	if effectsConfigured(config.LightingConfig{Effects: map[string]string{"chrono": "pulsation", "calm": "", "score": ""}}) {
		t.Error("défauts v11.6.0 : rien à lire (la pulsation reste à RunChronoPulse)")
	}
	if !effectsConfigured(config.LightingConfig{Effects: map[string]string{"calm": "vague-lente"}}) {
		t.Error("un effet de salle configuré")
	}
	if !effectsConfigured(config.LightingConfig{HostEffects: config.HostEffectsTable{"g1": {"score": "couleur-equipe"}}}) {
		t.Error("un effet de groupe animateur configuré")
	}
	if effectsConfigured(config.LightingConfig{HostEffects: config.HostEffectsTable{"g1": {"score": ""}}}) {
		t.Error("blanc fixe = rien à lire")
	}
}
