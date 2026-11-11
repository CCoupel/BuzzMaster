package effects

import (
	"fmt"
	"regexp"
	"strconv"
	"unicode/utf8"
)

var (
	reID    = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
	reColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

var colorKeywords = map[string]bool{"scene": true, "theme": true, "team": true, "urgency": true}

type validator struct {
	file  string
	lines map[string]int
	errs  []Error
}

func (v *validator) add(path, format string, args ...any) {
	v.errs = append(v.errs, Error{File: v.file, Line: lineOf(v.lines, path), Message: fmt.Sprintf(format, args...)})
}

// validate checks e against the schema of §4 and fills its defaults
// (target, min_groups). The effective pattern of every stage is validated too.
func validate(file string, e *Effect, lines map[string]int) []Error {
	v := &validator{file: file, lines: lines}

	switch {
	case e.ID == "":
		v.add("id", "la clé « id » est obligatoire")
	case !reID.MatchString(e.ID):
		v.add("id", "l'identifiant « %s » est invalide : 1 à %d caractères parmi a-z, 0-9 et -", e.ID, MaxIDLen)
	}
	if n := utf8.RuneCountInString(e.Name); n < 1 || n > MaxNameLen {
		v.add("name", "la clé « name » est obligatoire et compte de 1 à %d caractères", MaxNameLen)
	}
	if utf8.RuneCountInString(e.Description) > MaxDescLen {
		v.add("description", "la description dépasse %d caractères", MaxDescLen)
	}
	switch e.Version {
	case 0:
		v.add("version", "la clé « version » est obligatoire")
	case SchemaVersion:
	default:
		v.add("version", "la version %d du format n'est pas prise en charge (seule la version %d l'est)", e.Version, SchemaVersion)
	}
	if len(e.Moments) == 0 {
		v.add("moments", "« moments » est obligatoire et ne peut pas être vide (chrono, calm, score)")
	}
	for i, m := range e.Moments {
		if !Moment(m).Valid() {
			v.add("moments."+strconv.Itoa(i), "le moment « %s » est inconnu (chrono, calm ou score)", m)
		}
	}
	switch e.Target {
	case "":
		e.Target = TargetGroups
	case TargetGroups, TargetAll:
	default:
		v.add("target", "target « %s » inconnu (groups ou all)", e.Target)
	}
	if e.MinGroups == 0 {
		e.MinGroups = 1
	}
	if e.Target == TargetAll && e.MinGroups > 1 {
		v.add("min_groups", "min_groups n'a pas de sens avec target: all")
	} else if e.MinGroups < 1 || e.MinGroups > MaxGroups {
		v.add("min_groups", "min_groups doit être compris entre 1 et %d", MaxGroups)
	}
	if e.DurationMs < 0 || e.DurationMs > MaxDurationMs {
		v.add("duration_ms", "duration_ms doit être compris entre 0 et %d", MaxDurationMs)
	}
	if e.HasMoment(MomentScore) && e.DurationMs <= 0 {
		v.add("duration_ms", "duration_ms est obligatoire (supérieur à 0) pour un effet du moment score")
	}
	if len(e.Looks) == 0 {
		v.add("looks", "« looks » est obligatoire : au moins une apparence")
	}
	for name, l := range e.Looks {
		v.look("looks."+name, name, l)
	}
	if e.Pattern.Type == "" {
		v.add("pattern", "« pattern » est obligatoire (avec un « type »)")
	} else {
		v.pattern("pattern", e.Pattern, e.Looks)
		v.allPattern("pattern", *e)
	}
	if len(e.Stages) > MaxStages {
		v.add("stages", "%d étapes au plus", MaxStages)
	}
	for i, st := range e.Stages {
		p := "stages." + strconv.Itoa(i)
		if st.When.empty() {
			v.add(p+".when", "l'étape %d n'a aucune condition (remaining_lte, remaining_gt, groups_lt, groups_gte)", i+1)
		}
		if st.When.GroupsLT != nil && *st.When.GroupsLT < 0 || st.When.GroupsGTE != nil && *st.When.GroupsGTE < 0 {
			v.add(p+".when", "les conditions sur les groupes ne peuvent pas être négatives")
		}
		for name, l := range st.Looks {
			v.look(p+".looks."+name, name, l)
		}
		if e.Target == TargetAll && (st.When.GroupsLT != nil || st.When.GroupsGTE != nil) {
			v.add(p+".when", "groups_lt et groups_gte n'ont pas de sens avec target: all")
		}
		if len(st.Looks) > 0 || !isZeroPattern(st.Pattern) {
			looks, pat := applyStage(e.Looks, e.Pattern, st)
			v.pattern(p+".pattern", pat, looks)
			v.allPattern(p+".pattern", Effect{Target: e.Target, Pattern: pat})
		}
	}
	return v.errs
}

func isZeroPattern(p Pattern) bool {
	return p.Type == "" && p.Phases == nil && p.Cells == nil && p.Rest == "" && p.StepMs == 0 && p.Wrap == nil &&
		p.Direction == "" && p.Source == "" && p.On == "" && p.Off == "" && p.Edge == "" && p.Count == nil &&
		p.Ratio == nil && p.Stops == nil && p.Travel == "" && p.Origin == "" && p.Repeat == nil
}

func (v *validator) look(path, name string, l Appearance) {
	if l.Color != "" && !colorKeywords[l.Color] && !reColor.MatchString(l.Color) {
		v.add(path+".color", "la couleur « %s » de l'apparence « %s » est invalide (scene, theme, team, urgency ou #RRGGBB)", l.Color, name)
	}
	if l.Intensity != nil && (*l.Intensity < 0 || *l.Intensity > 100) {
		v.add(path+".intensity", "l'intensité de l'apparence « %s » doit être comprise entre 0 et 100", name)
	}
	if l.FadeMs < 0 || l.FadeMs > MaxFadeMs {
		v.add(path+".fade_ms", "fade_ms de l'apparence « %s » doit être compris entre 0 et %d", name, MaxFadeMs)
	}
}

func (v *validator) ref(path, name string, looks map[string]Appearance) {
	if _, ok := looks[name]; !ok {
		v.add(path, "l'apparence « %s » n'est pas définie dans looks", name)
	}
}

func (v *validator) step(path string, ms int) {
	if ms < MinStepMs || ms > MaxStepMs {
		v.add(path, "%s doit être compris entre %d et %d ms", lastKey(path), MinStepMs, MaxStepMs)
	}
}

func lastKey(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i+1:]
		}
	}
	return path
}

func oneOf(s string, allowed ...string) bool {
	for _, a := range allowed {
		if s == a {
			return true
		}
	}
	return false
}

// pattern validates the effective pattern p (base, or base overridden by a
// stage) against the effective looks.
func (v *validator) pattern(path string, p Pattern, looks map[string]Appearance) {
	switch p.Type {
	case PatternUniform:
		if n := len(p.Phases); n < 1 || n > MaxPhases {
			v.add(path+".phases", "uniform demande de 1 à %d phases", MaxPhases)
		}
		for i, ph := range p.Phases {
			pp := path + ".phases." + strconv.Itoa(i)
			v.ref(pp+".look", ph.Appearance, looks)
			if ph.Ms < MinStepMs || ph.Ms > MaxStepMs {
				v.add(pp+".ms", "la durée d'une phase doit être comprise entre %d et %d ms", MinStepMs, MaxStepMs)
			}
		}
	case PatternShift:
		if n := len(p.Cells); n < 1 || n > MaxGroups {
			v.add(path+".cells", "shift demande de 1 à %d cellules", MaxGroups)
		}
		for i, c := range p.Cells {
			v.ref(path+".cells."+strconv.Itoa(i), c, looks)
		}
		if p.Rest == "" {
			v.add(path, "shift demande « rest » (l'apparence des groupes hors du bloc)")
		} else {
			v.ref(path+".rest", p.Rest, looks)
		}
		v.step(path+".step_ms", p.StepMs)
		if !oneOf(p.Direction, "", "forward", "backward", "bounce") {
			v.add(path+".direction", "direction « %s » inconnue (forward, backward ou bounce)", p.Direction)
		}
	case PatternGauge:
		if !oneOf(p.Source, "remaining", "elapsed") {
			v.add(path+".source", "gauge demande source: remaining ou elapsed")
		}
		for _, k := range []struct{ key, val string }{{"on", p.On}, {"off", p.Off}} {
			if k.val == "" {
				v.add(path, "gauge demande « %s » (une apparence)", k.key)
			} else {
				v.ref(path+"."+k.key, k.val, looks)
			}
		}
		if p.Edge != "" {
			v.ref(path+".edge", p.Edge, looks)
		}
		if !oneOf(p.Direction, "", "forward", "backward") {
			v.add(path+".direction", "direction « %s » inconnue pour gauge (forward ou backward)", p.Direction)
		}
	case PatternRandom:
		v.step(path+".step_ms", p.StepMs)
		for _, k := range []struct{ key, val string }{{"on", p.On}, {"off", p.Off}} {
			if k.val == "" {
				v.add(path, "random demande « %s » (une apparence)", k.key)
			} else {
				v.ref(path+"."+k.key, k.val, looks)
			}
		}
		switch {
		case p.Count == nil && p.Ratio == nil:
			v.add(path, "random demande « count » ou « ratio »")
		case p.Count != nil && p.Ratio != nil:
			v.add(path+".ratio", "random n'accepte pas « count » et « ratio » ensemble")
		case p.Count != nil && (*p.Count < 0 || *p.Count > MaxGroups):
			v.add(path+".count", "count doit être compris entre 0 et %d", MaxGroups)
		case p.Ratio != nil && (*p.Ratio < 0 || *p.Ratio > 1):
			v.add(path+".ratio", "ratio doit être compris entre 0 et 1")
		}
	case PatternGradient:
		if n := len(p.Stops); n < 2 || n > 6 {
			v.add(path+".stops", "gradient demande de 2 à 6 apparences dans stops")
		}
		for i, s := range p.Stops {
			v.ref(path+".stops."+strconv.Itoa(i), s, looks)
		}
		if !oneOf(p.Travel, "", "none", "forward", "backward", "outward") {
			v.add(path+".travel", "travel « %s » inconnu (none, forward, backward ou outward)", p.Travel)
		} else if p.Travel != "" && p.Travel != "none" {
			v.step(path+".step_ms", p.StepMs)
		}
		if !oneOf(p.Origin, "", "first", "last", "center", "team") {
			v.add(path+".origin", "origin « %s » inconnue (first, last, center ou team)", p.Origin)
		}
	default:
		v.add(path+".type", "le type de motif « %s » est inconnu (uniform, shift, gauge, random ou gradient)", p.Type)
	}
}

// applyStage returns the looks and pattern in force when stage st applies: its
// looks replace the base ones of the same name, the non-zero keys of its
// pattern replace those of the base pattern (§4.4).
func applyStage(baseLooks map[string]Appearance, base Pattern, st Stage) (map[string]Appearance, Pattern) {
	looks := baseLooks
	if len(st.Looks) > 0 {
		looks = make(map[string]Appearance, len(baseLooks)+len(st.Looks))
		for k, l := range baseLooks {
			looks[k] = l
		}
		for k, l := range st.Looks {
			looks[k] = l
		}
	}
	p := base
	o := st.Pattern
	if o.Type != "" {
		p.Type = o.Type
	}
	if o.Phases != nil {
		p.Phases = o.Phases
	}
	if o.Cells != nil {
		p.Cells = o.Cells
	}
	if o.Rest != "" {
		p.Rest = o.Rest
	}
	if o.StepMs != 0 {
		p.StepMs = o.StepMs
	}
	if o.Wrap != nil {
		p.Wrap = o.Wrap
	}
	if o.Direction != "" {
		p.Direction = o.Direction
	}
	if o.Source != "" {
		p.Source = o.Source
	}
	if o.On != "" {
		p.On = o.On
	}
	if o.Off != "" {
		p.Off = o.Off
	}
	if o.Edge != "" {
		p.Edge = o.Edge
	}
	if o.Count != nil {
		p.Count = o.Count
	}
	if o.Ratio != nil {
		p.Ratio = o.Ratio
	}
	if o.Stops != nil {
		p.Stops = o.Stops
	}
	if o.Travel != "" {
		p.Travel = o.Travel
	}
	if o.Origin != "" {
		p.Origin = o.Origin
	}
	if o.Repeat != nil {
		p.Repeat = o.Repeat
	}
	return looks, p
}

// allPattern enforces that `target: all` only uses the uniform pattern (§4.6).
func (v *validator) allPattern(path string, e Effect) {
	if e.Target == TargetAll && e.Pattern.Type != "" && e.Pattern.Type != PatternUniform {
		v.add(path+".type", "target: all n'accepte que le motif uniform (« %s » demande des groupes)", e.Pattern.Type)
	}
}
