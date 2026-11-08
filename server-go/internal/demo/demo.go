// Package demo builds the development test data and the showcase demo data
// (teams, players, questions, backgrounds, history). It reaches the application
// through the narrow Host interface. Extracted from cmd/server (#291, Phase B).
package demo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"buzzcontrol/assets"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

// Host is what the demo data needs from the application.
type Host interface {
	Engine() *game.Engine
	Config() *config.Config
	ResolveCategoryMeta(key string) (name, imageURL, color string)
	SaveBackgroundsConfig()
}

// InitTestData creates test teams and bumpers for development
func InitTestData(h Host) {
	server.LogInfo(game.LogComponentApp, "Initializing test data...")

	// 6 teams with different colors (scores will be calculated from bumpers)
	teams := map[string]*game.Team{
		"Les Rouges": {
			Name:  "Les Rouges",
			Color: []int{239, 68, 68}, // Red
			Score: 0,
		},
		"Les Bleus": {
			Name:  "Les Bleus",
			Color: []int{59, 130, 246}, // Blue
			Score: 0,
		},
		"Les Verts": {
			Name:  "Les Verts",
			Color: []int{34, 197, 94}, // Green
			Score: 0,
		},
		"Les Jaunes": {
			Name:  "Les Jaunes",
			Color: []int{234, 179, 8}, // Yellow
			Score: 0,
		},
		"Les Violets": {
			Name:  "Les Violets",
			Color: []int{168, 85, 247}, // Purple
			Score: 0,
		},
		"Les Oranges": {
			Name:  "Les Oranges",
			Color: []int{249, 115, 22}, // Orange
			Score: 0,
		},
	}

	// Fake bumpers (2-3 per team) with answer colors for QCM mode
	bumpers := map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {
			Name:        "Alice",
			Team:        "Les Rouges",
			Score:       8,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorRed,
		},
		"AA:BB:CC:DD:EE:02": {
			Name:        "Bob",
			Team:        "Les Rouges",
			Score:       7,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorGreen,
		},
		"AA:BB:CC:DD:EE:03": {
			Name:        "Charlie",
			Team:        "Les Bleus",
			Score:       6,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorYellow,
		},
		"AA:BB:CC:DD:EE:04": {
			Name:        "Diana",
			Team:        "Les Bleus",
			Score:       6,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorBlue,
		},
		"AA:BB:CC:DD:EE:05": {
			Name:        "Ethan",
			Team:        "Les Verts",
			Score:       5,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorRed,
		},
		"AA:BB:CC:DD:EE:06": {
			Name:        "Fiona",
			Team:        "Les Verts",
			Score:       3,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorGreen,
		},
		"AA:BB:CC:DD:EE:07": {
			Name:        "George",
			Team:        "Les Jaunes",
			Score:       7,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorYellow,
		},
		"AA:BB:CC:DD:EE:08": {
			Name:        "Hannah",
			Team:        "Les Jaunes",
			Score:       3,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorBlue,
		},
		"AA:BB:CC:DD:EE:09": {
			Name:    "Ivan",
			Team:    "Les Violets",
			Score:   3,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:10": {
			Name:    "Julia",
			Team:    "Les Violets",
			Score:   2,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:11": {
			Name:        "Kevin",
			Team:        "Les Oranges",
			Score:       4,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorYellow, // C
		},
		"AA:BB:CC:DD:EE:12": {
			Name:        "Laura",
			Team:        "Les Oranges",
			Score:       5,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorRed, // A
		},
	}

	h.Engine().SetTeams(teams)
	h.Engine().SetBumpers(bumpers)
	h.Engine().RecalculateAllTeamScores()

	// Create test questions with different categories
	createTestQuestions(h)

	server.LogInfo(game.LogComponentApp, "Test data initialized: %d teams, %d bumpers", len(teams), len(bumpers))
}

// createTestQuestions creates test questions with various categories
func createTestQuestions(h Host) {
	questionsDir := h.Config().Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}

	// Ensure questions directory exists
	os.MkdirAll(questionsDir, 0755)

	// Check if questions already exist
	entries, _ := os.ReadDir(questionsDir)
	if len(entries) > 0 {
		server.LogInfo(game.LogComponentApp, "Questions already exist (%d), skipping test questions", len(entries))
		return
	}

	server.LogInfo(game.LogComponentApp, "Creating test questions with categories...")

	testQuestions := []map[string]interface{}{
		// GEOGRAPHY
		{
			"ID":            "1",
			"QUESTION":      "Quelle est la capitale de l'Australie?",
			"ANSWER":        "Canberra",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "SPEEDY",
			"CATEGORY":      "GEOGRAPHY",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         0,
		},
		// ENTERTAINMENT - QCM
		{
			"ID":       "2",
			"QUESTION": "Quel acteur joue Iron Man dans les films Marvel?",
			"ANSWER":   "Robert Downey Jr.",
			"POINTS":   "10",
			"TIME":     "20",
			"TYPE":     "QCM",
			"CATEGORY": "ENTERTAINMENT",
			"QCM_ANSWERS": map[string]string{
				"RED":    "Chris Evans",
				"GREEN":  "Robert Downey Jr.",
				"YELLOW": "Chris Hemsworth",
				"BLUE":   "Mark Ruffalo",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         1,
		},
		// HISTORY
		{
			"ID":            "3",
			"QUESTION":      "En quelle annee a eu lieu la Revolution francaise?",
			"ANSWER":        "1789",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "SPEEDY",
			"CATEGORY":      "HISTORY",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         2,
		},
		// ARTS - QCM
		{
			"ID":       "4",
			"QUESTION": "Qui a peint la Joconde?",
			"ANSWER":   "Leonard de Vinci",
			"POINTS":   "10",
			"TIME":     "20",
			"TYPE":     "QCM",
			"CATEGORY": "ARTS",
			"QCM_ANSWERS": map[string]string{
				"RED":    "Michel-Ange",
				"GREEN":  "Leonard de Vinci",
				"YELLOW": "Raphael",
				"BLUE":   "Botticelli",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         3,
		},
		// SCIENCE
		{
			"ID":            "5",
			"QUESTION":      "Quel est le symbole chimique de l'or?",
			"ANSWER":        "Au",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "SPEEDY",
			"CATEGORY":      "SCIENCE",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         4,
		},
		// SPORTS - QCM
		{
			"ID":       "6",
			"QUESTION": "Dans quel sport utilise-t-on un volant?",
			"ANSWER":   "Badminton",
			"POINTS":   "10",
			"TIME":     "20",
			"TYPE":     "QCM",
			"CATEGORY": "SPORTS",
			"QCM_ANSWERS": map[string]string{
				"RED":    "Tennis",
				"GREEN":  "Badminton",
				"YELLOW": "Squash",
				"BLUE":   "Ping-pong",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         5,
		},
		// FOOD
		{
			"ID":            "7",
			"QUESTION":      "De quel pays vient le sushi?",
			"ANSWER":        "Japon",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "SPEEDY",
			"CATEGORY":      "FOOD",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         6,
		},
		// ANIMALS - QCM
		{
			"ID":       "8",
			"QUESTION": "Quel est le plus grand animal terrestre?",
			"ANSWER":   "Elephant d'Afrique",
			"POINTS":   "10",
			"TIME":     "20",
			"TYPE":     "QCM",
			"CATEGORY": "ANIMALS",
			"QCM_ANSWERS": map[string]string{
				"RED":    "Girafe",
				"GREEN":  "Elephant d'Afrique",
				"YELLOW": "Rhinoceros",
				"BLUE":   "Hippopotame",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         7,
		},
	}

	for _, q := range testQuestions {
		id := q["ID"].(string)
		questionDir := filepath.Join(questionsDir, id)
		os.MkdirAll(questionDir, 0755)

		data, err := json.MarshalIndent(q, "", "  ")
		if err != nil {
			server.LogError(game.LogComponentApp, "Failed to marshal question %s: %v", id, err)
			continue
		}

		questionFile := filepath.Join(questionDir, "question.json")
		if err := os.WriteFile(questionFile, data, 0644); err != nil {
			server.LogError(game.LogComponentApp, "Failed to write question %s: %v", id, err)
			continue
		}
	}

	server.LogInfo(game.LogComponentApp, "Created %d test questions", len(testQuestions))
}

// LoadDemoData creates comprehensive demo data for showcasing all features
func LoadDemoData(h Host) {
	server.LogInfo(game.LogComponentApp, "Loading demo data...")

	// Clear existing data first
	h.Engine().ClearHistory()

	// 6 teams with distinct colors and pre-filled TeamPoints
	teams := map[string]*game.Team{
		"Les Rouges": {
			Name:       "Les Rouges",
			Color:      []int{239, 68, 68}, // Red
			Score:      0,
			TeamPoints: 30, // Pre-filled for podium display
		},
		"Les Bleus": {
			Name:       "Les Bleus",
			Color:      []int{59, 130, 246}, // Blue
			Score:      0,
			TeamPoints: 45,
		},
		"Les Verts": {
			Name:       "Les Verts",
			Color:      []int{34, 197, 94}, // Green
			Score:      0,
			TeamPoints: 25,
		},
		"Les Jaunes": {
			Name:       "Les Jaunes",
			Color:      []int{234, 179, 8}, // Yellow
			Score:      0,
			TeamPoints: 35,
		},
		"Les Violets": {
			Name:       "Les Violets",
			Color:      []int{168, 85, 247}, // Purple
			Score:      0,
			TeamPoints: 20,
		},
		"Les Oranges": {
			Name:       "Les Oranges",
			Color:      []int{249, 115, 22}, // Orange
			Score:      0,
			TeamPoints: 40,
		},
	}

	// Players: 4 per team with all 4 QCM answer colors (RED=A, GREEN=B, YELLOW=C, BLUE=D)
	bumpers := map[string]*game.Bumper{
		// Les Rouges - 4 joueurs
		"DEMO:AA:BB:CC:01": {Name: "Alice", Team: "Les Rouges", Score: 12, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:02": {Name: "Antoine", Team: "Les Rouges", Score: 8, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:03": {Name: "Amelie", Team: "Les Rouges", Score: 6, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:04": {Name: "Arthur", Team: "Les Rouges", Score: 4, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Bleus - 4 joueurs
		"DEMO:AA:BB:CC:05": {Name: "Bruno", Team: "Les Bleus", Score: 15, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:06": {Name: "Brigitte", Team: "Les Bleus", Score: 10, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:07": {Name: "Baptiste", Team: "Les Bleus", Score: 7, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:08": {Name: "Berenice", Team: "Les Bleus", Score: 5, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Verts - 4 joueurs
		"DEMO:AA:BB:CC:09": {Name: "Clara", Team: "Les Verts", Score: 9, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:10": {Name: "Cedric", Team: "Les Verts", Score: 7, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:11": {Name: "Camille", Team: "Les Verts", Score: 5, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:12": {Name: "Cyril", Team: "Les Verts", Score: 3, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Jaunes - 4 joueurs
		"DEMO:AA:BB:CC:13": {Name: "David", Team: "Les Jaunes", Score: 11, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:14": {Name: "Delphine", Team: "Les Jaunes", Score: 9, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:15": {Name: "Dylan", Team: "Les Jaunes", Score: 6, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:16": {Name: "Diane", Team: "Les Jaunes", Score: 4, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Violets - 4 joueurs
		"DEMO:AA:BB:CC:17": {Name: "Emma", Team: "Les Violets", Score: 8, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:18": {Name: "Ethan", Team: "Les Violets", Score: 6, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:19": {Name: "Eva", Team: "Les Violets", Score: 4, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:20": {Name: "Eliot", Team: "Les Violets", Score: 2, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Oranges - 4 joueurs
		"DEMO:AA:BB:CC:21": {Name: "Felix", Team: "Les Oranges", Score: 13, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:22": {Name: "Fanny", Team: "Les Oranges", Score: 10, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:23": {Name: "Florian", Team: "Les Oranges", Score: 7, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:24": {Name: "Flore", Team: "Les Oranges", Score: 5, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
	}

	h.Engine().SetTeams(teams)
	h.Engine().SetBumpers(bumpers)
	h.Engine().RecalculateAllTeamScores()

	// Create demo questions
	createDemoQuestions(h)

	// Create demo backgrounds with varied opacities
	createDemoBackgrounds(h)

	// Create demo history events for PALMARES
	createDemoHistory(h)

	server.LogInfo(game.LogComponentApp, "Demo data loaded: %d teams, %d players", len(teams), len(bumpers))
}

// createDemoQuestions creates diverse demo questions
func createDemoQuestions(h Host) {
	questionsDir := h.Config().Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}

	// Clear existing questions
	os.RemoveAll(questionsDir)
	os.MkdirAll(questionsDir, 0755)

	demoQuestions := []map[string]interface{}{
		// GEOGRAPHY - QCM with hints + image
		{
			"ID":                   "demo1",
			"QUESTION":             "Quelle est la capitale de l'Australie?",
			"ANSWER":               "Canberra",
			"POINTS":               "10",
			"TIME":                 "20",
			"TYPE":                 "QCM",
			"CATEGORY":             "GEOGRAPHY",
			"POINTS_TARGET":        "TEAM",
			"QCM_HINTS_ENABLED":    true,
			"QCM_HINT_THRESHOLD_1": 0.25,
			"QCM_HINT_THRESHOLD_2": 0.125,
			"QCM_PENALTY_1":        0.67,
			"QCM_PENALTY_2":        0.33,
			"QCM_ANSWERS": map[string]string{
				"RED":    "Sydney",
				"GREEN":  "Canberra",
				"YELLOW": "Melbourne",
				"BLUE":   "Brisbane",
			},
			"QCM_CORRECT": "GREEN",
			"ORDER":       0,
			"MEDIA":       "/question/demo1/media.jpg",
		},
		// ENTERTAINMENT - Normal question
		{
			"ID":            "demo2",
			"QUESTION":      "Quel acteur joue Iron Man dans les films Marvel?",
			"ANSWER":        "Robert Downey Jr.",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "SPEEDY",
			"CATEGORY":      "ENTERTAINMENT",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         1,
		},
		// HISTORY - QCM with hints
		{
			"ID":                "demo3",
			"QUESTION":          "En quelle annee a debute la Premiere Guerre mondiale?",
			"ANSWER":            "1914",
			"POINTS":            "15",
			"TIME":              "25",
			"TYPE":              "QCM",
			"CATEGORY":          "HISTORY",
			"POINTS_TARGET":     "TEAM",
			"QCM_HINTS_ENABLED": true,
			"QCM_ANSWERS": map[string]string{
				"RED":    "1912",
				"GREEN":  "1914",
				"YELLOW": "1916",
				"BLUE":   "1918",
			},
			"QCM_CORRECT": "GREEN",
			"ORDER":       2,
		},
		// SCIENCE - Normal question + images
		{
			"ID":            "demo4",
			"QUESTION":      "Quel est le symbole chimique de l'or?",
			"ANSWER":        "Au",
			"POINTS":        "10",
			"TIME":          "20",
			"TYPE":          "SPEEDY",
			"CATEGORY":      "SCIENCE",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         3,
			"MEDIA":         "/question/demo4/media.jpg",
			"MEDIA_ANSWER":  "/question/demo4/media_answer.jpg",
		},
		// SPORTS - QCM
		{
			"ID":       "demo5",
			"QUESTION": "Combien de joueurs composent une equipe de football?",
			"ANSWER":   "11",
			"POINTS":   "10",
			"TIME":     "15",
			"TYPE":     "QCM",
			"CATEGORY": "SPORTS",
			"QCM_ANSWERS": map[string]string{
				"RED":    "9",
				"GREEN":  "11",
				"YELLOW": "13",
				"BLUE":   "15",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         4,
		},
		// ARTS - Normal question
		{
			"ID":            "demo6",
			"QUESTION":      "Qui a peint la Joconde?",
			"ANSWER":        "Leonard de Vinci",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "SPEEDY",
			"CATEGORY":      "ARTS",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         5,
		},
		// FOOD - QCM with hints + images
		{
			"ID":                "demo7",
			"QUESTION":          "De quel pays vient la pizza?",
			"ANSWER":            "Italie",
			"POINTS":            "10",
			"TIME":              "20",
			"TYPE":              "QCM",
			"CATEGORY":          "FOOD",
			"POINTS_TARGET":     "TEAM",
			"QCM_HINTS_ENABLED": true,
			"QCM_ANSWERS": map[string]string{
				"RED":    "France",
				"GREEN":  "Italie",
				"YELLOW": "Espagne",
				"BLUE":   "Grece",
			},
			"QCM_CORRECT":  "GREEN",
			"ORDER":        6,
			"MEDIA":        "/question/demo7/media.jpg",
			"MEDIA_ANSWER": "/question/demo7/media_answer.jpg",
		},
		// ANIMALS - Normal
		{
			"ID":            "demo8",
			"QUESTION":      "Quel est le plus grand animal terrestre?",
			"ANSWER":        "L'elephant d'Afrique",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "SPEEDY",
			"CATEGORY":      "ANIMALS",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         7,
		},
		// GEOGRAPHY - MEMORY game
		{
			"ID":            "demo9",
			"QUESTION":      "Associez les pays a leurs capitales",
			"ANSWER":        "",
			"POINTS":        "0",
			"TIME":          "90",
			"TYPE":          "MEMORY",
			"CATEGORY":      "GEOGRAPHY",
			"POINTS_TARGET": "TEAM",
			"MEMORY_PAIRS": []map[string]interface{}{
				{
					"ID":    1,
					"CARD1": map[string]interface{}{"TEXT": "France", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Paris", "IS_IMAGE": false},
				},
				{
					"ID":    2,
					"CARD1": map[string]interface{}{"TEXT": "Espagne", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Madrid", "IS_IMAGE": false},
				},
				{
					"ID":    3,
					"CARD1": map[string]interface{}{"TEXT": "Allemagne", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Berlin", "IS_IMAGE": false},
				},
				{
					"ID":    4,
					"CARD1": map[string]interface{}{"TEXT": "Italie", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Rome", "IS_IMAGE": false},
				},
			},
			"MEMORY_CONFIG": map[string]interface{}{
				"FLIP_DELAY":           3.0,
				"POINTS_PER_PAIR":      10,
				"ERROR_PENALTY":        0,
				"COMPLETION_BONUS":     20,
				"USE_TIMER":            true,
				"MEMORIZE_TIME":        5,
				"SHOW_DURING_MEMORIZE": true,
				"REVEAL_DELAY":         0.5,
			},
			"ORDER": 8,
		},
		// ENTERTAINMENT - MEMORY game
		{
			"ID":            "demo10",
			"QUESTION":      "Associez les superheros a leurs pouvoirs",
			"ANSWER":        "",
			"POINTS":        "0",
			"TIME":          "120",
			"TYPE":          "MEMORY",
			"CATEGORY":      "ENTERTAINMENT",
			"POINTS_TARGET": "TEAM",
			"MEMORY_PAIRS": []map[string]interface{}{
				{
					"ID":    1,
					"CARD1": map[string]interface{}{"TEXT": "Superman", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Vol", "IS_IMAGE": false},
				},
				{
					"ID":    2,
					"CARD1": map[string]interface{}{"TEXT": "Spider-Man", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Toiles", "IS_IMAGE": false},
				},
				{
					"ID":    3,
					"CARD1": map[string]interface{}{"TEXT": "Flash", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Vitesse", "IS_IMAGE": false},
				},
				{
					"ID":    4,
					"CARD1": map[string]interface{}{"TEXT": "Aquaman", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Eau", "IS_IMAGE": false},
				},
				{
					"ID":    5,
					"CARD1": map[string]interface{}{"TEXT": "Hulk", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Force", "IS_IMAGE": false},
				},
			},
			"MEMORY_CONFIG": map[string]interface{}{
				"FLIP_DELAY":           2.0,
				"POINTS_PER_PAIR":      15,
				"ERROR_PENALTY":        5,
				"COMPLETION_BONUS":     30,
				"USE_TIMER":            true,
				"MEMORIZE_TIME":        7,
				"SHOW_DURING_MEMORIZE": true,
				"REVEAL_DELAY":         0.5,
			},
			"ORDER": 9,
		},
	}

	for _, q := range demoQuestions {
		id := q["ID"].(string)
		questionDir := filepath.Join(questionsDir, id)
		os.MkdirAll(questionDir, 0755)

		data, err := json.MarshalIndent(q, "", "  ")
		if err != nil {
			server.LogError(game.LogComponentApp, "Failed to marshal demo question %s: %v", id, err)
			continue
		}

		questionFile := filepath.Join(questionDir, "question.json")
		if err := os.WriteFile(questionFile, data, 0644); err != nil {
			server.LogError(game.LogComponentApp, "Failed to write demo question %s: %v", id, err)
			continue
		}
	}

	// Extract demo question images from embedded assets
	demoImages := []struct {
		questionID string
		assetName  string
		destName   string
	}{
		{"demo1", "demo1_australia.jpg", "media.jpg"},
		{"demo4", "demo4_gold_miner.jpg", "media.jpg"},
		{"demo4", "demo4_periodic_table.jpg", "media_answer.jpg"},
		{"demo7", "demo7_pizza.jpg", "media.jpg"},
		{"demo7", "demo7_italy.jpg", "media_answer.jpg"},
	}

	for _, img := range demoImages {
		questionDir := filepath.Join(questionsDir, img.questionID)
		destPath := filepath.Join(questionDir, img.destName)

		data, err := assets.DemoAssets.ReadFile("demo/" + img.assetName)
		if err != nil {
			server.LogError(game.LogComponentApp, "Failed to read embedded %s: %v", img.assetName, err)
			continue
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			server.LogError(game.LogComponentApp, "Failed to write %s: %v", destPath, err)
			continue
		}
		server.LogDebug(game.LogComponentApp, "Extracted demo image: %s -> %s", img.assetName, destPath)
	}

	server.LogInfo(game.LogComponentApp, "Created %d demo questions with images", len(demoQuestions))
}

// createDemoBackgrounds creates demo backgrounds with varied opacities
func createDemoBackgrounds(h Host) {
	filesDir := h.Config().Storage.FilesDir
	if filesDir == "" {
		filesDir = "./data/files"
	}
	bgDir := filepath.Join(filesDir, "backgrounds")
	os.MkdirAll(bgDir, 0755)

	// Demo backgrounds: extract from embedded assets
	demoImages := []struct {
		filename string
		duration int
		opacity  float64
	}{
		{"demo_bg_1.jpg", 8, 100},
		{"demo_bg_2.jpg", 12, 80},
		{"demo_bg_3.jpg", 10, 60},
	}

	var backgrounds []game.Background
	for _, img := range demoImages {
		destPath := filepath.Join(bgDir, img.filename)
		// Extract from embedded assets if not exists
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			server.LogDebug(game.LogComponentApp, "Extracting demo background: %s", img.filename)
			data, err := assets.DemoAssets.ReadFile("demo/" + img.filename)
			if err != nil {
				server.LogError(game.LogComponentApp, "Failed to read embedded %s: %v", img.filename, err)
				continue
			}
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				server.LogError(game.LogComponentApp, "Failed to write %s: %v", img.filename, err)
				continue
			}
		}
		backgrounds = append(backgrounds, game.Background{
			Path:     "/files/backgrounds/" + img.filename,
			Duration: img.duration,
			Opacity:  img.opacity,
		})
	}

	h.Engine().SetBackgrounds(backgrounds)
	h.SaveBackgroundsConfig()
	server.LogInfo(game.LogComponentApp, "Created %d demo backgrounds", len(backgrounds))
}

// createDemoHistory creates demo history events for PALMARES view
func createDemoHistory(h Host) {
	baseTime := time.Now().Add(-1 * time.Hour).UnixMicro()

	// Create events for different categories and teams
	events := []game.GameEvent{
		// GEOGRAPHY events
		{
			Timestamp:        baseTime,
			QuestionID:       "demo1",
			QuestionText:     "Quelle est la capitale de l'Australie?",
			QuestionCategory: "GEOGRAPHY",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Bleus",
			TeamColor:        []int{59, 130, 246},
			Points:           10,
		},
		{
			Timestamp:        baseTime + 60000000,
			QuestionID:       "demo9",
			QuestionText:     "Associez les pays a leurs capitales",
			QuestionCategory: "GEOGRAPHY",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Oranges",
			TeamColor:        []int{249, 115, 22},
			Points:           40,
		},
		// ENTERTAINMENT events
		{
			Timestamp:        baseTime + 120000000,
			QuestionID:       "demo2",
			QuestionText:     "Quel acteur joue Iron Man?",
			QuestionCategory: "ENTERTAINMENT",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "PLAYER",
			WinnerID:         "DEMO:AA:BB:CC:05",
			PlayerName:       "Bruno",
			TeamName:         "Les Bleus",
			TeamColor:        []int{59, 130, 246},
			Points:           10,
		},
		{
			Timestamp:        baseTime + 180000000,
			QuestionID:       "demo10",
			QuestionText:     "Associez les superheros a leurs pouvoirs",
			QuestionCategory: "ENTERTAINMENT",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Rouges",
			TeamColor:        []int{239, 68, 68},
			Points:           60,
		},
		// HISTORY events
		{
			Timestamp:        baseTime + 240000000,
			QuestionID:       "demo3",
			QuestionText:     "Debut de la Premiere Guerre mondiale?",
			QuestionCategory: "HISTORY",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Verts",
			TeamColor:        []int{34, 197, 94},
			Points:           15,
		},
		// SCIENCE events
		{
			Timestamp:        baseTime + 300000000,
			QuestionID:       "demo4",
			QuestionText:     "Symbole chimique de l'or?",
			QuestionCategory: "SCIENCE",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "PLAYER",
			WinnerID:         "DEMO:AA:BB:CC:13",
			PlayerName:       "David",
			TeamName:         "Les Jaunes",
			TeamColor:        []int{234, 179, 8},
			Points:           10,
		},
		// SPORTS events
		{
			Timestamp:        baseTime + 360000000,
			QuestionID:       "demo5",
			QuestionText:     "Joueurs dans une equipe de football?",
			QuestionCategory: "SPORTS",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Violets",
			TeamColor:        []int{168, 85, 247},
			Points:           10,
		},
		// ARTS events
		{
			Timestamp:        baseTime + 420000000,
			QuestionID:       "demo6",
			QuestionText:     "Qui a peint la Joconde?",
			QuestionCategory: "ARTS",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "PLAYER",
			WinnerID:         "DEMO:AA:BB:CC:21",
			PlayerName:       "Felix",
			TeamName:         "Les Oranges",
			TeamColor:        []int{249, 115, 22},
			Points:           10,
		},
		// FOOD events
		{
			Timestamp:        baseTime + 480000000,
			QuestionID:       "demo7",
			QuestionText:     "De quel pays vient la pizza?",
			QuestionCategory: "FOOD",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Jaunes",
			TeamColor:        []int{234, 179, 8},
			Points:           7,
		},
		// ANIMALS events
		{
			Timestamp:        baseTime + 540000000,
			QuestionID:       "demo8",
			QuestionText:     "Plus grand animal terrestre?",
			QuestionCategory: "ANIMALS",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "PLAYER",
			WinnerID:         "DEMO:AA:BB:CC:01",
			PlayerName:       "Alice",
			TeamName:         "Les Rouges",
			TeamColor:        []int{239, 68, 68},
			Points:           10,
		},
	}

	// Enrich each demo event with resolved category metadata (v5.7.9)
	for i := range events {
		catName, catImageURL, catColor := h.ResolveCategoryMeta(events[i].QuestionCategory)
		events[i].CategoryDisplayName = catName
		events[i].CategoryImageURL = catImageURL
		events[i].CategoryColor = catColor
	}
	for _, event := range events {
		h.Engine().AddGameEvent(event)
	}

	server.LogInfo(game.LogComponentApp, "Created %d demo history events", len(events))
}
