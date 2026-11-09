package main

import (
	"context"

	"buzzcontrol/internal/ambiance"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/lighting/hue"
)

// ambiance_delegates.go — one-line delegates of the ambiance family to internal/ambiance (#295): the old
// method names are kept for the 21 event sites, the LightingProvider interface and the tests.

func (a *App) ambianceIsConfigured() bool { return a.ambCtl().IsConfigured() }

func (a *App) buildHueDriver() *hue.Driver { return a.ambCtl().BuildHueDriver() }

func (a *App) newAmbianceWriter(drv lighting.Driver) *lighting.Writer {
	return a.ambCtl().NewWriter(drv)
}

func (a *App) ambiance() *lighting.Writer { return a.ambCtl().Writer() }

func (a *App) setupAmbiance() { a.ambCtl().Setup() }

func (a *App) startAmbianceWriter() { a.ambCtl().StartWriter() }

func (a *App) reconfigureAmbiance() { a.ambCtl().Reconfigure() }

func (a *App) LightingDriver() *hue.Driver { return a.ambCtl().LightingDriver() }

func (a *App) deriveAmbianceEvent() lighting.Event { return a.ambCtl().DeriveEvent() }

func (a *App) ambianceCorrectTeams(state game.GameState) []string {
	return a.ambCtl().CorrectTeams(state)
}

func (a *App) runChronoPulse(ctx context.Context) { a.ambCtl().RunChronoPulse(ctx) }

func (a *App) currentScoreFlashTeam() string { return a.ambCtl().CurrentScoreFlashTeam() }

func (a *App) ambianceScene(ev lighting.Event) lighting.State { return a.ambCtl().Scene(ev) }

func (a *App) ambianceThemeColor() [3]int { return a.ambCtl().ThemeColor() }

func (a *App) setLightingMode(m ambiance.Mode) { a.ambCtl().SetMode(m) }

func (a *App) setLightingFlash(on bool) { a.ambCtl().SetFlash(on) }

func (a *App) lightingOverrideGeneral(autoColor [3]int, autoIntensity int) ([3]int, int) {
	return a.ambCtl().OverrideGeneral(autoColor, autoIntensity)
}

func (a *App) startScoreFlash(team string, points int) { a.ambCtl().StartScoreFlash(team, points) }

func (a *App) LightingMode() string { return a.ambCtl().LightingMode() }

func (a *App) LightingFlash() bool { return a.ambCtl().LightingFlash() }

func (a *App) SetLightingFlash(on bool) { a.ambCtl().SetLightingFlash(on) }

func (a *App) PreviewColor(role string) [3]int { return a.ambCtl().PreviewColor(role) }

func (a *App) shutdownExtinguishHueLighting() { a.ambCtl().ShutdownExtinguish() }

func (a *App) extinguishOnLightingDisabled() { a.ambCtl().ExtinguishOnDisabled() }

// Aliases of the identifiers the tests (and main.go) still use under their old names.
type (
	lightingMode       = ambiance.Mode
	flashPhaseOverride = ambiance.FlashPhaseOverride
)

const (
	lightingModeAuto         = ambiance.ModeAuto
	lightingModeOn           = ambiance.ModeOn
	lightingModeOff          = ambiance.ModeOff
	lightingOnIntensity      = ambiance.OnIntensity
	lightingFlashOnPhase     = ambiance.FlashOnPhase
	lightingFlashOffPhase    = ambiance.FlashOffPhase
	scoreFlashGoldPhase      = ambiance.ScoreFlashGoldPhase
	scoreFlashTeamPhase      = ambiance.ScoreFlashTeamPhase
	chronoPulseTransitionMs  = ambiance.ChronoPulseTransitionMs
	chronoPulseIntensityFull = ambiance.ChronoPulseIntensityFull
	chronoPulseIntensityLow  = ambiance.ChronoPulseIntensityLow
)

var (
	lightingOnColor          = ambiance.OnColor
	lightingFlashColor       = ambiance.FlashColor
	ambianceWarmWhite        = ambiance.WarmWhite
	ambianceScoreGold        = ambiance.ScoreGold
	ambianceSceneIdle        = ambiance.SceneIdle
	ambianceSceneReady       = ambiance.SceneReady
	ambianceSceneRunning     = ambiance.SceneRunning
	ambianceSceneTeamTurn    = ambiance.SceneTeamTurn
	ambianceChronoPulseRed   = ambiance.ChronoPulseRed
	ambianceActiveTeam       = ambiance.ActiveTeam
	ambianceSceneFor         = ambiance.SceneFor
	ambianceChronoPulseColor = ambiance.ChronoPulseColor
	orDefault                = ambiance.OrDefault
)

var _ = context.Background
var _ game.GamePhase
var _ lighting.Event
var _ *hue.Driver
