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
			// MEMOTION: initialize with empty (not nil) so JSON serializes [] and {} (not null)
			MotionCardStates:         make(map[string]string),
			MotionCardTeams:          make(map[string]string),
			MotionParticipatingTeams: []string{},
			MotionCurrentTeamColor:   []int{},
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
	if proto, ok := data["PROTOCOL"].(string); ok {
		bumper.Protocol = proto
	}
	// OTA firmware fields (v3.1.0+)
	if fwVersion, ok := data["FIRMWARE_VERSION"].(string); ok {
		bumper.FirmwareVersion = fwVersion
	}
	if isOutdated, ok := data["IS_OUTDATED"].(bool); ok {
		bumper.IsOutdated = isOutdated
	}
	if otaStatus, ok := data["OTA_STATUS"].(string); ok {
		bumper.OTAStatus = otaStatus
	}
	if otaPercent, ok := data["OTA_PERCENT"].(int); ok {
		bumper.OTAPercent = otaPercent
	}
	// Connection status (v3.6.5)
	if connected, ok := data["CONNECTED"].(bool); ok {
		bumper.Connected = connected
	}
	// ACK pending flag (v3.8.0)
	if ackPending, ok := data["ACK_PENDING"].(bool); ok {
		bumper.AckPending = ackPending
	}

	log.Printf("[Engine] Updated bumper %s: team=%s, name=%s, protocol=%s", id, bumper.Team, bumper.Name, bumper.Protocol)
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

	// Allow from: STOPPED, REVEALED, PREPARE, READY, NEW_GAME
	allowedPhases := e.state.Phase == PhaseStopped || e.state.Phase == PhaseRevealed ||
		e.state.Phase == PhasePrepare || e.state.Phase == PhaseReady ||
		e.state.Phase == PhaseNewGame
	if !allowedPhases {
		log.Printf("[Engine] Cannot ready game from phase %s", e.state.Phase)
		e.mu.Unlock()
		return
	}

	// Check if this is a NEW question (different from current)
	isNewQuestion := e.state.Question == nil || e.state.Question.ID != questionID

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

	// Always reset QCM hints state at PREPARE (new question or same question re-PREPARED)
	e.state.QcmInvalidated = []string{}

	// Reset Memory game state ONLY for NEW question
	// This allows team selection to persist during PREPARE→READY transition
	if isNewQuestion {
		// Use empty slices/maps instead of nil so they are serialized in JSON (not omitted)
		e.state.MemoryFlippedCards = []string{}
		e.state.MemoryMatchedPairs = []int{}
		e.state.MemoryErrors = 0
		e.state.MemoryCurrentTeam = ""
		e.state.MemoryCurrentTeamColor = []int{}
		e.state.MemoryTeamPairs = map[string]int{}
		e.state.MemoryTeamErrors = map[string]int{}
		e.state.MemoryParticipatingTeams = []string{}
		e.state.MemoryPairOwners = map[int]string{}

		// Reset MEMOTION state for new question (same pattern as Memory)
		e.state.MotionSubPhase = ""
		e.state.MotionSelected = ""
		e.state.MotionCardStates = make(map[string]string)
		e.state.MotionCardTeams = make(map[string]string)
		e.state.MotionCurrentTeam = ""
		e.state.MotionCurrentTeamColor = []int{}
		e.state.MotionParticipatingTeams = []string{}

		// For MEMOTION questions, also pre-populate card states so the frontend
		// can display UNPLAYED cards during the PREPARE phase (before START).
		if question != nil && question.Type == QuestionTypeMemotion {
			e.initMotionStateUnsafe()
		}
	}

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

// getActiveTeams returns teams that have at least one buzzer assigned.
// Must be called with e.mu held (read or write).
func (e *Engine) getActiveTeams() map[string]*Team {
	hasBumper := make(map[string]bool)
	for _, bumper := range e.data.Bumpers {
		if bumper.Team != "" {
			hasBumper[bumper.Team] = true
		}
	}
	active := make(map[string]*Team, len(hasBumper))
	for id, team := range e.data.Teams {
		if hasBumper[id] {
			active[id] = team
		}
	}
	return active
}

func (e *Engine) updateTeamsReady() {
	teamReadyCount := make(map[string]int)
	teamBumperCount := make(map[string]int)
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
		team.Ready = total > 0 && total == teamReadyCount[teamID]
	}
}

// AreAllTeamsReady returns true when every active team (≥1 buzzer assigned) is ready.
// Empty teams are ignored, matching the frontend display filter.
func (e *Engine) AreAllTeamsReady() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	active := e.getActiveTeams()
	if len(active) == 0 {
		return false
	}
	for _, team := range active {
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

	// MEMOTION: no countdown — grid is displayed immediately after START
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemotion {
		countdownDuration = 0
		log.Printf("[Engine] MEMOTION: no countdown, starting immediately")
	} else if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemory {
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

	// Auto-initialize Memory participating teams if not already set
	// This handles SOLO mode where admin doesn't select teams explicitly
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemory &&
		(e.state.MemoryParticipatingTeams == nil || len(e.state.MemoryParticipatingTeams) == 0) {
		// Get active teams (with buzzers) for auto-initialization
		allTeams := make([]string, 0, len(e.data.Teams))
		for teamName := range e.getActiveTeams() {
			allTeams = append(allTeams, teamName)
		}
		// Initialize with all teams
		e.state.MemoryParticipatingTeams = allTeams
		if len(allTeams) > 0 {
			e.state.MemoryCurrentTeam = allTeams[0]
			if team, exists := e.data.Teams[allTeams[0]]; exists {
				e.state.MemoryCurrentTeamColor = team.Color
			}
		}
		e.state.MemoryTeamPairs = make(map[string]int)
		e.state.MemoryTeamErrors = make(map[string]int)
		e.state.MemoryPairOwners = make(map[int]string)
		for _, teamName := range allTeams {
			e.state.MemoryTeamPairs[teamName] = 0
			e.state.MemoryTeamErrors[teamName] = 0
		}
		log.Printf("[Engine] Auto-initialized Memory participating teams: %v", allTeams)
	}

	// Initialize MEMOTION card states for fresh start
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemotion {
		e.initMotionStateUnsafe()
		log.Printf("[Engine] MEMOTION: grid initialized with %d cards", len(e.state.MotionCardStates))
	}

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

	// Start main timer — MEMOTION uses per-card timers (StartMotionCardTimer), not a global one
	if e.state.Question == nil || e.state.Question.Type != QuestionTypeMemotion {
		e.startTimer()
	}

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

	// Initialize MEMOTION card states (mirrors actualStart — StartImmediate bypasses it)
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemotion {
		e.initMotionStateUnsafe()
	}

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

	// Ignore buzz for MEMOTION questions - admin controls the card selection
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeMemotion {
		log.Printf("[Engine] Ignoring buzz for MEMOTION question from %s", bumperID)
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

	// Phase 3: QCM VPlayer invalidation logic
	// For QCM questions only, if the team has a VPlayer and this is a physical buzzer,
	// ignore the buzz (VPlayer replaces physical buzzers for QCM)
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeQCM {
		if !bumper.IsVPlayer && e.teamHasVPlayerUnsafe(teamID) {
			log.Printf("[Engine] Buzz ignored: team %s has VPlayer, physical bumper %s cannot buzz on QCM", teamID, bumperID)
			e.mu.Unlock()
			return
		}
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

	// For QCM questions, map button to AnswerColor (A=RED, B=GREEN, C=YELLOW, D=BLUE)
	if e.state.Question != nil && e.state.Question.Type == QuestionTypeQCM {
		buttonToColor := map[string]AnswerColor{
			"A": AnswerColorRed,
			"B": AnswerColorGreen,
			"C": AnswerColorYellow,
			"D": AnswerColorBlue,
		}
		if color, ok := buttonToColor[button]; ok {
			bumper.AnswerColor = color
		}
	}

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

// teamHasVPlayerUnsafe checks if a team has at least one VPlayer (virtual player)
// Caller must hold lock
func (e *Engine) teamHasVPlayerUnsafe(teamID string) bool {
	for _, bumper := range e.data.Bumpers {
		if bumper.Team == teamID && bumper.IsVPlayer {
			return true
		}
	}
	return false
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

// InitGame resets the game completely: scores, history, question statuses, and sets phase to NEW_GAME.
// This is the implementation of issue #66 — Bouton Start GAME.
func (e *Engine) InitGame() {
	e.mu.Lock()

	// Reset all bumper scores and times
	for _, bumper := range e.data.Bumpers {
		bumper.Score = 0
		bumper.Time = 0
	}

	// Reset all team scores
	for _, team := range e.data.Teams {
		team.Score = 0
		team.TeamPoints = 0
		team.Time = 0
	}

	// Clear history
	e.history = nil

	// Reset all question statuses
	e.questionStatuses = make(map[string]QuestionStatus)

	// Clear current question so admin UI doesn't show a stale selection
	e.state.Question = nil

	// Set phase to NEW_GAME
	e.state.Phase = PhaseNewGame

	// Reset MEMOTION state completely
	e.state.MotionSubPhase = ""
	e.state.MotionSelected = ""
	e.state.MotionCardStates = make(map[string]string)
	e.state.MotionCardTeams = make(map[string]string)
	e.state.MotionCurrentTeam = ""
	e.state.MotionCurrentTeamColor = []int{}
	e.state.MotionParticipatingTeams = []string{}

	log.Printf("[Engine] Game initialized: scores, history, question statuses reset")
	e.mu.Unlock()

	// Persist async
	go e.SaveHistory()
	go e.SaveTeams()
	go e.SaveBumpers()
	go e.SaveStatuses()
}

// SetQuizMeta sets the quiz metadata (name, theme, notes) on the game state.
// This is the implementation of issue #67 — Change QUESTIONS en QUIZ.
func (e *Engine) SetQuizMeta(name, theme, notes string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.QuizName = name
	e.state.QuizTheme = theme
	e.state.QuizNotes = notes
	log.Printf("[Engine] Quiz meta set: name=%q, theme=%q", name, theme)
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

// SetNewGameBackgrounds sets all NEW_GAME screen backgrounds (v4.0.4)
func (e *Engine) SetNewGameBackgrounds(backgrounds []Background) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.NewGameBackgrounds = backgrounds
}

// GetNewGameBackgrounds returns all NEW_GAME screen backgrounds (v4.0.4)
func (e *Engine) GetNewGameBackgrounds() []Background {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.NewGameBackgrounds
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

	bumpers := e.data.Bumpers
	if bumpers == nil {
		bumpers = make(map[string]*Bumper)
	}
	teams := e.data.Teams
	if teams == nil {
		teams = make(map[string]*Team)
	}

	data := &GameData{
		Game:    &e.state,
		Teams:   teams,
		Bumpers: bumpers,
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

		// Award pair to current team in multi-team mode
		if e.state.MemoryCurrentTeam != "" && e.state.MemoryTeamPairs != nil {
			e.state.MemoryTeamPairs[e.state.MemoryCurrentTeam]++
			log.Printf("[Engine] Pair awarded to team %s (total: %d pairs)", e.state.MemoryCurrentTeam, e.state.MemoryTeamPairs[e.state.MemoryCurrentTeam])
		}

		// Track ownership of this pair
		if e.state.MemoryCurrentTeam != "" {
			if e.state.MemoryPairOwners == nil {
				e.state.MemoryPairOwners = make(map[int]string)
			}
			e.state.MemoryPairOwners[firstPairID] = e.state.MemoryCurrentTeam
			log.Printf("[Engine] Pair %d ownership: team %s", firstPairID, e.state.MemoryCurrentTeam)
		}

		// Check if all pairs are matched (game complete)
		totalPairs := 0
		if e.state.Question != nil && e.state.Question.MemoryPairs != nil {
			totalPairs = len(e.state.Question.MemoryPairs)
		}
		isComplete = len(e.state.MemoryMatchedPairs) >= totalPairs && totalPairs > 0

		// Handle team rotation based on memory mode
		memoryMode := ""
		if e.state.Question != nil {
			memoryMode = e.state.Question.MemoryMode
			if memoryMode == "" {
				memoryMode = string(MemoryModeSolo)
			}
		}

		// Rotate team after match (mode: CHACUN_SON_TOUR)
		if memoryMode == string(MemoryModeChacunSonTour) {
			e.rotateToNextTeam()
		}
		// Mode TANT_QUE_JE_GAGNE: team keeps playing, no rotation

		log.Printf("[Engine] Memory MATCH! Pair %d found. Total matched: %d/%d. Complete: %v", firstPairID, len(e.state.MemoryMatchedPairs), totalPairs, isComplete)
		return true, false, 0, isComplete
	}

	// No match - increment error counter, caller should schedule flip-back after delay
	e.state.MemoryErrors++

	// Track error for current team in multi-team mode
	if e.state.MemoryCurrentTeam != "" && e.state.MemoryTeamErrors != nil {
		e.state.MemoryTeamErrors[e.state.MemoryCurrentTeam]++
	}

	// NOTE: Team rotation will happen in ClearMemoryFlippedCards() AFTER cards are hidden
	// This ensures players see which team made the error before the turn changes

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

// ClearMemoryFlippedCards resets all flipped cards and rotates team if needed
// This is called after the flip-back delay when cards are hidden
func (e *Engine) ClearMemoryFlippedCards() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear the flipped cards
	wasFlipped := len(e.state.MemoryFlippedCards) > 0
	e.state.MemoryFlippedCards = nil

	// Rotate team AFTER cards are hidden (only if we had 2 cards that didn't match)
	// This ensures the team rotation happens when cards are face-down, not while visible
	if wasFlipped && len(e.state.MemoryParticipatingTeams) > 0 {
		memoryMode := ""
		if e.state.Question != nil {
			memoryMode = e.state.Question.MemoryMode
			if memoryMode == "" {
				memoryMode = string(MemoryModeSolo)
			}
		}

		// Rotate team after error (both CHACUN_SON_TOUR and TANT_QUE_JE_GAGNE)
		// Only rotate in multi-team modes
		if memoryMode == string(MemoryModeChacunSonTour) || memoryMode == string(MemoryModeTantQueJeGagne) {
			e.rotateToNextTeam()
		}
	}

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

	// Create virtual bumper (VPlayer = true for virtual players, allows all QCM colors)
	bumper := &Bumper{
		Name:      name,
		Team:      "",
		Score:     0,
		IsVirtual: true,
		IsVPlayer: true, // VPlayer can answer all QCM colors
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

	// Create virtual bumper (VPlayer = true for virtual players, allows all QCM colors)
	bumper := &Bumper{
		Name:      name,
		Team:      "",
		Score:     0,
		IsVirtual: true,
		IsVPlayer: true, // VPlayer can answer all QCM colors
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

	// Assign team and answer color (only physical buzzers get color, VPlayers respond to all)
	bumper.Team = team
	if !bumper.IsVPlayer {
		bumper.AnswerColor = answerColor
	}
	// VPlayers keep AnswerColor empty (they respond to all colors)

	log.Printf("[Engine] Virtual player assigned: id=%s, team=%s, color=%s, isVPlayer=%v", bumperID, team, answerColor, bumper.IsVPlayer)

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

// SetMemoryParticipatingTeams sets the teams participating in a Memory game
func (e *Engine) SetMemoryParticipatingTeams(teams []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Allow setting teams during PREPARE or READY phases (before game starts)
	if e.state.Phase != PhasePrepare && e.state.Phase != PhaseReady {
		return &MemoryError{Reason: "NOT_IN_PREPARE_OR_READY_PHASE"}
	}

	if e.state.Question == nil || e.state.Question.Type != QuestionTypeMemory {
		return &MemoryError{Reason: "NOT_MEMORY_QUESTION"}
	}

	// Get memory mode (default to SOLO if empty)
	memoryMode := e.state.Question.MemoryMode
	if memoryMode == "" {
		memoryMode = string(MemoryModeSolo)
	}

	// Note: We don't validate team count here - allow incremental selection
	// The validation for minimum 2 teams happens at game START, not during selection

	// Validate all teams exist
	for _, teamName := range teams {
		if _, exists := e.data.Teams[teamName]; !exists {
			return &MemoryError{Reason: "TEAM_NOT_FOUND"}
		}
	}

	// Initialize Memory multi-team state
	e.state.MemoryParticipatingTeams = teams
	if len(teams) > 0 {
		e.state.MemoryCurrentTeam = teams[0]
		// Set current team color
		if team, exists := e.data.Teams[teams[0]]; exists {
			e.state.MemoryCurrentTeamColor = team.Color
		}
	}
	e.state.MemoryTeamPairs = make(map[string]int)
	e.state.MemoryTeamErrors = make(map[string]int)
	e.state.MemoryPairOwners = make(map[int]string)
	for _, teamName := range teams {
		e.state.MemoryTeamPairs[teamName] = 0
		e.state.MemoryTeamErrors[teamName] = 0
	}

	log.Printf("[Engine] Memory participating teams set: %v, current team: %s, mode: %s", teams, e.state.MemoryCurrentTeam, memoryMode)
	return nil
}

// rotateToNextTeam rotates to the next team in multi-team Memory mode
// Must be called with lock held
func (e *Engine) rotateToNextTeam() {
	if len(e.state.MemoryParticipatingTeams) == 0 {
		return
	}

	// Find current team index
	currentIndex := -1
	for i, team := range e.state.MemoryParticipatingTeams {
		if team == e.state.MemoryCurrentTeam {
			currentIndex = i
			break
		}
	}

	// Calculate next index (circular rotation)
	nextIndex := (currentIndex + 1) % len(e.state.MemoryParticipatingTeams)
	e.state.MemoryCurrentTeam = e.state.MemoryParticipatingTeams[nextIndex]

	// Update current team color
	if team, exists := e.data.Teams[e.state.MemoryCurrentTeam]; exists {
		e.state.MemoryCurrentTeamColor = team.Color
	}

	log.Printf("[Engine] Memory team rotated: %s → %s (index %d)",
		e.state.MemoryParticipatingTeams[currentIndex],
		e.state.MemoryCurrentTeam,
		nextIndex)
}

// MemoryError represents a Memory-specific error
type MemoryError struct {
	Reason string
}

func (e *MemoryError) Error() string {
	return e.Reason
}

// ============================================================
// MEMOTION — v5.0.0
// ============================================================

// motionDifficultyPoints maps card difficulty (1|2|3) to points awarded.
var motionDifficultyPoints = map[int]int{
	1: 1,
	2: 3,
	3: 5,
}

// initMotionStateUnsafe initialises MEMOTION card states for the current question.
// All cards are set to "UNPLAYED" and MotionSubPhase is reset to "GRID".
// Must be called with e.mu held (write).
func (e *Engine) initMotionStateUnsafe() {
	e.state.MotionSubPhase = "GRID"
	e.state.MotionSelected = ""
	e.state.MotionCardStates = make(map[string]string)
	if e.state.Question != nil {
		for _, card := range e.state.Question.MotionCards {
			e.state.MotionCardStates[card.ID] = "UNPLAYED"
		}
	}
}

// InitMotionState initialises MEMOTION card states (exported for direct use in tests).
func (e *Engine) InitMotionState() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.initMotionStateUnsafe()
}

// SelectMotionCard transitions a card from UNPLAYED to SELECTED and sets the sub-phase to SELECTED.
// The card is displayed fullscreen (RECTO face) on the TV, no timer starts here.
// Preconditions: Phase==STARTED, MotionSubPhase=="GRID", card must be UNPLAYED.
func (e *Engine) SelectMotionCard(cardID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStarted {
		return &MotionError{Reason: "NOT_STARTED"}
	}
	if e.state.MotionSubPhase != "GRID" {
		return &MotionError{Reason: "NOT_IN_GRID_SUBPHASE"}
	}
	st, exists := e.state.MotionCardStates[cardID]
	if !exists {
		return &MotionError{Reason: "CARD_NOT_FOUND"}
	}
	if st != "UNPLAYED" {
		return &MotionError{Reason: "CARD_NOT_UNPLAYED"}
	}

	e.state.MotionCardStates[cardID] = "SELECTED"
	e.state.MotionSelected = cardID
	e.state.MotionSubPhase = "SELECTED"

	log.Printf("[Engine] MEMOTION SelectMotionCard: cardID=%s → SELECTED", cardID)
	return nil
}

// FlipMotionCard transitions the selected card from SELECTED to QUESTION.
// Preconditions: Phase==STARTED, MotionSubPhase=="SELECTED".
// The caller (main.go) is responsible for starting the per-card timer after this call.
func (e *Engine) FlipMotionCard() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStarted {
		return &MotionError{Reason: "NOT_STARTED"}
	}
	if e.state.MotionSubPhase != "SELECTED" {
		return &MotionError{Reason: "NOT_IN_SELECTED_SUBPHASE"}
	}

	cardID := e.state.MotionSelected
	if cardID == "" {
		return &MotionError{Reason: "NO_CARD_SELECTED"}
	}
	if e.state.MotionCardStates[cardID] != "SELECTED" {
		return &MotionError{Reason: "CARD_NOT_IN_SELECTED_STATE"}
	}

	e.state.MotionCardStates[cardID] = "QUESTION"
	e.state.MotionSubPhase = "QUESTION"

	log.Printf("[Engine] MEMOTION FlipMotionCard: cardID=%s → QUESTION", cardID)
	return nil
}

// RevealMotionCard transitions the current card from QUESTION to REVEAL.
// Preconditions: Phase==STARTED, MotionSubPhase=="QUESTION".
func (e *Engine) RevealMotionCard() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStarted {
		return &MotionError{Reason: "NOT_STARTED"}
	}
	if e.state.MotionSubPhase != "QUESTION" {
		return &MotionError{Reason: "NOT_IN_QUESTION_SUBPHASE"}
	}

	cardID := e.state.MotionSelected
	if cardID != "" {
		e.state.MotionCardStates[cardID] = "REVEALED"
	}
	e.state.MotionSubPhase = "REVEAL"

	log.Printf("[Engine] MEMOTION RevealMotionCard: cardID=%s → REVEAL", cardID)
	return nil
}

// DoneMotionCard marks the active card as DONE, optionally awards points to winnerTeam,
// rotates the team if needed, and returns to the GRID sub-phase.
// Returns (pointsAwarded, isComplete, error).
func (e *Engine) DoneMotionCard(cardID string, winnerTeam string) (int, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStarted {
		return 0, false, &MotionError{Reason: "NOT_STARTED"}
	}
	if e.state.MotionSubPhase != "QUESTION" && e.state.MotionSubPhase != "REVEAL" && e.state.MotionSubPhase != "SELECTED" {
		return 0, false, &MotionError{Reason: "INVALID_SUBPHASE"}
	}

	// Cancellation from SELECTED subphase: reset card to UNPLAYED, return to GRID.
	// Use e.state.MotionSelected (server-authoritative) rather than client-supplied cardID.
	if e.state.MotionSubPhase == "SELECTED" {
		e.state.MotionCardStates[e.state.MotionSelected] = "UNPLAYED"
		e.state.MotionSelected = ""
		e.state.MotionSubPhase = "GRID"
		log.Printf("[Engine] MEMOTION DoneMotionCard: SELECTED → cancelled, cardID=%s back to UNPLAYED", cardID)
		return 0, false, nil
	}

	// Validate card exists
	if _, exists := e.state.MotionCardStates[cardID]; !exists {
		return 0, false, &MotionError{Reason: "CARD_NOT_FOUND"}
	}

	// Mark card as DONE
	e.state.MotionCardStates[cardID] = "DONE"

	// Award points and record winner if winnerTeam is provided
	points := 0
	if winnerTeam != "" {
		// Find card difficulty to calculate points
		if e.state.Question != nil {
			for _, card := range e.state.Question.MotionCards {
				if card.ID == cardID {
					pts, ok := motionDifficultyPoints[card.Difficulty]
					if ok {
						points = pts
					}
					break
				}
			}
		}
		if points > 0 {
			// UpdateTeamScore requires mu.Lock — we hold it here, so use unsafe version
			team, ok := e.data.Teams[winnerTeam]
			if ok {
				team.TeamPoints += points
				e.recalculateTeamScoreUnsafe(winnerTeam)
				log.Printf("[Engine] MEMOTION DoneMotionCard: team=%s +%dpts (difficulty-based)", winnerTeam, points)
			}
		}
		e.state.MotionCardTeams[cardID] = winnerTeam
	}

	// Team rotation based on question mode
	motionMode := ""
	if e.state.Question != nil {
		motionMode = e.state.Question.MotionMode
		if motionMode == "" {
			motionMode = string(MemoryModeSolo)
		}
	}

	switch motionMode {
	case string(MemoryModeChacunSonTour):
		// Rotate after every card regardless of outcome
		e.rotateMotionTeam()
	case string(MemoryModeTantQueJeGagne):
		// Rotate if no winner; keep if winner
		if winnerTeam == "" {
			e.rotateMotionTeam()
		}
	// MemoryModeSolo: no rotation
	}

	// Return to grid
	e.state.MotionSubPhase = "GRID"
	e.state.MotionSelected = ""

	// Check if all cards are DONE
	isComplete := true
	for _, st := range e.state.MotionCardStates {
		if st != "DONE" {
			isComplete = false
			break
		}
	}
	if len(e.state.MotionCardStates) == 0 {
		isComplete = false
	}

	log.Printf("[Engine] MEMOTION DoneMotionCard: cardID=%s winner=%s pts=%d complete=%v",
		cardID, winnerTeam, points, isComplete)
	return points, isComplete, nil
}

// SetMotionParticipatingTeams sets the teams participating in a MEMOTION game.
// Mirrors SetMemoryParticipatingTeams — allowed during PREPARE or READY phases.
func (e *Engine) SetMotionParticipatingTeams(teams []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhasePrepare && e.state.Phase != PhaseReady {
		return &MotionError{Reason: "NOT_IN_PREPARE_OR_READY_PHASE"}
	}
	if e.state.Question == nil || e.state.Question.Type != QuestionTypeMemotion {
		return &MotionError{Reason: "NOT_MEMOTION_QUESTION"}
	}

	// Validate all teams exist
	for _, teamName := range teams {
		if _, exists := e.data.Teams[teamName]; !exists {
			return &MotionError{Reason: "TEAM_NOT_FOUND"}
		}
	}

	e.state.MotionParticipatingTeams = teams
	if len(teams) > 0 {
		e.state.MotionCurrentTeam = teams[0]
		if team, exists := e.data.Teams[teams[0]]; exists {
			e.state.MotionCurrentTeamColor = team.Color
		}
	} else {
		e.state.MotionCurrentTeam = ""
		e.state.MotionCurrentTeamColor = []int{}
	}

	log.Printf("[Engine] MEMOTION SetMotionParticipatingTeams: teams=%v currentTeam=%s",
		teams, e.state.MotionCurrentTeam)
	return nil
}

// rotateMotionTeam advances MotionCurrentTeam to the next team in MotionParticipatingTeams.
// Must be called with e.mu held (write).
func (e *Engine) rotateMotionTeam() {
	if len(e.state.MotionParticipatingTeams) == 0 {
		return
	}

	// Find current team index
	currentIndex := -1
	for i, team := range e.state.MotionParticipatingTeams {
		if team == e.state.MotionCurrentTeam {
			currentIndex = i
			break
		}
	}

	// Circular rotation
	nextIndex := (currentIndex + 1) % len(e.state.MotionParticipatingTeams)
	prev := e.state.MotionCurrentTeam
	e.state.MotionCurrentTeam = e.state.MotionParticipatingTeams[nextIndex]

	// Update current team color
	if team, exists := e.data.Teams[e.state.MotionCurrentTeam]; exists {
		e.state.MotionCurrentTeamColor = team.Color
	}

	log.Printf("[Engine] MEMOTION team rotated: %s → %s (index %d)", prev, e.state.MotionCurrentTeam, nextIndex)
}

// StartMotionCardTimer starts a per-card countdown for MEMOTION.
// Unlike startTimer(), expiry does NOT stop the game — the admin can still act.
// If delay <= 0, no timer is started.
func (e *Engine) StartMotionCardTimer(delay int) {
	e.mu.Lock()

	if delay <= 0 {
		log.Printf("[Engine] MEMOTION StartMotionCardTimer: delay<=0, no timer started")
		e.mu.Unlock()
		return
	}

	// Stop any existing timer goroutine first (reuse same infrastructure as startTimer)
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

	// Set timer values
	e.state.CurrentTime = delay
	e.state.Delay = delay

	// Create new stop channel and ticker
	e.stopCh = make(chan struct{})
	e.timer = time.NewTicker(1 * time.Second)

	ticker := e.timer
	stopCh := e.stopCh

	log.Printf("[Engine] MEMOTION StartMotionCardTimer: delay=%ds", delay)
	e.mu.Unlock()

	go func() {
		for {
			select {
			case <-ticker.C:
				e.mu.Lock()
				// Guard: exit if game state changed unexpectedly
				if e.state.Phase != PhaseStarted || e.state.MotionSubPhase != "QUESTION" {
					e.mu.Unlock()
					ticker.Stop()
					return
				}
				e.state.CurrentTime--
				currentTime := e.state.CurrentTime
				e.mu.Unlock()

				if e.OnTimerTick != nil {
					e.OnTimerTick(currentTime)
				}

				if currentTime <= 0 {
					// Timer expired — do NOT call e.Stop(). Phase stays STARTED.
					ticker.Stop()
					log.Printf("[Engine] MEMOTION card timer expired — phase stays STARTED, admin must act")
					return
				}
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

// StopMotionCardTimer stops the per-card timer without changing the game phase.
// Resets CurrentTime to 0. Safe to call even if no timer is running.
func (e *Engine) StopMotionCardTimer() {
	e.mu.Lock()
	defer e.mu.Unlock()

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

	// Reset timer display without changing game phase
	e.state.CurrentTime = 0

	log.Printf("[Engine] MEMOTION StopMotionCardTimer: timer stopped, CURRENT_TIME=0")
}

// MotionError represents a MEMOTION-specific error
type MotionError struct {
	Reason string
}

func (e *MotionError) Error() string {
	return e.Reason
}
