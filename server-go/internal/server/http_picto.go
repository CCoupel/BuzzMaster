package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"buzzcontrol/internal/game"
)

// pictoWordFields names the form fields and question keys of one Picto-family
// type's "word source" block (contracts/picto.md §17.2): `PICTO` today,
// `RAFALE_PICTO` (LOT-3, rafale_picto_* / RAFALE_PICTO_*) tomorrow. The
// validation of the source, the filter and the words is ONE function
// (applyPictoWordSource) parameterised by this set.
type pictoWordFields struct {
	formSource, formCategories, formDifficulties, formWords string // multipart form fields
	keySource, keyCategories, keyDifficulties, keyWords     string // question.json keys
}

// pictoQuestionFields is the field set of the `PICTO` type.
var pictoQuestionFields = pictoWordFields{
	formSource: "picto_source", formCategories: "picto_categories", formDifficulties: "picto_difficulties", formWords: "picto_words",
	keySource: "PICTO_SOURCE", keyCategories: "PICTO_CATEGORIES", keyDifficulties: "PICTO_DIFFICULTIES", keyWords: "PICTO_WORDS",
}

// rafalePictoQuestionFields is the field set of the `RAFALE_PICTO` type (contracts/picto.md §17.2).
var rafalePictoQuestionFields = pictoWordFields{
	formSource: "rafale_picto_source", formCategories: "rafale_picto_categories", formDifficulties: "rafale_picto_difficulties", formWords: "rafale_picto_words",
	keySource: "RAFALE_PICTO_SOURCE", keyCategories: "RAFALE_PICTO_CATEGORIES", keyDifficulties: "RAFALE_PICTO_DIFFICULTIES", keyWords: "RAFALE_PICTO_WORDS",
}

// PICTO error codes of the RAFALE_PICTO save (contracts/picto.md §17.2).
const (
	pictoErrPassInvalid    = "PICTO_PASS_INVALID"
	pictoErrPoolEndInvalid = "PICTO_POOL_END_INVALID"
)

// applyRafalePictoFields validates and writes the RAFALE_PICTO fields of a question save: TIME (30–300 s), the
// word source block (one function with PICTO, parameterised by the field set), the pass cost (0 to 100 points),
// the pass limit (0 = unlimited, 1 = a single pass, anything else refused) and the end of the pool (END | LOOP).
// Zero values are not written (every field is omitempty). Refusals answer 400 prefixed by the code.
func (h *HTTPServer) applyRafalePictoFields(r *http.Request, question map[string]interface{}) (int, string) {
	t, err := strconv.Atoi(strings.TrimSpace(r.FormValue("time")))
	if err != nil || !game.ValidPictoTime(t) {
		return http.StatusBadRequest, game.PictoErrTimeInvalid + ": time doit être un entier de 30 à 300 secondes"
	}
	question["TIME"] = strconv.Itoa(t)
	if status, msg := h.applyPictoWordSource(r, question, rafalePictoQuestionFields); status != 0 {
		return status, msg
	}
	if raw := strings.TrimSpace(r.FormValue("rafale_picto_pass_cost")); raw != "" {
		c, err := strconv.Atoi(raw)
		if err != nil || c < 0 || c > 100 {
			return http.StatusBadRequest, pictoErrPassInvalid + ": le coût d'une passe est un entier de 0 à 100 points"
		}
		if c > 0 {
			question["RAFALE_PICTO_PASS_COST"] = c
		}
	}
	if raw := strings.TrimSpace(r.FormValue("rafale_picto_pass_limit")); raw != "" {
		l, err := strconv.Atoi(raw)
		if err != nil || (l != 0 && l != 1) {
			return http.StatusBadRequest, pictoErrPassInvalid + ": la limite de passes vaut 0 (illimitées) ou 1 (une seule passe)"
		}
		if l > 0 {
			question["RAFALE_PICTO_PASS_LIMIT"] = l
		}
	}
	switch end := strings.TrimSpace(r.FormValue("rafale_picto_pool_end")); end {
	case "":
	case game.PictoPoolEndEnd, game.PictoPoolEndLoop:
		question["RAFALE_PICTO_POOL_END"] = end
	default:
		return http.StatusBadRequest, pictoErrPoolEndInvalid + ": la fin de pool vaut END ou LOOP"
	}
	return 0, ""
}

// applyPictoFields validates and writes the PICTO fields of a question save
// (contracts/picto.md §11, §17.2): TIME (30–300 s), then the word source
// block. Returns a non-zero HTTP status and a message prefixed by the error
// code on refusal.
func (h *HTTPServer) applyPictoFields(r *http.Request, question map[string]interface{}) (int, string) {
	t, err := strconv.Atoi(strings.TrimSpace(r.FormValue("time")))
	if err != nil || !game.ValidPictoTime(t) {
		return http.StatusBadRequest, game.PictoErrTimeInvalid + ": time doit être un entier de 30 à 300 secondes"
	}
	// Store the validated, normalised duration — never the raw input (" 090" → "90").
	question["TIME"] = strconv.Itoa(t)
	return h.applyPictoWordSource(r, question, pictoQuestionFields)
}

// applyPictoWordSource validates and writes the source / filter / words of a
// Picto-family question (contracts/picto.md §17.2):
//   - source absent or WORDS: 1 to 20 words (§3.1), no filter written — a
//     v13.0.0 question without the source field stays exactly as it was;
//   - source RESERVOIR: at least one known category and one difficulty of 1 to
//     3, no word written;
//   - any other source: PICTO_SOURCE_INVALID.
//
// handleUploadQuestion rebuilds the question from scratch: every field read
// here MUST be written back, or it is destroyed (RAFALE bugfix 2026-08-31).
func (h *HTTPServer) applyPictoWordSource(r *http.Request, question map[string]interface{}, f pictoWordFields) (int, string) {
	source := strings.TrimSpace(r.FormValue(f.formSource))
	switch source {
	case "":
		// legacy: WORDS, nothing stored for the source
	case game.PictoSourceWords, game.PictoSourceReservoir:
		question[f.keySource] = source
	default:
		return http.StatusBadRequest, game.PictoErrSourceInvalid + ": source doit valoir WORDS ou RESERVOIR"
	}

	if source == game.PictoSourceReservoir {
		var cats []string
		if err := json.Unmarshal([]byte(r.FormValue(f.formCategories)), &cats); err != nil || len(cats) == 0 {
			return http.StatusBadRequest, game.PictoErrFilterInvalid + ": au moins une catégorie est exigée (tableau JSON)"
		}
		seenCat := map[string]bool{}
		outCats := make([]string, 0, len(cats))
		for _, c := range cats {
			if !h.isKnownCategory(c) {
				return http.StatusBadRequest, game.PictoErrFilterInvalid + ": catégorie inconnue " + strconv.Quote(c)
			}
			if !seenCat[c] {
				seenCat[c] = true
				outCats = append(outCats, c)
			}
		}
		var diffs []int
		if err := json.Unmarshal([]byte(r.FormValue(f.formDifficulties)), &diffs); err != nil || len(diffs) == 0 {
			return http.StatusBadRequest, game.PictoErrFilterInvalid + ": au moins une difficulté est exigée (tableau JSON)"
		}
		seenDiff := map[int]bool{}
		outDiffs := make([]int, 0, len(diffs))
		for _, d := range diffs {
			if d < 1 || d > 3 {
				return http.StatusBadRequest, game.PictoErrFilterInvalid + ": difficulté hors de 1 à 3"
			}
			if !seenDiff[d] {
				seenDiff[d] = true
				outDiffs = append(outDiffs, d)
			}
		}
		question[f.keyCategories] = outCats
		question[f.keyDifficulties] = outDiffs
		return 0, ""
	}

	var raw []string
	for _, line := range strings.Split(r.FormValue(f.formWords), "\n") {
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
	question[f.keyWords] = words
	return 0, ""
}
