// #290 (v11.6.0, LOT-4 Batch 2, révision 3) — fichiers de son PAR MANCHE dans le modèle : `Settings.SoundFiles` /
// `Partial.SoundFiles` (map cue → fichier), JSON `sound.files`, `Resolve` clé par clé, `Sources.sound.files[cue]` ∈
// {global, round} (jamais `direct`). Source : contracts/ambiance-settings.md §2 (ligne SoundFiles), §3, §4.2 point 4, §9.2.
// Écrit depuis le contrat avant le code (rouge : le champ SoundFiles n'existe pas encore). Préfixe tw290af.
package ambiancecfg

import (
	"encoding/json"
	"reflect"
	"testing"
)

func tw290afGlobal() Settings {
	g := tw290aGlobal()
	g.SoundFiles = map[string]string{"decompte": "decompte.wav"}
	for _, c := range tw290aCues {
		g.SoundFiles[c] = c + ".wav"
	}
	return g
}

func tw290afJSON(t *testing.T, v interface{}) map[string]interface{} { return tw290aJSON(t, v) }

func tw290afFiles(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	m := tw290afJSON(t, v)
	snd, _ := m["sound"].(map[string]interface{})
	files, _ := snd["files"].(map[string]interface{})
	return files
}

// ---------------------------------------------------------------------------
// Forme JSON : `sound.files`, forme complète côté Settings (15 cues + decompte), partielle côté Partial.
// ---------------------------------------------------------------------------

func TestFiles290_Settings_JSON_CompleteForm_FifteenCuesPlusCountdown(t *testing.T) {
	files := tw290afFiles(t, tw290afGlobal())
	if len(files) != len(tw290aCues)+1 {
		t.Fatalf("forme complète : %d entrées attendues (15 cues + decompte), got %d", len(tw290aCues)+1, len(files))
	}
	for _, c := range tw290aCues {
		if files[c] != c+".wav" {
			t.Errorf("sound.files.%s = %v, attendu %q (fichier du catalogue)", c, files[c], c+".wav")
		}
	}
	if files["decompte"] != "decompte.wav" {
		t.Errorf("sound.files.decompte = %v, attendu \"decompte.wav\"", files["decompte"])
	}
}

func TestFiles290_Settings_RoundTrip_KeepsFiles(t *testing.T) {
	g := tw290afGlobal()
	raw, _ := json.Marshal(g)
	var back Settings
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.SoundFiles, g.SoundFiles) {
		t.Errorf("aller-retour infidèle de SoundFiles : %v", back.SoundFiles)
	}
}

func TestFiles290_Partial_JSON_PartialMap_OmittedWhenEmpty(t *testing.T) {
	p := Partial{SoundFiles: map[string]string{"depart": "rounds/r2/depart.wav", "decompte": "rounds/r2/decompte.wav"}}
	files := tw290afFiles(t, p)
	if len(files) != 2 || files["depart"] != "rounds/r2/depart.wav" || files["decompte"] != "rounds/r2/decompte.wav" {
		t.Errorf("Partial.sound.files partiel attendu, got %v", files)
	}
	raw, _ := json.Marshal(Partial{SoundFiles: map[string]string{}})
	if string(raw) != "{}" {
		t.Errorf("une map de fichiers vide est omise (Partial vide ⇒ {}), got %s", raw)
	}
	raw, _ = json.Marshal(Partial{})
	if string(raw) != "{}" {
		t.Errorf("Partial vide ⇒ {}, got %s", raw)
	}
}

func TestFiles290_Partial_Unmarshal_ContractExample(t *testing.T) {
	const doc = `{"sound":{"muted":true,"files":{"depart":"rounds/r2/depart.wav"}},"light":{"mode":"ON"}}`
	var p Partial
	if err := json.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatal(err)
	}
	if p.SoundFiles["depart"] != "rounds/r2/depart.wav" || len(p.SoundFiles) != 1 {
		t.Errorf("SoundFiles attendu {depart}, got %v", p.SoundFiles)
	}
	if p.SoundMuted == nil || !*p.SoundMuted {
		t.Error("les autres champs restent lus")
	}
	raw, _ := json.Marshal(p)
	var back Partial
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, p) {
		t.Errorf("aller-retour infidèle : %+v / %v", back, err)
	}
}

func TestFiles290_Partial_FilesAreStrings(t *testing.T) {
	var p Partial
	if err := json.Unmarshal([]byte(`{"sound":{"files":{"depart":true}}}`), &p); err == nil {
		t.Error("une valeur de fichier non chaîne doit être refusée")
	}
}

// ---------------------------------------------------------------------------
// Resolve : clé par clé ; sources global / round.
// ---------------------------------------------------------------------------

func TestFiles290_Resolve_NoLayer_AllFilesGlobal(t *testing.T) {
	g := tw290afGlobal()
	got, src := Resolve(g)
	if !reflect.DeepEqual(got.SoundFiles, g.SoundFiles) {
		t.Errorf("sans couche : fichiers = global, got %v", got.SoundFiles)
	}
	srcFiles := tw290afFiles(t, src)
	for _, c := range append([]string{"decompte"}, tw290aCues...) {
		if srcFiles[c] != "global" {
			t.Errorf("source de sound.files.%s attendue \"global\", got %v", c, srcFiles[c])
		}
	}
}

func TestFiles290_Resolve_RoundOverridesOnlyItsFiles(t *testing.T) {
	g := tw290afGlobal()
	round := Partial{SoundFiles: map[string]string{"depart": "rounds/r2/depart.wav", "decompte": "rounds/r2/decompte.wav"}}
	got, src := Resolve(g, round)
	if got.SoundFiles["depart"] != "rounds/r2/depart.wav" || got.SoundFiles["decompte"] != "rounds/r2/decompte.wav" {
		t.Errorf("fichiers de manche attendus pour depart et decompte, got %v", got.SoundFiles)
	}
	for _, c := range tw290aCues {
		if c == "depart" {
			continue
		}
		if got.SoundFiles[c] != c+".wav" {
			t.Errorf("cue %s sans fichier de manche ⇒ fichier global, got %v", c, got.SoundFiles[c])
		}
	}
	srcFiles := tw290afFiles(t, src)
	if srcFiles["depart"] != "round" || srcFiles["decompte"] != "round" {
		t.Errorf("sources \"round\" attendues pour depart et decompte, got %v / %v", srcFiles["depart"], srcFiles["decompte"])
	}
	if srcFiles["gagne"] != "global" {
		t.Errorf("source \"global\" attendue pour gagne, got %v", srcFiles["gagne"])
	}
}

// Le fichier et l'activation sont INDÉPENDANTS : désactiver une cue ne change pas son fichier, et inversement.
func TestFiles290_Resolve_FileAndEnabledAreIndependent(t *testing.T) {
	g := tw290afGlobal()
	round := Partial{
		SoundFiles:  map[string]string{"reveal": "rounds/r2/reveal.wav"},
		CuesEnabled: map[string]bool{"depart": false},
	}
	got, src := Resolve(g, round)
	if got.CuesEnabled["depart"] || got.SoundFiles["depart"] != "depart.wav" {
		t.Errorf("depart coupée mais son fichier reste le global : enabled=%v file=%v", got.CuesEnabled["depart"], got.SoundFiles["depart"])
	}
	if !got.CuesEnabled["reveal"] || got.SoundFiles["reveal"] != "rounds/r2/reveal.wav" {
		t.Errorf("reveal active avec le fichier de manche : enabled=%v file=%v", got.CuesEnabled["reveal"], got.SoundFiles["reveal"])
	}
	sj := tw290afJSON(t, src)
	cues, _ := sj["sound"].(map[string]interface{})["cues"].(map[string]interface{})
	files := tw290afFiles(t, src)
	if cues["depart"] != "round" || files["depart"] != "global" || cues["reveal"] != "global" || files["reveal"] != "round" {
		t.Errorf("sources indépendantes attendues : cues.depart=round files.depart=global cues.reveal=global files.reveal=round, got %v / %v", cues, files)
	}
}

// Jamais `direct` : la couche directe ne porte aucun fichier (D1′ point 4), même si un Partial direct en contenait.
func TestFiles290_Resolve_SourceIsNeverDirect_ForFiles(t *testing.T) {
	g := tw290afGlobal()
	round := Partial{SoundFiles: map[string]string{"depart": "rounds/r2/depart.wav"}}
	direct := Partial{SoundMuted: tw290aB(true)} // un geste de régie sur un AUTRE champ
	got, src := Resolve(g, round, direct)
	if got.SoundFiles["depart"] != "rounds/r2/depart.wav" {
		t.Errorf("un geste direct sur un autre champ ne change aucun fichier, got %v", got.SoundFiles["depart"])
	}
	for name, s := range tw290afFiles(t, src) {
		if s == "direct" {
			t.Errorf("sound.files.%s : la source ne peut jamais être \"direct\"", name)
		}
	}
}

func TestFiles290_Resolve_PureNoMutation(t *testing.T) {
	g := tw290afGlobal()
	before := map[string]string{}
	for k, v := range g.SoundFiles {
		before[k] = v
	}
	round := Partial{SoundFiles: map[string]string{"depart": "rounds/r2/depart.wav"}}
	got, _ := Resolve(g, round)
	got.SoundFiles["gagne"] = "tampered.wav"
	if !reflect.DeepEqual(g.SoundFiles, before) {
		t.Error("Resolve ne doit pas partager la map SoundFiles du global avec le résultat")
	}
	if len(round.SoundFiles) != 1 || round.SoundFiles["depart"] != "rounds/r2/depart.wav" {
		t.Errorf("Resolve ne doit pas modifier la couche, got %v", round.SoundFiles)
	}
}

// Les 8 cues par type et le décompte sont des clés de première classe.
func TestFiles290_Resolve_TypedCuesAndCountdownAreOverridable(t *testing.T) {
	g := tw290afGlobal()
	round := Partial{SoundFiles: map[string]string{
		"qcm-indice": "rounds/r3/qcm-indice.wav", "memory-paire": "rounds/r3/memory-paire.wav", "rafale-fin-lot": "rounds/r3/rafale-fin-lot.wav",
		"decompte": "rounds/r3/decompte.wav",
	}}
	got, src := Resolve(g, round)
	srcFiles := tw290afFiles(t, src)
	for k := range round.SoundFiles {
		if got.SoundFiles[k] != round.SoundFiles[k] || srcFiles[k] != "round" {
			t.Errorf("%s : fichier et source de manche attendus, got %v / %v", k, got.SoundFiles[k], srcFiles[k])
		}
	}
}

// Non-régression : sans SoundFiles dans le global (ancien appelant), Resolve ne panique pas et les autres champs sont intacts.
func TestFiles290_Resolve_NilFilesMap_NoPanic(t *testing.T) {
	g := tw290aGlobal() // SoundFiles nil
	got, _ := Resolve(g, Partial{SoundMuted: tw290aB(true)})
	if !got.SoundMuted {
		t.Error("les autres champs sont résolus normalement")
	}
}
