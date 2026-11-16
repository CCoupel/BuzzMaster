package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"buzzcontrol/internal/game"
)

// ROUE « Roue de la Fortune » (v12.0.0, #145, contracts/roue.md §11): saving
// the fields of a ROUE question and the phrase preview of the editor.

// applyRoueFields reads the ROUE_* form fields into question (the map written
// to question.json). It returns a non-zero HTTP status and the 400 text
// (prefixed by the error code) when a field is refused; nothing is written then.
func applyRoueFields(r *http.Request, question map[string]interface{}) (int, string) {
	phrase := game.RoueCompactPhrase(r.FormValue("roue_phrase"))
	presentation := strings.TrimSpace(r.FormValue("roue_presentation"))
	mode := strings.TrimSpace(r.FormValue("ROUE_MODE"))
	gainsRule := strings.TrimSpace(r.FormValue("roue_gains_rule"))
	layout := strings.TrimSpace(r.FormValue("roue_wheel_layout"))

	if _, err := game.RoueBuildBoard(phrase); err != nil {
		return http.StatusBadRequest, err.Error()
	}
	if err := game.ValidateRoueFields(presentation, mode, gainsRule, layout); err != nil {
		return http.StatusBadRequest, err.Error()
	}
	question["ROUE_PHRASE"] = phrase
	if presentation != "" {
		question["ROUE_PRESENTATION"] = presentation
	}
	if mode == "" {
		mode = game.RoueModeTantQueJeGagne
	}
	question["ROUE_MODE"] = mode
	if gainsRule == "" {
		gainsRule = game.RoueGainsWinnerOnly
	}
	question["ROUE_GAINS_RULE"] = gainsRule
	if layout == "" {
		layout = game.RafaleWheelLayoutStrip
	}
	question["ROUE_WHEEL_LAYOUT"] = layout
	// Written only for false: absent means true (contract §3).
	if r.FormValue("roue_show_letters") == "false" {
		question["ROUE_SHOW_LETTERS"] = false
	}
	// A ROUE question has no answer / points / global timer (contract §3).
	question["ANSWER"] = ""
	return 0, ""
}

// handleRouePreview — POST /api/roue/preview (contracts/roue.md §11.2):
// validates a phrase and returns its layout, writes nothing.
func (h *HTTPServer) handleRouePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Phrase string `json:"PHRASE"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	layout, err := game.RoueBuildBoard(body.Phrase)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		code, msg := game.RoueErrPhraseInvalidChar, err.Error()
		if re, ok := err.(*game.RoueError); ok {
			code, msg = re.Code, re.Message
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"CODE": code, "MESSAGE": msg})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"COLS": layout.Cols, "MAX_ROWS": game.RoueBoardMaxRows, "ROWS": layout.Rows,
		"CONSONANTS": layout.Consonants, "LETTERS": layout.Letters,
	})
}
