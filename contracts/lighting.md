# Contrat — Éclairage d'ambiance : événements, pilote abstrait, écrivain

> **Issue** : #205 (milestone v10.0.0 — Éclairage ambiance)
> **Cadrage** : `_work/reports/planner-v10-cadrage-20260902-192243.md`
> **Plan** : `_work/reports/planner-v10-plan-205-20260902-203000.md`
> **Consommateurs** : #206 (pilote BLE), #207 (configuration + UI), #213 (éclairage par équipe),
> #208 (conduite `/anim` + restitution)
>
> Ce contrat est **normatif**. Il est écrit avant le code, comme l'exige #205. `dev-backend` peut
> l'ajuster si une contrainte technique l'impose — en documentant la raison, conformément à
> `contracts/README.md` — mais pas par confort d'implémentation.

---

## 1. Pourquoi ce contrat, et le piège qu'il évite

L'objectif de #205 est de faire réagir l'éclairage de la salle **aux mêmes événements de jeu** que
les LED des buzzers. L'audit du code (détaillé dans #205) a établi que le raccourci évident est un
piège :

`(*App).sendLEDSet(mac, payload)` (`cmd/server/main.go:3861`) est bien le goulot unique par lequel
passe **tout** le code LED. Mais il est **par buzzer** : un REVEAL s'y traduit en N appels dans une
boucle `for mac := range tb.Bumpers`. Y brancher l'éclairage produirait N ordres pour **un seul**
événement de jeu — soit exactement la rafale que le matériel ne supporte pas.

> **Règle fondatrice — ne jamais instrumenter la couche de rendu.**
> Le code LED se lit en deux couches. La **couche de rendu** (`sendLEDSetForBuzzer*`,
> `sendLEDSetMultiTeam`, `sendLEDSetAllBuzzers`, `sendLEDSetPause`, `sendLEDSetReveal`,
> `sendLEDSetStop`, `sendLEDSetRafaleTeams`, `sendLEDSetAllEntracteOff`, et le corps de
> `sendLEDSetComet`) calcule la couleur de **chaque buzzer** ; elle ne connaît aucun événement.
> La **couche événement** (les handlers `handle*`, les fonctions `broadcast*`, les callbacks de
> `setupCallbacks`) sait ce qui vient de se passer.
> **L'ambiance se branche exclusivement sur la couche événement** — les 21 sites recensés au §6.

---

## 2. Vocabulaire d'événements d'ambiance — normatif

### 2.1 Les genres

Package `internal/lighting`. Aucun import de `internal/game` ni de `internal/protocol` : ce
package doit rester testable seul.

```go
type EventKind string

const (
    KindIdle     EventKind = "IDLE"      // aucune partie en cours
    KindReady    EventKind = "READY"     // prêt à démarrer (PREPARE, READY, COUNTDOWN)
    KindRunning  EventKind = "RUNNING"   // question en cours (STARTED)
    KindBuzz     EventKind = "BUZZ"      // un buzz a interrompu la question
    KindPauseAll EventKind = "PAUSE_ALL" // pause générale, sans buzz
    KindReveal   EventKind = "REVEAL"    // réponse révélée
    KindTeamTurn EventKind = "TEAM_TURN" // changement d'équipe active (MEMORY/MEMOTION/RAFALE)
    KindEntracte EventKind = "ENTRACTE"  // entracte actif
    KindScore    EventKind = "SCORE"     // points attribués — impulsion, voir §2.3
)
```

Cette liste est **fermée pour v10.0.0**. Un besoin non couvert se traite en ajoutant un genre au
contrat, jamais en surchargeant la sémantique d'un genre existant.

### 2.2 Les équipes concernées

```go
type Event struct {
    Kind  EventKind
    Teams []string // noms d'équipe concernés, ordre significatif (premier = principal).
                   // Vide = aucune équipe concernée.
}
```

⚠️ **`Teams` contient des noms d'équipe, pas des identifiants.** Le modèle `game.Team`
(`internal/game/models.go`) **n'a pas de champ `ID`** : une équipe est désignée par son `Name`
partout dans le projet (`bumper.Team`, `sendLEDSetComet(teamID string)` reçoit en réalité un nom).
Ne pas inventer un identifiant qui n'existe pas.

`Teams` est un **slice** et non un champ unique parce que le REVEAL en QCM concerne plusieurs
équipes à la fois. C'est le seul cas, mais il est structurant.

Ce champ est ajouté à la demande de l'**amendement du 2026-09-02** sur #205 : sans lui, #213
(éclairage différencié par équipe) devrait rouvrir ce contrat. #205 le fait **circuler** ; #213 le
**résout** en ampoules.

### 2.3 Deux natures d'événement — distinction load-bearing

| Nature | Genres | Dérivable de `GameState` ? | Traitement |
|---|---|---|---|
| **Scène d'état** | IDLE, READY, RUNNING, BUZZ, PAUSE_ALL, REVEAL, TEAM_TURN, ENTRACTE | **Oui** | Ne jamais mémoriser la charge utile — voir §4.1 |
| **Impulsion** | SCORE | **Non** | Registre à une place avec échéance — voir §4.2 |

Une attribution de points est un **instant**, pas un état : rien dans `GameState` ne permet de
savoir, une seconde plus tard, qu'une équipe vient d'être créditée. C'est la seule exception, et
elle est traitée explicitement.

---

## 3. Interface du pilote — normatif

```go
// State est l'état d'éclairage souhaité, à un instant donné.
type State struct {
    Zones []ZoneState // en v10.0.0/#205 : toujours exactement une zone, "general".
                      // #213 y ajoute une zone par équipe.
}

type ZoneState struct {
    Zone      string // "general", ou un nom d'équipe (#213)
    Color     [3]int // RGB, 0-255
    Intensity int    // 0-255
}

// Driver applique un état d'éclairage sur du matériel.
type Driver interface {
    // Apply est appelé UNIQUEMENT depuis la goroutine unique de l'écrivain (§4).
    // Il a donc le droit de bloquer, et n'a PAS besoin d'être sûr en accès concurrent.
    // C'est une garantie du contrat, sur laquelle #206 peut s'appuyer.
    Apply(ctx context.Context, s State) error

    // Close libère les ressources. Idempotent, appelable même si Apply n'a jamais réussi.
    Close() error
}
```

**`Color`/`Intensity` reprennent délibérément le format et l'échelle de
`protocol.LEDSetPayload`** (`internal/protocol/messages.go:751` — RGB `[3]int` 0-255, intensité
0-255). Aucune conversion à cette couche.

> C'est ce qui garantit le critère de fait de #213 : « la salle et les buzzers montrent **la même**
> couleur pour **la même** équipe ». Deux rouges différents pour une même équipe seraient pires que
> pas de couleur du tout. La conversion vers le format du matériel (CIE xy pour les ampoules Hue)
> appartient au pilote #206, et à lui seul.

---

## 4. L'écrivain — normatif, et le point le plus facile à mal implémenter

L'écrivain est ce qui sépare les sites de jeu du matériel. Trois exigences **non négociables** :

1. **Jamais d'appel bloquant depuis un site de jeu.** Un équipement lent ne doit pas ralentir d'un
   millimètre une transition d'état.
2. **Dernier état gagnant.** Une rafale d'événements produit **un** ordre, pas N.
3. **Sûr en accès concurrent par construction.** Voir §5.

### 4.1 Scènes d'état — ne jamais mémoriser la charge utile

> **Invariant fondateur, repris tel quel de `BroadcastCoalescer`**
> (`cmd/server/broadcast_coalescer.go`, dont l'en-tête documente ce raisonnement) :
> **l'écrivain ne met jamais un état en mémoire tampon. Il retient seulement qu'un
> rafraîchissement est dû**, et re-dérive l'état depuis le `GameState` **vivant** au moment où il
> s'exécute réellement.

Conséquence directe : une émission différée est toujours *redondante* avec l'état qui existait
quand elle a été programmée — **jamais périmée, jamais en retard sur un ordre parti entre-temps**.
C'est ce qui rend « dernier état gagnant » correct **par construction** plutôt que par
discipline, et c'est ce qui rend inutile toute file d'attente d'événements.

Un tampon d'états serait un bug latent : deux callbacks concurrents (§5) pourraient y déposer deux
états dans un ordre arbitraire, et le plus ancien gagner.

### 4.2 Impulsions — registre à une place avec échéance

SCORE n'étant pas dérivable, il est mémorisé, mais dans un **registre à une place** :

```go
type pulse struct {
    kind     EventKind
    teams    []string
    deadline time.Time
}
```

- Une nouvelle impulsion **écrase** la précédente (dernier gagnant).
- Au moment d'appliquer : si une impulsion **non échue** est présente, c'est elle qui est rendue ;
  sinon on dérive de l'état vivant.
- L'écrivain **programme un rafraîchissement à l'échéance**, pour que la salle quitte la scène
  SCORE toute seule.

**Durée d'une impulsion SCORE : 4800 ms**, alignée sur le `time.AfterFunc(4800*time.Millisecond)`
qui restaure les LED en fin de `sendLEDSetComet` (`cmd/server/main.go:4619`). La salle et les
buzzers reviennent ainsi à la normale **au même instant**, par construction et non par
coïncidence.

### 4.3 Forme imposée

Une goroutine unique, un registre à une place, un canal de réveil de capacité 1 servant
**uniquement de signal**. Cette forme est imposée, pas suggérée :

```go
func (w *Writer) NotifyState() {          // JAMAIS bloquant
    if w == nil || !w.enabled { return }
    w.mu.Lock()
    w.refreshDue = true
    w.mu.Unlock()
    select { case w.wake <- struct{}{}: default: }  // signal, jamais d'attente
}

func (w *Writer) NotifyPulse(k EventKind, teams []string, d time.Duration) {
    if w == nil || !w.enabled { return }
    w.mu.Lock()
    w.pulse = &pulse{kind: k, teams: teams, deadline: w.now().Add(d)}
    w.refreshDue = true
    w.mu.Unlock()
    select { case w.wake <- struct{}{}: default: }
}
```

- `select { case ch <- v: default: }` est la **seule** forme d'envoi autorisée sur `wake`. Un envoi
  bloquant, même « qui ne bloquera jamais en pratique », est un refus de revue.
- Le canal ne transporte **aucune donnée** : il dit « regarde », pas « voici ».
- `w == nil` et `!w.enabled` rendent les 21 sites d'appel inconditionnels — **aucun `if` autour
  d'un `Notify*` dans `main.go`**. C'est ce qui garde le recensement du §6 lisible et vérifiable.

### 4.4 Débit

Après chaque `Apply`, l'écrivain attend au moins `MinInterval` avant le suivant.

**Valeur pour #205 : 100 ms**, marquée **provisoire** — elle sera recalée sur les mesures du spike
#204 (latence d'écriture BLE, désynchronisme entre ampoules). `dev-backend` l'expose en constante
nommée et documentée, pas en littéral dispersé.

Ce paramètre donne au critère « rafale bornée et mesurée » de #205 un sens testable :
**une rafale de N événements sur une durée T produit au plus `T/MinInterval + 1` appels à
`Apply`**, quel que soit N.

### 4.5 Cycle de vie

Calqué sur `AckManager`, le patron le plus propre du serveur — **pas** sur mDNS/DNS
(« best-effort, échec silencieux »), qui n'a aucun état à restituer :

- construction dans `(*App).setup()` ;
- `go w.Start(a.ctx)` dans `(*App).start()` (`cmd/server/main.go:949` pour le précédent) ;
- arrêt par `a.cancelCtx()` dans `(*App).stop()` (`main.go:992`).

**Éclairage non configuré ⇒ aucune goroutine n'est lancée.** Pas une goroutine qui tourne à vide :
aucune. C'est un critère de fait de #205.

---

## 5. Sûreté d'accès concurrent — normatif

> Les callbacks de l'Engine sont invoqués **hors lock, depuis ~12 sites, sans aucune
> sérialisation** : deux peuvent s'exécuter **en même temps** sur des goroutines différentes. Ce
> contrat est écrit noir sur blanc dans `internal/game/engine.go:220-241`, et son non-respect est
> la **cause racine du bug #121**.

Règles :

1. `NotifyState` / `NotifyPulse` sont sûrs en accès concurrent — c'est le rôle du mutex du §4.3.
2. Le mutex protège **uniquement** `refreshDue` et `pulse`. Il n'est **jamais** tenu pendant un
   appel à `Apply`, ni pendant une lecture de `GameState`. Un verrou tenu pendant une I/O matérielle
   rendrait le point 1 du §4 caduc.
3. `Driver.Apply` n'est appelé que depuis la goroutine unique de `Start` — garantie offerte à #206.
4. `go test -race ./...` doit être vert. Un test dédié doit lancer plusieurs goroutines appelant
   `NotifyState`/`NotifyPulse` en parallèle.

---

## 6. Recensement des sites émetteurs — normatif

**21 sites.** Référence : `cmd/server/main.go` sur la branche `milestone/v10.0.0`. La colonne
« fonction englobante » est l'identité stable du site — **pas le numéro de ligne**, qui change à
chaque édition (§7).

| Fonction englobante | Ligne | Événement de jeu | Appel d'ambiance | Équipes |
|---|---|---|---|---|
| `broadcastReady` | 3652 | PREPARE→READY | `NotifyState()` | — |
| `broadcastStart` | 3597 | START (→COUNTDOWN/STARTED) | `NotifyState()` | — |
| `broadcastStop` | 3604 | STOP | `NotifyState()` | — |
| `broadcastPause` | 3611 | **Buzz** (STARTED→PAUSED) | `NotifyState()` | équipe du buzzeur, dérivée de l'état |
| `broadcastPauseAll` | 3618 | Pause générale (admin) | `NotifyState()` | — |
| `broadcastContinue` | 3625 | CONTINUE | `NotifyState()` | — |
| `broadcastReveal` | 3659 | REVEAL | `NotifyState()` | équipes ayant bien répondu (vide si aucune) |
| `handlePoints` | 1750 | Points attribués | `NotifyPulse(KindScore, []string{teamID}, 4800ms)` | équipe créditée |
| `handleBumperPoints` | 2601 | Points bumper | `NotifyPulse(KindScore, …, 4800ms)` | équipe créditée |
| `handleTeamPoints` | 2668 | Points équipe | `NotifyPulse(KindScore, …, 4800ms)` | équipe créditée |
| `handleMotionDone` | 2459 | MEMOTION : gagnant désigné | `NotifyPulse(KindScore, WinnerTeam, 4800ms)` | équipe gagnante |
| `handleMotionDone` | 2508 | MEMOTION complet (auto-stop) | `NotifyState()` | — |
| `handleMotionSetTeams` | 2530 | MEMOTION : équipes définies | `NotifyState()` | équipe active |
| `handleFlipMemoryCard` | 2295 | MEMORY : retournement auto | `NotifyState()` | équipe active |
| `handleFlipMemoryCard` | 2302 | MEMORY : paire trouvée | `NotifyState()` | équipe active |
| `handleFlipMemoryCard` | 2327 | MEMORY : grille complète | `NotifyState()` | — |
| `handleMemorySetTeams` | 2354 | MEMORY : équipes définies | `NotifyState()` | équipe active |
| `setupCallbacks` (`OnRafaleTeamsChanged`) | 494 | RAFALE : équipe suivante | `NotifyState()` | équipe active |
| `handleEntracteSet` | 2804 | ENTRACTE ON | `NotifyState()` | — |
| `handleEntracteSet` | 2807 | ENTRACTE OFF | `NotifyState()` | — |
| `handleFullUpdate` | 1725 | Édition équipes/bumpers | `NotifyState()` | — (les couleurs d'équipe ont pu changer) |

### 6.1 Sites LED **sans** ambiance — décisions explicites

Ces fonctions contiennent un appel LED et **n'émettent délibérément aucun événement d'ambiance**.
Elles sont enregistrées comme telles (§7) : leur absence du registre ferait échouer le test.

| Fonction | Ligne | Pourquoi pas d'ambiance |
|---|---|---|
| `resendLEDOnReconnect` | 3925 | Resynchronisation **d'un seul appareil** qui se reconnecte. Rien n'a changé dans la partie ; la salle n'a aucune raison de réagir au retour d'un buzzer. |
| `sendLEDSetComet` (fin, `AfterFunc` +4,8 s) | 4619 | La sortie de la scène SCORE est déjà pilotée par l'**échéance de l'impulsion** (§4.2), calée sur la même durée. Émettre ici doublerait le rafraîchissement. |
| `broadcastLEDSet` | 3908 | **Code mort** — zéro site d'appel, documenté par l'audit #132. |
| `sendLEDSetToTeam` | 4561 | **Code mort** — zéro site d'appel, documenté par l'audit #132. |
| Toute la couche de rendu | 4019-4620 | Voir la règle fondatrice du §1. |

> Si `broadcastLEDSet` ou `sendLEDSetToTeam` était un jour réactivé, sa décision d'ambiance serait
> à reprendre. L'annotation dans le registre le dit explicitement.

### 6.2 Dérivation de l'état vivant

`NotifyState()` ne transporte rien (§4.1). C'est l'adaptateur côté `App` qui, **au moment de
l'application**, dérive `Event` depuis le `GameState` vivant :

| Condition sur l'état vivant | `Kind` | `Teams` |
|---|---|---|
| Entracte actif | `KindEntracte` | — |
| `PhaseStopped` (et `PhaseNewGame`, `PhaseEnroll` — aucune partie en cours) | `KindIdle` | — |
| `PhasePrepare` / `PhaseReady` / `PhaseCountdown` | `KindReady` | — |
| `PhaseStarted`, équipe active (MEMORY/MEMOTION/RAFALE) | `KindTeamTurn` | équipe active |
| `PhaseStarted`, pas d'équipe active | `KindRunning` | — |
| `PhasePaused`, un buzzeur identifié | `KindBuzz` | équipe du buzzeur |
| `PhasePaused`, aucun buzzeur | `KindPauseAll` | — |
| `PhaseRevealed` | `KindReveal` | équipes ayant bien répondu, **vide si aucune** |

L'entracte est testé **avant** la phase : c'est un mode transverse, pas une phase.

> ⚠️ `PhaseCountdown` **n'a pas** de rendu propre : la couche LED le groupe déjà avec
> `PhaseStopped/PhasePrepare/PhaseReady` (`sendLEDSetForBuzzerNormal`). L'ambiance suit le même
> groupement — ne pas inventer une scène de décompte, elle appartient au milestone v10.1 (#212).

### 6.3 Précisions d'implémentation (dev-backend, 2026-09-02) — ⚠️ Contract Modification

Trois points levés à l'implémentation de #205, sans changer l'intention du contrat :

1. **`KindTeamTurn` n'avait aucune ligne de dérivation** : la version initiale du tableau §6.2
   produisait `KindRunning` + équipe active en STARTED, et la table de scènes §8 ne colore
   `RUNNING` que d'un bleu neutre — la couleur de l'équipe active n'aurait jamais été rendue.
   La ligne est scindée : STARTED **avec** équipe active (`MemoryCurrentTeam` /
   `MotionCurrentTeam` / `RafaleCurrentTeam` selon `Question.Type`) → `KindTeamTurn`, sans →
   `KindRunning`. C'est ce que les sites « équipe active » du §6 attendaient.
2. **`PhaseNewGame` et `PhaseEnroll`** n'étaient pas cités : ils rejoignent `KindIdle` (aucune
   partie en cours ; la couche LED les rend déjà comme STOPPED via le `default` de
   `sendLEDSetForBuzzerNormal`).
3. **Identification du buzzeur en PAUSED** : `App.bumperBuzzState` appartient à la goroutine de
   dispatch (aucun mutex — voir le commentaire du struct `App`) et **ne doit pas** être lu depuis
   la goroutine de l'écrivain. L'adaptateur utilise l'état vivant du moteur : le bumper au
   `Time` de pression le plus récent (`Bumper.Time`, remis à zéro sur READY) donne l'équipe du
   buzz ; aucun `Time > 0` ⇒ `KindPauseAll`. Pour le REVEAL QCM, les équipes « ayant bien
   répondu » sont celles des bumpers ayant buzzé (`Time > 0`) avec `AnswerColor ==
   Question.QCMCorrect`, ordonnées par temps de pression, dédoublonnées. Ces lectures passent par
   une **nouvelle méthode moteur `Engine.GetTeamsAndBumpersSnapshot()`** (copie profonde sous
   verrou) — `GetTeamsAndBumpers()` rend les maps vivantes, inutilisables hors de la goroutine de
   dispatch sans course.

Le registre compte **24 entrées** : 15 `NotifyState` + 4 `NotifyPulse` (les 21 sites du §6
donnent 20 paires distinctes, `handleFlipMemoryCard` appelant trois fois `sendLEDSetAllBuzzers`)
+ 4 `NoAmbiance` du §6.1, dont 2 documentaires dont la fonction englobante est de la couche de
rendu (`sendLEDSetComet`, `sendLEDSetToTeam`). Le prédicat `ambianceIsRenderingLayer` (préfixe
`sendLEDSet`) est ce qui permet au test §7 d'exclure la couche de rendu de la comparaison.

---

## 7. Test d'exhaustivité — normatif

**Ce que le test doit attraper** : un nouveau site d'émission LED ajouté dans `main.go` **sans
décision d'ambiance**. C'est la classe de défaut la plus probable du milestone (« les buzzers
s'allument, la salle non »), de la même famille que la fuite de #128.

**Construction imposée** — analyse syntaxique, pas expression régulière :

1. Analyser `cmd/server/main.go` avec `go/parser` + `go/ast` (**stdlib** — aucune dépendance
   ajoutée).
2. Parcourir l'AST et collecter l'ensemble des paires
   **(nom de la fonction englobante, nom de la fonction LED appelée)** pour tout appel dont le
   sélecteur commence par `sendLEDSet`.
3. Comparer cet ensemble au **registre** déclaré dans le code (`cmd/server/ambiance.go`) : une map
   des mêmes paires vers leur décision (`NotifyState`, `NotifyPulse`, ou `NoAmbiance` + motif).
4. Échouer si les deux ensembles diffèrent, en **nommant** la ou les paires en trop ou manquantes.

**Pourquoi la paire (englobante, appelée) et pas la ligne** : un test indexé sur des numéros de
ligne casserait à chaque édition de `main.go` et serait désarmé en trois jours. La paire est
stable et porte la sémantique.

**Limite assumée et à documenter dans le test** : ajouter un **deuxième** appel à
`sendLEDSetAllBuzzers` dans une fonction déjà enregistrée ne fera pas échouer le test. C'est
volontaire — c'est le même site sémantique émettant le même genre d'événement. Ce que le test
garantit, c'est qu'aucune **fonction** ni aucun **type d'appel LED** nouveau n'entre sans décision.

Message d'échec attendu, à l'adresse de celui qui vient de casser le test :

```
ambiance: site LED sans décision d'ambiance — handleNouveauTruc -> sendLEDSetAllBuzzers
  Ajoute une entrée dans ambianceSiteRegistry (cmd/server/ambiance.go) :
  soit NotifyState/NotifyPulse, soit NoAmbiance avec le motif.
  Voir contracts/lighting.md §6.
```

---

## 8. Table de scènes v1 — normatif

**Câblée en dur pour #205** (l'édition par l'utilisateur est renvoyée à #210, milestone v10.1).

| `Kind` | Couleur RGB | Intensité | Justification |
|---|---|---|---|
| `KindIdle` | `{255, 214, 170}` blanc chaud | 120 | La salle reste **praticable** hors partie. |
| `KindReady` | `{255, 255, 255}` | 200 | Attention montante. |
| `KindRunning` | `{40, 90, 255}` bleu | 160 | Neutre, ne concurrence aucune couleur d'équipe. |
| `KindBuzz` | couleur de l'équipe | 255 | **Exactement** le RGB de ses buzzers (§3). |
| `KindPauseAll` | `{255, 170, 0}` ambre | 120 | Distinct du buzz : rien n'est joué. |
| `KindReveal`, `Teams` non vide | `{0, 220, 60}` vert | 255 | Au moins une bonne réponse. |
| `KindReveal`, `Teams` vide | `{230, 30, 30}` rouge | 255 | Personne n'a trouvé. |
| `KindTeamTurn` | couleur de l'équipe active | 200 | |
| `KindScore` | couleur de l'équipe créditée | 255 | Équivalent salle du COMET, même durée. |
| `KindEntracte` | `{255, 214, 170}` blanc chaud | 100 | **Divergence assumée** — voir ci-dessous. |

> **Pourquoi l'entracte n'éteint pas la salle**, alors que `sendLEDSetAllEntracteOff`
> (`main.go:2819`) éteint tous les buzzers : les buzzers s'éteignent pour cesser d'attirer
> l'attention. La salle, elle, doit rester praticable — des gens se lèvent, circulent, reviennent.
> Plonger la pièce dans le noir pendant l'entracte serait le contraire du service rendu. Cette
> divergence est **délibérée** et ne doit pas être « corrigée » par alignement sur les buzzers.

**Résolution de la couleur d'équipe** — réutiliser la machinerie existante, **jamais une seconde
palette** : `teamColorPalette` (`main.go:3687`), `teamColorToRGB` (3838),
`nearestPaletteColorByHue` (3797), `dimIntensityFor` (3779).

⚠️ `teamColorToRGB` prend un **`*game.Bumper`**, pas un nom d'équipe. L'ambiance ne dispose que
d'un nom. L'adaptateur doit donc extraire le chemin de résolution
`nom d'équipe → engine.GetTeam(nom) → ColorName → teamColorPalette`, **en factorisant** avec
`teamColorToRGB` plutôt qu'en le recopiant. Le gris de repli `{128, 128, 128}` d'équipe inconnue
est conservé à l'identique.

---

## 9. Configuration — la part de #205 seulement

#205 n'introduit **pas** le schéma de configuration : il appartient à #207. #205 n'a besoin que
d'un prédicat :

```go
// IsConfigured indique si l'éclairage d'ambiance est utilisable.
// #205 : renvoie toujours false (aucun pilote réel n'existe encore).
// #207 : dérive de la section `lighting` de config.json.
func (a *App) ambianceIsConfigured() bool
```

**Conséquences pour #205 :**
- Le comportement observable par défaut est **strictement celui d'aujourd'hui**.
- Aucune goroutine, aucun appel matériel, aucune ligne de log.
- Le pilote réellement livré par #205 est un **pilote factice de test** (`internal/lighting`,
  enregistrant les `State` reçus), qui rend toute la mécanique testable **sans aucun matériel**.

⚠️ **La section de configuration se nommera `lighting`, pas `ambiance`** (#207) : « ambiance »
désigne déjà la catégorie de sauvegarde couvrant `game-config.json` (`BackupPage.jsx`, #152).

---

## 10. Hors de ce contrat

| Sujet | Issue |
|---|---|
| Pilote BLE réel, conversion RGB → CIE xy, appairage | #206 |
| Schéma de configuration, endpoints HTTP, écran d'administration | #207 |
| Affectation ampoule → équipe, résolution de `Teams` en zones, règles de dégradation | #213 |
| Conduite manuelle depuis `/anim`, restitution d'état à l'arrêt | #208 |
| Édition des scènes, effets répartis, synchronisation sur le minuteur | v10.1 (#210, #211, #212) |
