# Contrat — Bruitage d'événement : vocabulaire, moteur abstrait, câblage

> **Issue** : #227 (milestone v11.0 — Ambiance de musique d'événement, #34)
> **Spike de faisabilité** : #226 — verdict `_work/reports/spike-226-20260921-103221.md`,
> programme de démonstration `spike/audio/` (branche `spike/226-audio-bluetooth`)
> **Cadrage** : `_work/reports/plan-20260921-095033.md`
> **Plan de dev** : `_work/reports/plan-dev-227-20260921-102500.md`
> **Consommateurs** : #228 (pilote audio réel, `oto`), #229 (sons livrés), #230 (interface
> d'administration + endpoints d'upload), #231 (v11.1 — bruitages fins par type de question)
>
> Ce contrat est **normatif**. Il est écrit avant le code, comme l'exige la démarche du projet.
> `dev-backend` peut l'ajuster si une contrainte technique l'impose — en documentant la raison,
> conformément à `contracts/README.md` — mais pas par confort d'implémentation.

---

## 1. Pourquoi ce contrat, et ce qu'il évite

Le spike #226 a établi une voie technique prouvée (`ebitengine/oto/v3` v3.5.1, `go 1.25.0`,
`purego` + `jfreymuth/pulse`, aucun `cgo`) et deux contraintes structurantes qui doivent être
gravées **avant** tout code, sous peine de découvrir le défaut à l'exécution, sur un son précis,
en conditions réelles :

1. **Un seul contexte audio par processus** (§3 du verdict du spike) — une seule fréquence
   d'échantillonnage et un seul nombre de canaux pour **tous** les sons de l'application. Le
   format doit donc être **canonique et imposé**, pas négocié fichier par fichier.
2. **Aucun bus d'événements n'existe dans le projet** (même constat que `contracts/lighting.md`
   §1/§4.1, valable ici avec **encore plus de force**) : contrairement à la lumière, qui peut se
   contenter de re-dériver un état, **le son ne peut jamais se contenter de cela** — un bruitage
   manqué ne se rattrape pas en re-dérivant l'état vivant une seconde plus tard, l'instant est
   déjà passé.

Le second point est la raison pour laquelle ce contrat introduit un vocabulaire (`audio.Cue`)
**distinct** de celui de la lumière (`lighting.EventKind`), même si les deux partagent la même
source d'événements de jeu.

---

## 2. Vocabulaire de cues — normatif

### 2.1 Le type

Package `internal/audio`. Aucun import de `internal/game` ni de `internal/lighting` : ce package
doit rester testable seul, même exigence que `internal/lighting` (§2.1 de `lighting.md`).

```go
type Cue string

const (
    CueDepart        Cue = "depart"         // entrée en STARTED
    CueTempsEcoule   Cue = "temps-ecoule"   // expiration du chrono global (distinct d'un STOP régie)
    CueGagne         Cue = "gagne"          // points crédités
    CuePerdu         Cue = "perdu"          // paire ratée MEMORY, invalidation/timeout RAFALE
    CueReveal        Cue = "reveal"         // entrée en REVEALED
    CueEntracteDebut Cue = "entracte-debut" // entrée en entracte
    CueEntracteFin   Cue = "entracte-fin"   // sortie d'entracte
)
```

Cette liste est **fermée pour v11.0**. Un besoin non couvert se traite en ajoutant une cue au
contrat, jamais en surchargeant la sémantique d'une cue existante. Les bruitages fins par type de
question (indice QCM, sélection de carte MEMORY/MEMOTION, etc. — catalogue étendu proposé au
cadrage §C) appartiennent à v11.1 (#231), hors périmètre ici.

### 2.2 Une seule nature : l'impulsion — et pourquoi il n'y en a pas d'autre

`contracts/lighting.md` §2.3 distingue deux natures (scène d'état, dérivable ; impulsion, non
dérivable). **Le son n'a qu'une seule nature : toutes les cues sont des impulsions.** Un son est
un instant, jamais un état — il n'existe aucune notion de « bruitage en cours » comparable à une
scène lumineuse qui reste affichée. C'est la distinction *load-bearing* de ce contrat : le moteur
décrit au §4 ne dérive jamais rien, il **rejoue exactement ce qu'on lui a demandé de jouer, une
fois**.

### 2.3 Pas de champ `Teams`/`Points`

`lighting.Event` porte `Teams` et `Points` parce que la scène ou l'impulsion lumineuse en a besoin
pour choisir une couleur. **Aucune cue sonore de v11.0 ne varie selon l'équipe ou le score** — le
catalogue de sons livrés (#229) est un son par cue, point. Si un futur besoin (ex. graduer un son
de victoire selon les points, à la manière du clignotement SCORE) l'exigeait, ce serait un
amendement explicite de ce contrat, pas une extension silencieuse de `Cue`.

---

## 3. Format audio canonique — normatif

**WAV PCM 16 bits, 44 100 Hz, stéréo.** Confirmé contre le format que le spike a effectivement
fait sonner (`spike/audio/tone.go` : `sampleRate = 44100`, `channelCount = 2`,
`oto.FormatSignedInt16LE`) — ce n'est pas un choix arbitraire fait à froid, c'est celui déjà
vérifié par compilation/exécution sur les deux cibles CI.

**Conséquences normatives, en cascade depuis la contrainte « un seul contexte » du §1 :**

- Tout fichier son (livré en #229, ou téléversé en #230) **doit** se conformer à ce format
  exactement. Un fichier non conforme est **refusé à l'upload** (§7, `contracts/http-endpoints.md`)
  — jamais accepté puis rééchantillonné à la volée, jamais accepté puis silencieusement injouable.
- **Aucun format de compression n'est accepté** (`.mp3` en particulier) — décision utilisateur
  déjà actée au cadrage : le décodeur MP3 Go de référence est annoncé non maintenu par son auteur,
  et l'ajouter contredirait la même prudence de dépendances qui a établi `oto` comme seule voie
  viable (spike §2.1).
- La constante de format est déclarée **une seule fois**, dans `internal/audio`, et réutilisée
  partout où une vérification de conformité est nécessaire (allowlist d'upload, extraction des
  sons livrés) — jamais recopiée.

---

## 4. Interface du pilote — normatif

```go
// Output joue un flux PCM déjà décodé et déjà conforme au format canonique
// (§3) sur du matériel réel. Play est appelé UNIQUEMENT depuis la
// goroutine unique de lecture du moteur (§5) : il n'a PAS besoin d'être
// sûr en accès concurrent. Même contrat que lighting.Driver.Apply
// (contracts/lighting.md §3) — délibérément symétrique.
//
// Play DOIT bloquer jusqu'à la fin RÉELLE du rendu — jusqu'à ce que le son
// ait effectivement fini de jouer sur le matériel, ou que ctx soit annulé.
// Amendement normatif du 2026-09-21 (#228) : voir la note ci-dessous, ce
// n'est pas une reformulation cosmétique.
type Output interface {
    Play(ctx context.Context, pcm io.Reader) error

    // Close libère les ressources. Idempotent, appelable même si Play n'a
    // jamais réussi.
    Close() error
}
```

**Ce que cette interface exclut délibérément**, pour que #228 n'ait aucune ambiguïté :

- **Aucune notion de sélection de périphérique** dans cette interface — c'est une décision du
  pilote concret, pas du moteur (§6.2 ci-dessous, note de traçabilité systemd).
- **Aucun format négociable** — `pcm` est toujours au format canonique du §3 ; un `Output` n'a
  jamais à inspecter ni convertir quoi que ce soit.

> **Amendement normatif (2026-09-21, #228) — `Play` DOIT bloquer, ce n'est plus une option.**
> La version précédente de ce contrat disait « aucune garantie de non-blocage côté pilote... un
> pilote réel PEUT légitimement attendre la fin de la lecture ». C'était un **trou de contrat** :
> rien n'empêchait une implémentation de `Play` qui rend la main dès que les octets sont confiés
> au lecteur — **exactement le bug rencontré et corrigé pendant le spike** (`oto.Player.Play()` est
> asynchrone par nature ; un `Play` qui retourne sitôt l'appel passé ferme le flux avant qu'un seul
> octet n'ait été lu). Cette implémentation aurait été **parfaitement conforme** à l'ancienne
> formulation du contrat, et aucun test de #227 ne l'aurait vue : `FakeOutput` répond
> instantanément par défaut.
>
> **Exigence, sans ambiguïté** : un pilote réel (#228) doit attendre `IsPlaying() == false` (ou
> l'équivalent de la bibliothèque retenue) avant de retourner de `Play` — jamais réussir puis
> rendre la main immédiatement. L'attente doit rester sensible à l'annulation de `ctx`, pour que
> l'arrêt du serveur ne soit jamais retardé par un son en cours. `FakeOutput` (`Delay`/`Gate`)
> permet de tester cette exigence sans matériel — voir `internal/audio/engine_test.go`.
>
> **Conséquence de second ordre, à connaître** : le moteur lit **strictement séquentiellement**
> (§5.2/§5.3) et `Play` bloque désormais pour de vrai — une rafale de cues **s'étale** dans le
> temps au lieu de se superposer. C'est une contrainte d'architecture qui pèse sur le choix des
> sons (#229 : sons courts, cible < 1 s), pas un défaut de ce contrat.

> **Amendement normatif (2026-09-21, #230) — un accesseur exporté doit signaler un `Output`
> neutre.** `NewOutput` (#228) ne renvoie jamais d'erreur et ne remonte **aucune information
> d'état** : une dégradation à la construction (contexte refusé, jamais prêt, matériel absent)
> produit un `noopOutput` **indiscernable de l'extérieur** d'un pilote réel qui fonctionne — les
> deux satisfont l'interface `Output` à l'identique. `GET /api/sound/status`
> (`contracts/http-endpoints.md` §Sound) a besoin de distinguer les deux cas pour établir son
> état, et ne peut pas le faire aujourd'hui : `Engine.Enabled()` rend `true` dès qu'un `Output`
> est attaché, neutre ou non, et `Stats.PlayErrors` ne comble pas ce trou — un `noopOutput` ne
> produit jamais d'erreur non plus (§5.5, dégradation silencieuse jusqu'au bout).
>
> **Exigence pour le Lot B de #230** : `internal/audio` expose une fonction — par exemple
> `IsNeutral(o Output) bool` — vraie pour le `noopOutput` retourné par `NewOutput` en cas de
> dégradation (`output.go`/`output_other.go`) et pour tout `Output` nil, fausse pour un pilote réel
> attaché. **Purement additif** : aucune logique de construction, de lecture ou de dégradation
> existante n'est modifiée, seule une lecture d'état est ajoutée.

---

## 5. Le moteur — normatif, forme imposée

Calqué sur `internal/lighting.Writer`, qui a fait ses preuves, **et** sur le prototype exécuté du
spike (`spike/audio/robustness.go`, scénario B) qui a mesuré la propriété clé : une file bornée à
goroutine unique absorbe un périphérique bloqué indéfiniment sans jamais faire attendre
l'appelant (latence d'appel maximale mesurée : 17,85 µs contre un périphérique délibérément gelé).

### 5.1 Entrée — jamais bloquante

```go
// PlayCue signale une cue à jouer. NE BLOQUE JAMAIS — appelée depuis la
// goroutine de dispatch du jeu, elle ne doit jamais attendre ni le moteur
// ni le périphérique.
func (e *Engine) PlayCue(c Cue) (accepted bool) {
    select {
    case e.queue <- c:
        return true
    default:
        return false // file saturée : rejetée et comptée, jamais attendue
    }
}
```

- `select { case ch <- v: default: }` est la **seule** forme d'envoi autorisée. Un envoi
  bloquant, même « qui ne bloquera jamais en pratique », est un refus de revue — identique à la
  règle de `lighting.md` §4.3.
- `e == nil` et son moteur désactivé rendent tous les sites d'appel inconditionnels — **aucun
  `if` autour d'un `PlayCue` dans `main.go`**, même principe que `lighting.md` §4.3 : c'est ce qui
  garde le futur registre de sites (§6) lisible et vérifiable.

### 5.2 File bornée, jamais de coalescence « dernier gagnant »

**Différence délibérée avec `lighting.Writer`.** L'écrivain lumière coalesce (§4.1/§4.2 de
`lighting.md`) parce qu'une scène ou une impulsion en remplace une autre sans perte de sens — la
salle rattrape toujours l'état courant. **Le son ne coalesce jamais** : deux cues différentes ne
sont pas interchangeables, et une file à une place avec « dernier gagnant » ferait disparaître un
`perdu` suivi de près par un `gagne` (ou l'inverse) sans que cela soit jamais voulu.

- File **FIFO bornée** (capacité à définir par `dev-backend` en Lot B, en fonction de la mesure de
  rafale du spike, §2.6 du verdict).
- **Dépassement journalisé et compté**, jamais d'attente, jamais d'agrandissement dynamique.
- L'ordre d'émission est préservé pour les cues acceptées ; seules celles arrivées quand la file
  est pleine sont perdues.

### 5.3 Une seule goroutine lit le périphérique

Même contrat que `lighting.md` §3 pour `Driver.Apply` : `Output.Play` n'est appelé **que** depuis
cette goroutine, ce qui lui donne le droit de bloquer sans aucune exigence de sûreté concurrente.

### 5.4 Détection de front, sous verrou propre au moteur

Contrainte documentée (`internal/game/engine.go:220-241`, citée à l'identique dans
`lighting.md` §5) : les callbacks de l'Engine sont invoqués **hors lock**, depuis une douzaine de
sites, sans aucune sérialisation — deux peuvent s'exécuter en même temps sur des goroutines
différentes. C'est la cause racine du bug #121, et elle s'applique ici comme à la lumière.

Toute cue qui serait un jour dérivée d'un changement de phase plutôt que d'un site d'appel direct
devra détecter ce changement sous un verrou **propre au moteur audio** (jamais partagé avec
l'écrivain lumière) — capacité que l'implémentation doit offrir, même si aucune cue de v11.0 n'en
a besoin aujourd'hui : les sept sites du §6.2 sont tous des **appels directs** insérés au point
exact de l'événement (ex. la cue `temps-ecoule` s'insère directement sur la branche
`result.currentTime <= 0` de `internal/game/engine.go` avant l'appel à `e.Stop()` — un site qui ne
se déclenche naturellement qu'une fois par expiration de chrono, sans polling ni re-dérivation
nécessaire).

### 5.5 Garanties de fait, reprises telles quelles de `lighting.md` §4.5 et #205

- **Pas de son parasite au démarrage** : le premier front détecté après le lancement du serveur
  ne doit rien jouer — applicable à toute cue dérivée (§5.4), sans objet pour les sites d'appel
  direct.
- **Son désactivé ⇒ aucune goroutine lancée.** Pas une goroutine qui tourne à vide : aucune.
- **Aucune goroutine, aucun appel matériel, aucune ligne de log** tant que le son n'est pas
  configuré — comportement observable par défaut strictement identique à aujourd'hui.

### 5.6 Cycle de vie

Calqué sur `AckManager`, comme l'écrivain lumière :

- construction dans `(*App).setup()` ;
- `go e.Start(a.ctx)` dans `(*App).start()` ;
- arrêt par `a.cancelCtx()` dans `(*App).stop()`.

---

## 6. Le fan-out et la frontière des sites — normatif, piège majeur

### 6.1 Ce que le fan-out remplace, et ce qu'il ne remplace jamais

Le plan de cadrage envisage un futur point d'insertion unique côté `cmd/server`
(`a.notifyAmbiance()` / `a.notifyAmbiancePulse()`) appelant **à la fois** l'écrivain lumière et le
moteur audio. L'inventaire complet des appelants actuels de `a.ambiance()` révèle une frontière
qui doit être **normative**, pas laissée à la discipline :

| Famille | Où | Fan-out audio ? |
|---|---|---|
| **Événements de jeu** | Les entrées `ambianceNotifyState`/`ambianceNotifyPulse` de `ambianceSiteRegistry` (`cmd/server/ambiance.go`) — `broadcastReady`, `broadcastStart`, `broadcastStop`, `broadcastPause`, `broadcastPauseAll`, `broadcastContinue`, `broadcastReveal`, `handlePoints`, `handleBumperPoints`, `handleTeamPoints`, `handleMotionDone` (×2), `handleMotionSetTeams`, `handleFlipMemoryCard`, `handleMemorySetTeams`, `setupCallbacks`/`OnRafaleTeamsChanged`, `handleEntracteSet` (×2), `handleFullUpdate`, `onPhaseStarted` | ✅ **oui, exclusivement** |
| **Conduite lumineuse manuelle** | `setLightingMode`, `setLightingFlash`, `runLightingFlash` (`cmd/server/ambiance_override.go`) — sélecteur ON/AUTO/OFF et bascule Flash de la régie | ❌ **jamais** |
| **Célébration lumineuse SCORE** | `runScoreFlash` (`cmd/server/ambiance_override.go`) — ré-émissions de phase du clignotement or, pas un nouvel événement de jeu | ❌ **jamais** |
| **Reconnexion du pont** | `OnReconnect` (`cmd/server/ambiance.go`, callback du driver Hue) — resynchronisation de ce qui est déjà vrai, pas un événement nouveau | ❌ **jamais** |
| **Cycle de vie de l'écrivain** | `startAmbianceWriter`, `reconfigureAmbiance` (`cmd/server/ambiance.go`) — premier rafraîchissement au démarrage/à la reconfiguration | ❌ **jamais** |
| **Pulsation du chronomètre** | `runChronoPulse` (`cmd/server/ambiance.go`) — **notifie toutes les 100 ms** (`chronoPulseTickInterval`) | ❌ **ABSOLUMENT JAMAIS** |

> **Un remplacement aveugle de `a.ambiance()` par le fan-out rendrait le serveur inutilisable** :
> la pulsation du chronomètre déclencherait dix tentatives de lecture par seconde, et chaque
> mouvement du sélecteur manuel ou du Flash de la régie ferait du bruit sans qu'aucune partie ne
> soit en cours.

> **Précision d'implémentation (Lot B, 2026-09-21) — ⚠️ Contract Modification, même esprit que
> `lighting.md` §6.3.** Le fan-out **n'a finalement pas remplacé** les 22 appels
> `a.ambiance().NotifyState()`/`NotifyPulse()` existants : ceux-ci restent **intacts, tels quels**.
> `notifySound(cue audio.Cue)` (`cmd/server/sound.go`) est un point d'entrée **sonore seul**,
> **ajouté** en plus de l'appel lumière existant aux seuls sites qui portent effectivement une cue
> (`handlePoints`, `handleBumperPoints`, `handleTeamPoints`, `handleMotionDone`,
> `handleFlipMemoryCard`, `broadcastReveal`, `handleEntracteSet`, `onPhaseStarted`,
> `setupCallbacks` pour `OnRafaleInvalid`/`OnTimeUp`) — soit 9 fonctions sur les 18 du tableau
> ci-dessus, pas 18. Les sites sans cue (`broadcastStart`, `broadcastPause`, etc.) n'appellent
> `notifySound` nulle part et ne figurent donc pas dans `soundSiteRegistry`.
> **Raison** : un remplacement complet aurait exigé de toucher les 22 sites pour un gain nul sur
> les ~13 qui ne portent aucun son en v11.0, pour un risque de régression bien plus large sur du
> code déjà normatif et testé (`ambianceSiteRegistry`) — la frontière du §6.1 (qui doit/ne doit
> jamais sonner) reste **identique** avec cette forme plus simple, et le test-garde du §6.2 la
> vérifie exactement pareil (un seul sélecteur `notifySound` à rechercher, calqué sur la
> coordination directe avec `test-writer`, voir `cmd/server/sound_sites_test.go`). Deux sites
> **sonores sans aucune contrepartie lumineuse** existent aussi (`OnRafaleInvalid` — RAFALE classique
> uniquement, jamais la variante MEMOTION — et `OnTimeUp`, qui porte les deux : voir son propre appel
> combiné dans `setupCallbacks`) : le son n'est pas strictement un sous-ensemble de la lumière.

### 6.2 Cette frontière doit être vérifiée par un test-garde, pas par discipline

Même raisonnement que `lighting.md` §7 : un test AST symétrique à `ambiance_sites_test.go` doit
vérifier **à la fois** l'exhaustivité des sept sites du §6.1 (aucune cue oubliée — le défaut
« la salle s'allume, le son ne part pas » de #205 a son miroir exact ici : « le son part, rien
d'autre ne change » n'est pas un risque, mais l'inverse — « l'événement a lieu, aucune cue n'est
jouée » — l'est) **et** l'absence de tout appel au fan-out sonore depuis les cinq familles
interdites du tableau ci-dessus. Cette seconde vérification est **propre à ce contrat** : le
test-garde lumineux (`lighting.md` §7) n'a jamais eu à l'exclure, puisque son point d'entrée
(`NotifyState`) est précisément celui que ces familles doivent continuer à appeler.

Spécification complète du test, registre et message d'échec attendu : à la charge de Lot C
(`test-writer`), conformément au plan de dev §3.5/§C.1 — ce contrat pose la frontière, pas
l'implémentation du test.

### 6.3 `CuesDisabled` — filtrage par cue, normatif (2026-09-21, #230)

Nouveau champ de `SoundConfig` (`internal/config/config.go`) :

```go
CuesDisabled map[string]bool `json:"cues_disabled,omitempty"`
```

**On stocke ce qui est ÉTEINT, jamais ce qui est allumé.** Avec un `CueEnabled` (l'inverse), une
carte absente de `config.json` se désérialise en `nil` : toute lecture naïve rend `false`, et
**tous les sons deviendraient muets sur chaque configuration existante** — il faudrait compenser
dans `ApplyDefaults`, peupler les sept clés, et rester vigilant à chaque cue ajoutée. En stockant
les cues **désactivées**, la valeur zéro — carte absente, vide ou `nil` — signifie exactement
« rien n'est désactivé », c'est-à-dire le comportement d'aujourd'hui. Aucune migration, et une cue
ajoutée en #231 est active sans que personne n'ait rien à faire. Même discipline que celle qui a
rendu `Bank` nil-safe en #227 (§5) : la valeur zéro doit être le comportement correct.

**Point de filtrage, normatif : dans `notifySound` (`cmd/server/sound.go`), avant l'appel au
moteur — jamais ailleurs.**

```go
func (a *App) notifySound(c audio.Cue) {
    if config.Get().Sound.CuesDisabled[string(c)] {
        return
    }
    a.sound().PlayCue(c)
}
```

Trois raisons :
- `internal/audio` n'importe rien du serveur (§2, package testable seul) — la configuration n'a
  pas à y entrer, le filtrage ne peut donc pas vivre dans le moteur ;
- `notifySound` est le **point d'entrée unique** du fan-out, garanti par le test-garde AST du §6.2
  — une cue désactivée ne peut donc pas passer par un chemin détourné ;
- filtrer **avant** `PlayCue` évite de consommer une place de file et de gonfler
  `Stats.Accepted` pour un son qui ne sera jamais joué.

**Lecture à l'appel, jamais en cache** dans un champ de l'`App` — sinon une bascule depuis
l'interface ne prendrait effet qu'au redémarrage. Même discipline que `a.buildAudioOutput()` lit
`config.Get().Sound` à chaque construction plutôt qu'une fois pour toutes.

> ⚠️ **`notifySound` est du code de #227, déjà revu et testé — ceci est une extension ADDITIVE,
> pas une réouverture.** Le corps existant est inchangé, une condition est ajoutée en amont ; à
> valeur zéro (`CuesDisabled` absent/vide/nil), le comportement est **identique au bit près** à
> celui livré en #227 ; aucun site d'émission, aucune signature, aucun contrat de concurrence
> n'est modifié ; le test-garde AST du §6.2 reste valide tel quel (le sélecteur scanné ne change
> pas). **Test exigé** : une configuration vide produit exactement les mêmes cues qu'aujourd'hui —
> c'est ce test qui transforme « additif » d'une affirmation en une propriété vérifiée.

**Interaction avec l'interrupteur général — normatif** : `sound.enabled` reste **maître**. Éteint,
il coupe tout (le moteur n'est même pas démarré, contract §5.5), quels que soient les sept
interrupteurs de `CuesDisabled`. Une cue individuellement désactivée **reste testable** par
`POST /api/sounds/<cue>/test` (`contracts/http-endpoints.md` §Sound) : ce test appelle le moteur
**directement**, jamais via `notifySound` — tester est un geste explicite de l'utilisateur, seul
le déroulé de la partie doit rester muet pour une cue désactivée.

**L'interrupteur général est asymétrique — normatif (2026-09-21, #230 addendum)** :

| Bascule | Effet |
|---|---|
| **Éteindre** (`enabled` → `false`) | **Immédiat**, sans redémarrage |
| **Rallumer** (`enabled` → `true`) | **Prend effet au prochain démarrage du serveur** |

Ce n'est pas un compromis technique accepté par défaut : c'est ce que la conception de
`notifySound` produit **sans rien coder de spécial** pour l'un ou l'autre sens, et c'est la raison
pour laquelle l'asymétrie est délibérément retenue plutôt que comblée par une reconstruction à
chaud du moteur.

`notifySound` lit `config.Get().Sound.Enabled` à chaque appel et sort tôt si c'est faux — exactement
la même lecture-à-l'appel que pour `CuesDisabled` ci-dessus :

```go
func (a *App) notifySound(c audio.Cue) {
    if !config.Get().Sound.Enabled {
        return
    }
    if config.Get().Sound.CuesDisabled[string(c)] {
        return
    }
    a.sound().PlayCue(c)
}
```

- **Éteindre pendant que le serveur tourne** : le moteur a été construit et sa goroutine tourne
  (`enabled` était vrai au démarrage) — cette garde le rend muet **immédiatement**, sans démonter
  ni le moteur ni sa goroutine ni sa file : rien ne le distingue d'un `CuesDisabled` qui listerait
  les sept cues à la fois.
- **Rallumer pendant que le serveur tourne** : si `enabled` était faux au démarrage,
  `a.buildAudioOutput()` a renvoyé `nil` (contract §5.5) et le moteur est un no-op de naissance —
  repasser `enabled` à `true` ne construit rien après coup, `PlayCue` reste sans effet. Rallumer
  exige donc un redémarrage, **sans qu'aucune branche du code n'ait besoin de le décider** : c'est
  une conséquence directe de l'hypothèse « le moteur est construit une seule fois, au démarrage »
  déjà posée par #227/#228/#229 (§5) et jamais rouverte ici.

**Pourquoi ne pas reconstruire le moteur à chaud** (rejeté explicitement, pas seulement non
implémenté) : démonter proprement un moteur en fonctionnement — goroutine, périphérique audio déjà
ouvert (`internal/audio/output_oto.go`) — n'est prévu par aucune conception actuelle, pour un
bénéfice limité (rallumer le son est un geste rare comparé à l'éteindre en urgence en pleine
partie, le geste que cette asymétrie sert en priorité). L'interface (`docs/mockups/sound-config-230.html`
§02, révision 5) l'affiche explicitement à côté de l'interrupteur plutôt que de laisser
l'utilisateur attendre un son qui ne viendra pas.

**Aucun endpoint dédié pour ces deux réglages** (activation générale, `CuesDisabled`) : ils
s'écrivent par le patch partiel additif existant `POST /config.json` avec un corps
`{ "sound": {...} }`, exactement comme `{ "lighting": {...} }` le fait déjà pour l'éclairage
(`contracts/http-endpoints.md` §Configuration). Aucun nouvel endpoint n'est introduit pour ce seul
besoin.

**Fusion champ par champ — normatif** (`internal/server/http.go`, `handleConfig`) : la section
`sound` suit le même régime que `lighting`/`ai`, et pour la même raison — deux contrôles
**indépendants** écrivent cette section (l'interrupteur général et les sept par cue), chacun
n'envoyant que la clé qu'il possède. Un remplacement en bloc effacerait silencieusement l'un des
deux réglages à chaque écriture de l'autre. Règle : **chaque champ présent est appliqué, chaque
champ absent est préservé**.

`cues_disabled` est une carte : trois états doivent être distingués, ce qu'un `json.Unmarshal` seul
ne permet pas (absente et vide se désérialisent identiquement) — le JSON brut de la requête est
donc interrogé, comme la branche `lighting` le fait déjà pour ses propres champs :

| Dans le JSON reçu | Effet |
|---|---|
| clé **absente** | valeur existante **préservée** |
| clé présente, objet **vide** `{}` | carte **vidée** (plus aucune cue désactivée) |
| clé présente, objet **peuplé** | carte **remplacée** (le frontend renvoie toujours l'ensemble complet, jamais un patch partiel de la carte elle-même) |

Chaque clé de `cues_disabled` est validée contre le catalogue fermé des sept cues
(`soundCueFromString`, même garde que sur `{cue}` dans les URL de §Sound) — une clé inconnue est
rejetée en `400`, jamais persistée silencieusement, et rejette la requête entière (aucune écriture
partielle en cas d'erreur).

---

## 7. Upload — renvoi

Le contrat HTTP de l'upload de sons (allowlist `.wav` uniquement) est spécifié dans
`contracts/http-endpoints.md` §Sound — l'implémentation des endpoints appartient à #230, cette
section n'est qu'une réservation normative du format et de la contrainte, pour que #230 ne
découvre pas le format canonique après coup.

---

## 8. Note de traçabilité — hypothèse systemd non vérifiée sur matériel réel

> Ce choix est fait **par prudence, sans vérification sur Raspberry Pi réel**. L'hypothèse
> sous-jacente est que le serveur tourne en unité **system** (`docs/ADMIN_GUIDE.md:1297-1303`,
> `server-go/README.md:113`) et doit joindre un `pipewire-pulse` de session utilisateur. Les
> tâches 0.2 et 0.3 du spike #226 n'ont pas pu être vérifiées faute de Raspberry Pi disponible
> pour l'agent — voir `_work/reports/spike-226-20260921-103221.md` §2.7.
>
> **Décision retenue (mitigation par prudence)** : ajouter `XDG_RUNTIME_DIR` et `PULSE_SERVER` à
> l'unité *system* existante (`/etc/systemd/system/buzzcontrol.service`), et **écarter** la
> conversion en unité *utilisateur* — BuzzControl écoute sur les ports privilégiés 80 (HTTP/WS) et,
> en option, 53 (portail captif DNS), qu'une unité utilisateur non privilégiée ne pourrait plus
> ouvrir sans configuration supplémentaire : on réparerait le son en cassant le serveur.
> Complément nécessaire : `/run/user/<uid>` n'existe que si une session de cet utilisateur est
> ouverte — activer `loginctl enable-linger <user>` sur une machine sans session interactive.
>
> **Si cette hypothèse s'avère fausse une fois un Raspberry Pi disponible, la correction doit
> rester locale au pilote (#228) et à la documentation de déploiement.**

**Exigence de conception qui en découle, normative dès #227** : la résolution du point de sortie
audio (sélection de sink, variables d'environnement, repli ALSA) **ne doit jamais être codée en
dur dans le moteur** décrit au §5. Le moteur ne connaît que l'interface `Output` du §4 ; tout ce
qui touche à l'environnement d'exécution appartient exclusivement au pilote concret (#228). C'est
ce qui garantit qu'une hypothèse fausse se corrige dans un seul fichier, jamais dans le moteur ni
dans le câblage des sites de jeu.

---

## 9. Hors de ce contrat

| Sujet | Issue |
|---|---|
| Pilote audio réel (`oto`), sélection de sink, repli ALSA/PulseAudio | #228 |
| Sons livrés (`//go:embed`), extraction conditionnelle, bouton de restauration | #229 |
| Interface d'administration, endpoints d'upload/liste/suppression | #230 |
| Bruitages fins par type de question (indice QCM, cartes MEMORY/MEMOTION, RAFALE) | v11.1 (#231) |
| Compensation de latence son/lumière (délai configurable sur l'ambiance Hue) | Faisabilité tranchée par le spike (§3 du verdict, `_work/reports/spike-226-20260921-103221.md`) — implémentation renvoyée à une issue de câblage ultérieure, hors périmètre Lot A/B de #227 |
| Amendement du gel de dépendances `TestCA7_NoNewThirdPartyDependency` pour `oto`/`purego`/`jfreymuth/pulse` | #228 (le bump de toolchain seul est fait ici, §10 — voir `contracts/CHANGELOG.md`) |

---

## 10. Média sonore long attaché à une question — normatif (v11.1, #219)

> **Issue** : #219 (milestone #37 — « v11.1 — Question sonore »)
> **Cadrage utilisateur** : commentaire #219 du 2026-09-22 — ce n'est **pas** un nouveau type de
> question, c'est un **média** attachable à une question, au même titre qu'une image.
> **Plan** : `_work/reports/plan-20260922-093142.md` (§0 porte le relevé de code qui fonde cette
> section)
> **Maquette normative** : `docs/mockups/question-sound-219.html`
>
> Cette section est **additive**. Elle n'amende ni §4 (`Output`), ni §5 (le moteur), ni §2 (le
> catalogue de cues) : ces trois-là restent **inchangés au bit près**.
>
> **Périmètre v11.1 (arbitrage utilisateur, GATE 2 du 2026-09-22)** : **SPEEDY, QCM, ARDOISE**.
> MEMORY, MEMOTION, ENTRACTE et RAFALE sont **hors périmètre** — ces types portent leurs médias sur
> leurs cartes (ou n'en portent pas du tout), ce qui demande une décision de conception propre,
> renvoyée à un chantier ultérieur. L'analyse correspondante est conservée dans
> `_work/reports/plan-20260922-103848.md` §1.

### 10.1 Pourquoi un second chemin, et non une huitième cue

Quatre propriétés du chemin des cues rendent celui-ci **structurellement** inapte à un média de
question. Elles sont listées ici pour qu'aucune relecture ultérieure ne reprenne l'idée
« il suffirait d'ajouter une cue » :

| Obstacle | Où | Conséquence si ignoré |
|---|---|---|
| Plafond dur de **10 s** | `otoPlayMaxWait`, `internal/audio/output_oto.go` | Extrait **tronqué à 10 s, silencieusement** — `Play` journalise et rend `nil` |
| **Sérialisation stricte** (une goroutine, `Play` bloquant réel, file de 8) | §5.2/§5.3 + amendement §4 | Tout bruitage survenant **pendant** le média attend derrière lui, puis est perdu au-delà de 8 |
| Catalogue de cues **CLOS**, **un** fichier par cue | §2.1 + `FileBank.PCM` | Un média **par question** n'a pas de cue et ne peut pas en avoir une |
| Validation d'upload calibrée pour des bruitages (5 s / 2 Mo) | `internal/audio/validate.go` | L'assouplir casserait la garantie qui protège la file des cues |

À l'inverse, `oto` **autorise** la coexistence : `Context.NewPlayer` est documenté
**concurrent-safe** et le mux d'`oto` **mixe** plusieurs `Player` simultanés. Le second chemin
n'enlève donc rien au premier — il joue **à côté**.

### 10.2 Interface — normative

```go
// MediaPlayer joue un média sonore LONG attaché à une question, sur un
// chemin entièrement distinct de celui des cues (§5). Il ne partage avec
// le moteur de bruitages QUE le contexte audio du processus (§10.3).
//
// Contrairement à Output.Play (§4), AUCUNE méthode ne bloque : la lecture
// est asynchrone, et l'état est lisible à tout instant. C'est cette
// asymétrie qui rend possibles les contrôles de conduite (rejouer, pause,
// stop) que l'interface Output n'offre pas et n'a jamais eu à offrir.
type MediaPlayer interface {
    // Play démarre la lecture du fichier désigné. VOIX UNIQUE : un Play
    // pendant une lecture en cours REMPLACE celle-ci (arrêt net), il ne
    // superpose jamais deux extraits.
    Play(path string) error
    Pause()
    Resume()
    Stop()
    State() MediaState
    Close() error
}

type MediaState string

const (
    MediaIdle    MediaState = "IDLE"
    MediaPlaying MediaState = "PLAYING"
    MediaPaused  MediaState = "PAUSED"
)
```

- **Voix unique, normatif.** Deux extraits ne se superposent jamais. Une question chasse l'autre.
- **Lecture en flux depuis le disque**, jamais un `os.ReadFile`. Un média de 45 s pèse ~8 Mo de
  PCM : le charger d'un bloc pour chaque lecture est un gaspillage inutile alors que
  `oto.Player` consomme un `io.Reader` progressivement. La source **doit** être un
  `io.ReadSeeker` — `Seek(0, io.SeekStart)` **est** le mécanisme de rejeu.
- **Dégradation silencieuse identique à §5.5** : plateforme non ciblée, contexte refusé, fichier
  absent ou non conforme ⇒ implémentation neutre, aucune goroutine, aucune erreur propagée dans
  le jeu. Un accesseur exporté (`IsNeutralMedia`) signale le cas, symétriquement à `IsNeutral`
  (§4, amendement #230).

### 10.3 Le contexte `oto` est un singleton de processus — refactor imposé

**`oto` n'autorise qu'un seul contexte audio par processus** (§1, spike #226 §2) — c'est la racine
même du format canonique du §3. Or ce contexte est aujourd'hui créé **à l'intérieur** de
`newOtoOutput` et détenu en privé par `otoOutput` (`internal/audio/output_oto.go`). Le
`MediaPlayer` ne peut donc pas en obtenir un, et n'a **pas le droit** d'en créer un second.

**Exigence normative** : le contexte est extrait en **singleton de package** (`sync.Once`), dont
`newOtoOutput` **et** le backend `oto` du `MediaPlayer` se servent tous deux.

> ⚠️ **Refactor d'un code déjà revu et testé (#228) — c'est une ré-organisation, pas une
> réouverture.** L'attente du canal `ready`, le délai de 5 s (`otoContextReadyTimeout`), le
> pré-armement (`prime`) et la dégradation vers `noopOutput` restent **exactement** où ils sont,
> dans le même ordre. Aucune signature publique n'est modifiée. Les tests-gardes
> `play_blocks_228_test.go`, `output_228_test.go`, `cross_compile_228_test.go` et
> `isneutral_230_test.go` doivent passer **sans une ligne de modification** : toute modification
> de l'un d'eux est le signe que le refactor a changé un comportement, et **doit** être refusée en
> revue. Le bug que l'amendement §4 a gravé — `Play` qui rend la main avant la fin réelle du
> rendu — est précisément celui qu'une ré-organisation maladroite de ce fichier ferait revenir.

> ⚠️ **Piège de durée de vie, non évident.** `oto` documente : « *A player must be kept reachable
> as long as it should keep playing. A player is closed when it becomes unreachable* » — la
> fermeture passe par un **finalizer GC**. Le `MediaPlayer` **doit** conserver une référence forte
> sur le `Player` courant pendant toute la lecture. Sans cela, le son se coupe **au hasard**, plus
> souvent sur une machine chargée, de façon **irreproductible** — le pire profil de défaut
> possible. Ce n'est pas une précaution théorique : c'est une contrainte explicite de la
> bibliothèque.

### 10.4 Format et limites — distinctes de celles des cues, jamais confondues

Le format canonique du **§3 s'applique**, avec **une seule tolérance propre au média de question**
— voir l'encart mono ci-dessous : WAV PCM 16 bits, 44 100 Hz, **mono ou stéréo**. **Aucun format
compressé n'est accepté**, `.mp3` compris — la décision utilisateur du cadrage v11.0 (§3) vaut ici
telle quelle, et `go.mod` ne contient **aucun** décodeur ni rééchantillonneur de fréquence.

> ⚠️ **Mono accepté — arbitrage utilisateur QUALIF v11.1 (2026-09-22), scope strictement limité au
> média de question.** Un retour QUALIF a établi qu'un WAV **mono** doit être accepté pour le son
> de question, alors que le §3 (cues, #229/#230) reste **strictement stéréo, INCHANGÉ** —
> `extractCanonicalPCM` (`internal/audio/bank.go`), partagée par `FileBank.PCM` (lecture des cues)
> et `ValidateUpload` (upload des cues), **n'est pas touchée**. Le validateur du média de question
> utilise un parseur **dédié** (`extractQuestionSoundPCM`, `internal/audio/validate_media.go`),
> volontairement dupliqué plutôt que paramétré, précisément pour ne courir **aucun** risque de faire
> déborder cette tolérance sur les cues.
>
> Un mono accepté est **suréchantillonné en stéréo à l'upload** (`upmixMonoToStereo` : chaque
> échantillon dupliqué sur les deux voies) — une opération arithmétique pure sur du PCM déjà
> décodé, **pas** un « rééchantillonnage à la volée » au sens que le §3 exclut (qui parle de
> changer la fréquence ou de décoder un codec compressé). Le fichier **réellement stocké sur
> disque** (`BuildCanonicalWAV`) est donc **toujours** stéréo canonique — le pilote de lecture
> (`media_oto.go`, #228, déjà revu) ne comporte **aucune** branche supplémentaire pour un second cas
> de figure ; il ne voit jamais de mono.

Les **limites**, en revanche, sont **propres au média** :

| Constante | Valeur | Motif |
|---|---|---|
| `MaxQuestionSoundDuration` | **30 s** *(arbitrage utilisateur, GATE 2 du 2026-09-22)* | Un extrait de question dure typiquement 10 à 30 s. Au-delà, le coût disque (10,1 Mo/minute) et le poids des sauvegardes deviennent le facteur limitant, pas l'audio |
| `MaxQuestionSoundBytes` | **6 Mio** (`6 << 20`) | 30 s canoniques pèsent 5,05 Mio ; 6 Mio laisse **+18,9 %** pour l'en-tête RIFF et d'éventuels chunks `LIST`/`INFO` (que `extractCanonicalPCM` traverse déjà) sans autoriser un fichier abusif |

**Aucun seuil d'avertissement fixe.** Un seuil intermédiaire ne dirait que « vous approchez du
plafond », ce que le plafond dit déjà. L'avertissement — **non bloquant**, comme l'était celui des
cues — est **contextuel** : il compare la durée du son au temps de réponse de la question
(`Question.TIME`), deux valeurs présentes dans la **même** requête multipart. Il n'est émis qu'en
mode simultané (§10.7) : en mode différé la situation ne peut pas se produire.

> ⚠️ **Normatif : `MaxSoundDuration` (5 s) et `MaxUploadBytes` (2 Mio) ne sont PAS modifiées.**
> Elles protègent la file des cues (§5.2) et leur justification — « le moteur lit strictement
> séquentiellement » — reste entièrement valable **pour les cues**. Les deux jeux de constantes
> coexistent délibérément. Un test-sentinelle vérifie que les valeurs des cues sont inchangées :
> « factoriser » les deux jeux en un seul est le défaut le plus probable de ce lot, et il
> supprimerait silencieusement la garantie du §5.2.

> ℹ️ **Amendement (v11.1, arbitrage mono ci-dessus).** Avant cet arbitrage, le validateur du média
> réutilisait directement `extractCanonicalPCM` (`internal/audio/bank.go`), le même parseur que les
> cues — c'était vrai jusqu'à ce que la tolérance mono, propre au média de question, exige un
> parseur distinct (`extractQuestionSoundPCM`) pour ne jamais toucher au comportement des cues. Les
> deux parseurs restent construits sur les **mêmes constantes** de format (`format.go` :
> `SampleRate`/`BitsPerSample`), donc ne peuvent pas diverger sur ce qui est canonique — seule la
> tolérance sur le NOMBRE DE VOIES diffère entre les deux, délibérément.

### 10.4bis Le champ est commun à tous les types — la restriction est une décision d'éditeur

> **Normatif.** `Question.SOUND` et `Question.SOUND_TIMER_DELAYED` vivent sur `Question`, **hors
> `TypedContent`** : ils sont donc persistés, diffusés et lus **quel que soit le type**. Le serveur
> **ne doit pas** porter de garde « ce type n'a pas le droit d'avoir un son ».

C'est exactement le régime de `MEDIA` aujourd'hui — structurellement commun aux sept types, masqué
seulement à l'édition. Trois raisons de s'y tenir :

- une telle garde n'apporterait **aucune sécurité** : `GET /questions` est public et sans
  authentification, et ce champ n'est pas un secret (même raisonnement que `contracts/models.md`
  §`EXPLANATION`) ;
- elle rendrait l'élargissement futur du périmètre **plus coûteux qu'il ne doit l'être** : sans
  garde, rouvrir MEMORY/MEMOTION/ENTRACTE sera un changement **purement frontend** ;
- elle romprait la symétrie avec `MEDIA`, qu'une relecture ultérieure prendrait pour une
  incohérence à « corriger » — dans un sens ou dans l'autre.

Ce point est écrit **parce que c'est la correction spontanée qu'une revue est tentée d'exiger** en
voyant un champ restreint à trois types côté interface et ouvert à tous côté modèle.

---

### 10.5 Frontière avec le fan-out des cues — vérifiée par test, pas par discipline

Le §6.2 impose déjà un test-garde AST sur un **registre fermé** de sites autorisés à appeler
`notifySound`. Cette section lui ajoute une **frontière symétrique** :

- L'adaptateur du média vit dans un **fichier distinct** (`cmd/server/question_sound.go`), jamais
  dans `cmd/server/sound.go`.
- Il n'appelle **jamais** `notifySound` ni `PlayCue` — un média n'est pas une cue.
- Réciproquement, `sound.go` n'appelle **jamais** le `MediaPlayer`.

Les deux chemins ne se touchent qu'en un seul point : le contexte `oto` partagé (§10.3). Cette
exclusion mutuelle **doit** être couverte par un test, au même titre que le registre du §6.2 — la
discipline seule ne suffit pas, c'est précisément le raisonnement que §6.2 tient déjà.

### 10.6 Ce que cette section ne couvre pas

| Sujet | Statut |
|---|---|
| *Ducking* (atténuer le média pendant un bruitage) | **Hors v11.1.** `oto.Player.SetVolume` le rendrait trivial, mais il recouplerait les deux chemins que §10.5 sépare. Suite possible, jamais un effet de bord |
| Lecture en boucle (ambiance d'ENTRACTE prolongée) | **Hors v11.1.** Une lecture = un passage |
| Position de lecture diffusée dans le `GameState` | **Exclu.** Seul l'**état** (`IDLE`/`PLAYING`/`PAUSED`) est diffusé, à chaque changement. Le chronomètre est le seul flux à cadence du projet ; on n'en ajoute pas un second |
| Sélection du périphérique de sortie | Inchangé — `SoundConfig.Device` n'est toujours pas honoré par `oto` (§9, `OutputConfig`) |
| Média sonore de **réponse** (symétrique de `MEDIA_ANSWER`) | Non demandé au cadrage. Additif si un jour souhaité |

> ℹ️ Cette table est celle du périmètre initial. Le **chronomètre de réponse différé**, ajouté par
> l'arbitrage du GATE 2 du 2026-09-22, est **dans** le périmètre — voir §10.7 ci-dessous.

### 10.7 Chronomètre de réponse différé — normatif (arbitrage utilisateur, GATE 2 du 2026-09-22)

Chaque son porte un réglage propre : le chronomètre de réponse démarre **en même temps que lui**
(valeur par défaut, comportement d'avant ce lot) ou **à sa fin**.

```go
// sur Question
SoundTimerDelayed bool `json:"SOUND_TIMER_DELAYED,omitempty"`
```

**La valeur zéro est le comportement correct** — `false` = simultané = ce que fait le serveur
aujourd'hui. Aucune migration, aucun `question.json` existant modifié. Même discipline que
`CuesDisabled` (§6.3), et pour la même raison : un réglage dont l'absence changerait le
comportement serait un piège à chaque configuration existante.

**« La fin du son » recouvre deux évènements, jamais un seul** : la fin naturelle de la lecture
**et** l'arrêt manuel par l'animateur. Les deux libèrent le chronomètre, et la libération est
**idempotente** — les deux peuvent survenir.

**La libération est irréversible pour la question en cours.** Un « Rejouer » après le démarrage du
chronomètre ne le regèle **jamais** : l'animateur disposerait sinon d'un bouton « rendre du temps »
que rien ne justifie.

**Libération pendant une pause de jeu** : si le son se termine alors que la partie est en pause, le
chronomètre démarre **à la reprise**, jamais pendant la pause.

> ⚠️ **Règle de non-blocage — la plus importante de cette section.**
> **Si la lecture ne démarre pas réellement, le chronomètre démarre immédiatement.** Lecteur neutre
> (plateforme non ciblée, contexte audio refusé), fichier absent ou non conforme,
> `sound.enabled = false` : dans tous ces cas le mode différé **dégrade vers le mode simultané**.
>
> C'est l'application directe de la dégradation silencieuse du §5.5, avec sa conséquence explicitée
> une fois pour toutes : **elle doit dégrader vers le comportement normal, jamais vers un
> blocage.** Sans cette règle, une enceinte débranchée fige une question en direct, sans message et
> sans recours — le pire défaut que ce lot puisse produire.
>
> **Complément normatif** : un **chien de garde** libère le chronomètre au plus tard à
> `MaxQuestionSoundDuration + 2 s` après le lancement, même si aucune fin de lecture n'a été
> signalée (goroutine de surveillance morte, pilote figé). La dégradation ne doit pas dépendre d'un
> seul chemin de détection.

**L'interface doit dire pourquoi le chronomètre ne bouge pas.** Un chronomètre figé au temps plein
pendant 30 s sera pris pour une panne — par l'animateur comme par le public. `/anim`, `/admin`
**et la TV** portent une mention explicite (« le chronomètre démarrera à la fin du son »), jamais
un simple chiffre immobile. L'état est diffusé par le serveur (`GAME.ANSWER_TIMER_WAITING`), jamais
déduit côté navigateur.

**Points de couplage, par famille de chronomètre** — les deux sont distincts et ne se partagent
aucun code :

| Chronomètre | Types | Où il démarre aujourd'hui | Forme du différé |
|---|---|---|---|
| **Global** | SPEEDY, QCM, ARDOISE | `Engine.actualStart()`, appel `e.startTimer()` sous verrou | **Ne pas créer le ticker** — `CurrentTime` est déjà au temps plein, le chronomètre s'affiche donc figé sans état d'affichage à inventer. Une méthode exportée `ReleaseDeferredTimer()` le crée à la fin du son |

> ⚠️ **La garde de type existante d'`actualStart()` est préservée à l'identique.** MEMOTION et
> ENTRACTE n'ont jamais démarré le chronomètre global (MEMOTION utilise un chronomètre **par
> carte**, démarré par `(*App).handleMotionFlip` et non par le moteur ; ENTRACTE n'a aucun
> chronomètre). L'ajout du différé s'insère **autour** de l'appel existant, il ne remplace pas cette
> garde. Un chronomètre global qui démarrerait sur MEMOTION serait une régression, pas un effet de
> bord acceptable.
>
> *Hors périmètre v11.1, conservé parce que c'est un relevé de code vérifié* : le jour où MEMOTION
> recevra un son, son différé ne demandera **aucune modification du moteur** — il suffira de ne pas
> appeler `StartMotionCardTimer` au retournement et de laisser le rappel de fin de lecture le faire.

> ⚠️ `Pause()`/`Continue()` **n'arrêtent pas le ticker** — `processTimerTick` se contente de ne
> rien décrémenter hors `PhaseStarted`. « Mettre le chronomètre en attente » en arrêtant le ticker
> est donc impossible : la seule forme correcte est de **ne pas le créer**.

> ⚠️ `actualStart()` est un site que RAFALE, ENTRACTE, MEMOTION et MEMORY se partagent déjà, avec
> des commentaires d'ordonnancement explicites (« must run BEFORE startTimer() »). La garde du
> différé s'ajoute **autour de l'appel existant**, au même endroit, **sans déplacer une seule ligne
> voisine**.
