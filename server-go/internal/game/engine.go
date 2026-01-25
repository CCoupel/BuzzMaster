package game

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Engine manages the game state and logic
type Engine struct {
	state            GameState
	data             *TeamsAndBumpers
	questionStatuses map[string]QuestionStatus // Track question statuses across selections
	history          []GameEvent
	historyPath      string
	teamsPath        string
	bumpersPath      string
	statusesPath     string // Path to question_statuses.json
	mu               sync.RWMutex
	timer            *time.Ticker
	stopCh           chan struct{}
	countdownTimer   *time.Ticker
	countdownStopCh  chan struct{}
	pendingDelay     int // Store delay for after countdown

	// Callbacks
	OnStateChange   func(phase GamePhase)
	OnTimerTick     func(currentTime int)
	OnCountdownTick func(countdownTime int)
	OnBuzzerPress   func(bumperID, teamID string, pressTime int64, button string)
	OnQCMHint       func(invalidatedColor string, remainingAnswers int) // QCM hint callback
}

// NewEngine creates a new game engine
func NewEngine() *Engine {
	return &Engine{
		state: GameState{
			Phase:              PhaseStopped,
			Delay:              30,
			CurrentTime:        0,
			Page:               "GAME",
			VirtualPlayerLimit: 20, // Default limit
		},
		data:             NewTeamsAndBumpers(),
		questionStatuses: make(map[string]QuestionStatus),
		stopCh:           make(chan struct{}),
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

// SetPhase sets the game phase
func (e *Engine) SetPhase(phase GamePhase) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Phase = phase
	log.Printf("[Engine] Phase set to: %s", phase)
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

// GetQuestionStatus returns the status of a question by ID
func (e *Engine) GetQuestionStatus(id string) QuestionStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if status, ok := e.questionStatuses[id]; ok {
		return status
	}
	return StatusAvailable
}

// setQuestionStatus updates both the current question and the status map (must hold lock)
func (e *Engine) setQuestionStatus(status QuestionStatus) {
	if e.state.Question != nil {
		e.state.Question.Status = status
		e.questionStatuses[e.state.Question.ID] = status
		go e.SaveStatuses() // Persist to disk
	}
}

// UpdateBumper updates or creates a bumper
func (e *Engine) UpdateBumper(id string, data map[string]interface{}) {
	e.mu.Lock()

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
	e.mu.Unlock()

	// Auto-save bumpers to disk
	go e.SaveBumpers()
}

// UpdateTeam updates or creates a team
func (e *Engine) UpdateTeam(id string, team *Team) {
	e.mu.Lock()
	e.data.Teams[id] = team
	e.mu.Unlock()

	// Auto-save teams to disk
	go e.SaveTeams()
}

// SetTeams sets all teams
func (e *Engine) SetTeams(teams map[string]*Team) {
	e.mu.Lock()
	// Ensure team NAME field is populated from the map key
	for teamID, team := range teams {
		if team.Name == "" {
			team.Name = teamID
		}
	}
	e.data.Teams = teams
	e.mu.Unlock()

	// Auto-save teams to disk
	go e.SaveTeams()
}

// SetBumpers sets all bumpers and synchronizes VirtualPlayerCount
func (e *Engine) SetBumpers(bumpers map[string]*Bumper) {
	e.mu.Lock()
	e.data.Bumpers = bumpers
	// Synchronize VirtualPlayerCount with actual bumper count
	e.state.VirtualPlayerCount = e.countVirtualPlayersUnsafe()
	e.mu.Unlock()

	// Auto-save bumpers to disk
	go e.SaveBumpers()
}

// Ready prepares a new question round
func (e *Engine) Ready(questionID string, question *Question) {
	e.mu.Lock()

	// Allow from: STOPPED, REVEALED, PREPARE, READY
	allowedPhases := e.state.Phase == PhaseStopped || e.state.Phase == PhaseRevealed ||
		e.state.Phase == PhasePrepare || e.state.Phase == PhaseReady
	if !allowedPhases {
		log.Printf("[Engine] Cannot ready game from phase %s", e.state.Phase)
		e.mu.Unlock()
		return
	}

	e.state.Phase = PhasePrepare
	e.state.Question = question
	e.setQuestionStatus(StatusPrepare)

	// Reset bumper times
	for _, bumper := range e.data.Bumpers {
		bumper.Time = 0
		bumper.Button = ""
		bumper.Status = ""
		bumper.Ready = false
		bumper.HintsAtBuzz = 0
	}

	// Reset team times
	for _, team := range e.data.Teams {
		team.Time = 0
		team.Bumper = ""
		team.Status = ""
		team.Ready = false
	}

	// Reset Memory game state for new question
	e.state.MemoryFlippedCards = nil
	e.state.MemoryMatchedPairs = nil
	e.state.MemoryErrors = 0

	// Reset QCM hints state for new question
	e.state.QcmInvalidated = nil

	log.Printf("[Engine] Game ready with question: %s", questionID)

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhasePrepare)
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

	if e.state.Phase != PhasePrepare {
		e.mu.Unlock()
		return
	}

	e.state.Phase = PhaseReady
	e.setQuestionStatus(StatusReady)
	log.Printf("[Engine] All teams ready, transitioning to READY")

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseReady)
	}
}

// Start begins the game round with a countdown
// For Memory questions, uses MEMORIZE_TIME from config (default 5s) plus cascade animation times
// For other questions, uses 3-second countdown
func (e *Engine) Start(delay int) {
	e.mu.Lock()

	// Store delay for after countdown
	e.pendingDelay = delay

	// Determine countdown duration
	countdownDuration := 3 // default for normal/QCM questions
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemory {
		memorizeTime := 5 // default
		if e.state.Question.MemoryConfig != nil && e.state.Question.MemoryConfig.MemorizeTime > 0 {
			memorizeTime = e.state.Question.MemoryConfig.MemorizeTime
		}

		// Calculate cascade animation duration for Memory questions
		// Frontend uses: STAGGER_DELAY = 200ms per card, FLIP_ANIMATION = 600ms
		cardCount := 0
		if e.state.Question.MemoryPairs != nil {
			cardCount = len(e.state.Question.MemoryPairs) * 2
		}
		// cascadeDuration = (cardCount * 200ms + 600ms) / 1000, rounded up
		cascadeDurationMs := cardCount*200 + 600
		cascadeDurationSecs := (cascadeDurationMs + 999) / 1000 // round up

		// Total = cascade_reveal + memorize_time + cascade_hide
		countdownDuration = cascadeDurationSecs + memorizeTime + cascadeDurationSecs
		log.Printf("[Engine] Memory countdown: cards=%d, cascade=%ds, memorize=%ds, total=%ds",
			cardCount, cascadeDurationSecs, memorizeTime, countdownDuration)
	}

	// Enter COUNTDOWN phase
	e.state.Phase = PhaseCountdown
	e.state.CountdownTime = countdownDuration
	e.state.Delay = delay
	e.state.CurrentTime = delay

	log.Printf("[Engine] Starting %d-second countdown before game (delay=%d)", countdownDuration, delay)

	// Start countdown timer
	e.startCountdown()

	// Capture callbacks before releasing lock
	stateCallback := e.OnStateChange
	countdownCallback := e.OnCountdownTick
	e.mu.Unlock()

	// Notify state change
	if stateCallback != nil {
		stateCallback(PhaseCountdown)
	}

	// Broadcast initial countdown value immediately
	// The ticker will then broadcast remaining values at 1-second intervals
	if countdownCallback != nil {
		countdownCallback(countdownDuration)
	}
}

// startCountdown starts the 3-2-1 countdown timer
func (e *Engine) startCountdown() {
	if e.countdownTimer != nil {
		e.countdownTimer.Stop()
	}

	// Create new stop channel for this countdown instance
	e.countdownStopCh = make(chan struct{})
	e.countdownTimer = time.NewTicker(1 * time.Second)

	// Capture references locally
	ticker := e.countdownTimer
	stopCh := e.countdownStopCh

	go func() {
		for {
			select {
			case <-ticker.C:
				e.mu.Lock()
				if e.state.Phase == PhaseCountdown {
					e.state.CountdownTime--
					countdownTime := e.state.CountdownTime
					e.mu.Unlock()

					if e.OnCountdownTick != nil {
						e.OnCountdownTick(countdownTime)
					}

					if countdownTime <= 0 {
						// Countdown finished, start the actual game
						e.actualStart()
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

// actualStart is called after countdown finishes to start the actual game
func (e *Engine) actualStart() {
	e.mu.Lock()

	// Stop countdown timer
	if e.countdownStopCh != nil {
		select {
		case <-e.countdownStopCh:
			// Already closed
		default:
			close(e.countdownStopCh)
		}
		e.countdownStopCh = nil
	}
	if e.countdownTimer != nil {
		e.countdownTimer.Stop()
		e.countdownTimer = nil
	}

	e.state.Phase = PhaseStarted
	e.state.CountdownTime = 0
	e.state.GameTime = time.Now().UnixMicro()

	e.setQuestionStatus(StatusStarted)

	// Clear Memory game state for fresh start
	e.state.MemoryFlippedCards = nil
	e.state.MemoryMatchedPairs = nil
	e.state.MemoryErrors = 0

	// Reset QCM hints state for fresh start
	e.state.QcmInvalidated = nil

	// Reset bumper times
	for _, bumper := range e.data.Bumpers {
		bumper.Time = 0
		bumper.Button = ""
		bumper.Status = ""
		bumper.HintsAtBuzz = 0
	}

	for _, team := range e.data.Teams {
		team.Time = 0
		team.Bumper = ""
		team.Status = ""
	}

	log.Printf("[Engine] Countdown finished, game started with delay %d", e.pendingDelay)

	// Start main timer
	e.startTimer()

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseStarted)
	}
}

// StartImmediate starts the game immediately without countdown (for tests)
func (e *Engine) StartImmediate(delay int) {
	e.mu.Lock()

	e.pendingDelay = delay
	e.state.Phase = PhaseStarted
	e.state.CountdownTime = 0
	e.state.GameTime = time.Now().UnixMicro()
	e.state.Delay = delay
	e.state.CurrentTime = delay

	e.setQuestionStatus(StatusStarted)

	// Reset Memory and QCM state
	e.state.MemoryFlippedCards = nil
	e.state.MemoryMatchedPairs = nil
	e.state.MemoryErrors = 0
	e.state.QcmInvalidated = nil

	// Reset bumper times
	for _, bumper := range e.data.Bumpers {
		bumper.Time = 0
		bumper.Button = ""
		bumper.Status = ""
		bumper.HintsAtBuzz = 0
	}

	for _, team := range e.data.Teams {
		team.Time = 0
		team.Bumper = ""
		team.Status = ""
	}

	log.Printf("[Engine] Game started immediately (no countdown) with delay %d", delay)

	// Start main timer
	e.startTimer()

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseStarted)
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
				if e.state.Phase == PhaseStarted {
					e.state.CurrentTime--
					currentTime := e.state.CurrentTime
					totalTime := e.state.Delay

					// Check for QCM hint invalidation
					var qcmHintCallback func(string, int)
					var invalidatedColor string
					var remainingAnswers int
					if e.shouldTriggerQCMHint(currentTime, totalTime) {
						invalidatedColor, remainingAnswers = e.invalidateRandomWrongAnswer()
						if invalidatedColor != "" {
							qcmHintCallback = e.OnQCMHint
						}
					}

					e.mu.Unlock()

					if e.OnTimerTick != nil {
						e.OnTimerTick(currentTime)
					}

					// Call QCM hint callback outside of lock
					if qcmHintCallback != nil && invalidatedColor != "" {
						qcmHintCallback(invalidatedColor, remainingAnswers)
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

// shouldTriggerQCMHint checks if a QCM hint should be triggered at the current time
// Must be called with lock held
func (e *Engine) shouldTriggerQCMHint(currentTime, totalTime int) bool {
	// Check if question is QCM with hints enabled
	if e.state.Question == nil || e.state.Question.Type != QuestionTypeQCM || !e.state.Question.QCMHintsEnabled {
		return false
	}

	// Get thresholds from question config (or use defaults)
	// Threshold 1: % of total time remaining for first hint (default 25%)
	// Threshold 2: % of total time remaining for second hint (default 12.5%)
	t1Percent := e.state.Question.QCMHintThreshold1
	t2Percent := e.state.Question.QCMHintThreshold2
	if t1Percent <= 0 {
		t1Percent = 0.25 // Default 25%
	}
	if t2Percent <= 0 {
		t2Percent = 0.125 // Default 12.5%
	}
	threshold1 := int(float64(totalTime) * t1Percent)
	threshold2 := int(float64(totalTime) * t2Percent)

	// Safety constraints:
	// - Min 1s between hints: threshold1 - threshold2 >= 1
	// - Last hint >= 1s before end: threshold2 >= 1
	if threshold1 <= 0 || threshold2 < 1 || (threshold1 - threshold2) < 1 {
		// Constraints not met, disable hints for this question
		return false
	}

	invalidatedCount := len(e.state.QcmInvalidated)

	// Check if we hit threshold 1 (first hint)
	if currentTime == threshold1 && invalidatedCount == 0 {
		log.Printf("[Engine] QCM hint threshold 1 reached: time=%d, total=%d, threshold=%d", currentTime, totalTime, threshold1)
		return true
	}

	// Check if we hit threshold 2 (second hint)
	if currentTime == threshold2 && invalidatedCount == 1 {
		log.Printf("[Engine] QCM hint threshold 2 reached: time=%d, total=%d, threshold=%d", currentTime, totalTime, threshold2)
		return true
	}

	return false
}

// invalidateRandomWrongAnswer invalidates a random wrong QCM answer
// Must be called with lock held
// Returns the invalidated color and the number of remaining valid answers
func (e *Engine) invalidateRandomWrongAnswer() (string, int) {
	if e.state.Question == nil || e.state.Question.QCMAnswers == nil {
		return "", 0
	}

	correctAnswer := e.state.Question.QCMCorrect
	allColors := []string{"RED", "GREEN", "YELLOW", "BLUE"}

	// Find wrong answers that haven't been invalidated yet
	var availableWrongAnswers []string
	for _, color := range allColors {
		if color == correctAnswer {
			continue // Skip correct answer
		}
		// Check if already invalidated
		isInvalidated := false
		for _, inv := range e.state.QcmInvalidated {
			if inv == color {
				isInvalidated = true
				break
			}
		}
		if !isInvalidated {
			availableWrongAnswers = append(availableWrongAnswers, color)
		}
	}

	if len(availableWrongAnswers) == 0 {
		return "", 4 - len(e.state.QcmInvalidated)
	}

	// Pick a random wrong answer to invalidate
	randomIndex := rand.Intn(len(availableWrongAnswers))
	invalidatedColor := availableWrongAnswers[randomIndex]

	// Add to invalidated list
	e.state.QcmInvalidated = append(e.state.QcmInvalidated, invalidatedColor)

	// Calculate remaining valid answers (4 total - invalidated count)
	remainingAnswers := 4 - len(e.state.QcmInvalidated)

	log.Printf("[Engine] QCM hint: invalidated %s, remaining answers: %d", invalidatedColor, remainingAnswers)
	return invalidatedColor, remainingAnswers
}

// Stop ends the game round
func (e *Engine) Stop() {
	e.mu.Lock()

	// Signal countdown timer goroutine to stop (if running)
	if e.countdownStopCh != nil {
		select {
		case <-e.countdownStopCh:
			// Already closed
		default:
			close(e.countdownStopCh)
		}
		e.countdownStopCh = nil
	}
	if e.countdownTimer != nil {
		e.countdownTimer.Stop()
		e.countdownTimer = nil
	}

	// Signal main timer goroutine to stop
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

	e.state.Phase = PhaseStopped
	e.state.CurrentTime = 0
	e.state.CountdownTime = 0

	e.setQuestionStatus(StatusStopped)

	log.Printf("[Engine] Game stopped")

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseStopped)
	}
}

// Pause pauses the game (single buzzer)
func (e *Engine) Pause() {
	e.mu.Lock()

	e.state.Phase = PhasePaused

	e.setQuestionStatus(StatusPaused)

	log.Printf("[Engine] Game paused")

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhasePaused)
	}
}

// PauseAll pauses for all buzzers
func (e *Engine) PauseAll() {
	e.Pause()
}

// Continue resumes the game
func (e *Engine) Continue() {
	e.mu.Lock()

	e.state.Phase = PhaseStarted

	e.setQuestionStatus(StatusStarted)

	log.Printf("[Engine] Game continued")

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseStarted)
	}
}

// Reveal shows the answer
func (e *Engine) Reveal() string {
	e.mu.Lock()

	// Allow reveal from STOPPED or PAUSED
	if e.state.Phase != PhaseStopped && e.state.Phase != PhasePaused {
		log.Printf("[Engine] Cannot reveal from phase %s (must be STOPPED or PAUSED)", e.state.Phase)
		e.mu.Unlock()
		return ""
	}

	// Stop timer if revealing from PAUSED
	if e.state.Phase == PhasePaused {
		// Signal countdown timer goroutine to stop (if running)
		if e.countdownStopCh != nil {
			select {
			case <-e.countdownStopCh:
				// Already closed
			default:
				close(e.countdownStopCh)
			}
			e.countdownStopCh = nil
		}
		if e.countdownTimer != nil {
			e.countdownTimer.Stop()
			e.countdownTimer = nil
		}

		// Signal main timer goroutine to stop
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
	}

	e.state.Phase = PhaseRevealed

	var answer string
	if e.state.Question != nil {
		e.setQuestionStatus(StatusRevealed)
		answer = e.state.Question.Answer
		log.Printf("[Engine] Answer revealed")
	}

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseRevealed)
	}

	return answer
}


// ProcessButtonPress handles a button press from a buzzer
func (e *Engine) ProcessButtonPress(bumperID string, pressTime int64, button string) {
	e.mu.Lock()

	if e.state.Phase != PhaseStarted {
		log.Printf("[Engine] Ignoring button press, game not started")
		e.mu.Unlock()
		return
	}

	// Ignore buzz for MEMORY questions - admin controls the game
	if e.state.Question != nil && e.state.Question.Type == "MEMORY" {
		log.Printf("[Engine] Ignoring buzz for MEMORY question from %s", bumperID)
		e.mu.Unlock()
		return
	}

	bumper, ok := e.data.Bumpers[bumperID]
	if !ok {
		log.Printf("[Engine] Unknown bumper: %s", bumperID)
		e.mu.Unlock()
		return
	}

	// Check if bumper already pressed
	if bumper.Time > 0 {
		log.Printf("[Engine] Bumper %s already pressed", bumperID)
		e.mu.Unlock()
		return
	}

	teamID := bumper.Team
	if teamID == "" {
		// Bumper has no team - allow individual press
		bumper.Time = pressTime
		bumper.Button = button
		bumper.Status = "PAUSE"
		e.mu.Unlock()
		return
	}

	team, ok := e.data.Teams[teamID]
	if !ok {
		e.mu.Unlock()
		return
	}

	// Check if team already has a press - only ONE player per team can buzz
	if team.Time > 0 {
		log.Printf("[Engine] Team %s already buzzed, ignoring bumper %s", teamID, bumperID)
		e.mu.Unlock()
		return
	}

	// Record the press for both bumper and team
	bumper.Time = pressTime
	bumper.Button = button
	bumper.Status = "PAUSE"

	// Store the number of QCM hints at buzz time for per-player penalty calculation
	bumper.HintsAtBuzz = len(e.state.QcmInvalidated)

	team.Time = pressTime
	team.Bumper = bumperID
	team.Status = "PAUSE"

	log.Printf("[Engine] Button press: bumper=%s, team=%s, button=%s, time=%d",
		bumperID, teamID, button, pressTime)

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnBuzzerPress
	e.mu.Unlock()

	if callback != nil {
		callback(bumperID, teamID, pressTime, button)
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

// UpdateBumperScore updates the bumper score and recalculates team score
func (e *Engine) UpdateBumperScore(bumperID string, points int) int {
	e.mu.Lock()

	bumper, ok := e.data.Bumpers[bumperID]
	if !ok {
		log.Printf("[Engine] UpdateBumperScore: bumper not found: %s", bumperID)
		e.mu.Unlock()
		return 0
	}

	bumper.Score += points
	log.Printf("[Engine] Bumper score update: bumper=%s, points=%+d, newScore=%d",
		bumperID, points, bumper.Score)

	// Recalculate team score as sum of all bumpers in team
	if bumper.Team != "" {
		e.recalculateTeamScoreUnsafe(bumper.Team)
	}

	score := bumper.Score
	e.mu.Unlock()

	// Auto-save (scores are part of bumper/team data)
	go e.SaveBumpers()
	go e.SaveTeams()

	return score
}

// UpdateTeamScore adds points directly to team's own TeamPoints (not bumpers)
func (e *Engine) UpdateTeamScore(teamName string, points int) int {
	e.mu.Lock()

	team, ok := e.data.Teams[teamName]
	if !ok {
		log.Printf("[Engine] UpdateTeamScore: team not found: %s", teamName)
		e.mu.Unlock()
		return 0
	}

	// Add points directly to TeamPoints (independent from bumper scores)
	team.TeamPoints += points
	log.Printf("[Engine] Team points update: team=%s, points=%+d, newTeamPoints=%d",
		teamName, points, team.TeamPoints)

	// Recalculate total team score (TeamPoints + bumper scores)
	e.recalculateTeamScoreUnsafe(teamName)

	score := team.Score
	e.mu.Unlock()

	// Auto-save teams to disk
	go e.SaveTeams()

	return score
}

// recalculateTeamScoreUnsafe sets team score to TeamPoints + sum of bumper scores (caller must hold lock)
func (e *Engine) recalculateTeamScoreUnsafe(teamName string) {
	team, ok := e.data.Teams[teamName]
	if !ok {
		return
	}

	var bumperTotal int
	for _, bumper := range e.data.Bumpers {
		if bumper.Team == teamName {
			bumperTotal += bumper.Score
		}
	}

	team.Score = team.TeamPoints + bumperTotal
	log.Printf("[Engine] Team score recalculated: team=%s, teamPoints=%d, bumperTotal=%d, totalScore=%d",
		teamName, team.TeamPoints, bumperTotal, team.Score)
}

// RecalculateAllTeamScores recalculates scores for all teams based on bumper scores
func (e *Engine) RecalculateAllTeamScores() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for teamName := range e.data.Teams {
		e.recalculateTeamScoreUnsafe(teamName)
	}
	log.Printf("[Engine] All team scores recalculated")
}

// RAZScores resets all scores to zero
func (e *Engine) RAZScores() {
	e.mu.Lock()

	for _, bumper := range e.data.Bumpers {
		bumper.Score = 0
		bumper.Time = 0
	}

	for _, team := range e.data.Teams {
		team.Score = 0
		team.TeamPoints = 0
		team.Time = 0
	}

	// Clear history as well
	e.history = nil

	log.Printf("[Engine] All scores and history reset")
	e.mu.Unlock()

	// Save all data to disk
	go e.SaveHistory()
	go e.SaveTeams()
	go e.SaveBumpers()
}

// ClearBumpers removes all bumpers (keeps teams intact)
func (e *Engine) ClearBumpers() {
	e.mu.Lock()
	e.data.Bumpers = make(map[string]*Bumper)

	// Reset team references to bumpers
	for _, team := range e.data.Teams {
		team.Bumper = ""
		team.Time = 0
		team.Status = ""
		team.Ready = false
	}

	log.Printf("[Engine] All bumpers cleared and dissociated from teams")
	e.mu.Unlock()

	// Auto-save empty bumpers and updated teams
	go e.SaveBumpers()
	go e.SaveTeams()
}

// ClearAll removes all teams and bumpers
func (e *Engine) ClearAll() {
	e.mu.Lock()
	e.data.Bumpers = make(map[string]*Bumper)
	e.data.Teams = make(map[string]*Team)
	log.Printf("[Engine] All teams and bumpers cleared")
	e.mu.Unlock()

	// Auto-save empty data
	go e.SaveTeams()
	go e.SaveBumpers()
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

// SetBackgrounds sets all backgrounds
func (e *Engine) SetBackgrounds(backgrounds []Background) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Backgrounds = backgrounds
}

// GetBackgrounds returns all backgrounds
func (e *Engine) GetBackgrounds() []Background {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Backgrounds
}

// AddBackground adds a background
func (e *Engine) AddBackground(bg Background) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Backgrounds = append(e.state.Backgrounds, bg)
}

// RemoveBackground removes a background by path
func (e *Engine) RemoveBackground(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	newBgs := make([]Background, 0, len(e.state.Backgrounds))
	for _, bg := range e.state.Backgrounds {
		if bg.Path != path {
			newBgs = append(newBgs, bg)
		}
	}
	e.state.Backgrounds = newBgs
}

// ClearBackgrounds removes all backgrounds
func (e *Engine) ClearBackgrounds() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Backgrounds = nil
	e.state.CurrentBackgroundIndex = 0
}

// GetCurrentBackgroundIndex returns the current background index
func (e *Engine) GetCurrentBackgroundIndex() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.CurrentBackgroundIndex
}

// SetCurrentBackgroundIndex sets the current background index
func (e *Engine) SetCurrentBackgroundIndex(index int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.state.Backgrounds) > 0 {
		e.state.CurrentBackgroundIndex = index % len(e.state.Backgrounds)
	} else {
		e.state.CurrentBackgroundIndex = 0
	}
}

// NextBackground advances to the next background and returns the new index
func (e *Engine) NextBackground() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.state.Backgrounds) > 0 {
		e.state.CurrentBackgroundIndex = (e.state.CurrentBackgroundIndex + 1) % len(e.state.Backgrounds)
	}
	return e.state.CurrentBackgroundIndex
}

// GetCurrentBackgroundDuration returns the duration of the current background in seconds
func (e *Engine) GetCurrentBackgroundDuration() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.state.Backgrounds) == 0 {
		return 0
	}
	bg := e.state.Backgrounds[e.state.CurrentBackgroundIndex]
	if bg.Duration <= 0 {
		return 10 // Default 10 seconds
	}
	return bg.Duration
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


// ForceReady forces transition to READY phase (debug function, skips PONG wait)
func (e *Engine) ForceReady() {
	e.mu.Lock()

	if e.state.Phase != PhasePrepare {
		log.Printf("[Engine] ForceReady: not in PREPARE phase, current: %s", e.state.Phase)
		e.mu.Unlock()
		return
	}

	// Mark all bumpers as ready
	for _, bumper := range e.data.Bumpers {
		bumper.Ready = true
	}

	// Mark all teams as ready
	for _, team := range e.data.Teams {
		team.Ready = true
	}

	e.state.Phase = PhaseReady
	e.setQuestionStatus(StatusReady)
	log.Printf("[Engine] FORCE_READY: transitioning to READY")

	// Release lock BEFORE calling callback to avoid deadlock
	callback := e.OnStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(PhaseReady)
	}
}

// IsGameStopped returns true if game is stopped
func (e *Engine) IsGameStopped() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == PhaseStopped
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
	return e.state.Phase == PhaseStarted
}

// AddGameEvent adds an event to the history and saves to disk
func (e *Engine) AddGameEvent(event GameEvent) {
	e.mu.Lock()
	e.history = append(e.history, event)
	log.Printf("[Engine] Game event added: type=%s, winner=%s, points=%d",
		event.EventType, event.WinnerName, event.Points)
	e.mu.Unlock()

	// Save history to disk (non-blocking, async)
	go e.SaveHistory()
}

// GetHistory returns the game event history
func (e *Engine) GetHistory() []GameEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	// Return a copy to avoid race conditions
	result := make([]GameEvent, len(e.history))
	copy(result, e.history)
	return result
}

// ClearHistory clears the game event history
func (e *Engine) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = nil
	log.Printf("[Engine] History cleared")
}

// SetHistoryPath sets the path for history persistence
func (e *Engine) SetHistoryPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.historyPath = path
	log.Printf("[Engine] History path set to: %s", path)
}

// SaveHistory persists history to disk
func (e *Engine) SaveHistory() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.historyPath == "" {
		return nil // No path configured, skip
	}

	// Ensure directory exists
	dir := filepath.Dir(e.historyPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create history directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(e.history, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal history: %v", err)
		return err
	}

	if err := os.WriteFile(e.historyPath, data, 0644); err != nil {
		log.Printf("[Engine] Failed to save history: %v", err)
		return err
	}

	log.Printf("[Engine] History saved: %d events to %s", len(e.history), e.historyPath)
	return nil
}

// LoadHistory loads history from disk
func (e *Engine) LoadHistory() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.historyPath == "" {
		return nil // No path configured, skip
	}

	data, err := os.ReadFile(e.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No history file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read history: %v", err)
		return err
	}

	var history []GameEvent
	if err := json.Unmarshal(data, &history); err != nil {
		log.Printf("[Engine] Failed to parse history: %v", err)
		return err
	}

	e.history = history
	log.Printf("[Engine] History loaded: %d events from %s", len(history), e.historyPath)
	return nil
}

// RecalculateScoresFromHistory recalculates all scores from history events
func (e *Engine) RecalculateScoresFromHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Reset all scores first
	for _, bumper := range e.data.Bumpers {
		bumper.Score = 0
	}
	for _, team := range e.data.Teams {
		team.Score = 0
		team.TeamPoints = 0
	}

	// Replay all events
	playerPoints := 0
	teamPoints := 0

	for _, event := range e.history {
		switch event.WinnerType {
		case "PLAYER":
			// Add points to bumper
			if bumper, ok := e.data.Bumpers[event.WinnerID]; ok {
				bumper.Score += event.Points
				playerPoints += event.Points
			}
		case "TEAM":
			// Add points to team's TeamPoints
			if team, ok := e.data.Teams[event.TeamName]; ok {
				team.TeamPoints += event.Points
				teamPoints += event.Points
			}
		}
	}

	// Recalculate all team total scores (TeamPoints + bumper scores)
	for teamName := range e.data.Teams {
		e.recalculateTeamScoreUnsafe(teamName)
	}

	log.Printf("[Engine] Scores recalculated from history: %d events, playerPoints=%d, teamPoints=%d",
		len(e.history), playerPoints, teamPoints)
}

// SetTeamsPath sets the path for teams persistence
func (e *Engine) SetTeamsPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.teamsPath = path
	log.Printf("[Engine] Teams path set to: %s", path)
}

// SetBumpersPath sets the path for bumpers persistence
func (e *Engine) SetBumpersPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bumpersPath = path
	log.Printf("[Engine] Bumpers path set to: %s", path)
}

// SaveTeams persists teams to disk
func (e *Engine) SaveTeams() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.teamsPath == "" {
		return nil
	}

	dir := filepath.Dir(e.teamsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create teams directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(e.data.Teams, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal teams: %v", err)
		return err
	}

	if err := os.WriteFile(e.teamsPath, data, 0644); err != nil {
		log.Printf("[Engine] Failed to save teams: %v", err)
		return err
	}

	log.Printf("[Engine] Teams saved: %d teams to %s", len(e.data.Teams), e.teamsPath)
	return nil
}

// LoadTeams loads teams from disk
func (e *Engine) LoadTeams() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.teamsPath == "" {
		return nil
	}

	data, err := os.ReadFile(e.teamsPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No teams file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read teams: %v", err)
		return err
	}

	var teams map[string]*Team
	if err := json.Unmarshal(data, &teams); err != nil {
		log.Printf("[Engine] Failed to parse teams: %v", err)
		return err
	}

	e.data.Teams = teams
	log.Printf("[Engine] Teams loaded: %d teams from %s", len(teams), e.teamsPath)
	return nil
}

// SaveBumpers persists bumpers to disk
func (e *Engine) SaveBumpers() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.bumpersPath == "" {
		return nil
	}

	dir := filepath.Dir(e.bumpersPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create bumpers directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(e.data.Bumpers, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal bumpers: %v", err)
		return err
	}

	if err := os.WriteFile(e.bumpersPath, data, 0644); err != nil {
		log.Printf("[Engine] Failed to save bumpers: %v", err)
		return err
	}

	log.Printf("[Engine] Bumpers saved: %d bumpers to %s", len(e.data.Bumpers), e.bumpersPath)
	return nil
}

// LoadBumpers loads bumpers from disk
func (e *Engine) LoadBumpers() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.bumpersPath == "" {
		return nil
	}

	data, err := os.ReadFile(e.bumpersPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No bumpers file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read bumpers: %v", err)
		return err
	}

	var bumpers map[string]*Bumper
	if err := json.Unmarshal(data, &bumpers); err != nil {
		log.Printf("[Engine] Failed to parse bumpers: %v", err)
		return err
	}

	e.data.Bumpers = bumpers
	log.Printf("[Engine] Bumpers loaded: %d bumpers from %s", len(bumpers), e.bumpersPath)
	return nil
}

// SetStatusesPath sets the path for question statuses persistence
func (e *Engine) SetStatusesPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.statusesPath = path
	log.Printf("[Engine] Statuses path set to: %s", path)
}

// SaveStatuses persists question statuses to disk
func (e *Engine) SaveStatuses() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.statusesPath == "" {
		return nil
	}

	dir := filepath.Dir(e.statusesPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create statuses directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(e.questionStatuses, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal statuses: %v", err)
		return err
	}

	if err := os.WriteFile(e.statusesPath, data, 0644); err != nil {
		log.Printf("[Engine] Failed to save statuses: %v", err)
		return err
	}

	log.Printf("[Engine] Statuses saved: %d statuses to %s", len(e.questionStatuses), e.statusesPath)
	return nil
}

// LoadStatuses loads question statuses from disk
func (e *Engine) LoadStatuses() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.statusesPath == "" {
		return nil
	}

	data, err := os.ReadFile(e.statusesPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No statuses file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read statuses: %v", err)
		return err
	}

	var statuses map[string]QuestionStatus
	if err := json.Unmarshal(data, &statuses); err != nil {
		log.Printf("[Engine] Failed to parse statuses: %v", err)
		return err
	}

	e.questionStatuses = statuses
	log.Printf("[Engine] Statuses loaded: %d statuses from %s", len(statuses), e.statusesPath)
	return nil
}

// ClearStatuses resets all question statuses
func (e *Engine) ClearStatuses() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.questionStatuses = make(map[string]QuestionStatus)
	log.Printf("[Engine] Question statuses cleared")
}

// SaveAll saves teams, bumpers, history and statuses to disk
func (e *Engine) SaveAll() {
	go e.SaveTeams()
	go e.SaveBumpers()
	go e.SaveHistory()
	go e.SaveStatuses()
}

// FlipMemoryCard handles flipping a Memory card with game logic
// Returns: (isMatch bool, shouldFlipBack bool, flipDelay int, isComplete bool)
// - isMatch: true if the 2nd card matches the 1st (same pair)
// - shouldFlipBack: true if we have 2 non-matching cards and need to flip back
// - flipDelay: milliseconds to wait before flipping back (from question config)
// - isComplete: true if all pairs have been matched (game complete)
func (e *Engine) FlipMemoryCard(cardID string) (isMatch bool, shouldFlipBack bool, flipDelay int, isComplete bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Only allow flipping during STARTED phase
	if e.state.Phase != PhaseStarted {
		log.Printf("[Engine] Memory flip ignored: game not in STARTED phase (current: %s)", e.state.Phase)
		return false, false, 0, false
	}

	// Extract pair ID from card ID (format: "pairID-cardNum", e.g., "1-1", "2-2")
	pairID := e.extractPairID(cardID)
	if pairID == 0 {
		log.Printf("[Engine] Memory flip ignored: invalid card ID format: %s", cardID)
		return false, false, 0, false
	}

	// Check if this pair is already matched
	for _, matchedPairID := range e.state.MemoryMatchedPairs {
		if matchedPairID == pairID {
			log.Printf("[Engine] Memory flip ignored: pair %d already matched", pairID)
			return false, false, 0, false
		}
	}

	// Check if card is already flipped
	for _, id := range e.state.MemoryFlippedCards {
		if id == cardID {
			log.Printf("[Engine] Memory flip ignored: card %s already flipped", cardID)
			return false, false, 0, false
		}
	}

	// Don't allow more than 2 cards to be flipped at once
	if len(e.state.MemoryFlippedCards) >= 2 {
		log.Printf("[Engine] Memory flip ignored: already 2 cards flipped")
		return false, false, 0, false
	}

	// Add card to flipped cards
	e.state.MemoryFlippedCards = append(e.state.MemoryFlippedCards, cardID)
	log.Printf("[Engine] Memory card %s flipped (revealed)", cardID)

	// If only 1 card flipped, just wait for second card
	if len(e.state.MemoryFlippedCards) == 1 {
		return false, false, 0, false
	}

	// Two cards are now flipped - check if they match
	firstCardID := e.state.MemoryFlippedCards[0]
	secondCardID := e.state.MemoryFlippedCards[1]
	firstPairID := e.extractPairID(firstCardID)
	secondPairID := e.extractPairID(secondCardID)

	// Get flip delay from question config (config is in seconds, convert to ms)
	flipDelay = 3000 // Default 3 seconds
	if e.state.Question != nil && e.state.Question.MemoryConfig != nil && e.state.Question.MemoryConfig.FlipDelay > 0 {
		flipDelay = int(e.state.Question.MemoryConfig.FlipDelay * 1000)
	}

	if firstPairID == secondPairID {
		// MATCH! Add to matched pairs and clear flipped cards
		e.state.MemoryMatchedPairs = append(e.state.MemoryMatchedPairs, firstPairID)
		e.state.MemoryFlippedCards = nil

		// Check if all pairs are matched (game complete)
		totalPairs := 0
		if e.state.Question != nil && e.state.Question.MemoryPairs != nil {
			totalPairs = len(e.state.Question.MemoryPairs)
		}
		isComplete = len(e.state.MemoryMatchedPairs) >= totalPairs && totalPairs > 0

		log.Printf("[Engine] Memory MATCH! Pair %d found. Total matched: %d/%d. Complete: %v", firstPairID, len(e.state.MemoryMatchedPairs), totalPairs, isComplete)
		return true, false, 0, isComplete
	}

	// No match - increment error counter, caller should schedule flip-back after delay
	e.state.MemoryErrors++
	log.Printf("[Engine] Memory NO MATCH (error #%d). Cards %s and %s will flip back after %dms", e.state.MemoryErrors, firstCardID, secondCardID, flipDelay)
	return false, true, flipDelay, false
}

// extractPairID extracts the pair ID from a card ID (format: "pairID-cardNum")
func (e *Engine) extractPairID(cardID string) int {
	var pairID, cardNum int
	_, err := parseCardID(cardID, &pairID, &cardNum)
	if err != nil {
		return 0
	}
	return pairID
}

// parseCardID parses "pairID-cardNum" format
func parseCardID(cardID string, pairID, cardNum *int) (bool, error) {
	n, _ := parseCardIDParts(cardID)
	if n >= 1 {
		parts := splitCardID(cardID)
		if len(parts) == 2 {
			*pairID = parseInt(parts[0])
			*cardNum = parseInt(parts[1])
			return true, nil
		}
	}
	return false, nil
}

func splitCardID(cardID string) []string {
	var parts []string
	current := ""
	for _, c := range cardID {
		if c == '-' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func parseCardIDParts(cardID string) (int, error) {
	count := 0
	for _, c := range cardID {
		if c == '-' {
			count++
		}
	}
	return count + 1, nil
}

func parseInt(s string) int {
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// ClearMemoryFlippedCards resets all flipped cards
func (e *Engine) ClearMemoryFlippedCards() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.MemoryFlippedCards = nil
	log.Printf("[Engine] Memory flipped cards cleared")
}

// GetMemoryFlippedCards returns the list of flipped card IDs
func (e *Engine) GetMemoryFlippedCards() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.MemoryFlippedCards
}

// CalculateMemoryScore calculates the score for a Memory game based on matched pairs, errors, and config
// Returns: score, matchedPairs, totalPairs, errors, isComplete
func (e *Engine) CalculateMemoryScore() (int, int, int, int, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.state.Question == nil || e.state.Question.Type != QuestionTypeMemory {
		return 0, 0, 0, 0, false
	}

	// Get config values with defaults
	pointsPerPair := 10
	errorPenalty := 0
	completionBonus := 0

	if e.state.Question.MemoryConfig != nil {
		if e.state.Question.MemoryConfig.PointsPerPair > 0 {
			pointsPerPair = e.state.Question.MemoryConfig.PointsPerPair
		}
		errorPenalty = e.state.Question.MemoryConfig.ErrorPenalty
		completionBonus = e.state.Question.MemoryConfig.CompletionBonus
	}

	// Calculate stats
	matchedPairs := len(e.state.MemoryMatchedPairs)
	totalPairs := len(e.state.Question.MemoryPairs)
	errors := e.state.MemoryErrors
	isComplete := matchedPairs == totalPairs && totalPairs > 0

	// Calculate score: (matched × pointsPerPair) + completionBonus - (errors × errorPenalty)
	score := matchedPairs * pointsPerPair
	if isComplete {
		score += completionBonus
	}
	score -= errors * errorPenalty

	// Score cannot be negative
	if score < 0 {
		score = 0
	}

	log.Printf("[Engine] Memory score: matched=%d/%d, errors=%d, complete=%v, score=%d (perPair=%d, bonus=%d, penalty=%d)",
		matchedPairs, totalPairs, errors, isComplete, score, pointsPerPair, completionBonus, errorPenalty)

	return score, matchedPairs, totalPairs, errors, isComplete
}

// SetEnrollmentActive enables or disables virtual player enrollment (QR code display)
func (e *Engine) SetEnrollmentActive(active bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.EnrollmentActive = active
	log.Printf("[Engine] Enrollment active: %v", active)
}

// GetEnrollmentActive returns whether enrollment is currently active
func (e *Engine) GetEnrollmentActive() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.EnrollmentActive
}

// SetShowQRCode sets whether the QR code should be displayed on TV
func (e *Engine) SetShowQRCode(show bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.ShowQRCode = show

	// When showing QR code, switch to ENROLL phase
	// When hiding QR code, return to STOPPED phase
	if show {
		e.state.Phase = PhaseEnroll
		log.Printf("[Engine] Show QR code: enabled, switching to ENROLL phase")
	} else {
		e.state.Phase = PhaseStopped
		log.Printf("[Engine] Show QR code: disabled, returning to STOPPED phase")
	}
}

// GetShowQRCode returns whether the QR code should be displayed
func (e *Engine) GetShowQRCode() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.ShowQRCode
}

// GetVirtualPlayerCount returns the current count of enrolled virtual players
func (e *Engine) GetVirtualPlayerCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.VirtualPlayerCount
}

// IncrementVirtualPlayerCount increments the virtual player counter
func (e *Engine) IncrementVirtualPlayerCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.VirtualPlayerCount++
	count := e.state.VirtualPlayerCount
	log.Printf("[Engine] Virtual player count: %d", count)
	return count
}

// ResetVirtualPlayerCount resets the virtual player counter to zero
func (e *Engine) ResetVirtualPlayerCount() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.VirtualPlayerCount = 0
	log.Printf("[Engine] Virtual player count reset")
}

// GetVirtualPlayerLimit returns the maximum number of virtual players allowed
func (e *Engine) GetVirtualPlayerLimit() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.state.VirtualPlayerLimit == 0 {
		return 20 // Default limit
	}
	return e.state.VirtualPlayerLimit
}

// SetVirtualPlayerLimit sets the maximum number of virtual players allowed
func (e *Engine) SetVirtualPlayerLimit(limit int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if limit < 1 {
		limit = 20 // Minimum 1, default to 20 if invalid
	}
	e.state.VirtualPlayerLimit = limit
	log.Printf("[Engine] Virtual player limit set to: %d", limit)
}

// CreateVirtualPlayer creates a new virtual player (bumper) during enrollment
func (e *Engine) CreateVirtualPlayer(name string) (string, *Bumper, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check phase ENROLL
	if e.state.Phase != PhaseEnroll {
		return "", nil, &EnrollmentError{Reason: "ENROLLMENT_CLOSED"}
	}

	// Check limit
	currentCount := e.countVirtualPlayersUnsafe()
	limit := e.state.VirtualPlayerLimit
	if limit == 0 {
		limit = 20 // Default
	}
	if currentCount >= limit {
		return "", nil, &EnrollmentError{Reason: "LIMIT_REACHED"}
	}

	// Generate unique ID
	id := "vjoueur_" + name + "_" + time.Now().Format("20060102_150405")

	// Create virtual bumper
	bumper := &Bumper{
		Name:      name,
		Team:      "",
		Score:     0,
		IsVirtual: true,
		Status:    "READY",
	}

	e.data.Bumpers[id] = bumper

	// Increment virtual player count in GameState
	e.state.VirtualPlayerCount++
	log.Printf("[Engine] Virtual player created: id=%s, name=%s, count=%d", id, name, e.state.VirtualPlayerCount)

	// Save bumpers to disk (in goroutine to avoid blocking)
	go e.SaveBumpers()

	return id, bumper, nil
}

// countVirtualPlayersUnsafe counts virtual players (caller must hold lock)
func (e *Engine) countVirtualPlayersUnsafe() int {
	count := 0
	for _, b := range e.data.Bumpers {
		if b.IsVirtual {
			count++
		}
	}
	return count
}

// CountVirtualPlayers counts virtual players (thread-safe)
func (e *Engine) CountVirtualPlayers() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.countVirtualPlayersUnsafe()
}

// SyncVirtualPlayerCount synchronizes VirtualPlayerCount with actual bumper count
// This should be called after loading bumpers from disk
func (e *Engine) SyncVirtualPlayerCount() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.VirtualPlayerCount = e.countVirtualPlayersUnsafe()
	log.Printf("[Engine] Virtual player count synchronized: %d", e.state.VirtualPlayerCount)
}

// EnrollmentError represents an enrollment rejection error
type EnrollmentError struct {
	Reason string
}

func (e *EnrollmentError) Error() string {
	return e.Reason
}

// StartEnrollment starts the virtual player enrollment process
func (e *Engine) StartEnrollment(maxPlayers int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.EnrollmentActive = true
	e.state.ShowQRCode = true
	e.state.VirtualPlayerLimit = maxPlayers
	log.Printf("[Engine] Enrollment started with limit: %d", maxPlayers)
}

// StopEnrollment stops the virtual player enrollment process
func (e *Engine) StopEnrollment() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.EnrollmentActive = false
	e.state.ShowQRCode = false
	log.Printf("[Engine] Enrollment stopped")
}

// HandleVirtualPlayerConnect handles a virtual player connection request
// Returns (bumperID, bumper, error)
func (e *Engine) HandleVirtualPlayerConnect(name, sessionID string) (string, *Bumper, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if enrollment is active
	if !e.state.EnrollmentActive {
		return "", nil, &EnrollmentError{Reason: "ENROLLMENT_CLOSED"}
	}

	// Check if limit is reached
	virtualCount := e.countVirtualPlayersUnsafe()
	if virtualCount >= e.state.VirtualPlayerLimit {
		return "", nil, &EnrollmentError{Reason: "ENROLLMENT_FULL"}
	}

	// Validate name (2-20 characters, alphanumeric + spaces)
	if len(name) < 2 || len(name) > 20 {
		return "", nil, &EnrollmentError{Reason: "INVALID_NAME"}
	}

	// Check if name is already taken
	for _, bumper := range e.data.Bumpers {
		if bumper.Name == name && bumper.IsVirtual {
			return "", nil, &EnrollmentError{Reason: "PSEUDO_TAKEN"}
		}
	}

	// Generate unique ID using sessionID
	id := sessionID
	if id == "" {
		id = "vplayer_" + name + "_" + time.Now().Format("20060102_150405")
	}

	// Create virtual bumper
	bumper := &Bumper{
		Name:      name,
		Team:      "",
		Score:     0,
		IsVirtual: true,
		Status:    "READY",
	}

	e.data.Bumpers[id] = bumper
	e.state.VirtualPlayerCount++

	log.Printf("[Engine] Virtual player connected: id=%s, name=%s, sessionID=%s", id, name, sessionID)
	return id, bumper, nil
}

// reconnectVPlayer reconnects an existing virtual player
func (e *Engine) reconnectVPlayer(sessionID string) (string, *Bumper, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Find existing bumper with this sessionID
	for id, bumper := range e.data.Bumpers {
		if bumper.IsVirtual && id == sessionID {
			log.Printf("[Engine] Virtual player reconnected: id=%s, name=%s", id, bumper.Name)
			return id, bumper, nil
		}
	}

	return "", nil, &EnrollmentError{Reason: "SESSION_NOT_FOUND"}
}

// AssignVirtualPlayer assigns a virtual player to a team and answer color
func (e *Engine) AssignVirtualPlayer(bumperID, team string, answerColor AnswerColor) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	bumper, exists := e.data.Bumpers[bumperID]
	if !exists {
		return &EnrollmentError{Reason: "BUMPER_NOT_FOUND"}
	}

	if !bumper.IsVirtual {
		return &EnrollmentError{Reason: "NOT_VIRTUAL_PLAYER"}
	}

	// Check if team exists
	if _, exists := e.data.Teams[team]; !exists {
		return &EnrollmentError{Reason: "TEAM_NOT_FOUND"}
	}

	// Assign team and answer color
	bumper.Team = team
	bumper.AnswerColor = answerColor

	log.Printf("[Engine] Virtual player assigned: id=%s, team=%s, color=%s", bumperID, team, answerColor)

	// Save bumpers to disk
	go e.SaveBumpers()

	return nil
}

// GetEnrollmentStatus returns enrollment active status
func (e *Engine) GetEnrollmentStatus() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.EnrollmentActive
}
