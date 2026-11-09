# Contrat — Réglages d'ambiance mutualisés : global ← manche ← geste direct (v11.6.0, #290)

> **Normatif.** Introduit par le plan v11.6.0 (`_work/reports/planner-plan-v11.6.0.md`, Révision 2, GATE 2 du
> 2026-11-09 — décision utilisateur Q1 : « tous les réglages son + éclairage de la configuration globale peuvent être
> surchargés par la manche, il faut mutualiser ce paramétrage »). Hypothèses validées au GATE 2 : **D1′** (précédence),
> **D7** (périmètre), **D8** (mode d'éclairage persisté). Maquettes : `docs/mockup/v11.6.0/ui/ambiance_settings_panel__*.html`,
> `admin_questions_list__round_ambiance_override.html`, `admin_navbar__ambiance_popover_round_source.html`.
>
> Ce contrat **ne remplace** ni `sound.md` ni `lighting.md` : il définit **où** les réglages de comportement sont lus
> (résolus) et **comment** une manche ou un geste de régie les surcharge. Les clés de `config.json` existantes ne bougent pas.

---

## 1. Pourquoi un modèle unique

Avant v11.6.0, chaque réglage de comportement était lu directement dans `config.Get()` par le site qui en avait besoin
(`notifySound` : `Muted`, `Enabled`, `CuesDisabled` ; `question_sound.go` : `Muted` ; `countdown_sound.go` : `Muted`,
`CountdownDisabled`, `CountdownOnce` ; `internal/ambiance` : mode runtime ON/AUTO/OFF). Surcharger « par manche » en
ajoutant un `if round…` à chacun de ces sites multiplierait les règles de précédence et rendrait la prochaine couche
(scènes/effets déclaratifs, v11.7.0–v11.8.0) impossible à ajouter proprement.

Le modèle est donc **une structure**, **une fonction de résolution** et **un accesseur** :

```
Settings  = l'ensemble complet des réglages de comportement (toujours toutes les clés)
Partial   = une surcouche : chaque champ optionnel ; absent = « hérité »
Resolve(global Settings, layers ...Partial) → (Settings effectifs, Sources par champ)
```

Et **un seul composant React** (`AmbianceSettingsPanel`) pour éditer un `Settings` (mode global) ou un `Partial`
(mode surcharge).

---

## 2. Périmètre — ce qui est un réglage de comportement (D7)

| Champ (`Settings`) | JSON (manche et API) | Source globale (`config.json`) | Zéro / défaut | Surchargeable |
|---|---|---|---|---|
| `SoundMuted` | `sound.muted` | `sound.muted` (§11 `sound.md`) | `false` | ✅ |
| `CountdownEnabled` | `sound.countdown.enabled` | `!sound.countdown_disabled` | `true` | ✅ |
| `CountdownLoop` | `sound.countdown.loop` | `!sound.countdown_once` | `true` | ✅ |
| `CuesEnabled[cue]` | `sound.cues.<cue>` | `!sound.cues_disabled[cue]` — **toutes** les cues du registre (`sound.md` §2.1, cues par type comprises) | `true` | ✅ (par cue) |
| `LightMode` | `light.mode` | **`lighting.default_mode`** (nouveau, D8 ; absent ⇒ `"AUTO"`) | `"AUTO"` | ✅ |
| `SoundFiles[cue]` ✨ r3 | `sound.files.<cue>` (`cue` ∈ 15 cues ∪ `decompte`) | catalogue : `"<cue>.wav"` / `"decompte.wav"` (`data/files/sounds/`) | fichier du catalogue | ✅ (par cue, **fichier propre à la manche** — §9) |

**Hors du modèle — jamais surchargeable** (matériel, cycle de vie du processus, ou contenu du catalogue) :
`sound.enabled` (ouverture de la sortie, lié au redémarrage — `sound.md` §1/§11.1), `sound.device`,
`sound.ambiance_compensation_ms`, `lighting.enabled` (interrupteur général #286), `lighting.bridge_*`, `lighting.api_key`,
`lighting.lights[]` (ampoules, rôles, équipes), les **médias de question** (déjà par question). Le Flash (`lighting.md` §10.1)
est un geste ponctuel, pas un réglage : hors modèle.

> **Révision 3 (GATE LOT-4 refusé, 2026-11-09) — D7′.** La phrase « les fichiers des sons sont hors surcharge » de la
> révision 2 est **annulée** (malentendu au GATE 2) : les fichiers des **15 bruitages et du décompte** sont surchargeables
> **par manche**, via `SoundFiles` (§9). Le **catalogue global** reste le seul fichier hors manche.

Le JSON externe est **structuré** (`sound.countdown.enabled`) et **positif** (`enabled`, pas `disabled`) : c'est la forme
que le composant React et les manches manipulent. La conversion depuis/vers les clés « négatives » de `config.json`
(`cues_disabled`, `countdown_disabled`, `countdown_once`, `muted`) est faite **une seule fois**, dans `FromConfig` /
`ToConfigPatch` (§5). Aucune migration : `config.json` garde ses clés.

---

## 3. Formes JSON

### 3.1 `Settings` (complet)

```json
{
  "sound": {
    "muted": false,
    "countdown": { "enabled": true, "loop": true },
    "cues": { "depart": true, "temps-ecoule": true, "gagne": true, "perdu": true, "reveal": false,
              "entracte-debut": true, "entracte-fin": true,
              "qcm-indice": true, "memory-carte": true, "memory-paire": true,
              "memotion-selection": true, "memotion-fin-carte": true,
              "rafale-bonne": true, "rafale-mauvaise": true, "rafale-fin-lot": true }
  },
  "light": { "mode": "AUTO" }
}
```

> **r3** : `sound` gagne `"files": { "depart": "depart.wav", "…": "…", "decompte": "decompte.wav" }` — forme complète : toutes les
> cues + `decompte`, valeurs relatives à `data/files/sounds/`. En `Partial` : `"files": { "<cue>": "rounds/<roundID>/<cue>.wav" }`
> (partiel, seules les cues dont la manche a son fichier). `Sources.sound.files[cue]` ∈ `global | round` (jamais `direct`).

`cues` contient **toutes** les cues du registre, dans l'ordre normatif de `sound.md` §2.1 (global puis par type) — jamais
une clé manquante en forme complète.

### 3.2 `Partial` (surcouche — `Round.AMBIANCE`, couche directe)

Mêmes clés, **toutes optionnelles** ; un champ absent = hérité. `cues` est une map **partielle** (seules les cues
surchargées y figurent). `{}` = tout hérité. Exemple (manche « Blind-test ») :

```json
{ "sound": { "countdown": { "enabled": false }, "cues": { "qcm-indice": false } }, "light": { "mode": "ON" } }
```

Validation (serveur, à l'écriture — `ROUNDS_UPDATE`, `POST /config.json`) : `light.mode ∈ {"ON","AUTO","OFF"}` ; clés de
`cues` ∈ registre `audio.Cue` ; types booléens stricts. Toute clé inconnue ⇒ refus (`INVALID_AMBIANCE` pour `ROUNDS_UPDATE`,
`400` pour `POST /config.json`) — jamais ignorée en silence (une faute de frappe serait sinon une surcharge invisible).

### 3.3 `Sources`

Pour chaque champ résolu : `"global" | "round" | "direct"`. Même structure que `Settings`, valeurs = source :

```json
{ "sound": { "muted": "direct", "countdown": { "enabled": "round", "loop": "global" }, "cues": { "depart": "global", "qcm-indice": "round", "…": "global" } }, "light": { "mode": "round" } }
```

---

## 4. Résolution et précédence — normatif (D1′)

Trois couches, du plus général au plus précis : **global** (persisté, `config.json`) ← **manche** (persisté,
`rounds.json`, `Round.AMBIANCE`) ← **direct** (runtime, gestes de régie). `Resolve` est une fonction **pure** :
pour chaque champ, la **dernière couche qui le définit** gagne ; la source est celle de cette couche.

### 4.1 Couche manche — cycle de vie

- **Manche effective** = `ROUND_ID` de la question courante (`""` / inconnu = « Sans manche » = aucune couche manche).
- La couche manche est **recalculée quand la manche effective change** : au `READY` d'une question dont `ROUND_ID`
  diffère de la manche effective précédente, au `NEW_GAME` (manche effective ← aucune), au `RESTORE` (manche de la
  question restaurée). Pas au START, pas à la question suivante de la même manche.
- Une **modification de `Round.AMBIANCE` par `ROUNDS_UPDATE` pendant la manche** s'applique **immédiatement** (la
  couche manche est relue après application de l'update) — l'éditeur n'a pas à attendre la manche suivante.
- Les questions « Sans manche » et les ENTRACTE hors manche : couche manche vide. Une ENTRACTE **dans** une manche
  (entractes automatiques #245) suit la manche.

### 4.2 Couche directe — gestes de régie

Gestes directs (tous existants, aucun nouveau canal) : `POST /config.json {"sound":{"muted"}}` (popover « Son »),
`POST /api/lighting/mode` (popover « Éclairage général »), interrupteurs par cue / décompte / « une fois-en boucle » de
la page Ambiance (`POST /config.json {"sound":{"cues_disabled"|"countdown_disabled"|"countdown_once"}}`), et
`POST /config.json {"lighting":{"default_mode"}}` (nouveau, page Ambiance › Lumière).

Règle **champ par champ**, à chaque geste direct sur un champ F :
1. la couche directe reçoit F (F devient `"direct"`) ;
2. **si la manche effective ne surcharge pas F**, la valeur est **aussi écrite dans le global** (persistée) — c'est le
   comportement v11.4.0 (« son coupé persisté », #286) **conservé hors surcharge** ;
3. si la manche effective surcharge F, le global **n'est pas touché** : le geste vaut pour la manche en cours seulement.
4. **Fichiers (r3)** : la couche directe **ne porte jamais** `sound.files` — un geste de régie ne change pas de fichier ; un
   fichier de manche ne réécrit jamais le catalogue ; remplacer un son du catalogue ne touche aucun fichier de manche.
5. Exception matérielle : `POST /api/lighting/mode` n'écrit **jamais** le global (`default_mode`) — le mode du popover
   reste un geste de conduite (lighting.md §10.1, inchangé) ; seul `POST /config.json {"lighting":{"default_mode"}}`
   écrit le réglage persisté.

La couche directe est **vidée** quand la manche effective change (4.1) et au `NEW_GAME`. Elle **n'est pas persistée**
(un redémarrage repart de global ← manche de la question restaurée).

### 4.3 Application des réglages résolus

- **Son** : lecture **paresseuse** — `notifySound`, `questionSoundAdapter`, `countdownSoundAdapter` lisent l'accesseur
  résolu **au moment de jouer** (§5.2). Un changement de résolution **n'arrête pas** une lecture en cours **sauf** pour
  `SoundMuted` passant à `true`, qui garde ses effets de bord v11.4.0 (`sound.md` §11.2 : arrêt du média et du décompte)
  — déclenchés par la **résolution** (geste direct, entrée de manche), plus seulement par l'écriture de `config.json`.
- **Lumière** : `LightMode` résolu est **poussé** à `ambiance.Controller.SetLightingMode` à chaque changement de
  résolution (même chemin que le popover aujourd'hui). `AUTO` = dérivation automatique (lighting.md §6.2).
- **Diffusion** : `GameState.AMBIANCE_EFFECTIVE = { SETTINGS, SOURCES, ROUND_ID }` (`game-state.md`), recalculé et
  diffusé à **chaque** changement de résolution (geste, manche, `ROUNDS_UPDATE`, `POST /config.json`), toujours sérialisé.

### 4.4 Ce que la résolution ne fait pas

- Elle ne lit jamais `sound.enabled` ni `lighting.enabled` : ces gardes matérielles restent **en amont**, telles quelles
  (`notifySound` : `if !sc.Enabled return` ; `ambiance` : interrupteur général #286). Son coupé par la manche **et**
  sortie indisponible : la pastille d'en-tête dit « Son inactif » (matériel) — la source « manche » ne s'affiche que sur un
  réglage résolu.
- Elle n'invente pas d'état pour une cue inconnue : une clé hors registre est refusée à l'écriture (§3.2).

---

## 5. Implémentation imposée

### 5.1 Package `internal/ambiancecfg` (nouveau)

```go
package ambiancecfg // n'importe ni internal/config, ni internal/game, ni internal/audio : testable seul

type LightMode string // "ON" | "AUTO" | "OFF"

type Settings struct {
    SoundMuted       bool
    CountdownEnabled bool
    CountdownLoop    bool
    CuesEnabled      map[string]bool // clé = nom de cue ; forme complète : toutes les cues du registre
    LightMode        LightMode
}

type Partial struct {
    SoundMuted       *bool
    CountdownEnabled *bool
    CountdownLoop    *bool
    CuesEnabled      map[string]bool // partielle : seules les cues surchargées
    LightMode        *LightMode
}

type Source string // "global" | "round" | "direct"
type Sources struct { SoundMuted, CountdownEnabled, CountdownLoop, LightMode Source; CuesEnabled map[string]Source }

// Resolve : pure, déterministe ; layers dans l'ordre manche puis direct.
func Resolve(global Settings, layers ...Partial) (Settings, Sources)
// Validate : light.mode, clés de cues contre `known` (registre fourni par l'appelant), types.
func (p Partial) Validate(known func(cue string) bool) error
// JSON : Settings et Partial (dés)sérialisent la forme §3 (structurée, positive) ; Partial omet les champs nil ;
// `{}` ⇔ Partial vide. MarshalJSON de Partial ne produit jamais `null` pour une map vide : la clé `cues` est omise.
```

L'adaptateur `FromConfig(cfg config.Config, cues []string) Settings` et `ToConfigPatch(field, value)` vivent dans
`cmd/server` (ou `internal/server`), **pas** dans `ambiancecfg`.

### 5.2 Accesseur unique côté serveur

```go
// cmd/server/round_ambiance.go
func (a *App) ambianceSettings() ambiancecfg.Settings // résolu, lecture sous RLock, jamais en cache côté appelant
```

**Interdit après LOT-4** (test-garde AST, même patron que `sound.md` §6.2) : toute lecture de
`config.Get().Sound.Muted`, `.CuesDisabled`, `.CountdownDisabled`, `.CountdownOnce` et de `Lighting.DefaultMode`
hors de l'adaptateur `FromConfig`. `notifySound` devient :

```go
func (a *App) notifySound(c audio.Cue) {
    if !config.Get().Sound.Enabled { return }          // garde matérielle, inchangée (§4.4)
    s := a.ambianceSettings()
    if s.SoundMuted || !s.CuesEnabled[string(c)] { return }
    a.sound().PlayCue(c)
}
```

`CuesEnabled` d'une cue absente de la map (forme complète : impossible ; défensif) vaut **`true`** — la valeur zéro
reste « rien n'est coupé » (discipline `sound.md` §6.3). `POST /api/sounds/{cue}/test` et `/decompte/test` ignorent
toujours le modèle (geste explicite).

### 5.3 Persistance

- **Global** : clés `config.json` existantes (`sound.muted`, `sound.cues_disabled`, `sound.countdown_disabled`,
  `sound.countdown_once`) + **`lighting.default_mode`** (`string`, `omitempty`, absent ⇒ `AUTO`, D8). Écriture par
  `POST /config.json` (fusion champ par champ existante, `handleConfig`) — `default_mode` validé `∈ {ON,AUTO,OFF}`.
- **Manche** : `Round.AMBIANCE` (`Partial`, `models.md` §Manches) dans `rounds.json`, écrit par `ROUNDS_UPDATE`
  (`websocket-actions.md`). **Toujours sérialisé** dans `GameState.ROUNDS[]` (`{}` si hérité — règle « pas d'omitempty »
  dans `GameState`) ; dans `rounds.json` il peut être absent (lecture : absent ⇒ `{}`).
- **Direct** : mémoire du processus uniquement.

### 5.4 API de lecture

`GET /api/ambiance/settings` → `{ "global": Settings, "effective": Settings, "sources": Sources, "round_id": string,
"round_override": Partial }` (`http-endpoints.md` §Ambiance). Sert au composant en mode **surcharge** (`base` = `global`)
et au popover (fallback si `GameState` n'est pas encore reçu).

---

## 6. Composant React mutualisé — `AmbianceSettingsPanel`

Un seul composant, **deux modes** (maquettes `ambiance_settings_panel__global_per_type_tabs` et `__round_override`) :

| Prop | Type | Rôle |
|---|---|---|
| `mode` | `'global' \| 'override'` | Rendu des contrôles : bi/tri-états en global, **tri-états « Hérité / … »** en override |
| `value` | `Settings` (global) / `Partial` (override) | Valeur éditée |
| `base` | `Settings` | Valeurs globales affichées en italique sous « Hérité » (override uniquement) |
| `onChange(path, value \| null)` | fn | `path` = chemin JSON (`sound.cues.qcm-indice`, `light.mode`) ; `null` = retour à « Hérité » (override) |
| `cues` | `[{cue, type, label, sub}]` | Registre affiché, groupé en onglets **Global / QCM / MEMORY / MEMOTION / RAFALE** (`sound.md` §2.1) |
| `renderCatalogue(cue)` | fn → ReactNode \| undefined | **Mode global seulement** : colonnes fichier / durée / Remplacer / Restaurer / Tester, fournies par `SoundsManager` ; absentes en override (D7) |
| `sections` | `['sound','countdown','cues','light']` | Sections rendues (LOT-3 : sans `light` tant que `default_mode` n'existe pas côté serveur ; LOT-4 : toutes) |
| `sources` | `Sources` (optionnel) | Marqueurs « manche » / « direct » (popover, lecture seule) |

Invariants : un onglet dont au moins une cue est surchargée porte un point « • » ; le compteur de l'en-tête de la
fenêtre de manche = nombre de champs définis dans `value` (cues comptées une à une) ; « Tout remettre à Hérité » ⇒
`onChange('*', null)`. **Test de parité** exigé : la liste des chemins éditables du composant = la liste des clés de
`Settings` (§2) — un champ ajouté d'un côté sans l'autre fait échouer le test.

Hôtes : `SoundsManager` (page Ambiance › Son, mode global, `renderCatalogue` fourni), `AmbiancePage` › Lumière (mode global,
`sections=['light']`), `RoundAmbianceDialog` (mode override, ouvert depuis `RoundGroup` 🎛), `AmbiancePopoverPanel`
(lecture des `sources` pour les étiquettes « manche » / « direct » — il **n'embarque pas** le panel, il n'en lit que le
modèle).

---

## 7. Extensibilité (v11.7.0 / v11.8.0)

Un futur réglage déclaratif (scène ou effet par événement, `light.scenes…`) s'ajoute comme **un champ de `Settings`**
(+ son pointeur dans `Partial`, + une ligne au §2, + une section du composant). `Resolve`, les couches, `Round.AMBIANCE`,
`AMBIANCE_EFFECTIVE` et la fenêtre de manche n'ont pas à changer de forme. C'est la raison d'être de la mutualisation.

## 8. Non-régression exigée

- `config.json` v11.5.0 (sans `default_mode`, avec `cues_disabled`/`muted`/`countdown_*`) ⇒ `FromConfig` reproduit
  **exactement** le comportement actuel ; `rounds.json` v11.2 (sans `AMBIANCE`) ⇒ couche manche vide partout.
- Tests v11.4.0 (`*_286_test`, `*_287_test`, `*_288_test`, `*_230_test`) verts **après** passage à l'accesseur.
- Hors toute surcharge de manche, un geste du popover a **le même effet persisté** qu'en v11.4.0 (§4.2 point 2).

---

## 9. Fichiers de son par manche — normatif (révision 3, #290, LOT-4 Batch 2)

> Décision utilisateur du 2026-11-09 (GATE LOT-4 refusé) : « je dois aussi pouvoir assigner un autre fichier de son ».
> Périmètre : les **15 bruitages** et le **son du décompte**. Maquette `ambiance_settings_panel__round_override_files.html`
> (remplace `…__round_override`). Plan : `_work/reports/planner-plan-v11.6.0-r3-lot4.md` (décisions C1–C10, D10–D13).

### 9.1 Stockage
`data/files/sounds/rounds/<roundID>/<cue>.wav` — `cue` ∈ `audio.AllCues()` ∪ `decompte`, nom **déterministe** (un fichier par
(manche, cue) ; remplacement = écriture temporaire + `rename`, jamais de suffixe aléatoire). `roundID` n'est jamais réutilisé
(`models.md` §Manches, `NEXT_ID`). Le dossier des sons étant couvert par la sauvegarde `medias` (`files/sounds`, récursif) et par
la remise à zéro « médias » (`RemoveAll` + régénération du catalogue), les fichiers de manche suivent **le même sort** sans code
dédié. Ils **ne figurent pas** dans `sounds.json` (manifeste du catalogue) : un fichier de manche est toujours « personnalisé ».

### 9.2 Modèle
`Round.AMBIANCE.sound.files[cue] = "rounds/<roundID>/<cue>.wav"` (chemin relatif au dossier des sons) — **la vérité est
`rounds.json`**, jamais l'existence du fichier. Validation stricte à l'écriture : clé ∈ cues ∪ `decompte`, valeur
`^rounds/<ce roundID>/<clé>\.wav$` (aucune traversée, aucune autre manche). `Settings.SoundFiles` (global) = `"<cue>.wav"` pour
toutes les cues et `"decompte.wav"`. `Resolve` fusionne clé par clé ; `Sources.sound.files[cue]` ∈ `global | round`.
**`ROUNDS_UPDATE` ignore `sound.files`** (toujours recopié depuis l'existant) : seule l'API §9.3 l'écrit.

### 9.3 API — `http-endpoints.md` §Rounds sounds
`GET /api/rounds/{id}/sounds` → `{ round_id, files: [{cue, path, present, duration_seconds|null}] }` ;
`POST /api/rounds/{id}/sounds/{cue}` (multipart `file`) → **immédiat** (D10) : `ToCanonical` (#232) → validateur **du catalogue**
pour cette cue (`ValidateUpload` ≤ 5 s / décompte ≤ 30 s, plafonds et avertissement identiques) → écriture dans
`sounds/rounds/<id>/` → `rounds.json` sous le **verrou des fichiers de questions** (celui de `ROUNDS_UPDATE`) → `refreshAmbiance()`
+ `UPDATE` ; `DELETE /api/rounds/{id}/sounds/{cue}` (« Revenir au fichier global ») : fichier supprimé, clé retirée, même chemin ;
`POST /api/rounds/{id}/sounds/{cue}/test` : joue le fichier **effectif** de la ligne (manche si présent, sinon catalogue), en
ignorant `muted` / interrupteurs (geste explicite, comme `/api/sounds/{cue}/test`). `{id}` validé contre `rounds.json`,
`{cue}` contre `audio.AllCues()` ∪ `decompte`, avant tout usage.

### 9.4 Lecture et repli
`audio.RelBank` (interface **optionnelle**, type-assertée sur la banque — revue LOT-4b m2, `sound.md` §15) porte `PCMAt(rel string) (pcm []byte, ok bool)` ; `audio.Engine` gagne `PlayCueAt(c Cue, rel string)` (file
`{cue, rel}`, garanties §5 de `sound.md` inchangées ; `PlayCue(c)` ≡ `PlayCueAt(c, "<cue>.wav")`). `FileBank.PCMAt` lit
`<dir>/<rel>` ; fichier **absent ou non conforme ⇒ repli sur `<cue>.wav`** avec un `WARN` (dédoublonné par `rel` jusqu'au prochain
succès). `notifySound` : `a.sound().PlayCueAt(c, s.SoundFiles[string(c)])` ; décompte : `filePath()` = `soundsDir/<s.SoundFiles["decompte"]>`
avec le même repli. **Aucune réconciliation automatique** de `rounds.json` (D12) : une référence orpheline est signalée (`present:false`),
jamais corrigée en silence.

### 9.5 Nettoyage
Suppression d'une manche (`DELETED_ROUNDS`, `KEEP` ou `DELETE`) ⇒ `RemoveAll(sounds/rounds/<rid>)` **après** l'écriture réussie de
`rounds.json` ; « Tout remettre à Hérité » ⇒ `DELETE` de chaque fichier de la manche, avec confirmation nommant leur nombre (D11).
`POST /api/sounds/{cue}/restore` et `restore-defaults` **ne touchent pas** les fichiers de manche.

### 9.6 Composant (`AmbianceSettingsPanel`, mode override)
Colonne **Fichier** sur chaque bruitage et sur le décompte, rendue par le slot `renderFile(cue)` (symétrique de `renderCatalogue`,
fourni par `RoundAmbianceDialog`) : hérité = nom global en italique (`défaut`/`personnalisé`, durée) + Remplacer… / Tester ;
fichier de manche = nom + durée + badge « manche » + Remplacer… / Revenir au fichier global / Tester / Écouter ici
(`/files/sounds/rounds/<rid>/<cue>.wav?v=<mtime>`) ; manquant = signalé. Activation (tri-état) et fichier sont **indépendants**.
Remplacer / Revenir sont immédiats, Enregistrer / Annuler ne portent que sur les tri-états et le mode. Le test de parité des
chemins (§6) couvre `sound.files.*`.

