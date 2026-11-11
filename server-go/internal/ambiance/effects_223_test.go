package ambiance

// #223 (v11.7.0, LOT-2) — lecteur d'effets : moments, précédence, repli, groupes vides, groupe animateur,
// arrêt sans écriture fantôme, non-régression de la suite State. Source : contracts/light-effects.md §2.1, §2.5,
// §5.1 à §5.3, §6. Horloge injectée (Controller.EffectsClock) et EffectsStep() exporté : aucune attente réelle.
// Ces tests modifient le singleton config : non parallèles. Aides préfixées tw223.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

type tw223Source struct {
	cat  effects.Catalogue
	room map[string]string
	host map[string]map[string]string // id de groupe animateur → moment → id d'effet
}

func (s *tw223Source) EffectsCatalogue() effects.Catalogue { return s.cat }
func (s *tw223Source) EffectSettings() (map[string]string, map[string]map[string]string) {
	return s.room, s.host
}

type tw223Env struct {
	c     *Controller
	src   *tw223Source
	eng   *game.Engine
	now   time.Time
	dir   string
	setup func(groups []config.LightingGroupEntry)
}

const tw223Calm = `
id: %s
name: Test
version: 1
moments: [calm]
target: groups
looks: { a: { color: "#FF0000" } }
pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }
`

func tw223Effect(id, moments, target string, minGroups int, color string) string {
	src := "id: " + id + "\nname: Test\nversion: 1\nmoments: [" + moments + "]\ntarget: " + target + "\n"
	if minGroups > 1 {
		src += "min_groups: " + string(rune('0'+minGroups)) + "\n"
	}
	if moments == "score" {
		src += "duration_ms: 3000\n"
	}
	return src + "looks: { a: { color: \"" + color + "\" } }\npattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n"
}

func tw223New(t *testing.T) *tw223Env {
	t.Helper()
	prev := config.Get()
	t.Cleanup(func() { config.SetInstance(prev) })
	cfg := *prev
	cfg.Lighting.Groups = nil
	config.SetInstance(&cfg)

	dir := t.TempDir()
	eng := game.NewEngine()
	c := New(fakeHost{engine: eng})
	env := &tw223Env{c: c, eng: eng, dir: dir, now: time.Unix(1_800_000_000, 0)}
	c.EffectsClock = func() time.Time { return env.now }
	c.LightingWriter.Store(lighting.NewWriter(lighting.Config{Driver: lighting.NewFakeDriver(), Derive: c.DeriveEvent, Scene: c.Scene}))
	env.src = &tw223Source{room: map[string]string{"chrono": "pulsation", "calm": "", "score": ""}, host: map[string]map[string]string{"h1": {"chrono": "", "calm": "", "score": ""}}}
	c.SetEffectsSource(env.src)
	env.setup = func(groups []config.LightingGroupEntry) {
		next := *config.Get()
		next.Lighting.Groups = groups
		config.SetInstance(&next)
	}
	env.reload()
	return env
}

func (e *tw223Env) writeUser(t *testing.T, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(e.dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	e.reload()
}

func (e *tw223Env) reload() {
	e.src.cat = effects.LoadDir(os.DirFS(filepath.Join("..", "..", "assets", "effects")), e.dir)
}

func (e *tw223Env) step() { e.c.EffectsStep() }

// setHost règle l'effet d'un moment pour le groupe animateur d'identifiant id (réglage global ← manche, déjà résolu).
func (e *tw223Env) setHost(id, moment, effect string) {
	if e.src.host[id] == nil {
		e.src.host[id] = map[string]string{"chrono": "", "calm": "", "score": ""}
	}
	e.src.host[id][moment] = effect
}

func grp(n int, lights ...string) config.LightingGroupEntry {
	return config.LightingGroupEntry{Name: "G" + string(rune('A'+n)), Role: config.GroupRoleSalle, Lights: lights, NearTeams: []string{}}
}

func hostGrp() config.LightingGroupEntry {
	return config.LightingGroupEntry{ID: "h1", Name: "Pupitre", Role: config.GroupRoleAnimateur, Lights: []string{"H"}, NearTeams: []string{}}
}

func zonesWithGroup(st lighting.State, pred func(int) bool) []lighting.ZoneState {
	var out []lighting.ZoneState
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral && pred(z.Group) {
			out = append(out, z)
		}
	}
	return out
}

func roomGroups(st lighting.State) []lighting.ZoneState {
	return zonesWithGroup(st, func(g int) bool { return g > 0 })
}

func hostEntry(st lighting.State) (lighting.ZoneState, bool) {
	z := zonesWithGroup(st, func(g int) bool { return g == lighting.HostGroup(1) })
	if len(z) == 0 {
		return lighting.ZoneState{}, false
	}
	return z[0], true
}

var tw223Kinds = []lighting.EventKind{
	lighting.KindIdle, lighting.KindReady, lighting.KindCountdown, lighting.KindRunning, lighting.KindBuzz,
	lighting.KindPauseAll, lighting.KindReveal, lighting.KindTeamTurn, lighting.KindEntracte, lighting.KindScore, lighting.KindTimeUp,
}

// ---------- non-régression : sans groupe ni réglage, la suite State est celle de v11.6.0 ----------

func TestEffects223_NoGroupsNoSettings_StateIdenticalToV1160(t *testing.T) {
	env := tw223New(t)
	for _, k := range tw223Kinds {
		before := env.c.Scene(lighting.Event{Kind: k, Teams: []string{"Rouges"}, Points: 5})
		env.step() // le lecteur ne publie rien
		after := env.c.Scene(lighting.Event{Kind: k, Teams: []string{"Rouges"}, Points: 5})
		if len(after.Zones) != len(before.Zones) {
			t.Fatalf("%s : le lecteur ne doit rien ajouter sans groupe ni réglage (%d → %d zones)", k, len(before.Zones), len(after.Zones))
		}
		for i := range before.Zones {
			if before.Zones[i] != after.Zones[i] {
				t.Errorf("%s : zone %d modifiée : %+v → %+v", k, i, before.Zones[i], after.Zones[i])
			}
		}
		if g := roomGroups(after); len(g) != 0 {
			t.Errorf("%s : aucune entrée de groupe attendue : %+v", k, g)
		}
		if _, ok := hostEntry(after); ok {
			t.Errorf("%s : aucune entrée animateur sans groupe animateur", k)
		}
		if after.Zones[0].Zone != lighting.ZoneGeneral || after.Zones[0].Group != 0 {
			t.Errorf("%s : l'entrée {general, Group 0} reste la première : %+v", k, after.Zones[0])
		}
	}
}

func TestEffects223_GroupsButDefaultSettings_NoEntryAdded_PulsationStaysLegacy(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B"), grp(2, "C"), grp(3, "D")})
	// chrono = « pulsation » (défaut) : le code existant RunChronoPulse reste seul actif
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.step()
	st := env.c.Scene(lighting.Event{Kind: lighting.KindRunning})
	if g := roomGroups(st); len(g) != 0 {
		t.Errorf("la pulsation (défaut) n'ajoute aucune entrée de groupe : %+v", g)
	}
}

// ---------- groupes vides intercalés : N = groupes non vides, numéros réels ----------

func TestEffects223_EmptyGroupsInterleaved_FrameUsesRealNumbers(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1), grp(2, "C"), grp(3, "D")}) // groupe 2 vide
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.src.room["calm"] = "calm-fx"
	env.step()
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	g := roomGroups(st)
	if len(g) != 3 {
		t.Fatalf("N = 3 groupes non vides : 3 entrées attendues, obtenu %+v", g)
	}
	got := []int{g[0].Group, g[1].Group, g[2].Group}
	if got[0] != 1 || got[1] != 3 || got[2] != 4 {
		t.Errorf("numéros réels 1, 3, 4 (le groupe vide 2 garde son numéro, jamais écrit) : %v", got)
	}
	for _, z := range g {
		if z.Color != [3]int{255, 0, 0} || z.Intensity != 255 {
			t.Errorf("couleur de l'effet : %+v", z)
		}
	}
	if st.Zones[0].Group != 0 {
		t.Error("l'entrée Group 0 reste en tête (§2.2)")
	}
}

func TestEffects223_HostGroupNeverCountsInN(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A"), grp(1, "B")})
	// chenillard demande 3 groupes : N = 2 (l'animateur ne compte pas) ⇒ inapplicable ⇒ aucune trame
	env.src.room["chrono"] = "chenillard"
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.step()
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindRunning})); len(g) != 0 {
		t.Errorf("N = 2 < min_groups 3 : repli sur la valeur par défaut du moment, aucune trame : %+v", g)
	}
}

// ---------- moments ----------

func TestEffects223_ChronoMoment_NativeChenillard_SixGroups(t *testing.T) {
	env := tw223New(t)
	// 6 groupes : traînée à 50 % visible (thème blanc par défaut dans ce harnais)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B"), grp(2, "C"), grp(3, "D"), grp(4, "E"), grp(5, "F")})
	env.src.room["chrono"] = "chenillard"
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.step()
	g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindRunning}))
	if len(g) != 6 {
		t.Fatalf("chenillard à 6 groupes : une entrée par groupe, obtenu %+v", g)
	}
	dim := 0
	for _, z := range g {
		if z.Intensity < 200 {
			dim++
		}
	}
	if dim != 2 {
		t.Errorf("traînée à 50 %% : 2 groupes atténués autour de la tête, obtenu %d : %+v", dim, g)
	}
	// l'effet avance avec l'horloge injectée (un groupe par seconde) sans attente réelle
	first := g
	env.now = env.now.Add(time.Second)
	env.step()
	second := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindRunning}))
	same := true
	for i := range first {
		if first[i] != second[i] {
			same = false
		}
	}
	if same {
		t.Error("une seconde plus tard (horloge injectée), la tête du chenillard a avancé")
	}
}

func TestEffects223_CalmMoment_AppliesToIdleReady_NotToOtherMoments(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.src.room["calm"] = "calm-fx"
	env.step() // état STOPPED ⇒ KindIdle ⇒ moment calm
	for _, k := range []lighting.EventKind{lighting.KindIdle, lighting.KindPauseAll, lighting.KindEntracte} {
		if g := roomGroups(env.c.Scene(lighting.Event{Kind: k})); len(g) != 2 {
			t.Errorf("%s (phase calme) : effet appliqué : %+v", k, g)
		}
	}
	for _, k := range []lighting.EventKind{lighting.KindReady, lighting.KindBuzz, lighting.KindReveal, lighting.KindTeamTurn, lighting.KindCountdown} {
		if g := roomGroups(env.c.Scene(lighting.Event{Kind: k})); len(g) != 0 {
			t.Errorf("%s n'est pas une phase calme : aucun effet : %+v", k, g)
		}
	}
}

func TestEffects223_ScoreMoment_WindowBoundedToThePulse(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "score.yaml", tw223Effect("score-fx", "score", "groups", 1, "#00FF00"))
	env.src.room["score"] = "score-fx"
	env.step()
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}})); len(g) != 0 {
		t.Fatal("pas d'effet de score avant l'attribution")
	}
	env.c.StartScoreFlash("Rouges", 5)
	t.Cleanup(func() { env.c.StartScoreFlash("", 0) })
	env.step()
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}})); len(g) != 2 {
		t.Fatalf("pendant l'impulsion SCORE, l'effet « équipe qui marque » joue : %+v", g)
	}
	env.now = env.now.Add(lighting.ScorePulseDuration + time.Millisecond)
	env.step()
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}})); len(g) != 0 {
		t.Errorf("l'effet est borné à la durée de l'impulsion : %+v", g)
	}
}

func TestEffects223_ScorePrimesOverCalm(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.writeUser(t, "score.yaml", tw223Effect("score-fx", "score", "groups", 1, "#00FF00"))
	env.src.room["calm"], env.src.room["score"] = "calm-fx", "score-fx"
	env.step()
	env.c.StartScoreFlash("Rouges", 5)
	t.Cleanup(func() { env.c.StartScoreFlash("", 0) })
	env.step()
	g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}}))
	if len(g) != 2 || g[0].Color != [3]int{0, 255, 0} {
		t.Errorf("score prime sur calm pendant sa durée : %+v", g)
	}
}

// ---------- précédence ON / OFF / Flash ----------

func TestEffects223_Precedence_OnOffFlashSuppressRoomEffect_AutoRestoresIt(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.src.room["calm"] = "calm-fx"
	env.step()
	ev := lighting.Event{Kind: lighting.KindIdle}
	if len(roomGroups(env.c.Scene(ev))) != 2 {
		t.Fatal("effet actif en AUTO")
	}
	for _, mode := range []string{"ON", "OFF"} {
		if err := env.c.SetLightingMode(mode); err != nil {
			t.Fatal(err)
		}
		env.step()
		st := env.c.Scene(ev)
		if g := roomGroups(st); len(g) != 0 {
			t.Errorf("mode %s : OverrideGeneral s'applique, aucune trame : %+v", mode, g)
		}
	}
	if err := env.c.SetLightingMode("AUTO"); err != nil {
		t.Fatal(err)
	}
	env.c.SetLightingFlash(true)
	env.step()
	if g := roomGroups(env.c.Scene(ev)); len(g) != 0 {
		t.Errorf("Flash engagé : aucune trame : %+v", g)
	}
	env.c.SetLightingFlash(false)
	env.step()
	if len(roomGroups(env.c.Scene(ev))) != 2 {
		t.Error("retour en AUTO sans Flash : l'effet reprend")
	}
}

// ---------- repli sur la valeur par défaut du moment ----------

func TestEffects223_Fallback_AbsentInvalidInapplicableWrongMoment_NoFrame(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "cassé.yaml", "id: casse\nbidule: 1\n")
	env.writeUser(t, "grand.yaml", tw223Effect("grand", "calm", "groups", 5, "#FF0000")) // min_groups 5, N = 2
	env.writeUser(t, "chrono.yaml", tw223Effect("chrono-only", "chrono", "groups", 1, "#FF0000"))
	for name, id := range map[string]string{"absent": "n-existe-pas", "invalide": "casse", "inapplicable": "grand", "autre moment": "chrono-only"} {
		env.src.room["calm"] = id
		env.step()
		if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(g) != 0 {
			t.Errorf("effet %s : repli sur la valeur par défaut (aucun effet en phase calme), obtenu %+v", name, g)
		}
	}
}

func TestEffects223_ChangeOfSettingTakesEffectAtNextStep_NoRestart(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.step()
	if len(roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))) != 0 {
		t.Fatal("aucun effet réglé")
	}
	env.src.room["calm"] = "calm-fx"
	env.step()
	if len(roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))) != 2 {
		t.Error("réglage changé à chaud : pris en compte au pas suivant")
	}
	env.src.room["calm"] = ""
	env.step()
	if len(roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))) != 0 {
		t.Error("retour à « aucun effet » : la trame disparaît")
	}
}

// ---------- groupe animateur ----------

func TestEffects223_Host_DefaultFixedWhiteAtEveryKind_RoomEffectNeverReachesIt(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.src.room["calm"] = "calm-fx"
	env.step()
	for _, k := range tw223Kinds {
		z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: k, Teams: []string{"Rouges"}}))
		if !ok || z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
			t.Errorf("%s : l'animateur reste en blanc fixe : %+v (%v)", k, z, ok)
		}
	}
}

func TestEffects223_Host_EffectPerMoment_WhiteSceneColour(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "host.yaml", tw223Effect("host-fx", "calm", "all", 1, "#00FF00"))
	env.setHost("h1", "calm", "host-fx")
	env.step()
	z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if !ok || z.Color != [3]int{0, 255, 0} {
		t.Fatalf("effet de l'animateur en phase calme : %+v (%v)", z, ok)
	}
	// l'effet de l'animateur ne s'applique qu'à son moment
	z, _ = hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindReveal}))
	if z.Color != [3]int{255, 255, 255} {
		t.Errorf("hors du moment calm, blanc fixe : %+v", z)
	}
	// la salle n'est pas concernée par l'effet de l'animateur
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(g) != 0 {
		t.Errorf("l'effet de l'animateur n'écrit aucun groupe de salle : %+v", g)
	}
}

func TestEffects223_Host_UsesSceneColourWhite_ForSceneLooks(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A")})
	env.writeUser(t, "host.yaml", tw223Effect("host-scene", "calm", "all", 1, "scene"))
	env.setHost("h1", "calm", "host-scene")
	env.step()
	z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if !ok || z.Color != [3]int{255, 255, 255} {
		t.Errorf("la couleur `scene` vaut blanc pour le groupe animateur : %+v", z)
	}
}

func TestEffects223_Host_InsensitiveToOnOffFlash_StillPlaysItsEffect(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "host.yaml", tw223Effect("host-fx", "calm", "all", 1, "#00FF00"))
	env.setHost("h1", "calm", "host-fx")
	for _, mode := range []string{"OFF", "ON"} {
		if err := env.c.SetLightingMode(mode); err != nil {
			t.Fatal(err)
		}
		env.step()
		if z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); !ok || z.Color != [3]int{0, 255, 0} || z.Intensity != 255 {
			t.Errorf("mode %s : l'animateur garde son effet : %+v", mode, z)
		}
	}
	if err := env.c.SetLightingMode("AUTO"); err != nil {
		t.Fatal(err)
	}
	env.c.SetLightingFlash(true)
	t.Cleanup(func() { env.c.SetLightingFlash(false) })
	env.step()
	if z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); !ok || z.Color != [3]int{0, 255, 0} {
		t.Errorf("Flash : l'animateur garde son effet : %+v", z)
	}
}

func TestEffects223_Host_FallbackToFixedWhite_AbsentInvalidInapplicable(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A")})
	env.writeUser(t, "cassé.yaml", "id: casse\nbidule: 1\n")
	env.writeUser(t, "multi.yaml", tw223Effect("multi", "calm", "groups", 3, "#FF0000")) // plusieurs groupes : inapplicable à l'animateur
	for name, id := range map[string]string{"absent": "nope", "invalide": "casse", "plusieurs groupes": "multi"} {
		env.setHost("h1", "calm", id)
		env.step()
		z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
		if !ok || z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
			t.Errorf("effet %s : blanc fixe attendu, obtenu %+v", name, z)
		}
	}
}

func TestEffects223_Host_TargetGroupsWithMinOneIsApplicable(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A")})
	env.writeUser(t, "uni.yaml", tw223Effect("uni", "calm", "groups", 1, "#0000FF"))
	env.setHost("h1", "calm", "uni")
	env.step()
	if z, _ := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); z.Color != [3]int{0, 0, 255} {
		t.Errorf("target: groups avec min_groups ≤ 1 : applicable à l'animateur : %+v", z)
	}
}

func TestEffects223_Host_ScoreEffect_TeamColour_BoundedToThePulse(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A")})
	env.setHost("h1", "score", "couleur-equipe") // natif target: all
	env.step()
	env.c.StartScoreFlash("Rouges", 5)
	t.Cleanup(func() { env.c.StartScoreFlash("", 0) })
	env.step()
	z, _ := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}}))
	if want := (fakeHost{}).TeamNameColor("Rouges"); z.Color != want || z.Intensity < 250 {
		t.Errorf("couleur de l'équipe créditée sur l'animateur : %+v, attendu %v", z, want)
	}
	env.now = env.now.Add(lighting.ScorePulseDuration + time.Millisecond)
	env.step()
	z, _ = hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}}))
	if z.Color != [3]int{255, 255, 255} {
		t.Errorf("après l'impulsion, l'animateur redevient blanc : %+v", z)
	}
}

func TestEffects223_Host_WithoutHostGroup_SettingsKeptButNoEntry(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A")})
	env.writeUser(t, "host.yaml", tw223Effect("host-fx", "calm", "all", 1, "#00FF00"))
	env.setHost("h1", "calm", "host-fx")
	env.step()
	if _, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); ok {
		t.Error("sans groupe animateur, les réglages sont conservés mais sans effet : aucune entrée GroupHost")
	}
}

// ---------- arrêt : le lecteur se tait avant l'extinction, aucune écriture fantôme ----------

func TestEffects223_Quiesce_ReaderStopsPublishing_NoGhostWrite(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.writeUser(t, "host.yaml", tw223Effect("host-fx", "calm", "all", 1, "#00FF00"))
	env.src.room["calm"] = "calm-fx"
	env.setHost("h1", "calm", "host-fx")
	env.step()
	if len(roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))) != 2 {
		t.Fatal("effet actif avant l'arrêt")
	}
	env.c.quiesceAmbiance()
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(g) != 0 {
		t.Errorf("après l'arrêt, la trame publiée est retirée : %+v", g)
	}
	env.step() // un tic tardif ne republie rien
	env.now = env.now.Add(time.Second)
	env.step()
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(g) != 0 {
		t.Errorf("le lecteur ne republie jamais après quiesce : %+v", g)
	}
	if z, _ := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); z.Color == [3]int{0, 255, 0} {
		t.Error("l'effet de l'animateur n'est plus publié non plus")
	}
}

func TestEffects223_RunEffects_Idempotent_StopsWithContext_NoRealWait(t *testing.T) {
	env := tw223New(t)
	env.c.EffectsTick = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func(i int) { env.c.RunEffects(ctx); finished <- i }(i)
	}
	select { // une seule goroutine de lecture : l'autre appel rend la main tout de suite
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("un second RunEffects doit retourner immédiatement (une seule goroutine)")
	}
	select {
	case <-finished:
		t.Fatal("la goroutine de lecture ne s'arrête pas avant l'annulation du contexte")
	default:
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("RunEffects s'arrête avec le contexte")
	}
}

func TestEffects223_DisabledLighting_NoWriter_NoFrame(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.src.room["calm"] = "calm-fx"
	env.c.LightingWriter.Store(nil) // éclairage non configuré / pilote absent (§5.3 point 1)
	env.step()
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(g) != 0 {
		t.Errorf("pilote absent : le lecteur ne publie rien : %+v", g)
	}
}
