package ambiance

// #223 (v11.7.0, LOT-2) — lecteur d'effets : moments, précédence, repli, groupes vides, groupe animateur,
// arrêt sans écriture fantôme, non-régression de la suite State. Source : contracts/light-effects.md §2.1, §2.5,
// §5.1 à §5.3, §6. Horloge injectée (Controller.EffectsClock) et EffectsStep() exporté : aucune attente réelle.
// Ces tests modifient le singleton config : non parallèles. Aides préfixées tw223.

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// tw223Source : source des réglages RÉSOLUS (global ← manche) par zone, voir zones_api_spec_test.go.
type tw223Source struct {
	cat effects.Catalogue
	eff map[string]map[string]string  // zone → moment → id d'effet
	spd map[string]map[string]float64 // zone → moment → vitesse
}

type tw223Env struct {
	c     *Controller
	src   *tw223Source
	eng   *game.Engine
	now   time.Time
	dir   string
	setup func(groups []config.LightingGroupEntry)
}

// tw223Effect : effet utilisateur au format pas à pas (version: 2). target « all » : une action sur g0 (jouable sur un groupe
// animateur) ; « groups » : une action sur chacun des groupes g1…g12 (les absents sont ignorés ; inapplicable à un animateur).
// minGroups > 1 ⇒ groupes_min. `color` : « #RRGGBB » ou une couleur nommée.
func tw223Effect(id, moments, target string, minGroups int, color string) string {
	col := color
	if len(col) > 0 && col[0] == '#' {
		col = `"` + col + `"`
	}
	act := "[" + col + ", 100]"
	body := "g0: " + act
	if target == "groups" {
		parts := make([]string, 12)
		for i := range parts {
			parts[i] = "g" + strconv.Itoa(i+1) + ": " + act
		}
		body = strings.Join(parts, ", ")
	}
	src := "id: " + id + "\nnom: Test\nversion: 2\nmoments: [" + moments + "]\n"
	if minGroups > 1 {
		src += "groupes_min: " + strconv.Itoa(minGroups) + "\n"
	}
	return src + "duree_tick_ms: 1000\npas:\n  - { " + body + " }\n"
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
	env.src = tw223DefaultSource()
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

func grp(n int, lights ...string) config.LightingGroupEntry {
	return config.LightingGroupEntry{ID: "g" + string(rune(0x61+n)), Role: config.GroupRoleSalle, Lights: lights}
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

func TestEffects223_GroupsButDefaultSettings_PulsationPlayedByReader_OnG0Only(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B"), grp(2, "C"), grp(3, "D")})
	// chrono = « pulsation » (défaut) : effet natif joué par le lecteur, qui n'écrit que g0 (révision 3, §5.5)
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.step()
	st := env.c.Scene(lighting.Event{Kind: lighting.KindRunning})
	if g := roomGroups(st); len(g) != 0 {
		t.Errorf("la pulsation n'écrit que g0 : aucune entrée de groupe : %+v", g)
	}
	if st.Zones[0].Group != 0 || st.Zones[0].Intensity != 255 {
		t.Errorf("tick 0 : phase haute au thème à 100 %% sur l'entrée Group 0 : %+v", st.Zones[0])
	}
	env.now = env.now.Add(500 * time.Millisecond) // tick de 500 ms : phase basse
	env.step()
	if z := env.c.Scene(lighting.Event{Kind: lighting.KindRunning}).Zones[0]; z.Intensity < 126 || z.Intensity > 130 {
		t.Errorf("tick 1 : phase basse à 50 %% : %+v", z)
	}
}

// ---------- groupes vides intercalés : N = groupes non vides, numéros réels ----------

func TestEffects223_EmptyGroupsInterleaved_FrameUsesRealNumbers(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1), grp(2, "C"), grp(3, "D")}) // groupe 2 vide
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.setEffect("ambiance", "calm", "calm-fx")
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
	env.setEffect("ambiance", "chrono", "chenillard")
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.step()
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindRunning})); len(g) != 0 {
		t.Errorf("N = 2 < min_groups 3 : repli sur la valeur par défaut du moment, aucune trame : %+v", g)
	}
}

// ---------- moments ----------

func TestEffects223_ChronoMoment_NativeChenillardTraine_SixGroups_AdvancesWithInjectedClock(t *testing.T) {
	env := tw223New(t)
	// 6 groupes : traînée à 50 % visible (thème blanc par défaut dans ce harnais)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B"), grp(2, "C"), grp(3, "D"), grp(4, "E"), grp(5, "F")})
	env.setEffect("ambiance", "chrono", "chenillard-traine")
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.step()
	g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindRunning}))
	if len(g) == 0 {
		t.Fatalf("chenillard-traine à 6 groupes : des entrées de groupe pour la traînée : %+v", g)
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
	// un groupe par tick de 1 s, sans attente réelle
	first := tw223GroupsSnapshot(env)
	env.now = env.now.Add(time.Second)
	env.step()
	if tw223GroupsSnapshot(env) == first {
		t.Error("une seconde plus tard (horloge injectée), la tête du chenillard a avancé d'un groupe")
	}
}

// tw223GroupsSnapshot : la trame de salle sous forme de chaîne comparable.
func tw223GroupsSnapshot(env *tw223Env) string {
	var b strings.Builder
	for _, z := range roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindRunning})) {
		b.WriteString(strconv.Itoa(z.Group) + ":" + strconv.Itoa(z.Intensity) + ";")
	}
	return b.String()
}

func TestEffects223_CalmMoment_AppliesToIdleReady_NotToOtherMoments(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.setEffect("ambiance", "calm", "calm-fx")
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
	env.setEffect("ambiance", "score", "score-fx")
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
	env.setEffect("ambiance", "calm", "calm-fx")
	env.setEffect("ambiance", "score", "score-fx")
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
	env.setEffect("ambiance", "calm", "calm-fx")
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
		env.setEffect("ambiance", "calm", id)
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
	env.setEffect("ambiance", "calm", "calm-fx")
	env.step()
	if len(roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))) != 2 {
		t.Error("réglage changé à chaud : pris en compte au pas suivant")
	}
	env.setEffect("ambiance", "calm", "")
	env.step()
	if len(roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))) != 0 {
		t.Error("retour à « aucun effet » : la trame disparaît")
	}
}

// ---------- zone Animateur (§2.5) : blanc fixe sans effet, effet par moment, hors ON / OFF / Flash ----------

func TestEffects223_Host_DefaultFixedWhiteAtEveryKind_RoomEffectNeverReachesIt(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.setEffect("ambiance", "calm", "calm-fx")
	env.step()
	for _, k := range tw223Kinds {
		z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: k, Teams: []string{"Rouges"}}))
		if !ok || z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
			t.Errorf("%s : la zone animateur reste en blanc fixe : %+v (%v)", k, z, ok)
		}
	}
}

func TestEffects223_Host_EffectPerMoment_ThenFixedWhiteOutsideTheMoment(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "host.yaml", tw223Effect("host-fx", "calm", "all", 1, "#00FF00"))
	env.setHost("calm", "host-fx")
	env.step()
	z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if !ok || z.Color != [3]int{0, 255, 0} {
		t.Fatalf("effet de la zone animateur en phase calme : %+v (%v)", z, ok)
	}
	z, _ = hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindReveal}))
	if z.Color != [3]int{255, 255, 255} {
		t.Errorf("hors du moment calm, blanc fixe : %+v", z)
	}
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(g) != 0 {
		t.Errorf("l'effet de la zone animateur n'écrit aucun groupe de la zone ambiance : %+v", g)
	}
}

func TestEffects223_Host_SceneColourIsWhite(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A")})
	env.writeUser(t, "host.yaml", tw223Effect("host-scene", "calm", "all", 1, "scene"))
	env.setHost("calm", "host-scene")
	env.step()
	z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if !ok || z.Color != [3]int{255, 255, 255} {
		t.Errorf("la couleur `scene` vaut blanc pour la zone animateur : %+v", z)
	}
}

func TestEffects223_Host_InsensitiveToOnOffFlash_StillPlaysItsEffect(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "host.yaml", tw223Effect("host-fx", "calm", "all", 1, "#00FF00"))
	env.setHost("calm", "host-fx")
	for _, mode := range []string{"OFF", "ON"} {
		if err := env.c.SetLightingMode(mode); err != nil {
			t.Fatal(err)
		}
		env.step()
		if z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); !ok || z.Color != [3]int{0, 255, 0} || z.Intensity != 255 {
			t.Errorf("mode %s : la zone animateur garde son effet : %+v", mode, z)
		}
	}
	if err := env.c.SetLightingMode("AUTO"); err != nil {
		t.Fatal(err)
	}
	env.c.SetLightingFlash(true)
	t.Cleanup(func() { env.c.SetLightingFlash(false) })
	env.step()
	if z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); !ok || z.Color != [3]int{0, 255, 0} {
		t.Errorf("Flash : la zone animateur garde son effet : %+v", z)
	}
}

func TestEffects223_Host_FallbackToFixedWhite_AbsentInvalidInapplicable(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A")}) // N = 1 groupe non vide dans la zone animateur
	env.writeUser(t, "cassé.yaml", "id: casse\nbidule: 1\n")
	env.writeUser(t, "grand.yaml", tw223Effect("grand", "calm", "groups", 3, "#FF0000")) // demande 3 groupes, la zone en a 1
	for name, id := range map[string]string{"absent": "nope", "invalide": "casse", "demande trop de groupes": "grand"} {
		env.setHost("calm", id)
		env.step()
		z, ok := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
		if !ok || z.Color != [3]int{255, 255, 255} || z.Intensity != 255 {
			t.Errorf("effet %s : blanc fixe attendu, obtenu %+v", name, z)
		}
	}
}

func TestEffects223_Host_SameRuleAsRoom_EffectWritingGroupsIsApplicableWhenEnoughGroups(t *testing.T) {
	// révision 4 : la règle « un effet qui n'écrit que g0 » est supprimée ; la zone animateur applique les mêmes règles que la salle
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp("H1"), hostGrp("H2"), grp(0, "A")})
	env.writeUser(t, "uni.yaml", tw223Effect("uni", "calm", "groups", 2, "#0000FF")) // g1…g12, groupes_min 2
	env.setHost("calm", "uni")
	env.step()
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	for g := 1; g <= 2; g++ {
		z, ok := areaEntry(st, lighting.AreaHost, g)
		if !ok || z.Color != [3]int{0, 0, 255} {
			t.Errorf("zone animateur, groupe %d : bleu (effet applicable, N = 2 ≥ groupes_min) : %+v (%v)", g, z, ok)
		}
	}
}

func TestEffects223_Host_ChenillardRunsOnItsOwnGroups(t *testing.T) {
	// un chenillard peut tourner sur les groupes de la zone animateur (§2.5, symétrie)
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp("H1"), hostGrp("H2"), hostGrp("H3"), grp(0, "A")})
	env.writeUser(t, "tour.yaml", "id: tour\nnom: Tour\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\nvariables:\n  gA: g1; +1\npas:\n  - { g0: [\"#0000FF\", 40], gA: [\"#FFFFFF\", 100] }\n")
	env.setHost("calm", "tour")
	start := env.now
	var heads []int
	for tick := 0; tick < 4; tick++ {
		env.now = start.Add(time.Duration(tick) * time.Second)
		env.step()
		st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
		for g := 1; g <= 3; g++ {
			if z, ok := areaEntry(st, lighting.AreaHost, g); ok && z.Color == [3]int{255, 255, 255} {
				heads = append(heads, g)
			}
		}
	}
	if len(heads) != 4 || heads[0] != 1 || heads[1] != 2 || heads[2] != 3 || heads[3] != 1 {
		t.Errorf("la tête parcourt g1, g2, g3 de la zone animateur puis reboucle : %v", heads)
	}
}

func TestEffects223_Host_ScoreEffect_TeamColour_BoundedToThePulse(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A")})
	env.setHost("score", "couleur-equipe")
	env.step()
	env.c.StartScoreFlash("Rouges", 5)
	t.Cleanup(func() { env.c.StartScoreFlash("", 0) })
	env.step()
	z, _ := hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}}))
	if want := (fakeHost{}).TeamNameColor("Rouges"); z.Color != want || z.Intensity < 250 {
		t.Errorf("couleur de l'équipe créditée sur la zone animateur : %+v, attendu %v", z, want)
	}
	env.now = env.now.Add(lighting.ScorePulseDuration + time.Millisecond)
	env.step()
	z, _ = hostEntry(env.c.Scene(lighting.Event{Kind: lighting.KindScore, Teams: []string{"Rouges"}}))
	if z.Color != [3]int{255, 255, 255} {
		t.Errorf("après l'impulsion, la zone animateur redevient blanche : %+v", z)
	}
}

func TestEffects223_Host_ZoneWithoutGroup_SettingsKeptButNoEntry(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A")})
	env.writeUser(t, "host.yaml", tw223Effect("host-fx", "calm", "all", 1, "#00FF00"))
	env.setHost("calm", "host-fx")
	env.step()
	if es := areaEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.AreaHost); len(es) != 0 {
		t.Errorf("zone animateur sans groupe : elle n'existe pas, aucune entrée : %+v", es)
	}
}

func TestEffects223_Host_EmptyGroupsOnly_ZoneDoesNotExist(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), {ID: "hv", Role: config.GroupRoleAnimateur, Lights: []string{}}})
	env.step()
	if es := areaEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.AreaHost); len(es) != 0 {
		t.Errorf("zone sans groupe non vide : aucune entrée : %+v", es)
	}
}

// ---------- zone Public (§2.5) : jamais pilotée sans effet ----------

func TestEffects223_Public_NoEffect_NothingIsEverSent_AtAnyKind(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), pubGrp("P1", "P2")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.setEffect("ambiance", "calm", "calm-fx") // un effet de salle ne touche pas la zone public
	env.step()
	for _, k := range tw223Kinds {
		if es := areaEntries(env.c.Scene(lighting.Event{Kind: k, Teams: []string{"Rouges"}, Points: 5}), lighting.AreaPublic); len(es) != 0 {
			t.Errorf("%s : sans effet pour la zone public, ni scène de salle ni blanc fixe : %+v", k, es)
		}
	}
}

func TestEffects223_Public_NoEffect_NotEvenAtStartup(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{pubGrp("P1")})
	env.step()
	if es := areaEntries(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.AreaPublic); len(es) != 0 {
		t.Errorf("au démarrage rien n'est envoyé à la zone public : %+v", es)
	}
}

func TestEffects223_Public_EffectPlaysOnItsOwnGroups_PerMoment(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), pubGrp("P1"), pubGrp("P2")})
	env.writeUser(t, "pub.yaml", tw223Effect("pub-fx", "calm", "groups", 1, "#00FF00"))
	env.setPublic("calm", "pub-fx")
	env.step()
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	for g := 1; g <= 2; g++ {
		if z, ok := areaEntry(st, lighting.AreaPublic, g); !ok || z.Color != [3]int{0, 255, 0} {
			t.Errorf("zone public, groupe %d : vert : %+v (%v)", g, z, ok)
		}
	}
	if g := roomGroups(st); len(g) != 0 {
		t.Errorf("aucune écriture croisée : la zone ambiance n'est pas touchée : %+v", g)
	}
	if es := areaEntries(env.c.Scene(lighting.Event{Kind: lighting.KindRunning}), lighting.AreaPublic); len(es) != 0 {
		t.Errorf("hors du moment réglé, plus rien n'est émis pour la zone public (elle garde l'état de l'effet) : %+v", es)
	}
}

func TestEffects223_Public_IndependentOfOnOffFlash(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), pubGrp("P1")})
	env.writeUser(t, "pub.yaml", tw223Effect("pub-fx", "calm", "all", 1, "#00FF00"))
	env.setPublic("calm", "pub-fx")
	for _, mode := range []string{"OFF", "ON"} {
		if err := env.c.SetLightingMode(mode); err != nil {
			t.Fatal(err)
		}
		env.step()
		if z, ok := areaEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.AreaPublic, 0); !ok || z.Color != [3]int{0, 255, 0} {
			t.Errorf("mode %s : la zone public garde son effet : %+v (%v)", mode, z, ok)
		}
	}
	if err := env.c.SetLightingMode("AUTO"); err != nil {
		t.Fatal(err)
	}
	env.c.SetLightingFlash(true)
	t.Cleanup(func() { env.c.SetLightingFlash(false) })
	env.step()
	if z, ok := areaEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.AreaPublic, 0); !ok || z.Color != [3]int{0, 255, 0} {
		t.Errorf("Flash : la zone public garde son effet : %+v (%v)", z, ok)
	}
}

func TestEffects223_Public_PostAppliedAndHeldAtEndOfMoment_OnlyZoneWherePostIsVisible(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), pubGrp("P1")})
	env.writeUser(t, "pub.yaml", "id: pub-post\nnom: Pub\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [\"#00FF00\", 100] }\npost:\n  { g0: [\"#FF0000\", 100] }\n")
	env.setPublic("calm", "pub-post")
	env.step() // moment calm : l'effet joue (vert)
	if z, ok := areaEntry(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}), lighting.AreaPublic, 0); !ok || z.Color != [3]int{0, 255, 0} {
		t.Fatalf("pendant le moment : vert : %+v (%v)", z, ok)
	}
	env.eng.StartImmediate(20) // le moment calm se termine
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.now = env.now.Add(time.Second)
	env.step()
	if z, ok := areaEntry(env.c.Scene(lighting.Event{Kind: lighting.KindRunning}), lighting.AreaPublic, 0); !ok || z.Color != [3]int{255, 0, 0} {
		t.Errorf("fin du moment : POST (rouge) appliqué et tenu en zone public : %+v (%v)", z, ok)
	}
}

func TestEffects223_Public_NoPost_LampsKeepTheLastStateOfTheEffect(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), pubGrp("P1")})
	env.writeUser(t, "pub.yaml", tw223Effect("pub-fx", "calm", "all", 1, "#00FF00"))
	env.setPublic("calm", "pub-fx")
	env.step()
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.now = env.now.Add(time.Second)
	env.step()
	// sans POST, le serveur n'envoie plus rien : aucune entrée (les lampes gardent leur dernier état sur le pont)
	if es := areaEntries(env.c.Scene(lighting.Event{Kind: lighting.KindRunning}), lighting.AreaPublic); len(es) != 0 {
		t.Errorf("sans POST, rien n'est (re)envoyé à la zone public : %+v", es)
	}
}

func TestEffects223_Post_InAmbianceAndHostZones_NeverVisible_SceneRepaintsTheZone(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), hostGrp()})
	post := "id: avec-post\nnom: P\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [\"#00FF00\", 100] }\npost:\n  { g0: [\"#FF0000\", 100] }\n"
	env.writeUser(t, "post.yaml", post)
	env.setEffect("ambiance", "calm", "avec-post")
	env.setHost("calm", "avec-post")
	env.step()
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.now = env.now.Add(time.Second)
	env.step()
	st := env.c.Scene(lighting.Event{Kind: lighting.KindRunning})
	if st.Zones[0].Color == [3]int{255, 0, 0} && st.Zones[0].Area == lighting.AreaAmbiance && st.Zones[0].Group == 0 && st.Zones[0].Intensity == 255 {
		// la scène du chronomètre (pulsation) repeint la zone : le rouge de POST ne doit pas rester
		t.Errorf("POST n'est pas visible en zone ambiance : la scène suivante la repeint : %+v", st.Zones[0])
	}
	if z, _ := hostEntry(st); z.Color != [3]int{255, 255, 255} {
		t.Errorf("POST n'est pas visible en zone animateur : blanc fixe : %+v", z)
	}
}

// ---------- arrêt : le lecteur se tait avant l'extinction, aucune écriture fantôme ----------

func TestEffects223_Quiesce_ReaderStopsPublishing_NoGhostWrite(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{hostGrp(), grp(0, "A"), grp(1, "B")})
	env.writeUser(t, "calm.yaml", tw223Effect("calm-fx", "calm", "groups", 1, "#FF0000"))
	env.writeUser(t, "host.yaml", tw223Effect("host-fx", "calm", "all", 1, "#00FF00"))
	env.setEffect("ambiance", "calm", "calm-fx")
	env.setHost("calm", "host-fx")
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
	env.setEffect("ambiance", "calm", "calm-fx")
	env.c.LightingWriter.Store(nil) // éclairage non configuré / pilote absent (§5.3 point 1)
	env.step()
	if g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})); len(g) != 0 {
		t.Errorf("pilote absent : le lecteur ne publie rien : %+v", g)
	}
}
