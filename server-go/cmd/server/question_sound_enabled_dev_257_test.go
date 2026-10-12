package main

import (
	"testing"

	"buzzcontrol/internal/config"
)

// TestQuestionSoundAdapter_Enabled_NilInjection_FollowsConfig covers the production default
// of enabled(): with no injected soundEnabled, the adapter follows config.Get().Sound.Enabled.
// It touches the process-wide singleton, so it is not parallel and restores it afterwards.
func TestQuestionSoundAdapter_Enabled_NilInjection_FollowsConfig(t *testing.T) {
	prev := config.Get()
	t.Cleanup(func() { config.SetInstance(prev) })

	qs := &questionSoundAdapter{}
	for _, want := range []bool{true, false, true} {
		config.SetInstance(&config.Config{Sound: config.SoundConfig{Enabled: want}})
		if got := qs.enabled(); got != want {
			t.Errorf("enabled() = %v with config Sound.Enabled=%v (nil injection must follow config)", got, want)
		}
	}
}

// An injected function wins over the config singleton.
func TestQuestionSoundAdapter_Enabled_Injection_OverridesConfig(t *testing.T) {
	prev := config.Get()
	t.Cleanup(func() { config.SetInstance(prev) })
	config.SetInstance(&config.Config{Sound: config.SoundConfig{Enabled: true}})

	qs := &questionSoundAdapter{soundEnabled: func() bool { return false }}
	if qs.enabled() {
		t.Error("enabled() must use the injected function, not the config")
	}
}
