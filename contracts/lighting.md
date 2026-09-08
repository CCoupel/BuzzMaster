# Contrat — Éclairage d'ambiance : événements, pilote abstrait, écrivain

> **Issue** : #205 (milestone v10.0.0 — Éclairage ambiance)
> **Cadrage** : `_work/reports/planner-v10-cadrage-20260902-192243.md`
> **Plan** : `_work/reports/planner-v10-plan-205-20260902-203000.md`
> **Consommateurs** : #206 (pilote BLE), #207 (configuration + UI), #213 (éclairage par équipe),
> #208 (commandes manuelles `/admin/ambiance` + restitution)
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

### 2.4 `Event.Points` — normatif (2026-09-08)

`Event` porte, en plus de `Kind` et `Teams`, un champ **`Points int`** :

- renseigné **uniquement** pour `KindScore`, avec le nombre de points marqués ;
- **`0` pour tout autre genre** — champ optionnel, aucun appelant existant n'est invalidé ;
- seul consommateur : le nombre de clignotements de l'impulsion SCORE (§8.1).

Les quatre sites d'émission `NotifyPulse(KindScore, …)` de `cmd/server/main.go` doivent le
renseigner ; sans lui, la proportionnalité du §8.1 n'a pas d'entrée.

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

Le registre compte **23 entrées** : 15 `NotifyState` + 4 `NotifyPulse` (les 21 sites du §6
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

### 8.1 Ampoules d'équipe — intensité et impulsion SCORE (révision du 2026-09-08)

Une ampoule de rôle `team` rend **toujours** la couleur de son équipe (`hue-bridge.md` §5.2). Ce
que l'état du jeu module, c'est son **intensité** — transposition littérale de
`sendLEDSetForBuzzerNormal`, la référence que fixe l'intention « gérer ces Hue de la même façon
que les buzzers » :

| Situation | Intensité |
|---|---|
| Hors partie, `PREPARE`, `READY`, `COUNTDOWN` | **pleine** — comme un buzzer, `SOLID` 255 |
| `STARTED` / `PAUSED` / `REVEALED`, équipe **distinguée** par l'événement | **pleine** |
| `STARTED` / `PAUSED` / `REVEALED`, équipe **non distinguée** | **atténuée** — `dimIntensityFor()`, la fonction **déjà employée par les buzzers**, jamais un second seuil |

> Un buzzer n'est atténué **que** pendant le jeu actif. Hors partie il est à pleine intensité — la
> salle suit la même règle, sans quoi les deux s'atténueraient à contretemps.

**Impulsion SCORE — clignotement or, proportionnel aux points :**

| Paramètre | Valeur |
|---|---|
| Couleurs alternées | **couleur de l'équipe créditée** ↔ **or `{255, 190, 0}`**, pleine intensité |
| Cadence | **400 ms / 400 ms** — celle du Flash du §10.1, jamais un second réglage |
| Nombre de clignotements | **`clamp(points, 1, 6)`** — un par point marqué, plafonné |
| Durée totale | **`ScorePulseDuration` (4800 ms), constante quel que soit le score** : après ses `N` clignotements, l'ampoule tient la couleur d'équipe à pleine intensité jusqu'à l'échéance |

> **Pourquoi 6** : `4800 / 800 = 6` exactement. Le plafond n'est pas un renoncement, c'est ce que
> le pulse contient — et au-delà, l'œil ne compte plus. Garder une durée **constante** préserve
> `ScorePulseDuration` comme réglage unique et le registre d'impulsion à une place du §4.2.

> ⚠️ **Seule dérogation à « toujours la couleur de son équipe »**, avec l'extinction du §10.4. Elle
> est **bornée** : transitoire, déclenchée par le score **de cette équipe-là**, alternée avec **sa
> propre** couleur et y revenant. Elle **célèbre** l'identité au lieu de l'effacer — ne pas la
> « corriger » au nom du §5.2.

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

## 10. Conduite manuelle, restitution et arrêt — normatif (#208, décisions du 2026-09-07)

> Section ajoutée à la reprise du milestone v10.0.0. Les trois questions que #208 laissait
> explicitement ouvertes sont tranchées ici par décision utilisateur du 2026-09-07
> (`_work/handoff/gate1-decisions-v10-20260907.md`). Elles ne sont plus des choix d'implémentation.

### 10.1 Mode d'éclairage manuel ↔ scènes automatiques

> **Révision de périmètre du 2026-09-07.** Ces commandes ont d'abord été cadrées sur la tablette
> `/anim`. **Elles n'y ont jamais eu leur place** : rien dans le cadrage du milestone ne demandait
> que l'**animateur** pilote l'éclairage de la salle. Elles vivent sur l'écran d'administration
> `/admin/ambiance` (`AmbiancePage.jsx`, #207), aux côtés de la sélection des ampoules.
>
> **Ce qui a changé de main, c'est l'opérateur — pas la nature de l'outil.** C'est bien un
> **instrument de conduite en direct**, utilisé par la **régie** *pendant* une partie, en réaction
> au jeu : souligner un moment, faire tomber la salle, la relever. Ce n'est **pas** un écran de
> configuration qu'on n'ouvrirait qu'entre deux soirées.
> ⚠️ Toute justification reposant sur « cet écran n'est normalement pas ouvert en séance » est donc
> **fausse** et a été retirée de ce contrat (correction utilisateur du 2026-09-07). Ce recadrage
> **renforce** la règle de tenue du §10.1.1 : un jugement pris en direct par la régie doit tenir
> jusqu'à ce qu'elle-même en décide autrement.
> L'association effet↔événement **configurable par l'utilisateur** reste renvoyée à #210 (v10.1) —
> c'est elle, la « seconde phase », **pas** la table de scènes câblée du §8, qui est livrée et
> reste en place (confirmé au GATE du 2026-09-07).

> **Historique des deux révisions de ce paragraphe** — utile parce que des documents rédigés entre
> les deux circulent encore :
> 1. *Gestes ponctuels sans mémoire*, recouverts par le prochain événement de jeu — **abandonné**.
> 2. *Bascules à état* qui tiennent jusqu'à leur relâche — **conservé quant au fond**.
> 3. **Version en vigueur** : un **sélecteur tri-état** qui donne enfin un nom à l'état « relâché ».
>    La sémantique du point 2 est **inchangée** ; seule l'affordance change, et l'état de repos
>    devient explicite au lieu d'être l'absence des autres.
> 4. Une quatrième formulation — *annulation automatique du mode au premier événement de jeu, avec
>    retour visuel sur AUTO* — a été rédigée puis **annulée le même jour** (confirmation utilisateur
>    du 2026-09-07). **Elle n'a jamais été en vigueur.** Le retour à AUTO est **manuel**, et lui
>    seul. Si un document parle d'annulation automatique, il est périmé.

#### Deux contrôles, et deux seulement

**1. Mode d'éclairage général — sélecteur à trois positions, exclusives par construction :**

| Position | Effet **sur la zone `general`** |
|---|---|
| **ON** | forcée à pleine intensité, blanc neutre |
| **AUTO** | *(position normale)* suit **l'état du jeu** — dérivation automatique, §6.2 |
| **OFF** | forcée à `{"on": false}` |

**2. Flash — bascule séparée**, `ON`/`OFF` : clignotement, pour souligner un moment de jeu ou
identifier une ampoule. Même portée que le sélecteur (zone `general`).

##### ⚠️ Portée : la zone `general`, **jamais** les ampoules d'une équipe active — normatif

Le sélecteur agit sur la **zone `general` au sens de `hue-bridge.md` §5.2**, et sur elle seule :

> les ampoules de rôle `general`, **plus** toute ampoule d'équipe **dont l'équipe n'est pas nommée
> dans l'état courant**.

Les ampoules **affectées à une équipe** restent **toujours pilotées par la dérivation de jeu**
(#213), **quelle que soit la position du sélecteur**. La régie force l'ambiance de la salle ; elle
ne débranche jamais l'information « quelle équipe joue ».

> ⚠️ **Révisé le 2026-09-08.** La version précédente faisait retomber les ampoules d'équipe dans
> `general` hors partie, et affirmait qu'« hors partie, un OFF éteint tout ». **C'est faux
> désormais** : `hue-bridge.md` §5.2 a supprimé toute retombée. Ce qui ne valait que pendant une
> partie est **étendu à tout instant** — la règle en devient plus simple, pas plus complexe.

Conséquence unique, valable **en permanence** :

- Un **OFF** éteint la zone `general` **et laisse chaque ampoule d'équipe à la couleur de son
  équipe** — en partie comme hors partie. C'est délibéré, et c'est l'effet le plus utile du
  dispositif : la salle s'efface, les équipes restent désignées par la lumière.

- **Installation sans aucune ampoule d'équipe** — cas prévu par `hue-bridge.md` §5.7 : toutes les
  ampoules sont de rôle `general`, y compris l'information « quelle équipe joue », que la scène
  générale porte elle-même (`KindTeamTurn`). Un **OFF** y éteint donc **tout**. La garantie « les
  ampoules d'équipe restent allumées » n'existe que si des ampoules **leur sont affectées** —
  c'est une raison de plus d'en affecter au moins une par équipe, pas un défaut de la règle.

> Cette portée **corrige** une précision dérivée erronée du planner (« toutes les ampoules pilotées,
> zones d'équipe comprises »), écartée par l'utilisateur le 2026-09-07. Le raisonnement fautif était
> qu'« un OFF qui laisserait des ampoules allumées ne serait pas un OFF » : il confondait *éteindre
> la salle* et *éteindre l'installation*.

#### 10.1.1 AUTO est le seul chemin de retour — normatif

**Les deux mécanismes ne coexistent pas.** Le sélecteur **remplace** l'écrasement automatique par
le prochain événement de jeu ; il ne s'y ajoute pas.

1. **Tenue.** En position **ON** ou **OFF**, le mode **s'impose à l'éclairage** : les événements de
   jeu **ne le recouvrent pas**, y compris pendant une partie. C'est le sens même d'un mode.
2. **Pourquoi l'écrasement automatique est écarté.** Un événement de jeu qui reprendrait la main
   sans bouger le sélecteur laisserait celui-ci afficher **ON** pendant que la salle montre une
   scène de jeu : **le sélecteur mentirait**. Pire, **ON** et **AUTO** deviendraient deux positions
   au comportement identique, indiscernables. C'est exactement le défaut — montrer un état qui
   n'existe pas — que la présence d'une position **AUTO** explicite permet enfin d'éliminer.
3. **AUTO rend l'éclairage au jeu.** Passer le sélecteur sur **AUTO** — depuis ON comme depuis OFF
   — **ne signifie pas éteindre**. La scène est **re-dérivée depuis l'état de jeu vivant** et
   réappliquée immédiatement.
   > C'est **exactement** le mécanisme du §10.3 (resynchronisation au retour du pont), et ce doit
   > être **le même code**. Aucune mémorisation de « la scène d'avant » : rien n'est sauvegardé,
   > tout est re-dérivé (§4.1). Sauvegarder puis restaurer serait à la fois plus coûteux et faux,
   > la partie ayant pu avancer pendant le forçage.
4. **AUTO hors partie.** Sans partie en cours, la dérivation donne `KindIdle` (blanc chaud
   praticable, §8) — jamais l'obscurité. **AUTO n'éteint jamais la salle.**
5. **Position par défaut.** **AUTO** au démarrage du serveur. Le mode n'est **pas persisté** : il
   n'est écrit dans aucun fichier de configuration et ne survit pas à un redémarrage.
6. **Propriété serveur.** Le mode courant est un état **du serveur**, jamais du navigateur. Deux
   admins sur deux postes voient la même position. Un onglet fermé ne change rien au mode en cours.
7. **Retour du pont.** Si le pont tombe puis revient, la resynchronisation du §10.3 réapplique **le
   mode courant** : ON, OFF, ou la scène de jeu si AUTO. Un seul chemin de code — « ce que
   l'éclairage doit montrer maintenant ».
8. **Arrêt du serveur.** L'extinction totale du §10.4 s'applique quel que soit le mode ; elle n'a
   pas à le ramener sur AUTO d'abord.

> ⚠️ **Conséquence opérationnelle à assumer.** Un mode ON ou OFF tient **indéfiniment** jusqu'à ce
> qu'un opérateur le ramène sur AUTO, ou jusqu'à l'arrêt du serveur. Un **OFF** oublié laisse la
> salle éteinte et **aucun événement de jeu ne la rallumera**. C'est le prix assumé d'un mode qui
> mérite son nom — et le sélecteur tri-état le rend **beaucoup plus lisible** qu'un bouton
> « actif » : la position se lit d'un coup d'œil, et **AUTO** nomme explicitement le geste qui
> corrige la situation.
> **Depuis la révision du 2026-09-08, ce risque s'atténue nettement** : toute ampoule affectée à
> une équipe reste allumée **en permanence**, en partie comme hors partie. Dès qu'au moins une
> affectation existe, la salle n'est **jamais** totalement noire. Le cas dur se réduit à une
> installation **sans aucune ampoule d'équipe**.

#### 10.1.2 Flash et le sélecteur — précédence, pas exclusion

Flash est un contrôle **séparé** du sélecteur : les deux peuvent être engagés en même temps.

- **Tant que Flash est actif, il prime** sur la position du sélecteur — sans quoi un Flash demandé
  alors que la salle est sur OFF ne produirait rien de visible. Or c'est précisément la situation
  où la régie le déclenche : salle éteinte, un moment à souligner. Un effet inopérant au moment
  exact où on l'appelle serait un effet inutile.
- **Le sélecteur ne bouge pas** pour autant : Flash est une **couche transitoire**, pas un quatrième
  mode. Le sélecteur continue d'afficher le mode sous-jacent, qui reste vrai.
- **À l'extinction de Flash**, l'éclairage revient à ce que dit le sélecteur : ON, OFF, ou
  re-dérivation depuis l'état de jeu si AUTO.

*(Précision dérivée, planner — la demande pose Flash comme bascule séparée sans trancher son
interaction avec le sélecteur ; signalée pour relecture.)*

#### Implémentation du clignotement (Flash)

- Le clignotement est **porté par le pilote côté serveur**, jamais par le navigateur — un onglet
  fermé ne doit pas laisser les ampoules clignoter indéfiniment.
- Il réutilise la garde d'**opération unique en vol** de #207 (`lightingBusy`), sans en créer une
  seconde.
- `POST /api/lighting/test` (#207) reste le **flash ponctuel** de test d'une ampoule **nommée** :
  geste sans état, distinct de la bascule Flash, et **non remplacé** par elle.

#### 10.1.3 Canal — HTTP REST, jamais WebSocket

Ces commandes étendent la surface REST existante `/api/lighting/*` (#207), seul canal que
`AmbiancePage.jsx` utilise. **Aucune action WebSocket, aucune entrée d'allowlist** : la question ne
se posait que tant que la cible était `/anim`.

### 10.2 Restitution d'état — trois situations, trois décisions

| Situation | Comportement de l'éclairage | Action requise |
|---|---|---|
| **Fin de partie** | La dernière scène de jeu **reste affichée**. Aucun retour à un état neutre. | **Aucune** — hors périmètre v10.0.0 |
| **Perte du pont en cours de partie** | **Inchangé.** Seul le badge de statut de l'écran d'administration reflète la perte (§5.6 de `hue-bridge.md`). | **Aucune** sur l'éclairage — voir §10.3 pour le retour |
| **Arrêt du serveur** | **Extinction totale** : ampoules Hue **et** LED des buzzers. | **Oui — seul cas des trois**, voir §10.4 |

`hue-bridge.md` §5.5 reste la référence pour le comportement dégradé du pont ; la présente section
ne tranche que ce que l'**éclairage** fait pendant ce temps : rien.

### 10.3 Resynchronisation au retour du pont

Au retour d'un pont redevenu joignable, l'éclairage est **recalculé et réappliqué à partir de
l'état de jeu courant** — la scène correspondant à ce qui se passe dans le jeu **à cet instant**.
Jamais un état neutre, jamais la dernière scène connue avant la coupure.

> **Note d'implémentation.** La machinerie existe déjà : l'écrivain **re-dérive systématiquement
> depuis l'état vivant** à chaque `NotifyState` (§4.1, « ne jamais mémoriser la charge utile »).
> La resynchronisation se réduit donc à **déclencher un `NotifyState` sur la transition de
> reconnexion** du pilote. Aucun rejeu, aucun instantané à conserver. Si l'implémentation devient
> plus compliquée que cela, c'est le signe qu'on s'écarte du contrat.

**Si le sélecteur du §10.1 est sur ON ou OFF au retour du pont**, c'est **le mode** qui est
réappliqué, pas la scène de jeu : il est l'état courant de l'éclairage tant qu'il n'est pas ramené
sur AUTO. La resynchronisation réapplique donc « ce que l'éclairage doit montrer maintenant », dont
le mode fait partie — même formulation, même code, un seul chemin. En position **AUTO**, c'est bien
la scène de jeu qui est re-dérivée, comme décrit ci-dessus.

### 10.4 Extinction à l'arrêt du serveur — et le piège d'ordonnancement

L'arrêt du serveur éteint **les ampoules Hue et les LED des buzzers**.

> ⚠️ **Piège vérifié dans le code (`(*App).stop()`, `cmd/server/main.go`).**
> `stop()` appelle **`a.cancelCtx()` en tout premier**, avant `ardoiseCoalescer.Stop()`,
> `dnsServer.Stop()`, `mdnsServer.Stop()`, `httpServer.Stop()`, `broadcaster.Stop()` et
> `udpBcast.Stop()`.
>
> Une extinction écrite naïvement **après** ce `cancelCtx()` est **annulée à l'instant où elle est
> émise** : la requête HTTP vers le pont porte un contexte déjà mort, et les hubs WebSocket qui
> portent les buzzers sont en train de tomber. Le symptôme serait le pire possible — du code
> présent, une suite de tests verte, et la salle qui reste allumée sur la dernière scène.

**Exigences normatives :**
0. L'extinction porte sur **toutes** les ampoules pilotées, **y compris celles affectées à une
   équipe** — c'est l'une des deux dérogations à la règle « une ampoule d'équipe garde toujours sa
   couleur » (`hue-bridge.md` §5.2, révision du 2026-09-08). Cette règle régit l'**exploitation**,
   pas l'arrêt : laisser une ampoule allumée après l'arrêt du serveur reste inacceptable.
1. L'extinction s'effectue **avant** `a.cancelCtx()`.
2. Elle utilise **son propre contexte à échéance courte** (ordre de grandeur : 1 à 2 s), jamais
   `a.ctx`.
3. Elle **ne doit jamais retarder ni empêcher l'arrêt** si le pont ne répond pas. Un pont
   injoignable à l'arrêt est un cas **normal**, pas une erreur — au plus une ligne de log.

### 10.5 Tout événement qui change la salle doit notifier — même sans LED

Tout événement de jeu qui change ce que la salle doit montrer **doit** notifier l'écrivain,
**y compris lorsqu'il n'émet aucune LED de buzzer**.

Cas fondateur, constaté à la reprise du milestone : le drapeau `GameState.Entracte` d'une
**ENTRACTE programmée** (#214, ENTRACTE comme 7e type de question) est levé par
`startEntracteQuestionUnsafe()` depuis `actualStart()`, c'est-à-dire **à la fin du décompte**. Or
`broadcastStart()` — seul site `NotifyState` de la séquence de démarrage — n'est appelé qu'**au
lancement** du décompte (`GetPhase() == PhaseCountdown`). Entre les deux, aucune notification : la
fin de décompte ne déclenche que `OnStateChange` → `broadcastGameState` + `broadcastQuestions`,
dont **aucun n'émet de LED** et donc aucun n'est un site du registre §6. La salle restait sur la
scène READY/RUNNING au lieu de passer en `KindEntracte`.

> ⚠️ **Portée réelle du test d'exhaustivité §7 — à ne pas surestimer.**
> Il compare les sites **porteurs de LED**. Un événement qui change la salle **sans changer une
> seule LED de buzzer lui est structurellement invisible.** Le registre §6 est un filet à mailles
> connues, **pas une preuve d'exhaustivité fonctionnelle**. Toute nouvelle transition de jeu doit
> être confrontée à cette section, pas seulement au test.

---

## 11. Hors de ce contrat

| Sujet | Issue |
|---|---|
| Pilote BLE réel, conversion RGB → CIE xy, appairage | #206 |
| Schéma de configuration, endpoints HTTP, écran d'administration | #207 |
| Affectation ampoule → équipe, résolution de `Teams` en zones, règles de dégradation | #213 |
| Commandes manuelles `/admin/ambiance` — **composant et endpoints** (la règle est au §10.1) | #208 |
| Conduite de l'éclairage par l'animateur depuis `/anim` | **jamais demandé — hors périmètre** (révision du 2026-09-07) |
| Association configurable effet ↔ événement (« seconde phase ») | v10.1 (#210) |
| Restitution en fin de partie | **retiré du périmètre** (décision du 2026-09-07, §10.2) |
| Édition des scènes, effets répartis, synchronisation sur le minuteur | v10.1 (#210, #211, #212) |
