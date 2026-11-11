package server

// #299 (v11.7.0, LOT-3) — GET /api/lighting/effects/{id}/preview?groups=N&moment=chrono (contrat light-effects.md §7.3) :
// trames calculées par le serveur (effects.Preview), rien n'est joué, aucune E/S vers le pont.
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
	Groups     int    `json:"groups"`
	DurationMs int    `json:"duration_ms"`
	Loop       *bool  `json:"loop"`
	Frames     []struct {
		AtMs   int `json:"at_ms"`
		Groups []struct {
			Color     [3]int `json:"color"`
			Intensity int    `json:"intensity"`
			FadeMs    int    `json:"fade_ms"`
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
	if v.ID != "chenillard" || v.Groups != 6 || v.DurationMs != 20000 || v.Loop == nil || !*v.Loop {
		t.Fatalf("{id, groups, duration_ms: 20000, loop: true} : %+v", v)
	}
	if len(v.Frames) == 0 || len(v.Frames) > 400 || v.Frames[0].AtMs != 0 {
		t.Fatalf("des trames, 400 au plus, la première à 0 ms : %d", len(v.Frames))
	}
	for _, f := range v.Frames {
		if len(f.Groups) != 6 {
			t.Fatalf("une entrée par groupe (6) : %+v", f)
		}
		for _, g := range f.Groups {
			if g.FadeMs != 0 || g.Intensity < 0 || g.Intensity > 255 {
				t.Fatalf("fade_ms = 0, intensité 0-255 : %+v", g)
			}
		}
	}
	for _, key := range []string{`"at_ms"`, `"duration_ms"`, `"fade_ms"`, `"color"`, `"intensity"`, `"frames"`, `"loop"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("clé %s attendue dans la réponse : %s", key, raw[:200])
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
			if f.AtMs != want.Frames[i].AtMs || len(f.Groups) != len(want.Frames[i].Groups) {
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
}

func TestTW299_Preview_RefusedParameters_400(t *testing.T) {
	srv, _, _ := tw223Server(t)
	for _, q := range []string{"groups=0", "groups=13", "groups=-1", "groups=x", "groups=1.5", "moment=nuit", "moment=CHRONO"} {
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
