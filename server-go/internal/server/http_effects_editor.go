package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"buzzcontrol/internal/ambiancecfg"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/lighting/hue"
)

// Effect editor endpoints (v11.8.0, #210 — contracts/light-effects.md §11.2 to §11.4, §11.10):
// GET …/{id}/source, POST …/draft, PUT …/{id}, POST …/draft/try. The server is the only parser,
// validator and interpreter: the browser sends a document or a text and gets the verdict, the
// regenerated text, the grid view and the previews.

const editorBodyMax = 64 << 10

// DraftTrialer is optionally implemented by the LightingProvider (the App): the trial of an
// effect that is not in the catalogue (§11.10).
type DraftTrialer interface {
	StartDraftTrial(e effects.Effect, zone string, speed float64, seconds int) (int, error)
}

func (h *HTTPServer) draftTrialer() DraftTrialer {
	if h.Lighting == nil {
		return nil
	}
	t, _ := h.Lighting.(DraftTrialer)
	return t
}

// editorBody is the common body of draft, draft/try and PUT: exactly one of effect / yaml.
type editorBody struct {
	Effect  json.RawMessage `json:"effect"`
	Yaml    *string         `json:"yaml"`
	Groups  []int           `json:"groups"`
	Moment  string          `json:"moment"`
	Create  bool            `json:"create"`
	Zone    string          `json:"zone"`
	Speed   float64         `json:"speed"`
	Seconds int             `json:"seconds"`
}

func (b editorBody) hasEffect() bool {
	return len(b.Effect) > 0 && string(b.Effect) != "null"
}

// readEditorBody reads the JSON body (64 KB at most): 400 unreadable, 413 too big, 400 when
// not exactly one of effect / yaml. ok=false: the answer has been written.
func readEditorBody(w http.ResponseWriter, r *http.Request) (editorBody, bool) {
	var b editorBody
	r.Body = http.MaxBytesReader(w, r.Body, editorBodyMax)
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "corps trop gros (64 Ko au plus)", http.StatusRequestEntityTooLarge)
		} else if errors.Is(err, io.EOF) {
			http.Error(w, "corps JSON attendu", http.StatusBadRequest)
		} else {
			http.Error(w, "corps JSON invalide", http.StatusBadRequest)
		}
		return b, false
	}
	if b.hasEffect() == (b.Yaml != nil) {
		http.Error(w, "une seule des clés effect ou yaml est attendue", http.StatusBadRequest)
		return b, false
	}
	return b, true
}

// draftError is one validation problem of a draft (§11.3): no file name; line 0 for an `effect`
// draft, path for the place in the document.
type draftError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

func draftErrors(errs []effects.Error) []draftError {
	out := make([]draftError, 0, len(errs))
	for _, e := range errs {
		out = append(out, draftError{Line: e.Line, Message: e.Message, Path: e.Path})
	}
	return out
}

// draft is the outcome of reading a body as an effect draft.
type draft struct {
	ok          bool
	e           effects.Effect
	doc         *effects.Document // the document of the draft (null for an unparsable text)
	yaml        string            // regenerated text, "" when not serializable
	text        []byte            // the text to store as is (yaml drafts) or the regenerated one
	hasComments bool
	customOrder bool
	errs        []draftError
}

// readDraft validates the effect or yaml of b. Nothing is written.
func readDraft(b editorBody) draft {
	var d draft
	if b.Yaml != nil {
		data := []byte(*b.Yaml)
		d.hasComments = effects.HasComments(data)
		e, errs := effects.ValidateText(data)
		if len(errs) > 0 {
			d.errs = draftErrors(errs)
			return d
		}
		doc := effects.DocumentOf(e)
		text, serrs := effects.Serialize(doc)
		if len(serrs) > 0 { // cannot happen: the text has just been validated
			d.errs = draftErrors(serrs)
			return d
		}
		d.ok, d.e, d.doc, d.yaml, d.text, d.customOrder = true, e, &doc, text, data, effects.CustomOrder(e)
		return d
	}
	var in effects.Document
	if err := json.Unmarshal(b.Effect, &in); err != nil {
		d.errs = []draftError{{Message: "le document effect est illisible : " + err.Error()}}
		return d
	}
	d.doc = &in
	e, errs := effects.ParseDocument(in)
	if len(errs) > 0 {
		d.errs = draftErrors(errs)
		return d
	}
	doc := effects.DocumentOf(e)
	text, _ := effects.Serialize(doc)
	// The text that PUT would store goes through the same checks as a yaml draft.
	if _, verrs := effects.ValidateText([]byte(text)); len(verrs) > 0 {
		d.errs = draftErrors(verrs)
		return d
	}
	d.ok, d.e, d.doc, d.yaml, d.text = true, e, &doc, text, []byte(text)
	return d
}

// handleEffectSource serves GET /api/lighting/effects/{id}/source (§11.2).
func (h *HTTPServer) handleEffectSource(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := h.effectsStore()
	if st == nil {
		http.Error(w, "effet inconnu", http.StatusNotFound)
		return
	}
	data, en, err := st.EffectsSource(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	doc := effects.DocumentOf(en.Effect)
	writeEffectsJSON(w, map[string]any{
		"id": en.Effect.ID, "origin": en.Origin, "file": en.File, "read_only": en.Origin != effects.OriginUser,
		"yaml": string(data), "effect": doc, "has_comments": effects.HasComments(data),
		"custom_order": effects.CustomOrder(en.Effect), "grid": effects.BuildGrid(doc),
	})
}

// handleEffectDraft serves POST /api/lighting/effects/draft (§11.3): validates, converts and
// previews a draft; writes nothing, plays nothing, leaves the catalogue alone.
func (h *HTTPServer) handleEffectDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	b, ok := readEditorBody(w, r)
	if !ok {
		return
	}
	if len(b.Groups) > 8 {
		http.Error(w, "groups : 8 tailles au plus", http.StatusBadRequest)
		return
	}
	seen := map[int]bool{}
	for _, g := range b.Groups {
		if g < 1 || g > effects.MaxGroups || seen[g] {
			http.Error(w, "groups : des entiers distincts de 1 à 12", http.StatusBadRequest)
			return
		}
		seen[g] = true
	}
	if b.Moment != "" && !effects.Moment(b.Moment).Valid() {
		http.Error(w, "moment : chrono, calm ou score", http.StatusBadRequest)
		return
	}
	d := readDraft(b)
	resp := map[string]any{"ok": d.ok, "errors": d.errs, "effect": nil, "yaml": "", "has_comments": d.hasComments,
		"custom_order": d.customOrder, "grid": nil, "moment": b.Moment, "previews": []effects.PreviewResult{}}
	if d.doc != nil && (d.ok || b.Yaml == nil) {
		resp["effect"] = d.doc
	}
	if !d.ok {
		writeEffectsJSON(w, resp)
		return
	}
	moment := effects.Moment(b.Moment)
	if moment == "" {
		moment = effects.Moment(d.e.Moments[0])
	}
	resp["moment"] = string(moment)
	resp["yaml"] = d.yaml
	if !d.e.HasMoment(moment) {
		resp["ok"] = false
		resp["errors"] = []draftError{{Message: "le moment « " + string(moment) + " » n'est pas déclaré par l'effet", Path: "moments"}}
		writeEffectsJSON(w, resp)
		return
	}
	grid := effects.BuildGrid(*d.doc)
	resp["grid"] = grid
	previews := make([]effects.PreviewResult, 0, len(b.Groups))
	for _, g := range b.Groups {
		previews = append(previews, effects.Preview(d.e, g, moment))
	}
	resp["previews"] = previews
	writeEffectsJSON(w, resp)
}

// handleEffectPut serves PUT /api/lighting/effects/{id} (§11.4).
func (h *HTTPServer) handleEffectPut(w http.ResponseWriter, r *http.Request, id string) {
	st := h.effectsStore()
	if st == nil {
		http.Error(w, "stockage des effets indisponible", http.StatusServiceUnavailable)
		return
	}
	if effects.IsReserved(id) {
		http.Error(w, "identifiant réservé", http.StatusBadRequest)
		return
	}
	b, ok := readEditorBody(w, r)
	if !ok {
		return
	}
	var data []byte
	if b.Yaml != nil {
		data = []byte(*b.Yaml)
	} else {
		d := readDraft(b)
		if !d.ok {
			writeLightingJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "errors": d.errs})
			return
		}
		data = d.text
	}
	file, _, err := st.EffectsWrite(id, data, b.Create)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeEffectsJSON(w, map[string]any{"ok": true, "file": file, "catalogue": h.effectsView()})
}

// handleEffectDraftTry serves POST /api/lighting/effects/draft/try (§11.10): the draft is
// played on a zone, with the controls and the refusals of the trial of §7.4.
func (h *HTTPServer) handleEffectDraftTry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	b, ok := readEditorBody(w, r)
	if !ok {
		return
	}
	d := readDraft(b)
	if !d.ok {
		writeLightingJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "errors": d.errs})
		return
	}
	zone, speed, seconds := b.Zone, b.Speed, b.Seconds
	if zone == "" {
		zone = ambiancecfg.ZoneAmbiance
	}
	if speed == 0 {
		speed = 1
	}
	if !ambiancecfg.ValidZone(zone) || !ambiancecfg.ValidSpeed(speed) || seconds < 0 || seconds > 20 {
		http.Error(w, "zone (ambiance, animateur, public), speed (0,5 ; 0,75 ; 1 ; 1,5 ; 2) ou seconds (1 à 20) invalide", http.StatusBadRequest)
		return
	}
	if seconds == 0 {
		seconds = trialDefaultSeconds(d.e)
	}
	t := h.draftTrialer()
	h.playTrial(w, t != nil, func() (int, error) { return t.StartDraftTrial(d.e, zone, speed, seconds) })
}

// playTrial applies the driver checks of §7.4 then starts the trial with start and answers.
func (h *HTTPServer) playTrial(w http.ResponseWriter, available bool, start func() (int, error)) {
	drv := h.lightingDriver()
	if drv == nil || !available {
		writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": "disabled"})
		return
	}
	switch drv.Status().State {
	case hue.StateRefused:
		writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": "disabled"})
		return
	case hue.StateUnreachable:
		writeLightingJSON(w, http.StatusServiceUnavailable, map[string]string{"result": "unreachable"})
		return
	}
	ms, err := start()
	if err != nil {
		var tr trialRefusal
		if errors.As(err, &tr) {
			writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": tr.TrialReason()})
		} else {
			http.Error(w, err.Error(), http.StatusNotFound)
		}
		return
	}
	writeLightingJSON(w, http.StatusOK, map[string]any{"ok": true, "duration_ms": ms})
}
