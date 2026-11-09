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

> **Amendement v11.6.0 (#231, LOT-3) — la liste fermée gagne 8 cues « par type »**, chacune avec un son par défaut
> synthétisé (`internal/audio/synth`), un fichier remplaçable et un interrupteur (`CuesEnabled`, `ambiance-settings.md`) :
>
> | Cue | `Type()` | Site d'émission — **une cue par site (D4)** |
> |---|---|---|
> | `qcm-indice` | `QCM` | `broadcastQCMHint` — un indice barre une réponse |
> | `memory-carte` | `MEMORY` | `handleFlipMemoryCard` — carte retournée (face visible) |
> | `memory-paire` | `MEMORY` | `handleFlipMemoryCard` — paire trouvée (la paire ratée **garde** `perdu`) |
> | `memotion-selection` | `MEMOTION` | `handleMotionSelect` — carte choisie dans la grille |
> | `memotion-fin-carte` | `MEMOTION` | `handleMotionDone` — carte fermée **sans** gagnant (avec gagnant : `gagne`, inchangé) |
> | `rafale-bonne` | `RAFALE` | `handleRafaleValidate` — réponse validée |
> | `rafale-mauvaise` | `RAFALE` | `handleRafaleInvalidate` — réponse invalidée / passée (si ce site émet déjà `perdu`, `perdu` est **remplacé** par `rafale-mauvaise` : une seule cue par site) |
> | `rafale-fin-lot` | `RAFALE` | fin du lot — **amendement LOT-3 (code livré 042fe976)** : nouveau callback `Engine.OnRafaleLotEnd` (`setupCallbacks`), déclenché par le moteur quand le **réservoir est épuisé** ou le **nombre maximal de questions atteint**, manche RAFALE classique **et** carte MEMOTION RAFALE. (Le contrat citait `OnRafaleAnswer` avec `next == nil` : ce callback n'est pas appelé en fin de manche — fin = `roundEnded` → `OnStateChange(PhaseStopped)`.) Le « chrono de lot » **est** le chrono global : son expiration reste `temps-ecoule`, inchangé. Ordre à la dernière bonne réponse — **revue LOT-3 (m1)** : `rafale-bonne` **puis** `rafale-fin-lot` (la validation sonne avant la fin du lot ; la file du moteur de bruitages conserve cet ordre) |
>
> `audio.Cue` gagne `Type() string` (`""` pour les 7 cues globales) et `audio.AllCues()` dans l'ordre normatif : global
> puis `QCM`, `MEMORY`, `MEMOTION`, `RAFALE`. Le test d'exhaustivité (§6.2) couvre les 15 cues : chaque site d'émission
> émet **exactement une** cue, les 8 nouvelles ne passent **que** par `notifySound`. `GET /api/sounds` expose `type`
> (`http-endpoints.md`). Aucun changement pour les 7 cues v11.0 (noms, fichiers, sites) — octet pour octet.

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
  > **Amendement v11.6.0 (#232, LOT-1) — converti à l'upload, jamais à la volée.** Le format **stocké et joué** reste
  > strictement canonique ; mais un WAV PCM non conforme n'est plus **refusé** : il est **converti une fois, à l'upload**
  > (`internal/audio/resample.go`, `ToCanonical(raw) (pcm, ConversionInfo, error)`), puis écrit en canonique comme
  > aujourd'hui (`BuildCanonicalWAV`). Point d'entrée **unique**, appelé **avant** `extractCanonicalPCM` par
  > `ValidateUpload` (cues), `ValidateQuestionSound` (médias de question **et** de carte MEMOTION, §14) et le chemin
  > `decompte` — jamais un second convertisseur. Entrées acceptées (D2) : tags `1` (PCM entier 8/16/24/32 bits),
  > `3` (IEEE float 32 bits), `0xFFFE` extensible avec ces sous-formats ; 1 ou 2 canaux ; 8 000–192 000 Hz.
  > **`.mp3` et tout format compressé restent refusés** (décision 2026-09-21, ci-dessous, inchangée). Conversion
  > déterministe, Go pur, **aucune dépendance** (`TestCA7_NoNewThirdPartyDependency`) : PCM → flottant interne → FIR
  > fenêtré (sinc × Kaiser, polyphase, coupure `min(f_src, f_dst)/2`, ≥ 32 taps par phase) → arrondi 16 bits sans
  > dither → upmix mono (`upmixMonoToStereo`, inchangé). Fichier **déjà canonique ⇒ chemin court, octets identiques**
  > (non-régression). Qualité exigée : repliement < −40 dB sur un balayage sinus 48 000 → 44 100 Hz (test unitaire).
  > Plafonds : les limites de **durée** et les plafonds d'octets **canoniques** de chaque chemin (2 Mio cue, 6 Mio
  > média, §10.4) sont **inchangés** et évalués **après** conversion ; un plafond d'entrée **brute** = 4 × le plafond
  > canonique borne la mémoire (`413` au-delà). **Seul le résultat canonique est stocké** (pas d'original conservé).
  > Les réponses d'upload gagnent `converted: bool` et `source_format: string|null` (`http-endpoints.md`).
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

> **Amendement v11.4.0 (#288, §13.1)** : l'interface gagne `PlayLoop(path string) error` — lecture reprise au début à
> chaque fin naturelle, jamais d'`onNaturalEnd`. Le bloc ci-dessus est conservé tel quel comme état v11.1.

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
| Lecture en boucle (ambiance d'ENTRACTE prolongée) | ~~**Hors v11.1.** Une lecture = un passage~~ — **dans le périmètre depuis v11.4.0 (#288, §13)** : `SOUND_LOOP`, combinable avec le différé (fin du 1ᵉʳ passage) |
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

> **Amendement v11.4.0 (#288, §13.2, décision utilisateur 2026-11-05)** : `SOUND_TIMER_DELAYED` et `SOUND_LOOP` se
> **combinent** — « la fin du son » d'une lecture en boucle est la **fin du premier passage** ; le média continue
> ensuite en boucle, le chronomètre tourne.

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

> ⚠️ **Règle de non-blocage — portée restreinte à T1 par §10.8 (2026-09-23).**
> **Si la lecture ne démarre pas réellement ALORS QUE LA QUESTION A DÉJÀ DÉMARRÉ, le chronomètre
> démarre immédiatement.** Lecteur neutre (plateforme non ciblée, contexte audio refusé), fichier
> absent ou non conforme, `sound.enabled = false` : dans tous ces cas le mode différé **dégrade
> vers le mode simultané**. Une indisponibilité **connue avant le lancement** relève de §10.8 et
> **empêche la question de démarrer** — ce n'est pas une exception à cette règle, c'est un instant
> différent.
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

### 10.8 Média indisponible avant le lancement — normatif (arbitrage utilisateur, 2026-09-23)

> **Besoin** : « si le média n'est pas disponible, la question n'a plus de raison d'être » —
> remonté pendant la validation manuelle du Scénario 11 (#219/#236/#237), puis précisé au GATE 2 :
> **revalidation complète du format** à chaque contrôle, et **échappatoire par Ctrl+Clic**.
> **Plan** : `_work/reports/plan-20260923-094500.md`

#### 10.8.1 La frontière T0/T1 — et pourquoi il n'y a pas de contradiction

| | **T0 — avant le lancement** | **T1 — après le lancement** |
|---|---|---|
| Situation | L'indisponibilité est **déjà connue** | La question tourne déjà ; la lecture échoue, se dégrade, ou ne signale jamais sa fin |
| Règle | **BLOQUANT** — la question ne passe pas en `READY` | **JAMAIS BLOQUANT** — §10.7, dégradation silencieuse, chronomètre libéré |

Un bruitage manqué est une perte cosmétique ; un son de question manquant vide la question de son
contenu. Les deux règles portent donc sur **deux instants distincts**.

> ⚠️ **§10.7 n'est ni supprimée ni affaiblie.** Elle protégeait d'une question **figée en pleine
> partie, sans recours** — c'est toujours exactement ce qu'elle fait. Toute implémentation qui
> ferait disparaître la dégradation T1 au prétexte d'avoir ajouté la gate T0 est un **refus de
> revue**.

#### 10.8.2 « Média disponible » — définition unique, normative

Trois conditions, **toutes** requises :

1. `config.Get().Sound.Enabled` — sinon motif `DISABLED` ;
2. une sortie réelle est attachée — `!audio.IsNeutralMedia(player)` — sinon motif `OUTPUT` ;
3. le fichier existe **et se valide entièrement** au format canonique — sinon motif `FILE`.

**Une seule fonction porte cette définition**, réutilisée **à la fois** par la gate T0 **et** par la
dégradation T1 (`play()`). Deux définitions parallèles divergeraient et produiraient le pire
symptôme possible : une question bloquée alors que le son aurait fonctionné, ou l'inverse.

> **Arbitrage utilisateur du GATE 2 — la condition 3 est une revalidation COMPLÈTE, pas un simple
> `os.Stat`.** L'utilisateur a explicitement choisi de revérifier **l'intégrité** du fichier, et
> pas seulement sa présence : un WAV tronqué ou corrompu hors de l'éditeur doit être détecté
> **avant** le lancement, pas découvert à la lecture. La validation réutilise la fonction existante
> (`ValidateQuestionSound` / `extractCanonicalPCM`), **jamais un second parseur**.

#### 10.8.3 Cache de validation — normatif, et non facultatif

La gate est évaluée à **chaque transition `PREPARE ↔ READY`**, donc **à chaque PONG de buzzer**.
Une revalidation complète naïve relirait jusqu'à 6 Mio par buzzer et par question. **Un cache est
donc obligatoire**, et sa forme est imposée :

| | |
|---|---|
| **Clé** | chemin absolu du fichier |
| **Empreinte** | `mtime` **et** taille, obtenus par un `os.Stat` (bon marché) |
| **Hit** | empreinte identique à celle mémorisée ⇒ **réutiliser le verdict**, sans relire le contenu |
| **Miss** | aucune entrée, ou empreinte différente ⇒ **revalidation complète**, puis mémorisation |
| **Fichier disparu** | `os.Stat` en erreur ⇒ motif `FILE` **et suppression de l'entrée**, pour qu'un fichier restauré soit revalidé |

> ⚠️ **Les verdicts NÉGATIFS se mettent en cache aussi.** Un fichier corrompu qui ne serait pas
> mémorisé serait re-parsé intégralement à chaque PONG — exactement le coût que ce cache existe
> pour éviter, dans le cas précis où il est le plus élevé.

Le cache est borné par le nombre de questions portant un son : **aucune éviction n'est nécessaire**.
Une limite connue et acceptée : sur un système de fichiers dont le `mtime` a une granularité d'une
seconde, un fichier remplacé **dans la même seconde et à taille identique** passerait inaperçu — cas
de figure sans portée pratique ici, l'éditeur régénérant un nom de fichier aléatoire à chaque
téléversement.

> ⚠️ **La validation ne doit JAMAIS s'exécuter sous le verrou du moteur.** `internal/game`
> n'importe ni `internal/audio` ni `internal/config` : la disponibilité est calculée **par la
> couche applicative**, puis poussée dans le `GameState`, d'où `participantsConform` la lit. Un
> rappel injecté qui ferait de l'entrée/sortie disque depuis `reevaluatePrepareReadyUnsafe`
> tiendrait `e.mu` pendant une lecture de fichier — c'est l'implémentation naïve à refuser en revue.

#### 10.8.4 Où la gate vit — réutilisation, pas nouveau chemin

La gate est **une branche de `participantsConform`**, le prédicat qui pilote déjà
`PREPARE ↔ READY` (#172) et qui absorbe **déjà** des conditions étrangères aux participants
(catégories et difficultés RAFALE), avec sa propre justification : *« a second, parallel gating
function would duplicate the `reevaluatePrepareReadyUnsafe`/`ForceReady` wiring for no real
benefit »*.

- **Aucun nouveau chemin de refus n'est créé.** Les trois points de jonction existants consomment
  déjà ce prédicat.
- `Engine.Start()` refuse déjà toute phase ≠ `READY` (#172 B4) : **le contrôle serveur est acquis**.
- `participantsConform` est **pur sur `(Question, GameState)`** et doit le rester.
- **Une question sans son n'est jamais affectée** : la branche sort immédiatement si
  `Question.SOUND` est vide. La valeur zéro reste le comportement d'avant, sur **tous** les types.

#### 10.8.5 Le motif remonte comme un état, jamais comme une erreur

`GAME.QUESTION_SOUND_UNAVAILABLE` : `""` | `DISABLED` | `OUTPUT` | `FILE`, **sans `omitempty`**.

> ⚠️ **Diffusion — corrigé (dev-backend, 2026-09-23) : Admin/TV/VPlayer/Anim, pas « admin seul ».**
> `/anim` doit afficher ce motif (CA26 : « /anim affiche le motif mais n'offre aucune échappatoire »,
> `prepareWaitReason.js`/`AnimConductPanel.jsx`) — or `/anim` est routé vers exactement le même
> sérialiseur que TV/VPlayer (`SerializeForWebClient`), et ce lot ne touche pas
> `internal/protocol/messages.go` ni la sérialisation pour introduire une distinction. Le champ
> voyage donc avec le `GameState` ordinaire, comme `QUESTION_SOUND_STATE`/`ANSWER_TIMER_WAITING` —
> voir `contracts/game-state.md` pour la matrice complète et le détail du raisonnement. TV/VPlayer le
> reçoivent aussi, sans jamais l'afficher (aucune donnée confidentielle en jeu).

**Aucun canal d'erreur WebSocket n'est créé** — il n'en existe aucun pour
`admin`/`anim`, et la politique contractuelle est « journalisé en `WARN`, rien renvoyé au client ».
Le serveur publie le **fait** ; `web/src/utils/prepareWaitReason.js` en fait un libellé, comme il le
fait déjà pour les participants. Le fait doit venir du serveur parce que deux des trois motifs sont
**indécidables côté client**.

Les libellés **doivent nommer le remède réel**, parce qu'il est contre-intuitif : réactiver les sons
ou brancher une enceinte ne suffit pas, il faut **redémarrer le serveur** (§4 amendement #230 ;
§6.3).

#### 10.8.6 Échappatoire — le geste existant sur la question (arbitrage GATE 2, corrigé 2026-09-23)

Le blocage reste le **comportement par défaut**. Mais §10.8.8 décrit deux façons réelles de
verrouiller un quiz entier en direct ; une sortie de secours est donc nécessaire.

**Forme imposée : le `Ctrl`+clic DÉJÀ EXISTANT sur la question**, en phase `PREPARE` — celui qui
force le passage en `READY` (`Engine.ForceReady()`). **Ni un nouveau geste, ni une action sur le
bouton LANCER** : le contournement se joue à la frontière `PREPARE → READY`, là où vit la gate, et
non au lancement.

> ⚠️ **`ForceReady()` ne contourne PAS `participantsConform` aujourd'hui — et ne doit pas se mettre
> à le faire.** Il marque les buzzers prêts, puis **vérifie explicitement** la conformité et refuse
> si elle n'est pas atteinte. L'arbitrage #172 B5 en donne la raison : *« ForceReady only skips the
> PONG wait — it must not also skip participant-selection conformity, or the original bug (a
> non-conform MEMORY/MEMOTION round reaching STARTED) would return through this debug/admin door. »*
>
> **Exigence normative** : le contournement est **limité à la branche son**. `ForceReady()` lève un
> drapeau (`GAME.SOUND_GATE_BYPASSED`) que **seule** la branche son de `participantsConform` lit ;
> le refus existant sur les autres branches — participants MEMORY/MEMOTION, catégories et
> difficultés RAFALE — reste **intact**. Supprimer ce refus pour « simplifier » rouvrirait le bug
> que #172 B5 a fermé.

Trois propriétés qui en découlent, toutes acquises sans code supplémentaire :

1. **Admin uniquement** — `FORCE_READY` est déjà réservé à `ClientTypeAdmin` dans la liste blanche
   entrante. `/anim` affiche le motif mais n'offre pas le contournement, ce qui est cohérent : le
   geste est de toute façon impossible sur une tablette tactile.
2. **Aucun changement d'interface** — le geste existe, est câblé, et appartient déjà à une famille
   de contournements par modificateur (forcer `READY`, simuler un PONG, simuler un appui, forcer
   une mise à jour OTA). Il reste ainsi **non découvrable**, ce qui est voulu.
3. **Le bouton LANCER n'est pas modifié** — il garde son `disabled` natif, comme pour toutes les
   autres causes d'inactivité.

Le contournement **ne vaut que pour la question en cours** : il est remis à zéro à la sélection
d'une question (`Ready()`) et à l'arrêt (`stopUnsafe()`). Un clic simple sur la même question le
reprend donc à zéro.

Une question forcée joue **sans son**, et son chronomètre **ne** se met **jamais** en attente (le
mode différé n'a plus d'objet) : c'est la dégradation de §10.7, atteinte délibérément.

#### 10.8.7 Rafraîchissement et réversibilité

La disponibilité est recalculée **avant** chaque évaluation `PREPARE ↔ READY` — donc à chaque PONG —
ainsi qu'à la sélection d'une question, sur changement de configuration (`OnConfigUpdate`) et juste
avant `Engine.Start()`. Le cache du §10.8.3 rend ce rythme soutenable.

**Réversibilité, normative** : dès que la condition redevient vraie, la question repasse
**automatiquement** en `READY`, sans geste supplémentaire — strictement symétrique du retour arrière
`READY → PREPARE` de #172.

#### 10.8.8 ⚠️ Ce que cette gate rend coûteux — à connaître avant de l'implémenter

Deux propriétés **préexistantes** changent de gravité en devenant des conditions de lancement :

- **L'indisponibilité de sortie est définitive pour la vie du processus.** Le contexte `oto` est un
  singleton `sync.Once` qui **mémorise son erreur** ; il n'existe aucune boucle de reconnexion
  (§4, amendement #230). Une enceinte absente ou lente **au démarrage du serveur** rend donc toutes
  les questions à son injouables **jusqu'au redémarrage**, même une fois l'enceinte branchée.
- **Rallumer le son exige un redémarrage** (§6.3).

Avant cette section, ces situations coûtaient une question **silencieuse** ; elles coûtent désormais
une question **injouable**. C'est la raison d'être de l'échappatoire du §10.8.6, et
`docs/ADMIN_GUIDE.md` doit porter l'avertissement. Une re-sonde du contexte audio est **hors
périmètre** — elle rouvrirait la conception de #228.

### 10.9 Puce d'alerte globale — signal complémentaire, jamais bloquant (2026-09-23)

> **Besoin** : voir d'un coup d'œil qu'un quiz contient des questions sonores alors que l'audio
> n'est pas disponible, **sans** avoir à les découvrir une par une.
> **Ne remplace pas §10.8** — les deux coexistent. La puce dit « l'audio ne marchera pour aucune
> question » ; la gate dit « celle-ci a un problème ».

#### 10.9.1 Condition — deux critères globaux, jamais le fichier

La puce apparaît si, et seulement si :

1. le quiz contient **au moins une question** portant un `SOUND` ; **et**
2. l'audio n'est **pas** disponible globalement — `sound.enabled` faux **ou** sortie neutre.

Le second critère est **exactement** ce que `GET /api/sound/status` fusionne déjà en un booléen
(§Sound, « deux états, et c'est tout ») : la puce **consomme un contrat existant**, elle n'en crée
pas.

> ⚠️ **Le critère « fichier » de §10.8.2 ne s'applique PAS ici.** L'y inclure serait une erreur de
> granularité : il faudrait un contrôle disque par question pour alimenter un signal global, alors
> que ce cas est **déjà couvert question par question** par la gate, qui bloque et nomme le motif
> `FILE`. Les deux signaux se répartissent proprement.

#### 10.9.2 État dérivé — aucun calcul serveur, aucun endpoint nouveau

Les deux moitiés de la condition sont **déjà disponibles côté client** : la diffusion `QUESTIONS`
porte toutes les questions à l'admin, et `useSoundStatus()` interroge déjà
`GET /api/sound/status`. **Aucun agrégat serveur n'est à construire, aucune interrogation
supplémentaire n'est à ajouter.**

La puce étant un **état dérivé** et non stocké, elle **disparaît d'elle-même** dès que l'audio
redevient actif ou qu'aucune question ne porte plus de son : aucun cycle de vie, aucune remise à
zéro, aucune puce fantôme.

*Conséquence de périmètre* : `QUESTIONS` est diffusée à l'**admin seul**. La puce ne peut donc pas
exister sur `/anim` — sans gravité, l'animateur ne pouvant ni réactiver le son ni retirer un média.

#### 10.9.3 ⚠️ La pastille de menu « Ambiance » n'est PAS détournée

L'implémentation la plus courte — faire passer le haut-parleur du menu en variante « alerte » —
**contredit une décision explicite** (#234, `web/src/utils/soundState.js`) : *« DEUX formes
seulement, jamais une troisième « alerte » […] inactif — un choix normal de l'utilisateur, pas une
panne. »*

Cette décision reste **juste** : « son inactif » n'est pas une anomalie en soi. Le signal de cette
section est différent — « son inactif **alors que ce quiz en a besoin** », une incohérence entre le
contenu et l'environnement. Deux sujets, deux indicateurs.

**Exigence normative** : la puce est un composant **distinct**, et `soundStateGlyph` **reste à deux
formes**. Conflater les deux ferait dire deux choses à la même icône.

#### 10.9.4 Emplacement et libellé

Un composant partagé, monté sur la page **Quiz** (là où l'on agit : retirer les sons, voir
lesquelles en portent) **et** sur **`/admin`** (là où l'on joue, avant même de sélectionner une
question) — patron `RafalePoolAlert`, un composant pour deux surfaces.

Le libellé **nomme le remède**, aussi contre-intuitif ici qu'au §10.8.5 : réactiver le son ou
brancher l'enceinte **exige un redémarrage du serveur** (§10.8.8).

---

## 11. Son coupé (`sound.muted`) — réglage ponctuel du popover « Ambiance » — normatif (v11.4.0, #286, LOT-1)

> **Issue** : #286 (milestone v11.4.0, recadrée au GATE 1.5 du 2026-11-05). **Plan** :
> `_work/reports/planner-20261105-110000.md` §4.2. **Maquette** : `admin_navbar__ambiance_popover.html`.
> Section **additive** : §2 à §10 inchangés au bit près.

### 11.1 Pourquoi ce n'est PAS `sound.enabled`

`sound.enabled` est **asymétrique** (§6.3 : éteindre est immédiat, rallumer exige un redémarrage) **et** alimente
la gate T0 (§10.8) : à `false`, toute question portant un son est **empêchée de démarrer** (`DISABLED`). Un
réglage de régie « couper le son maintenant, le rendre tout à l'heure » ne peut remplir ni l'un ni l'autre de ces
rôles. C'est donc une **troisième garde**, indépendante des deux autres :

| Garde | Où | Portée | Rallumage |
|---|---|---|---|
| `sound.enabled` (§6.3) | `config.json` | construction du moteur + cues + média | redémarrage |
| `CuesDisabled` (§6.3) | `config.json` | une cue | immédiat |
| **`sound.muted`** (ici) | `config.json`, **persisté** (décision utilisateur Q2) | cues + média de question + son de décompte (§12, LOT-3) | immédiat |

### 11.2 Sémantique — normatif

- **Champ** : `SoundConfig.Muted bool \`json:"muted,omitempty"\`` — **on stocke ce qui est coupé** : valeur zéro
  (clé absente) = son actif = comportement actuel, aucune migration (même discipline que `CuesDisabled`).
  Persisté : un redémarrage **conserve** le son coupé — c'est pourquoi l'en-tête admin le montre en permanence
  (§11.4), seul garde-fou contre une soirée muette oubliée.
- **Écriture** : `POST /config.json {"sound": {"muted": true|false}}` — fusion champ par champ existante de
  `handleConfig` (§6.3 « Fusion champ par champ »), **aucun endpoint dédié** (même règle que `enabled` et
  `cues_disabled`). **Effet de bord à l'écriture** (dans `handleConfig`, après persistance, via le rappel
  `OnSoundMutedChange(muted bool)` de l'`App`) : passage à `true` ⇒ `questionSound().Stop()` (arrêt net,
  `QUESTION_SOUND_STATE` → `IDLE`, chronomètre différé libéré) **et** arrêt du son de décompte (§12). Passage à
  `false` ⇒ **rien n'est repris** (un `Rejouer` manuel reste possible).
- **Lecture** : `GET /api/sound/status` → `{active, muted}` (`http-endpoints.md` §Sound). Lue à l'appel,
  jamais en cache.
- **Cues** : `notifySound` (`cmd/server/sound.go`) gagne **une** garde, en tête :
  ```go
  func (a *App) notifySound(c audio.Cue) {
      sc := config.Get().Sound
      if sc.Muted { return }
      if !sc.Enabled { return }
      if sc.CuesDisabled[string(c)] { return }
      a.sound().PlayCue(c)
  }
  ```
  Corps existant inchangé ; le test-garde AST du §6.2 reste valide (même sélecteur).
- **Média de question (T1)** : dans `questionSoundAdapter.play`, `Muted` est traité **comme `DISABLED` à T1** :
  aucun `Play`, chronomètre différé libéré (`ReleaseDeferredTimer`, règle de non-blocage §10.7). **La gate T0
  (`questionSoundAvailability`, §10.8) ne lit jamais `Muted`** : une question sonore démarre normalement quand le
  son est coupé — `QUESTION_SOUND_UNAVAILABLE` reste `""`. Test exigé : muet, PREPARE→READY passe et le différé
  est libéré immédiatement à STARTED.
- **`POST /api/sounds/{cue}/test` ignore `Muted`** — geste explicite de l'utilisateur, même règle que `CuesDisabled`.
- **Son de décompte** (§12, LOT-3) : lit la même garde avant tout `Play`.

> **Amendement v11.6.0 (#290, LOT-4)** — `Muted`, `CuesDisabled`, `CountdownDisabled`, `CountdownOnce` ne sont plus
> lus directement dans `config.Get()` par `notifySound`, `question_sound.go` et `countdown_sound.go` : ils sont lus
> **résolus** (global ← manche ← geste direct) par l'accesseur unique `a.ambianceSettings()` —
> `contracts/ambiance-settings.md` §4–§5 (le corps de `notifySound` ci-dessus est remplacé par celui du §5.2 de ce
> contrat ; les effets de bord d'un passage à « coupé » (§11.2) sont déclenchés par la **résolution**, pas seulement par
> l'écriture de `config.json`). La persistance du geste « son coupé » est conservée **hors surcharge de manche** (D1′).

### 11.3 Interface — popover « Ambiance » de l'en-tête (ex-« Éclairage »)

- Le bouton « 💡 Éclairage » de `Navbar.jsx` devient **« Ambiance »** et son popover regroupe **deux sections** :
  « Éclairage général » (`LightingModePanel` inchangé, ON / AUTO / OFF + Flash) et **« Son »** — sélecteur
  `role="radiogroup"` à **deux positions ON / OFF** (le son n'a pas d'état automatique), avertissement quand coupé
  (il dit que le réglage survit au redémarrage), écriture par `POST /config.json`, `refresh()` de
  `useSoundStatus` après réponse, jamais de mise à jour optimiste.
- **Visibilité** : bouton présent dès qu'un pont Hue est configuré **ou** que `status.active` est vrai ; chaque
  section n'apparaît que si sa fonction existe. Éclairage d'ambiance désactivé (`lighting.enabled=false`, §11.5) :
  section Éclairage grisée avec renvoi vers Ambiance › Lumière.
- **Libellé du bouton** : « Ambiance » au repos ; « Mode ON/OFF engagé » et/ou « Son coupé » quand un réglage est
  engagé ; glyphes 💡 et 🔊/🔇 ; badge orange (rouge si lumière OFF).
- **Popover** : classe commune `.navbar-popover` (`left: 0`, `white-space: normal`, `max-width` bornée au viewport)
  — correctif #284.

### 11.4 Interrupteur général de l'éclairage d'ambiance — page Ambiance › Lumière (#286, partie lumière)

Nouveau bloc en tête de l'onglet Lumière : « Éclairage d'ambiance activé », **persisté** — c'est le champ existant
`lighting.enabled` (`LightingConfig`), écrit par `POST /config.json {"lighting": {"enabled": bool}}` (fusion
existante). Sémantique, normative : à `false`, **aucune scène n'est envoyée aux ampoules** (pilote détaché,
`a.ambiance()` sans pilote — chemin existant « non configuré »), les commandes ON / AUTO / OFF / Flash répondent
`409 not_configured` (déjà le cas sans pilote) et l'en-tête grise sa section Éclairage ; au basculement à `false`,
la zone `general` et les ampoules d'équipe sont **éteintes** (même geste que l'arrêt du serveur, `lighting.md`
§10.4) ; à `true`, le pilote est reconstruit depuis la configuration et `NotifyState()` ré-applique la scène
courante. Associer un pont force `enabled=true` (comportement existant, `http_lighting.go`). Ce champ est
distinct du mode ON/AUTO/OFF (non persisté, §10.1.1 de `lighting.md`).

### 11.5 Hors de cette section

Reconstruction à chaud du moteur son (rallumer `sound.enabled` sans redémarrage) : toujours rejetée (§6.3).
Atténuation (volume) : hors périmètre — le réglage est binaire.

---

## 12. Son de décompte — joué pendant chaque chronomètre de réponse — normatif (v11.4.0, #287, LOT-3)

> **Issue** : #287 (milestone v11.4.0). **Plan** : `.claude/workflow/plan-11.4.0.md` §5 LOT-3. **Décisions utilisateur**
> (GATE 1.5, 2026-11-05, `docs/mockup/DECISIONS.md` « admin_sounds_manager ») : joué **au début** de chaque
> chronomètre ; rejoué **en boucle** seulement si le commutateur « en boucle » est actif ; **tous** les chronomètres
> (global SPEEDY/QCM/ARDOISE, **manche** RAFALE, carte MEMOTION) ; **jamais si la question a un média audio** ;
> respecte « Son coupé » (§11) et « Bruitages activés ». **Révision 2026-11-05 (décisions utilisateur, GATE 2 LOT-3)** :
> la sonorisation RAFALE est **par manche, pas par question** ; **au buzz tout continue** (décompte et médias) ;
> exclusion dès qu'un média est attaché, différé ou non. **Maquette normative** :
> `docs/mockup/v11.4.0/ui/admin_sounds_manager__countdown_loop.html` (§01).
> Section **additive** : §1–§11 inchangés sauf amendements explicitement notés (§10.2, §10.6, §10.7).

### 12.1 Nature — un troisième chemin audio, ni cue ni média de question

Le décompte est un **son long** (≤ 30 s, format du §10.4), rejoué éventuellement en boucle, arrêté par un
évènement de jeu. Ce n'est **pas une cue** (§2.2 : une cue est une impulsion sans état, jamais arrêtée — `audio.Cue`
reste un catalogue fermé de 7 valeurs, `synth.Cues` et `soundCueFromString` **inchangés**) et ce n'est **pas le média
de question** (§10 : un média appartient à une question ; le décompte appartient à la configuration serveur et joue
sur toutes les questions). Il a donc son propre adaptateur, `cmd/server/countdown_sound.go`, bâti sur un **second**
`audio.MediaPlayer` (§10.2), adossé au **même** contexte `oto` partagé (§10.3 — `sharedOtoContext`, deux
`oto.Player` sur un contexte se **mixent**, c'est ce que `oto` garantit et ce que le spike #226 a vérifié).

| Chemin | Objet | Instance | Fichier | Appelle |
|---|---|---|---|---|
| Cues (§5) | impulsions | `audio.Engine` | `cmd/server/sound.go` | `notifySound` |
| Média de question (§10) | son d'une question | `audio.MediaPlayer` n°1 | `cmd/server/question_sound.go` | `questionSound()` |
| **Décompte (§12)** | son pendant le chrono | **`audio.MediaPlayer` n°2** | **`cmd/server/countdown_sound.go`** | **`countdownSound()`** |

**Frontière (même discipline que §10.5, vérifiée par test, jamais par discipline)** : `countdown_sound.go` n'appelle
jamais `notifySound`/`PlayCue` (extension de `sound_sites_test.go` : le sélecteur `notifySound` n'apparaît pas dans
ce fichier ; le registre `soundSiteRegistry` ne gagne **aucune** entrée) ; `question_sound.go` et `countdown_sound.go`
ne s'importent ni ne s'appellent (test AST : aucun sélecteur `countdownSound` dans `question_sound.go`, aucun
`questionSound` dans `countdown_sound.go`). Le seul lieu où les trois chemins se rencontrent est `main.go`.

### 12.2 Fichier et valeur par défaut

- **Chemin** : `data/files/sounds/decompte.wav` — même dossier que les cues, constante dédiée
  `synth.CountdownName = "decompte"` (type `string`, **jamais** un `audio.Cue`), `synth.CountdownFilename()`.
- **Format et limites** : ceux du média de question, §10.4 (WAV canonique, mono accepté et stocké stéréo, ≤ 30 s,
  ≤ 6 Mio), validés par **`audio.ValidateQuestionSound`** — jamais `ValidateUpload` (limite 5 s des cues), jamais un
  troisième validateur.
- **Défaut synthétisé** : `synth.GenerateCountdown() []byte` — tic-tac de **2,00 s** (deux « tic » brefs par seconde,
  ~1 kHz/~800 Hz alternés, enveloppe du §`tone.go`, amplitude ≤ 0,5 pour rester sous les cues), écrit par
  `synth.WriteAll` au démarrage **si absent** (même règle « jamais écraser » que les cues), restauré par
  `POST /api/sounds/decompte/restore` et par `POST /api/sounds/restore-defaults` (qui couvre désormais **7 + 1**
  fichiers — maquette §01 « Restaurer tout (7 bruitages + décompte) »).
- **Manifeste** `sounds.json` : `synth.ReconcileManifest` gagne l'entrée `"decompte"` (clé = nom de fichier sans
  extension, `Custom` = octets ≠ `GenerateCountdown()`). Le type de la map devient `map[string]ManifestEntry`
  (clés `string(cue)` pour les cues) — adaptation mécanique de `handleAPISoundsList` et de `manifest_229_test.go`
  (indexation par `string(c)`), **aucun changement de sémantique** pour les cues.

### 12.3 Configuration — `SoundConfig`, valeur zéro = actif, en boucle

```go
// internal/config/config.go — SoundConfig
CountdownDisabled bool `json:"countdown_disabled,omitempty"` // zéro = décompte ACTIF (défaut)
CountdownOnce     bool `json:"countdown_once,omitempty"`     // zéro = EN BOUCLE (défaut, décision #288) ; true = une seule lecture
```

- **On stocke l'écart au défaut** (même discipline que `CuesDisabled`/`Muted`) : un `config.json` antérieur donne
  « actif, en boucle » sans migration ni `ApplyDefaults`.
- **Écriture** : `POST /config.json {"sound": {"countdown_disabled": bool}}` et/ou `{"countdown_once": bool}` —
  fusion champ par champ de `handleConfig` (clé présente ⇒ écrite, absente ⇒ inchangée, même mécanisme que `muted`),
  **aucun endpoint dédié**. Effet de bord : passage de `countdown_disabled` à `true` ⇒ `countdownSound().Stop()`
  (rappel `OnSoundCountdownDisabledChange(disabled bool)` de l'`App`, voisin de `OnSoundMutedChange`). Un changement
  de `countdown_once` **ne touche pas** une lecture en cours (pris en compte au prochain démarrage de chrono).
- **Lecture** : `GET /api/sounds` → tableau `long` (`http-endpoints.md` §Sound, LOT-3) ; `GET /api/sound/status`
  **inchangé**.

### 12.4 Gardes de lecture — dans l'ordre, avant tout `Play`

```go
func (cs *countdownSoundAdapter) mayPlay(start game.AnswerTimerStart) bool {
    sc := config.Get().Sound
    if sc.Muted { return false }                 // §11 — son coupé
    if !sc.Enabled { return false }              // « Bruitages activés » éteint : moteur non démarré (§6.3)
    if sc.CountdownDisabled { return false }     // §12.3
    if start.QuestionSound != "" { return false }     // §12.6 — jamais si la question a un média audio
    return true
}
```

- `POST /api/sounds/decompte/test` **ignore** `Muted`, `Enabled`-éteint et `CountdownDisabled` au même titre que
  `/api/sounds/{cue}/test` (§11.2) — geste explicite ; il joue le fichier **une fois** (jamais en boucle), via
  `countdownSound().Test()` qui **remplace** une lecture de décompte en cours (voix unique) sans toucher au jeu.
- Lecteur neutre (`IsNeutralMedia`), fichier absent ou non conforme ⇒ dégradation silencieuse §5.5 : aucun son,
  aucune erreur de jeu, **aucun effet sur le chronomètre** (le décompte n'a pas de différé — il n'a rien à libérer).

### 12.5 Déclenchement — « le démarrage effectif d'un chronomètre »

Le décompte est piloté par **le moteur**, jamais par un changement de phase (une phase `STARTED` sans ticker — différé
§10.7, MEMOTION avant le premier retournement — ne joue rien). Nouveau rappel de `game.Engine` :

```go
// internal/game/engine.go
type AnswerTimerKind string

const (
    AnswerTimerGlobal     AnswerTimerKind = "GLOBAL"      // startTimer() — SPEEDY/QCM/ARDOISE, et la MANCHE RAFALE (voir 12.7)
    AnswerTimerMotionCard AnswerTimerKind = "MOTION_CARD" // StartMotionCardTimer — carte MEMOTION (retournement, carte RAFALE comprise)
)

type AnswerTimerStart struct {
    Kind          AnswerTimerKind
    Seconds       int    // durée programmée (> 0, sinon le rappel n'est pas émis)
    QuestionSound string // state.Question.Sound au moment du démarrage ("" si aucun)
}

// OnAnswerTimerStarted fires whenever a response timer ACTUALLY starts ticking.
// May be invoked WITH e.mu held (startTimer() runs under the lock): the
// callee must never call back into the Engine synchronously and must not
// block — read config, drive the MediaPlayer, return.
OnAnswerTimerStarted func(start AnswerTimerStart)

// OnMotionCardTimeUp fires when a MEMOTION card timer reaches 0 (the phase
// stays STARTED — admin must act — so no OnStateChange/OnTimeUp exists
// for this expiry today). Fired outside the lock, like OnTimerTick.
OnMotionCardTimeUp func()
```

**Sites d'émission (exhaustifs, vérifiés par test)** :

| Site moteur | Kind | Remarque |
|---|---|---|
| `startTimer()` (ses 3 appelants : `actualStart` simultané, `releaseDeferredTimerUnsafe`, `Continue()` après libération pendant la pause) | `GLOBAL` | une seule instrumentation, dans `startTimer()` — **pas** dans les appelants ; émis sous `e.mu`. Couvre la **manche RAFALE** (`actualStart` démarre le chrono de manche par ce même `startTimer()`) |
| `StartRafaleQuestionTimer(seconds)` | — | **jamais** (décision utilisateur 2026-11-05 : « la sonorisation est faite pour la manche, pas pour chaque question rapide ») — le ticker par question n'est pas instrumenté |
| `StartMotionCardTimer(delay)` après `delay > 0` | `MOTION_CARD` | retournement de carte (`handleMotionFlip`), carte RAFALE comprise (son chrono `RafaleRoundTime` est le chrono de manche de la carte) |
| `StartMotionMemorizeTimer` | — | **jamais** : la mémorisation n'est pas un chronomètre de réponse |
| Phase `COUNTDOWN` (3-2-1 avant le START, `OnCountdownTick`) | — | **jamais** : « décompte » au sens de ce contrat = pendant le chrono de réponse, pas avant |

### 12.6 Exclusion « média audio pendant le chrono »

**« Jamais si média »** (décision utilisateur 2026-11-05) : un décompte **n'est jamais joué** pour une question qui porte
un média audio (`SOUND != ""`), quel que soit son réglage — simultané, différé (§10.7) ou en boucle (§13). L'exclusion se
décide sur la **configuration de la question** (`AnswerTimerStart.QuestionSound`), jamais sur l'état réel du lecteur n°1
(un média muet, absent ou en échec ne « libère » pas le décompte — règle simple, prévisible, testable sans matériel).
Conséquence : `handleQuestionSound` (Rejouer) n'a **rien** à faire du décompte — il n'y en a jamais sur une question
sonore.

### 12.7 Chevauchements — voix unique, chrono le plus intérieur

Le lecteur n°2 est **à voix unique** (§10.2) : tout `Play`/`PlayLoop` remplace la lecture en cours. Règle :
**un chronomètre de manche ou de carte = une lecture** ; chaque `OnAnswerTimerStarted` relance le son depuis le début.
Le ticker **par question** RAFALE n'a aucune voix (12.5).

| Situation | Chronomètres actifs | Comportement |
|---|---|---|
| SPEEDY/QCM/ARDOISE | global | joue au `startTimer()`, s'arrête aux évènements de 12.8 |
| Manche RAFALE classique | global (manche) **+** question | **une seule lecture pour toute la manche**, démarrée au `startTimer()` de `actualStart`, en boucle ou une fois selon `countdown_once` ; les questions qui s'enchaînent (validation, invalidation, expiration) **ne la relancent ni ne l'arrêtent** ; `CuePerdu` (invalidation) part sur le chemin des cues et se **mixe** ; fin de manche (expiration `OnTimeUp`, plafond, épuisement ⇒ `STOPPED`) ⇒ arrêt |
| Carte MEMOTION (SPEEDY/QCM/MEMORY) | carte | joue au retournement (`StartMotionCardTimer`), s'arrête à l'expiration (`OnMotionCardTimeUp`), à la résolution de la carte (`StopMotionCardTimer`, sites de `main.go` : done/validate/reveal/close) ou aux évènements de 12.8 |
| Carte MEMOTION RAFALE | carte (`RafaleRoundTime`) **+** question | une lecture au `StartMotionCardTimer` de la carte (si `RafaleRoundTime > 0`, sinon aucune) ; les questions de la carte ne relancent rien ; expiration ou résolution de la carte ⇒ arrêt |
| Mémorisation (`MEMORIZE`) puis retournement | mémorisation, puis carte | rien pendant la mémorisation ; joue au retournement |
| Question avec média (§10/§13) | — | aucun décompte, 12.6 |
| Buzz (`PhasePaused` interne) | chrono figé par le moteur | **tout continue** (décision utilisateur) : décompte, média, boucle — ni arrêt ni pause ; à l'expiration ou à la fin de la question, arrêt normal (12.8) |
| PAUSE manuelle / CONTINUE | chrono figé | décompte **mis en pause** (`Pause()`) puis **repris** (`Resume()`) exactement comme le média de question (`ActionPause`/`ActionContinue`, §10.7) — jamais relancé depuis le début |

### 12.8 Arrêt — tout évènement qui termine ou suspend le chronomètre courant

`countdownSound().Stop()` est appelé depuis `main.go` sur **chacun** des sites suivants (test exigé : chaque site,
isolément, arrête un décompte en boucle) :

| Évènement | Site `main.go` | Déjà site de `questionSound().Stop()` ? |
|---|---|---|
| Temps écoulé (global, manche RAFALE) | `OnTimeUp` | oui |
| Expiration d'une question RAFALE | **rien** (12.7) ; fin de manche par plafond ou épuisement ⇒ `e.Stop()` **interne** au moteur, sans handler `ActionStop` ⇒ `OnStateChange(STOPPED)` est le **seul** signal : câblé pour cette raison (voir règle ci-dessous) | — |
| Expiration d'une carte MEMOTION | `OnMotionCardTimeUp` (nouveau) | — |
| Buzz (`Engine.Pause()` interne, `OnStateChange(PAUSED)`) | **aucun** — tout continue (décision utilisateur 2026-11-05) ; la branche `PAUSED` de `OnStateChange` ne touche **pas** au décompte | buzz : le média continue, inchangé |
| PAUSE manuelle / CONTINUE | `ActionPause` ⇒ `countdownSound().Pause()` ; `ActionContinue` ⇒ `countdownSound().Resume()` — à côté des appels `questionSound()` existants, mêmes gardes de phase | oui (Pause/Resume) |
| STOP, REVEAL, question (re)chargée (`Ready`), NEW_GAME | handlers `ActionStop`, `ActionReveal`, `handleQuestionReady`, `handleNewGame` | oui (mêmes lignes) |
| Résolution de carte MEMOTION | tous les appelants de `StopMotionCardTimer` | — |
| Son coupé (`muted` → `true`), décompte désactivé | `OnSoundMutedChange`, `OnSoundCountdownDisabledChange` | oui / — |

`Stop()`, `Pause()` et `Resume()` sont **idempotents** et sans effet sur le jeu (aucun champ `GameState`, aucun broadcast :
le décompte n'a pas d'état diffusé — §12.9). Ils **ne touchent jamais** le lecteur n°1. **Règle `OnStateChange`**
(précisée 2026-11-05, écart dev-backend) : `OnStateChange(STOPPED)` ⇒ `countdownSound().Stop()` — c'est le filet
pour toute fin de manche décidée **dans** le moteur (plafond/épuisement RAFALE via `e.Stop()` interne, sans handler
`ActionStop`) ; `OnStateChange(PAUSED)` ⇒ **rien** (ce serait l'arrêt au buzz, proscrit) ; `STARTED` ⇒ rien (le
démarrage est porté par `OnAnswerTimerStarted`, jamais par la phase). Les autres phases (`REVEALED`, `READY`,
`NEW_GAME`…) sont déjà couvertes par leurs handlers ; les traiter aussi dans `OnStateChange` est toléré (idempotence).

### 12.9 Hors de cette section

- **Aucun champ `GameState`** (pas de `COUNTDOWN_SOUND_STATE`) : rien à afficher, rien à synchroniser ; les clients ne
  savent pas qu'un décompte joue. `game-state.md` et `ws-payload-serialization.md` **inchangés**.
- Pas de volume ; pas de son de décompte **par question** (un seul fichier serveur) ; pas de décompte sur la phase
  `COUNTDOWN`, sur `MEMORIZE`, ni **par question RAFALE** (par manche seulement, 12.5/12.7).

### 12.10 Non-régression exigée

`sound_sites_test.go` (registre inchangé, aucun `notifySound` dans `countdown_sound.go`) ; `question_sound_*_test.go`
verts sans modification ; tests-gardes §10.3 (`play_blocks_228`, `output_228`, `cross_compile_228`,
`isneutral_230`, `oto_singleton_219`) verts sans modification ; `sound_muted_286_test.go` vert ; `config.json`
sans clé `countdown_*` ⇒ actif + boucle ; `GET /api/sounds.cues` **byte-identique** à v11.4.0 LOT-2 (le tableau `long`
est additif) ; `go test -race ./...`.

---

## 13. Lecture « une fois / en boucle » du média de question — normatif (v11.4.0, #288, LOT-3)

> **Issue** : #288. **Décision utilisateur** (GATE 1.5, `DECISIONS.md` « admin_sounds_manager ») : le commutateur
> une fois / en boucle est réservé au **média de question** et au **son de décompte** (§12.3 `countdown_once`), jamais
> aux bruitages. **Maquette** : `admin_sounds_manager__countdown_loop.html` §02 (bloc « Son de la question »).
> Amende §10.6 (« Lecture en boucle : hors v11.1 » — désormais **dans** le périmètre) et §10.2 (interface).

### 13.1 Interface `MediaPlayer` — amendement §10.2

```go
type MediaPlayer interface {
    Play(path string) error
    // PlayLoop (v11.4.0, #288) démarre la lecture et la REPREND au début à
    // chaque fin naturelle, sans fin : seul Stop/Close/un Play(Loop) ultérieur
    // y met un terme. onNaturalEnd est appelé EXACTEMENT UNE FOIS, à la fin
    // du PREMIER passage (« fin du son » au sens de §10.7), jamais aux
    // passages suivants. Voix unique comme Play. Implémentation neutre : no-op.
    PlayLoop(path string) error
    Pause()
    Resume()
    Stop()
    State() MediaState
    Close() error
}
```

- Implémentation `oto` : le `monitor` d'une lecture en boucle, à chaque fin naturelle détectée (`!IsPlaying()`),
  **reconstruit un `oto.Player`** sur la même `SectionReader` après `Seek(0, io.SeekStart)` (§10.2 : `Seek` **est** le
  mécanisme de rejeu), **sans changer de génération** et en conservant la référence forte (§10.3) ; `State()` reste
  `PLAYING` d'un tour à l'autre. Un `Pause()` pendant la boucle suspend ; `Resume()` reprend où elle en était.
- **Évènement « fin du 1ᵉʳ passage »** : à la **première** fin naturelle seulement, le `monitor` appelle `onNaturalEnd`
  (sans verrou, comme pour `Play`) **puis** relance le passage suivant ; un compteur de passages par génération
  (`passes`) garantit l'unicité. Un `Stop`/`Play`/`Close` avant la fin du 1ᵉʳ passage ⇒ jamais d'`onNaturalEnd` (règle
  `Play` inchangée : l'arrêt manuel est la responsabilité de l'appelant, qui libère lui-même le différé). Une `Pause()`
  pendant le 1ᵉʳ passage en retarde la fin d'autant. Test exigé (fake horloge/lecteur) : 3 passages ⇒ 1 seul
  `onNaturalEnd`, émis à la fin du 1ᵉʳ ; `Stop` pendant le 2ᵉ ⇒ aucun appel supplémentaire.
- `neutralMediaPlayer.PlayLoop` = no-op. Les **faux lecteurs de test** (`tw219qFakeMediaPlayer`,
  `tw219dFakeMediaPlayer`, et tout fake `test-writer`) gagnent la méthode — c'est le seul coût de l'extension, assumé.
- La note « Ne pas l'étendre » du commentaire de `media.go` est **remplacée** par le renvoi à cette section.

### 13.2 Modèle et upload — `SOUND_LOOP`

```go
// internal/game/models.go — Question (hors TypedContent, commun à tous les types comme SOUND, §10.4bis)
SoundLoop bool `json:"SOUND_LOOP,omitempty"` // false (zéro) = une fois = comportement v11.1
```

- Multipart `POST /questions` : champ `sound_loop` (`"true"`/absent), réécrit à chaque enregistrement (même discipline
  que `sound_timer_delayed`), **écrit dans `SOUND_LOOP` uniquement si un son est effectivement attaché** (préservé ou
  téléversé). Voir `http-endpoints.md` §Questions.
- **Boucle × différé — combinables (décision utilisateur 2026-11-05)** : `sound_loop=true` **et**
  `sound_timer_delayed=true` ⇒ les deux champs sont écrits ; à la lecture, le chronomètre démarre à la **fin du premier
  passage** (évènement `onNaturalEnd` de `PlayLoop`, §13.1 — c'est « la fin du son » de §10.7, libération idempotente
  et irréversible inchangée) et le média **continue en boucle** pendant le chronomètre. Le chien de garde §10.7
  (`MaxQuestionSoundDuration + 2 s`) reste la borne : un 1ᵉʳ passage ne peut pas dépasser 30 s. Aucun `400`, aucune
  règle de priorité.
- **Éditeur** (`QuestionsPage.jsx`, bloc « Son de la question », maquette §02 **amendée** : plus de ligne désactivée ni
  d'avertissement) : nouvelle ligne « Lecture : Une fois / En boucle » (`role="switch"`, `aria-label="Jouer le son en
  boucle jusqu'à la fin du chronomètre"`, patron `.quiz-switch` **identique** à « Chrono démarre »), **sous** la ligne
  « Chrono démarre », désactivée seulement quand **aucun son** n'est attaché (comme « Chrono démarre ») ; conseils :
  `Le son est joué une fois, puis s'arrête.` / `Le son est rejoué jusqu'à la fin du chronomètre.` / avec « À la fin » **et**
  boucle : `Le chronomètre démarre à la fin du premier passage ; le son continue en boucle.` ;
  `data.append('sound_loop', …)` comme `sound_timer_delayed` ; défauts/chargement/reset de `formData.soundLoop` depuis
  `question.SOUND_LOOP`. Absent des types sans section « Son » (MEMORY, MEMOTION, RAFALE, ENTRACTE) — décision
  d'éditeur §10.4bis, pas de garde serveur.

### 13.3 Lecture

- `questionSoundAdapter.play` : `q.SoundLoop` ⇒ `player.PlayLoop(path)` au lieu de `Play` ; **tout le reste est
  inchangé** : gate T0 (§10.8), `Muted` ⇒ `DISABLED` (§11.2), `QUESTION_SOUND_STATE` (`PLAYING` jusqu'à l'arrêt),
  `onNaturalEnd` (fin du 1ᵉʳ passage) ⇒ `ReleaseDeferredTimer` comme pour `Play` — mais **sans** passer
  `QUESTION_SOUND_STATE` à `IDLE` tant que la boucle joue (l'adaptateur lit `player.State()` avant d'écrire l'état) ;
  chien de garde §10.7 armé à l'identique.
- « Rejouer » (`QUESTION_SOUND PLAY`) relance **selon le réglage** (boucle ⇒ boucle).
- **Arrêt** : la boucle s'arrête sur **tous** les sites actuels de `questionSound().Stop()` (temps écoulé, STOP,
  REVEAL, question rechargée, NEW_GAME, `muted`) — **aucun site nouveau**. **Au buzz, la boucle continue** (décision
  utilisateur 2026-11-05 : « au buzz tout continue » — la mention « ou au buzz » du conseil de la maquette §02 est
  **retirée** du texte livré ; `OnBuzzerPress` reste non câblé, #247). PAUSE manuelle : `Pause()`/`Resume()` inchangés,
  la boucle reprend où elle en était. Une boucle **ne survit jamais à la question** (test exigé : chaque site arrête
  une lecture `PlayLoop`).
- Décompte (§12) : jamais sur une question qui a un média, boucle comprise (§12.6).

### 13.4 Non-régression exigée

`SOUND_LOOP` absent ⇒ `Play` (un passage), octet pour octet ; différé sans boucle : `onNaturalEnd` à la fin du passage unique, inchangé ; `question_sound_219_test.go`,
`question_sound_delayed_219_test.go`, `question_sound_enabled_dev_257_test.go` verts (fakes étendus de la seule
méthode `PlayLoop`) ; `rafale_upload_nonregression_test.go` et fixtures `models_roundtrip_test.go` inchangés
(`SOUND_LOOP` est `omitempty`) ; `QuestionsPage.sound.test.jsx` vert.

---

## 14. Média audio sur MEMORY, MEMOTION (hôte **et** carte) et ENTRACTE — normatif (v11.6.0, #235, LOT-2)

> Décisions utilisateur GATE 1.5/GATE 2 du 2026-11-09 : **Q2** ENTRACTE incluse, boucle activable ; **Q3** « les deux » —
> son d'hôte MEMOTION au START **et** son par carte au retournement ; **D3** MEMORY au niveau `Question.SOUND` ;
> **D9** la carte remplace l'hôte, sans reprise. Maquette `docs/mockup/v11.6.0/ui/admin_question_editor__sound_memory_memotion_entracte.html`.

### 14.1 Ce qui ne change pas

`Question.SOUND`, `SOUND_TIMER_DELAYED`, `SOUND_LOOP` restent **communs à tous les types** (§10.4bis) : l'ouverture à
MEMORY, ENTRACTE et à l'hôte MEMOTION est **une décision d'éditeur** (`QuestionsPage.jsx` lève son filtre de type) — aucun
champ nouveau sur `Question`, aucune garde serveur à retirer puisqu'il n'y en a pas. Format, limites (§10.4), gate T0
(§10.8), chien de garde (§10.7), voix unique (§10.2) : inchangés.

### 14.2 MEMORY — son de manche

Joué au **START** (phase de mémorisation comprise), `Play`/`PlayLoop` selon `SOUND_LOOP` ; `SOUND_TIMER_DELAYED` diffère
le chronomètre global ; **amendement revue LOT-2 (m4)** : `SOUND_TIMER_DELAYED` est écrit **sans condition** sur
`MEMORY_CONFIG.USE_TIMER` (aucun changement moteur : l'effet repose sur le chrono global existant ; sans chrono global, le
champ est stocké mais sans effet — pas de refus, à vérifier en QUALIF) ;
arrêt sur les sites existants de `questionSound().Stop()`. Décompte (§12.6) : jamais pendant ce média.

### 14.3 ENTRACTE — son d'entracte

- Éditeur : bloc « 🔊 Son de l'entracte » avec **Lecture : une fois / en boucle** ; **pas** de « Chrono démarre » (aucun
  chronomètre) — `sound_timer_delayed` n'est pas envoyé pour une ENTRACTE, et s'il l'était il est **ignoré** (écrit `false`).
- Lecture : à l'**activation** de l'entracte (`ENTRACTE_SET{ACTIVE:true}` et entrée automatique par « à suivre », même
  site que `CueEntracteDebut`) → `Play`/`PlayLoop` **après** la cue `entracte-debut` (la cue est une impulsion sur le
  moteur de bruitages, le média est sur le `MediaPlayer` : deux chemins, aucune attente) ; **arrêt** à la sortie
  (`ENTRACTE_SET{ACTIVE:false}`, même site que `CueEntracteFin`), au STOP, au NEW_GAME, au RESTORE.
- `Muted` résolu (`ambiance-settings.md`) ⇒ `DISABLED` à T1 comme pour une question (§11.2). Gestes `QUESTION_SOUND`
  (PLAY/PAUSE/RESUME/STOP) **applicables** pendant l'entracte (`QUESTION_SOUND_STATE` suit).

### 14.4 MEMOTION — son d'hôte **et** son de carte (Q3)

**Modèle** — `MotionCard` gagne trois champs **communs de carte** (`question-types.md` §3.1, jamais `OwnedFields`,
conservés quel que soit le `TYPE` de la carte) :

```go
Sound             string `json:"SOUND,omitempty"`               // "/question/<qid>/motion_<cardID>_sound_<rand4>.wav"
SoundTimerDelayed bool   `json:"SOUND_TIMER_DELAYED,omitempty"` // différé du CHRONO DE LA CARTE
SoundLoop         bool   `json:"SOUND_LOOP,omitempty"`
```

**Upload** (`POST /questions`, `http-endpoints.md`) — champs multipart `motion_card_<cardID>_sound` (fichier, validé par
`ValidateQuestionSound` **après** `ToCanonical` §3), `motion_card_<cardID>_sound_cleared`, `_sound_timer_delayed`,
`_sound_loop` — **tri-valués (amendement revue LOT-2, m4)** : `"true"` / `"false"` **explicite** / **absent = conservé**
(la carte inchangée n'envoie rien ; `"false"` désactive un réglage sans re-téléverser) ; écrits **seulement si un son est
attaché**, même règle que la question. Nom de fichier
régénéré côté serveur ; suppression du fichier à `_sound_cleared=true` et quand la carte disparaît (`removeCardMedia`).

**Préservation — normatif, piège majeur** : `handleUploadQuestion` reconstruit `MOTION_CARDS` depuis le payload client.
Le serveur **fusionne par `ID` de carte** avec le `question.json` existant : pour toute carte dont l'`ID` existe déjà et
dont le payload ne porte ni nouveau fichier ni `_sound_cleared`, `SOUND`/`SOUND_TIMER_DELAYED`/`SOUND_LOOP` sont
**recopiés depuis l'existant** (même discipline que `SOUND` hôte, `http.go` « piège de reconstruction »). **Jamais** une
préservation côté frontend (4 sites de sérialisation : destruction silencieuse garantie à terme). Test exigé : édition
du titre de la question hôte ⇒ sons de cartes intacts, octet pour octet dans `question.json`.

**Lecture — voix unique, « le plus intérieur gagne »** :
- **Hôte** : `Question.SOUND` joué au **START** de la manche MEMOTION (`Play`/`PlayLoop`) ; pas de « Chrono démarre »
  (pas de chrono global en MEMOTION) : `SOUND_TIMER_DELAYED` d'un hôte MEMOTION est ignoré (écrit `false`).
- **Carte** : au **retournement** de la carte (révélation de la face question — `handleMotionFlip`), si la carte a un
  `SOUND` : la lecture courante (hôte ou carte précédente) est **remplacée** (arrêt net, §10.2) ; `PlayLoop` si
  `SOUND_LOOP` ; `SOUND_TIMER_DELAYED` ⇒ le **chrono de la carte** (`MotionCard` durée) ne démarre qu'à la fin du
  premier passage ou au Stop animateur (`ReleaseDeferredTimer` sur le chrono de carte — extension du §10.7 ; §12.7
  « chrono le plus intérieur » s'applique au décompte).
- **D9 — pas de reprise** : à la fermeture de la carte (`MEMOTION_DONE`), révélation ou STOP, le son de carte s'arrête
  et le son d'hôte **ne reprend pas** (« une question chasse l'autre »). Une carte **sans** son laisse l'hôte continuer.
- **RESTORE coupe le média courant** (hôte, carte ou entracte) — câblé en LOT-2 (`OnAction "RESTORE"` → `questionSound().Stop()`,
  revue m1) : un média ne survit jamais à une restauration de sauvegarde.
- **Gate T0 appliquée au retournement** : si le fichier de la carte est indisponible (§10.8.2), le retournement **n'est
  pas refusé** (la carte s'ouvre, aucun son) ; le motif remonte comme un état (`QUESTION_SOUND_UNAVAILABLE`, §10.8.5) et
  la pastille d'alerte (§10.9) l'affiche. La gate T0 **de la question hôte** (PREPARE→READY) n'examine **que** le son
  d'hôte — jamais les cartes (sinon une manche entière serait bloquée par un fichier de carte).
- `QUESTION_SOUND_STATE` reflète le média **courant** quel qu'en soit le propriétaire ; nouveau champ sœur
  `QUESTION_SOUND_OWNER: "" | "HOST" | "CARD:<cardID>" | "ENTRACTE"` (`game-state.md`, toujours sérialisé). Les gestes
  `QUESTION_SOUND` agissent sur le média courant.
- Conduite : `phaseRules.showSoundRow(phase, question, selectedCardId = null)` — **signature à 3 paramètres (amendement
  revue LOT-2, m4)**, 3ᵉ optionnel, rétrocompatible : vrai pendant une carte sonore (`MOTION_CARDS[].ID === selectedCardId`
  avec `SOUND`) même si l'hôte est muet (admin et anim) ; grille d'édition : chip ♪ sur les
  cartes sonores.

### 14.5 Non-régression exigée

SPEEDY/QCM/ARDOISE : `question_sound_219_test.go`, `_delayed_`, `_enabled_dev_257_`, `media_loop_288_test.go` verts ;
`QuestionsPage.sound.test.jsx` vert ; une question MEMOTION **sans** son (hôte ni carte) : aucun appel au `MediaPlayer`
(test) ; `MOTION_CARDS` sans `SOUND` : `question.json` octet pour octet.

---

## 15. Réglages résolus — renvoi (v11.6.0, #290, LOT-4)

`Muted`, `CuesDisabled`, `CountdownDisabled`, `CountdownOnce` sont désormais des **réglages de comportement** du modèle
mutualisé `contracts/ambiance-settings.md` : lus résolus (global ← manche ← geste direct) par `a.ambianceSettings()`,
jamais directement dans `config.Get()` (test-garde §5.2 de ce contrat). Les §6.3, §11, §12.3–12.4 restent la référence
de la **sémantique** de chaque réglage ; `ambiance-settings.md` est la référence de **sa résolution**.

**Révision 3 (#290, fichiers par manche)** : le **fichier** d'une cue et du décompte est aussi résolu (`SoundFiles`,
`ambiance-settings.md` §9). **Amendement revue LOT-4b (m2)** : `PCMAt(rel)` n'est **pas** ajouté à `audio.Bank` mais porté par
l'interface **optionnelle** `audio.RelBank { PCMAt(rel string) (data []byte, ok bool) }`, **type-assertée** par le moteur sur la banque
(`e.bank.(RelBank)`) — une banque sans `RelBank` (fakes de tests, banques tierces) joue toujours `<cue>.wav` ; comportement
identique au libellé initial. `FileBank` implémente `RelBank`. `audio.Engine` gagne `PlayCueAt(c, rel)` (file `{cue, rel}`,
garanties §5 inchangées ; `PlayCue(c)` ≡ `PlayCueAt(c, "<cue>.wav")`) ; `FileBank.PCMAt` replie sur `<cue>.wav` (WARN) si le fichier
de manche manque ; `notifySound` et `countdownSoundAdapter.filePath()` passent le chemin résolu. Le format canonique (§3) et la
validation d'upload (§7) s'appliquent à l'identique aux fichiers de manche.

**Limite connue (revue LOT-4b, m3) — Windows** : le remplacement d'un fichier de manche (`writeFileAtomic` : écriture temporaire
puis `os.Rename` sur la cible) peut échouer **de façon transitoire** (« Accès refusé ») si la banque est en train de lire ce même
fichier (`os.ReadFile`, lecture brève, fichier entièrement chargé en mémoire). Même schéma que le catalogue ; cas rare ;
**relancer l'envoi** suffit. Aucune reprise automatique côté serveur ; à documenter dans le guide si la QUALIF Windows le rencontre.

