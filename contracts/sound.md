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
// (§3) sur du matériel réel. Apply est appelé UNIQUEMENT depuis la
// goroutine unique de lecture du moteur (§5) : il a donc le droit de
// bloquer, et n'a PAS besoin d'être sûr en accès concurrent. Même contrat
// que lighting.Driver.Apply (contracts/lighting.md §3) — délibérément
// symétrique.
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
- **Aucune garantie de non-blocage côté pilote** — c'est le moteur (§5) qui garantit que
  l'appelant du jeu ne bloque jamais, pas `Output.Play`. Un pilote réel (#228) peut légitimement
  attendre la fin de la lecture avant de retourner ; c'est même la forme la plus simple à
  implémenter par-dessus l'API asynchrone d'`oto` (le spike l'a vérifié : `oto.Player.Play()` ne
  remplit son tampon interne que sur un cycle de lecture différé — un pilote correct doit attendre
  `IsPlaying() == false`, jamais retourner sitôt l'appel `Play()` passé, sous peine de fermer le
  flux avant qu'un seul octet n'ait été lu — piège réel rencontré et corrigé pendant le spike, voir
  `spike/audio/README.md` §« Correctifs appliqués »).

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
