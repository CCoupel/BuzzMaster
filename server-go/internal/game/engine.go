package game

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Engine manages the game state and logic
type Engine struct {
	state   GameState
	data    *TeamsAndBumpers
	mu      sync.RWMutex
	timer   *time.Ticker
	stopCh  chan struct{}

	// Callbacks
	OnStateChange func(phase GamePhase)
	OnTimerTick   func(currentTime int)
	OnBuzzerPress func(bumperID, teamID string, pressTime int64, button string)
}

// NewEngine creates a new game engine
func NewEngine() *Engine {
	return &Engine{
		state: GameState{
			Phase:       PhaseStop,
			Delay:       30,
			CurrentTime: 0,
			Page:        "GAME",
		},
		data:   NewTeamsAndBumpers(),
		stopCh: make(chan struct{}),
	}
}

// GetState returns current game state
func (e *Engine) GetState() GameState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// GetPhase returns current phase
func (e *Engine) GetPhase() GamePhase {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase
}

// GetTeamsAndBumpers returns teams and bumpers data
func (e *Engine) GetTeamsAndBumpers() *TeamsAndBumpers {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.data
}

// GetTeam returns a specific team
func (e *Engine) GetTeam(id string) *Team {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.data.Teams[id]
}

// GetBumper returns a specific bumper
func (e *Engine) GetBumper(id string) *Bumper {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.data.Bumpers[id]
}

// UpdateBumper updates or creates a bumper
func (e *Engine) UpdateBumper(id string, data map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	bumper, exists := e.data.Bumpers[id]
	if !exists {
		bumper = &Bumper{}
		e.data.Bumpers[id] = bumper
	}

	// Update fields from data
	if name, ok := data["NAME"].(string); ok {
		bumper.Name = name
	}
	if team, ok := data["TEAM"].(string); ok {
		bumper.Team = team
	}
	if version, ok := data["VERSION"].(string); ok {
		bumper.Version = version
	}
	if ip, ok := data["IP"].(string); ok {
		bumper.IP = ip
	}

	log.Printf("[Engine] Updated bumper %s: team=%s, name=%s", id, bumper.Team, bumper.Name)
}

// UpdateTeam updates or creates a team
func (e *Engine) UpdateTeam(id string, team *Team) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data.Teams[id] = team
}

// SetTeams sets all teams
func (e *Engine) SetTeams(teams map[string]*Team) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data.Teams = teams
}

// SetBumpers sets all bumpers
func (e *Engine) SetBumpers(bumpers map[string]*Bumper) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data.Bumpers = bumpers
}

// Ready prepares a new question round
func (e *Engine) Ready(questionID string, question *Question) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStop && e.state.Phase != PhasePrepare && e.state.Phase != PhaseReady {
		log.Printf("[Engine] Cannot ready game from phase %s", e.state.Phase)
		return
	}

	e.state.Phase = PhasePrepare
	e.state.Question = question
	if question != nil {
		question.Status = StatusAvailable
	}

	// Reset bumper times
	for _, bumper := range e.data.Bumpers {
		bumper.Time = 0
		bumper.Button = ""
		bumper.Status = ""
		bumper.Ready = false
	}

	// Reset team times
	for _, team := range e.data.Teams {
		team.Time = 0
		team.Bumper = ""
		team.Status = ""
		team.Ready = false
	}

	log.Printf("[Engine] Game ready with question: %s", questionID)

	if e.OnStateChange != nil {
		e.OnStateChange(PhasePrepare)
	}
}

// SetBumperReady marks a bumper as ready (responded to PING)
func (e *Engine) SetBumperReady(bumperID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if bumper, ok := e.data.Bumpers[bumperID]; ok {
		bumper.Ready = true

		// Update team ready status
		e.updateTeamsReady()
	}
}

func (e *Engine) updateTeamsReady() {
	// For each team, check if all bumpers are ready
	teamBumperCount := make(map[string]int)
	teamReadyCount := make(map[string]int)

	for _, bumper := range e.data.Bumpers {
		if bumper.Team != "" {
			teamBumperCount[bumper.Team]++
			if bumper.Ready {
				teamReadyCount[bumper.Team]++
			}
		}
	}

	for teamID, team := range e.data.Teams {
		total := teamBumperCount[teamID]
		ready := teamReadyCount[teamID]
		team.Ready = total > 0 && total == ready
	}
}

// AreAllTeamsReady checks if all teams are ready
func (e *Engine) AreAllTeamsReady() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.data.Teams) == 0 {
		return false
	}

	for _, team := range e.data.Teams {
		if !team.Ready {
			return false
		}
	}
	return true
}

// TransitionToReady moves to READY phase when all buzzers responded
func (e *Engine) TransitionToReady() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase == PhasePrepare {
		e.state.Phase = PhaseReady
		log.Printf("[Engine] All teams ready, transitioning to READY")

		if e.OnStateChange != nil {
			e.OnStateChange(PhaseReady)
		}
	}
}

// Start begins the game round
func (e *Engine) Start(delay int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.Phase = PhaseStart
	e.state.Delay = delay
	e.state.CurrentTime = delay

	if e.state.Question != nil {
		e.state.Question.Status = StatusStarted
	}

	// Reset bumper times again
	for _, bumper := range e.data.Bumpers {
		bumper.Time = 0
		bumper.Button = ""
		bumper.Status = ""
	}

	for _, team := range e.data.Teams {
		team.Time = 0
		team.Bumper = ""
		team.Status = ""
	}

	log.Printf("[Engine] Game started with delay %d", delay)

	// Start timer
	e.startTimer()

	if e.OnStateChange != nil {
		e.OnStateChange(PhaseStart)
	}
}

func (e *Engine) startTimer() {
	if e.timer != nil {
		e.timer.Stop()
	}

	// Create new stop channel for this timer instance
	e.stopCh = make(chan struct{})
	e.timer = time.NewTicker(1 * time.Second)

	// Capture references locally to avoid nil pointer issues
	ticker := e.timer
	stopCh := e.stopCh

	go func() {
		for {
			select {
			case <-ticker.C:
				e.mu.Lock()
				if e.state.Phase == PhaseStart {
					e.state.CurrentTime--
					currentTime := e.state.CurrentTime
					e.mu.Unlock()

					if e.OnTimerTick != nil {
						e.OnTimerTick(currentTime)
					}

					if currentTime <= 0 {
						e.Stop()
					}
				} else {
					e.mu.Unlock()
				}
			case <-stopCh:
				return
			}
		}
	}()
}

// Stop ends the game round
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Signal timer goroutine to stop
	if e.stopCh != nil {
		select {
		case <-e.stopCh:
			// Already closed
		default:
			close(e.stopCh)
		}
		e.stopCh = nil
	}

	if e.timer != nil {
		e.timer.Stop()
		e.timer = nil
	}

	e.state.Phase = PhaseStop
	e.state.CurrentTime = 0

	if e.state.Question != nil {
		e.state.Question.Status = StatusStopped
	}

	log.Printf("[Engine] Game stopped")

	if e.OnStateChange != nil {
		e.OnStateChange(PhaseStop)
	}
}

// Pause pauses the game (single buzzer)
func (e *Engine) Pause() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.Phase = PhasePause

	if e.state.Question != nil {
		e.state.Question.Status = StatusStopped
	}

	log.Printf("[Engine] Game paused")

	if e.OnStateChange != nil {
		e.OnStateChange(PhasePause)
	}
}

// PauseAll pauses for all buzzers
func (e *Engine) PauseAll() {
	e.Pause()
}

// Continue resumes the game
func (e *Engine) Continue() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.Phase = PhaseStart

	if e.state.Question != nil {
		e.state.Question.Status = StatusStarted
	}

	log.Printf("[Engine] Game continued")

	if e.OnStateChange != nil {
		e.OnStateChange(PhaseStart)
	}
}

// Reveal shows the answer
func (e *Engine) Reveal() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Question != nil {
		e.state.Question.Status = StatusRevealed
		return e.state.Question.Answer
	}
	return ""
}

// ProcessButtonPress handles a button press from a buzzer
func (e *Engine) ProcessButtonPress(bumperID string, pressTime int64, button string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStart {
		log.Printf("[Engine] Ignoring button press, game not started")
		return
	}

	bumper, ok := e.data.Bumpers[bumperID]
	if !ok {
		log.Printf("[Engine] Unknown bumper: %s", bumperID)
		return
	}

	// Check if bumper already pressed
	if bumper.Time > 0 {
		log.Printf("[Engine] Bumper %s already pressed", bumperID)
		return
	}

	bumper.Time = pressTime
	bumper.Button = button
	bumper.Status = "PAUSE"

	teamID := bumper.Team
	if teamID == "" {
		return
	}

	team, ok := e.data.Teams[teamID]
	if !ok {
		return
	}

	// Check if this is the first/fastest press for the team
	if team.Time == 0 || pressTime < team.Time {
		team.Time = pressTime
		team.Bumper = bumperID
		team.Status = "PAUSE"
	}

	log.Printf("[Engine] Button press: bumper=%s, team=%s, button=%s, time=%d",
		bumperID, teamID, button, pressTime)

	if e.OnBuzzerPress != nil {
		e.OnBuzzerPress(bumperID, teamID, pressTime, button)
	}
}

// UpdateScore updates bumper and team scores
func (e *Engine) UpdateScore(bumperID string, points int) (bumperScore, teamScore int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	bumper, ok := e.data.Bumpers[bumperID]
	if !ok {
		return 0, 0
	}

	bumper.Score += points
	bumperScore = bumper.Score

	if bumper.Team != "" {
		if team, ok := e.data.Teams[bumper.Team]; ok {
			team.Score += points
			teamScore = team.Score
		}
	}

	log.Printf("[Engine] Score update: bumper=%s, points=%d, bumperScore=%d, teamScore=%d",
		bumperID, points, bumperScore, teamScore)

	return bumperScore, teamScore
}

// RAZScores resets all scores to zero
func (e *Engine) RAZScores() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, bumper := range e.data.Bumpers {
		bumper.Score = 0
		bumper.Time = 0
	}

	for _, team := range e.data.Teams {
		team.Score = 0
		team.Time = 0
	}

	log.Printf("[Engine] All scores reset")
}

// ClearBumpers removes all bumpers
func (e *Engine) ClearBumpers() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.data.Bumpers = make(map[string]*Bumper)
	e.data.Teams = make(map[string]*Team)

	log.Printf("[Engine] All bumpers cleared")
}

// SetPage sets the remote page
func (e *Engine) SetPage(page string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if page == "" || page == "null" {
		page = "GAME"
	}
	e.state.Page = page
}

// GetGameJSON returns game state as JSON
func (e *Engine) GetGameJSON() json.RawMessage {
	e.mu.RLock()
	defer e.mu.RUnlock()

	data := &GameData{
		Game:    &e.state,
		Teams:   e.data.Teams,
		Bumpers: e.data.Bumpers,
	}

	result, _ := json.Marshal(data)
	return result
}

// GetTeamsAndBumpersJSON returns teams and bumpers as JSON
func (e *Engine) GetTeamsAndBumpersJSON() json.RawMessage {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result, _ := json.Marshal(e.data)
	return result
}

// IsGameStopped returns true if game is stopped
func (e *Engine) IsGameStopped() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == PhaseStop
}

// IsGamePrepare returns true if game is in prepare phase
func (e *Engine) IsGamePrepare() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == PhasePrepare
}

// IsGameReady returns true if game is ready
func (e *Engine) IsGameReady() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == PhaseReady
}

// IsGameStarted returns true if game is started
func (e *Engine) IsGameStarted() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == PhaseStart
}
