package server

import (
	"net/http"
	"strconv"
	"strings"

	"buzzcontrol/internal/game"
)

// applyPictoFields validates and writes the PICTO fields of a question save
// (contracts/picto.md §11): TIME (30–300 s) and PICTO_WORDS (form field
// picto_words, one word per line, blank lines ignored). Returns a non-zero
// HTTP status and a message prefixed by the error code on refusal.
func applyPictoFields(r *http.Request, question map[string]interface{}) (int, string) {
	t, err := strconv.Atoi(strings.TrimSpace(r.FormValue("time")))
	if err != nil || !game.ValidPictoTime(t) {
		return http.StatusBadRequest, game.PictoErrTimeInvalid + ": time doit être un entier de 30 à 300 secondes"
	}
	// Store the validated, normalised duration — never the raw input (" 090" → "90").
	question["TIME"] = strconv.Itoa(t)
	var raw []string
	for _, line := range strings.Split(r.FormValue("picto_words"), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		raw = append(raw, line)
	}
	words, werr := game.ValidatePictoWords(raw)
	if werr != nil {
		return http.StatusBadRequest, werr.Error()
	}
	question["PICTO_WORDS"] = words
	return 0, ""
}
