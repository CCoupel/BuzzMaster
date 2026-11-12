package server

// #299 (v11.7.0, LOT-3) — GET /api/lighting/effects/{id}/preview?groups=N&moment=chrono (contrat light-effects.md §7.3) :
// trames calculées par le serveur (effects.Preview), rien n'est joué, aucune E/S vers le pont. Compléments e9d86cd7 : une trame
// par tick avec `tick` et `step`, `moment`, `tick_ms`, `chrono_s`, durée fixée par le moment (20 s ; 5 s pour score), plus de `fade_ms`,
// moment non déclaré ⇒ 400 ; catalogue : `steps` ; essai : `seconds` par défaut 5 pour un effet « score » seul.
// Réutilise tw223Server / tw223Post / tw223Zones. Noms de tests TestTW299_*.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"buzzcontrol/internal/effects"
)

type tw299Preview struct {
	ID         string `json:"id"`
	Moment     string `json:"moment"`
	Groups     int    `json:"groups"`
	TickMs     int    `json:"tick_ms"`
	DurationMs int    `json:"duration_ms"`
	ChronoS    *int   `json:"chrono_s"`
	Loop       *bool  `json:"loop"`
	Frames     []struct {
		AtMs   int    `json:"at_ms"`
		Tick   *int   `json:"tick"`
		Step   string `json:"step"`
		Groups []struct {
			Color     [3]int `json:"color"`
			Intensity int    `json:"intensity"`
		} `json:"groups"`
	} `json:"frames"`
}

func tw299Get(t *testing.T, srv *HTTPServer, path string) (int, tw299Preview, string) {
	t.Helper()
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	var v tw299Preview
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	return w.Code, v, w.Body.String()
}

func TestTW299_Preview_ContractShape_ChenillardSixGroups(t *testing.T) {
	srv, _, _ := tw223Server(t)
	code, v, raw := tw299Get(t, srv, "/api/lighting/effects/chenillard/preview?groups=6&moment=chrono")
	if code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	if v.ID != "chenillard" || v.Moment != "chrono" || v.Groups != 6 || v.TickMs != 1000 || v.DurationMs != 20000 || v.ChronoS == nil || *v.ChronoS != 20 || v.Loop == nil || !*v.Loop {
		t.Fatalf("{id, moment, groups: 6, tick_ms: 1000, duration_ms: 20000, chrono_s: 20, loop: true} : %+v", v)
	}
	if len(v.Frames) != 20 {
		t.Fatalf("une trame par tick de 1 s sur 20 s : %d", len(v.Frames))
	}
	for k, f := range v.Frames {
		if f.Tick == nil || *f.Tick != k || f.AtMs != k*1000 || f.Step != "pas:0" || len(f.Groups) != 6 {
			t.Fatalf("trame %d : {at_ms, tick, step: pas:0, 6 groupes} : %+v", k, f)
		}
		for _, g := range f.Groups {
			if g.Intensity < 0 || g.Intensity > 255 {
				t.Fatalf("intensité 0-255 : %+v", g)
			}
		}
	}
	for _, key := range []string{`"at_ms"`, `"tick"`, `"step"`, `"moment"`, `"tick_ms"`, `"duration_ms"`, `"chrono_s"`, `"color"`, `"intensity"`, `"frames"`, `"loop"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("clé %s attendue dans la réponse : %s", key, raw[:200])
		}
	}
	if strings.Contains(raw, "fade_ms") {
		t.Error("`fade_ms` est retiré des états de groupe")
	}
	if strings.Contains(raw, `"post"`) {
		t.Error("« post » n'apparaît jamais dans l'aperçu")
	}
}

func TestTW299_Preview_DurationAndChronoSecondsAreFixedByTheMoment(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", tw223UserEffect) // calm, score
	for _, c := range []struct {
		id, moment  string
		dur, chrono int
	}{
		{"pulsation", "chrono", 20000, 20}, {"degrade", "calm", 20000, 0}, {"couleur-equipe", "score", 5000, 0},
		{"mon-effet", "calm", 20000, 0}, {"mon-effet", "score", 5000, 0},
	} {
		code, v, raw := tw299Get(t, srv, "/api/lighting/effects/"+c.id+"/preview?groups=3&moment="+c.moment)
		if code != 200 {
			t.Fatalf("%s/%s : %d %s", c.id, c.moment, code, raw)
		}
		if v.Moment != c.moment || v.DurationMs != c.dur || v.ChronoS == nil || *v.ChronoS != c.chrono {
			t.Errorf("%s/%s : duration_ms %d (attendu %d), chrono_s %v (attendu %d) : le front lit ces valeurs", c.id, c.moment, v.DurationMs, c.dur, v.ChronoS, c.chrono)
		}
	}
}

func TestTW299_Preview_FramesAreTheEngine_NoSecondInterpreter(t *testing.T) {
	srv, _, prov := tw223Server(t)
	for _, id := range []string{"pulsation", "chenillard", "chenillard-traine", "couleur-equipe", "degrade"} {
		en, _ := prov.EffectsCatalogue().Get(id)
		m := en.Effect.Moments[0]
		code, v, raw := tw299Get(t, srv, "/api/lighting/effects/"+id+"/preview?groups=8&moment="+m)
		if code != 200 {
			t.Fatalf("%s : %d %s", id, code, raw)
		}
		want := effects.Preview(en.Effect, 8, effects.Moment(m))
		if len(v.Frames) != len(want.Frames) {
			t.Fatalf("%s : %d trames, attendu %d (celles du moteur)", id, len(v.Frames), len(want.Frames))
		}
		for i, f := range v.Frames {
			if f.AtMs != want.Frames[i].AtMs || f.Step != want.Frames[i].Step || f.Tick == nil || *f.Tick != want.Frames[i].Tick || len(f.Groups) != len(want.Frames[i].Groups) {
				t.Fatalf("%s trame %d : %+v / %+v", id, i, f, want.Frames[i])
			}
			for p, g := range f.Groups {
				w := want.Frames[i].Groups[p]
				if g.Color != w.Color || g.Intensity != w.Intensity {
					t.Fatalf("%s trame %d g%d : %+v / %+v", id, i, p+1, g, w)
				}
			}
		}
	}
}

func TestTW299_Preview_DefaultGroups_ConfiguredAmbianceNonEmpty_ElseSix(t *testing.T) {
	srv, _, _ := tw223Server(t)
	if _, v, _ := tw299Get(t, srv, "/api/lighting/effects/chenillard/preview"); v.Groups != 6 {
		t.Errorf("aucun groupe configuré : 6 par défaut : %d", v.Groups)
	}
	tw223Zones(t, srv) // zone Ambiance : A, (vide), B ⇒ 2 groupes non vides
	if _, v, _ := tw299Get(t, srv, "/api/lighting/effects/chenillard/preview"); v.Groups != 2 {
		t.Errorf("par défaut : le nombre de groupes non vides de la zone Ambiance : %d", v.Groups)
	}
	if _, v, _ := tw299Get(t, srv, "/api/lighting/effects/chenillard/preview?groups=9"); v.Groups != 9 {
		t.Errorf("groups explicite : %d", v.Groups)
	}
}

func TestTW299_Preview_StaysAvailableWhenTheEffectNeedsMoreGroupsThanConfigured(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Zones(t, srv) // 2 groupes ; chenillard-traine en demande 5
	if code, v, raw := tw299Get(t, srv, "/api/lighting/effects/chenillard-traine/preview?groups=6"); code != 200 || v.Groups != 6 || len(v.Frames) == 0 {
		t.Errorf("l'aperçu reste disponible (simulé avec 6 groupes) : %d %s", code, raw)
	}
}

func TestTW299_Preview_MomentDefaultsToTheFirstOfTheEffect_AndCanBeChosen(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", strings.Replace(tw223UserEffect, "moments: [calm, score]", "moments: [score, calm]", 1))
	_, byDefault, _ := tw299Get(t, srv, "/api/lighting/effects/mon-effet/preview?groups=3")
	_, explicit, _ := tw299Get(t, srv, "/api/lighting/effects/mon-effet/preview?groups=3&moment=score")
	if !reflect.DeepEqual(byDefault, explicit) {
		t.Error("sans moment : le premier moment déclaré par l'effet")
	}
	if code, _, _ := tw299Get(t, srv, "/api/lighting/effects/mon-effet/preview?groups=3&moment=calm"); code != 200 {
		t.Errorf("un autre moment déclaré : %d", code)
	}
	if byDefault.Moment != "score" || byDefault.DurationMs != 5000 {
		t.Errorf("sans moment : le premier moment déclaré (score : 5 s) : %q %d", byDefault.Moment, byDefault.DurationMs)
	}
}

func TestTW299_Preview_RefusedParameters_400(t *testing.T) {
	srv, _, _ := tw223Server(t)
	for _, q := range []string{"groups=0", "groups=13", "groups=-1", "groups=x", "groups=1.5", "moment=nuit", "moment=CHRONO", "moment=calm", "moment=score"} { // calm et score : non déclarés par le chenillard
		if code, _, _ := tw299Get(t, srv, "/api/lighting/effects/chenillard/preview?"+q); code != http.StatusBadRequest {
			t.Errorf("%s : 400 attendu, obtenu %d", q, code)
		}
	}
	for _, q := range []string{"groups=1", "groups=12"} {
		if code, _, _ := tw299Get(t, srv, "/api/lighting/effects/chenillard/preview?"+q); code != 200 {
			t.Errorf("%s : bornes incluses, obtenu %d", q, code)
		}
	}
}

func TestTW299_Preview_UnknownEffect_404_AndWithoutCatalogue_404(t *testing.T) {
	srv, _, _ := tw223Server(t)
	if code, _, _ := tw299Get(t, srv, "/api/lighting/effects/n-existe-pas/preview"); code != http.StatusNotFound {
		t.Errorf("effet inconnu : 404, obtenu %d", code)
	}
	bare, _ := setupTestHTTPServer(t)
	if code, _, _ := tw299Get(t, bare, "/api/lighting/effects/chenillard/preview"); code != http.StatusNotFound {
		t.Errorf("sans catalogue : 404, obtenu %d", code)
	}
}

func TestTW299_Preview_OnlyGET(t *testing.T) {
	srv, _, _ := tw223Server(t)
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, httptest.NewRequest(m, "/api/lighting/effects/chenillard/preview", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s : 405 attendu, obtenu %d", m, w.Code)
		}
	}
}

func TestTW299_Preview_NeverTouchesTheBridge_AndNeedsNoLighting(t *testing.T) {
	// pas de pilote, éclairage désactivé : l'aperçu répond quand même (pure simulation)
	srv, _, _ := tw223Server(t)
	if code, _, raw := tw299Get(t, srv, "/api/lighting/effects/chenillard/preview?groups=4"); code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	srv2, bridge, _ := tw223TrialServer(t)
	before := func() int { bridge.mu.Lock(); defer bridge.mu.Unlock(); return len(bridge.hits) }()
	if code, _, _ := tw299Get(t, srv2, "/api/lighting/effects/chenillard/preview?groups=4"); code != 200 {
		t.Fatalf("avec pilote : %d", code)
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if len(bridge.hits) != before {
		t.Errorf("aucune E/S vers le pont : %v", bridge.hits[before:])
	}
}

func TestTW299_Preview_UserEffectIsPreviewedLikeANative_AfterReload(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", tw223UserEffect)
	code, v, raw := tw299Get(t, srv, "/api/lighting/effects/mon-effet/preview?groups=3")
	if code != 200 || len(v.Frames) == 0 {
		t.Fatalf("%d %s", code, raw)
	}
	g := v.Frames[0].Groups[0]
	if g.Color != [3]int{255, 0, 0} {
		t.Errorf("g1 en rouge d'après le fichier de l'utilisateur : %+v", g)
	}
}

func TestTW299_CatalogueTry_EmptyBody_IsTheCatalogueButton_SalleSpeedOne(t *testing.T) {
	// « Essayer sur la salle » : appel sans zone ni vitesse ⇒ zone ambiance, ×1, 10 s (§7.4)
	srv, _, prov := tw223TrialServer(t)
	for _, body := range []string{"", "{}"} {
		if code, out := devDo(t, srv, "POST", "/api/lighting/effects/chenillard/try", body); code != 200 || out["duration_ms"] != float64(10000) {
			t.Fatalf("corps %q : %d %v", body, code, out)
		}
	}
	for _, c := range prov.calls {
		if c.zone != "ambiance" || c.speed != 1 || c.seconds != 10 {
			t.Errorf("essai du catalogue : zone ambiance, ×1, 10 s : %+v", c)
		}
	}
}

func TestTW299_Catalogue_StepsIsTheNumberOfStepsOfTheLoop(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "trois.yaml", "id: trois\nnom: Trois\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npre: { g0: [\"#0000FF\", 100] }\n"+
		"pas:\n  - { g0: [\"#FF0000\", 100] }\n  - { g0: [\"#00FF00\", 100] }\n  - { g0: [\"#FFFFFF\", 100] }\ndernier: { g0: [\"#000000\", 100] }\npost: { g0: [\"#000000\", 100] }\n")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/lighting/effects", nil))
	var v struct {
		Effects []struct {
			ID    string `json:"id"`
			Steps *int   `json:"steps"`
		} `json:"effects"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, e := range v.Effects {
		if e.Steps == nil {
			t.Fatalf("%s : `steps` toujours présent", e.ID)
		}
		got[e.ID] = *e.Steps
	}
	if got["trois"] != 3 {
		t.Errorf("trois pas dans la boucle (sans pre, dernier ni post) : %d", got["trois"])
	}
	for _, id := range []string{"pulsation", "chenillard", "couleur-equipe", "degrade"} {
		if got[id] < 1 {
			t.Errorf("natif %s : au moins un pas : %d", id, got[id])
		}
	}
	if got["chenillard"] != 1 {
		t.Errorf("chenillard : 1 pas : %d", got["chenillard"])
	}
}

func TestTW299_CatalogueTry_DefaultSeconds_FiveForAScoreOnlyEffect_TenOtherwise(t *testing.T) {
	srv, _, prov := tw223TrialServer(t)
	tw223WriteEffect(t, srv.Effects.(*tw223Provider), "mon-effet.yaml", tw223UserEffect) // calm + score : 10 s
	for _, c := range []struct {
		id      string
		seconds int
	}{{"couleur-equipe", 5}, {"pulsation", 10}, {"degrade", 10}, {"mon-effet", 10}} {
		code, out := devDo(t, srv, "POST", "/api/lighting/effects/"+c.id+"/try", `{}`)
		if code != 200 || out["duration_ms"] != float64(c.seconds*1000) {
			t.Fatalf("%s : duration_ms %v (attendu %d) : %d", c.id, out["duration_ms"], c.seconds*1000, code)
		}
	}
	for _, c := range prov.calls {
		if (c.id == "couleur-equipe") != (c.seconds == 5) {
			t.Errorf("seconds par défaut : 5 pour un effet score seul, 10 sinon : %+v", c)
		}
	}
	// une durée explicite reste respectée
	if _, out := devDo(t, srv, "POST", "/api/lighting/effects/couleur-equipe/try", `{"seconds":8}`); out["duration_ms"] != float64(8000) {
		t.Errorf("seconds explicite : %v", out)
	}
}
