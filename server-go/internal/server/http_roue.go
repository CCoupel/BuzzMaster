package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// ROUE « Roue de la Fortune » (v12.0.0, #145, contracts/roue.md §11): saving
// the fields of a ROUE question and the phrase preview of the editor.

// RoueDefaultTitle is the `question` text of a ROUE question with no presentation.
const RoueDefaultTitle = "Roue de la Fortune"

// roueFieldLeaksPhrase reports whether text contains the whole phrase once both
// are folded (upper case, accents removed, only letters and digits kept).
func roueFieldLeaksPhrase(text, phrase string) bool {
	norm := func(s string) string {
		folded, err := game.RoueFoldPhrase(s)
		if err != nil { // text with characters the board refuses: fold what can be, keep letters/digits
			folded = strings.ToUpper(s)
		}
		var b strings.Builder
		for _, r := range folded {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	p := norm(phrase)
	return p != "" && strings.Contains(norm(text), p)
}

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
	// Hardening (review LOT-1.1): the `question` field reaches the TV and the
	// tablets, so it must never carry the phrase to guess. If it contains the
	// phrase (compared folded, case and accents ignored) it is neutralised to the
	// presentation, or the default title.
	if q, _ := question["QUESTION"].(string); roueFieldLeaksPhrase(q, phrase) {
		if presentation != "" {
			question["QUESTION"] = presentation
		} else {
			question["QUESTION"] = RoueDefaultTitle
		}
	}
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

// ---- wheel configuration (contracts/roue.md §10, §11.3) ---------------------

func wheelParamsFromConfig(w config.WheelConfig) game.RoueWheelParams {
	return game.RoueWheelParams{
		MaxPoints: w.MaxPoints, Step: w.Step, BankruptCount: w.BankruptCount, LoseTurnCount: w.LoseTurnCount,
		SpinPeriodMs: w.SpinPeriodMs, SpinMaxMs: w.SpinMaxMs, StopDurationMs: w.StopDurationMs, FlipIntervalMs: w.FlipIntervalMs,
	}
}

func wheelConfigFromParams(p game.RoueWheelParams) config.WheelConfig {
	return config.WheelConfig{
		MaxPoints: p.MaxPoints, Step: p.Step, BankruptCount: p.BankruptCount, LoseTurnCount: p.LoseTurnCount,
		SpinPeriodMs: p.SpinPeriodMs, SpinMaxMs: p.SpinMaxMs, StopDurationMs: p.StopDurationMs, FlipIntervalMs: p.FlipIntervalMs,
	}
}

// marshalGameConfigWithWheelExtras encodes game-config.json plus the read-only
// `wheel_default` (for the « Roue par défaut » button) and, after a POST that
// corrected the step, `step_adjusted_from` (the step that was entered).
func marshalGameConfigWithWheelExtras(gs *config.GameSettings, stepAdjustedFrom int) ([]byte, error) {
	base, err := json.Marshal(gs)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	def, _ := json.Marshal(config.DefaultWheelConfig())
	m["wheel_default"] = def
	if stepAdjustedFrom > 0 {
		adj, _ := json.Marshal(stepAdjustedFrom)
		m["step_adjusted_from"] = adj
	}
	return json.Marshal(m)
}

// handleRoueWheelPreview — POST /api/roue/wheel-preview (§11.3): the wheel
// generated for parameters that are not saved (preview of the settings card).
func (h *HTTPServer) handleRoueWheelPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body config.WheelConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		writeRoueWheelError(w, "paramètres invalides : "+err.Error())
		return
	}
	np, adjusted, err := game.NormalizeRoueWheelParams(wheelParamsFromConfig(body))
	if err != nil {
		writeRoueWheelError(w, err.Error())
		return
	}
	segs := game.BuildRoueWheel(np)
	resp := map[string]interface{}{"COUNT": len(segs), "step": np.Step, "SEGMENTS": segs}
	if adjusted > 0 {
		resp["step_adjusted_from"] = adjusted
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeRoueWheelError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"CODE": "ROUE_WHEEL_INVALID", "MESSAGE": msg})
}
