# BuzzControl - Machine à États du Jeu

Ce document décrit la machine à états qui gouverne le déroulement d'une partie dans BuzzControl.

## États du Jeu (Phases)

| Phase | Description | Badge Timer |
|-------|-------------|-------------|
| **STOPPED** | État initial ou après arrêt du jeu. Aucune question active. | *(aucun)* |
| **PREPARE** | Question sélectionnée, en attente des PONG des buzzers. | `PREPARATION` (orange) |
| **READY** | Tous les buzzers ont répondu PONG, prêt à démarrer. | `PRET` (cyan) |
| **STARTED** | Jeu en cours, chronomètre actif. | `EN COURS` (vert) |
| **PAUSED** | Jeu en pause, chronomètre suspendu. | `PAUSE` (bleu) |
| **REVEALED** | Réponse affichée. | `REPONSE` (violet) |

## Diagramme des États

```
                    ┌─────────────────────────────────────┐
                    │                                     │
                    ▼                                     │
┌─────────┐ click ┌─────────┐  tous PONG  ┌───────┐      │
│ STOPPED │─quest─►│ PREPARE │────reçus───►│ READY │      │
└────┬────┘       └────▲────┘             └───┬───┘      │
     │                 │  ▲                   │          │
     │                 │  │            click START       │
     │                 │  │                   │          │
     │  ┌──────────┐   │  │                   ▼          │
     │  │ REVEALED │◄──┼──┼────────────┌─────────┐       │
     │  └────┬─────┘   │  │            │ STARTED │◄──┐   │
     │       │         │  │            └────┬────┘   │   │
     │       │         │  │                 │        │   │
     │       │         │  └─click question──┘        │   │
     │       │         │                    │        │   │
     ▲       │         │              click PAUSE    │   │
     │ click │         │                    │        │   │
     │REPONSE│         │                    ▼        │   │
     │       │         │               ┌────────┐    │   │
     │       │         │               │ PAUSED │────┘   │
     │       ▼         │               └───┬────┘ click  │
     └───────┴─────────┴───ou timer=0──────┘    CONTINUE │
                       │                                 │
                       └────────click question───────────┘
```

## Tableau des Transitions

| État Courant | État Cible | Événement | Comportement Admin | Comportement Joueur (TV) |
|--------------|------------|-----------|-------------------|-------------------------|
| **STOPPED** | PREPARE | Click question | Question mise en évidence (bordure bleue), équipes/joueurs grisés, boutons START/PAUSE/REPONSE désactivés | Efface l'écran, affiche "PREPAREZ-VOUS" |
| **REVEALED** | PREPARE | Click question | Idem ci-dessus | Idem ci-dessus |
| **PREPARE** | PREPARE | Click autre question | Change de question, renvoie PING, boutons inchangés | Réaffiche "PREPAREZ-VOUS" |
| **PREPARE** | PREPARE | Réception PONG | Le joueur concerné reprend sa couleur. L'équipe reprend sa couleur quand TOUS ses joueurs ont envoyé PONG. Boutons inchangés (désactivés) | Pas de changement |
| **PREPARE** | READY | Tous PONG reçus **ET participants conformes** | Bouton START devient actif. PAUSE/REPONSE restent désactivés | "PREPAREZ-VOUS" clignote |
| **READY** | PREPARE | Sélection participants non conforme (#172) | Retour automatique en attente, motif d'incompatibilité affiché | Réaffiche "PREPAREZ-VOUS" |
| **READY** | PREPARE | Click autre question | Change de question, renvoie PING, retour en attente des PONGs | Réaffiche "PREPAREZ-VOUS" |
| **READY** | STARTED | Click START | Bouton START devient STOP (actif). Bouton PAUSE actif. REPONSE désactivé. Timer démarre | Affiche la question + média. Chronomètre démarre |
| **STARTED** | PAUSED | Click PAUSE | STOP reste actif. PAUSE devient CONTINUE (actif). REPONSE désactivé | Chronomètre en pause (clignote) |
| **PAUSED** | STARTED | Click CONTINUE | STOP reste actif. CONTINUE devient PAUSE (actif). REPONSE désactivé | Chronomètre reprend |
| **STARTED** | STOPPED | Click STOP | STOP désactivé. PAUSE désactivé. REPONSE actif | Chronomètre arrêté, question visible (sans réponse) |
| **PAUSED** | STOPPED | Click STOP | STOP désactivé. PAUSE désactivé. REPONSE actif | Chronomètre arrêté, question visible (sans réponse) |
| **STARTED** | STOPPED | Timer = 0 | STOP désactivé. PAUSE désactivé. REPONSE actif | Chronomètre à 00:00, question visible (sans réponse) |
| **PAUSED** | STOPPED | Timer = 0 | STOP désactivé. PAUSE désactivé. REPONSE actif | Chronomètre à 00:00, question visible (sans réponse) |
| **STOPPED** | REVEALED | Click REPONSE | STOP désactivé. PAUSE désactivé. REPONSE désactivé | Affiche la réponse |

## Affichage TV par État

| État | Affichage TV (Question normale) | Affichage TV (Question QCM) |
|------|--------------------------------|----------------------------|
| **STOPPED** (initial) | Écran vide / Logo | Écran vide / Logo |
| **PREPARE** | "PREPAREZ-VOUS" fixe | "PREPAREZ-VOUS" fixe |
| **READY** | "PREPAREZ-VOUS" clignotant | "PREPAREZ-VOUS" clignotant + **4 réponses QCM affichées** |
| **STARTED** | Question + média + chronomètre actif | Question + média + chronomètre + 4 réponses QCM |
| **PAUSED** | Question + média + chronomètre clignotant | Question + média + chronomètre clignotant + 4 réponses QCM |
| **STOPPED** (après jeu) | Question + média + chronomètre arrêté (SANS réponse) | Question + média + chronomètre arrêté + 4 réponses QCM |
| **REVEALED** | Question + média + **RÉPONSE** | Question + média + **bonne réponse en couleur, mauvaises grisées** |

## États des Boutons Admin

| État | START/STOP | PAUSE/CONTINUE | REPONSE |
|------|------------|----------------|---------|
| **STOPPED** (initial) | Désactivé | Désactivé | Désactivé |
| **PREPARE** | Désactivé | Désactivé | Désactivé |
| **READY** | START (actif) | Désactivé | Désactivé |
| **STARTED** | STOP (actif) | PAUSE (actif) | Désactivé |
| **PAUSED** | STOP (actif) | CONTINUE (actif) | Désactivé |
| **STOPPED** (après jeu) | Désactivé | Désactivé | **Actif** |
| **REVEALED** | Désactivé | Désactivé | Désactivé |

> **Note**: Le bouton REPONSE n'est actif que dans l'état STOPPED après qu'une question ait été jouée (transition depuis STARTED ou PAUSED).

## Messages Broadcast (WebSocket/TCP)

| Transition | Message Broadcast | Destinataires |
|------------|-------------------|---------------|
| → PREPARE | PING | Buzzers (TCP) |
| → READY | READY | Web clients + Buzzers |
| → STARTED | START | Web clients + Buzzers |
| → PAUSED | PAUSE | Web clients + Buzzers |
| → STOPPED | STOP | Web clients + Buzzers |
| → REVEALED | REVEAL | Web clients |

## Gestion des Équipes/Joueurs

### Phase PREPARE
- Tous les joueurs et équipes sont grisés (en attente de PONG)
- À la réception d'un PONG d'un joueur :
  - Le joueur reprend sa couleur
  - L'équipe reprend sa couleur **uniquement** quand **tous** ses joueurs ont envoyé leur PONG

### Transition PREPARE → READY
- Se déclenche automatiquement quand **tous les buzzers connectés ont répondu PONG**
- **ET** que la sélection des équipes participantes est **conforme** aux règles du mode (#172)
- Si aucun buzzer n'est connecté, la transition est immédiate (si conformité OK)

### Règles de Conformité des Participants (#172)

La sortie de `PREPARE` vers `READY` dépend désormais de deux critères indépendants : les buzzers physiques ET la sélection des équipes.

**Pour les modes simples (SPEEDY, QCM, ARDOISE) :** Aucun changement — au moins une équipe active requise, déjà le cas auparavant.

**Pour les modes de sélection :**

| Type | Critère | Exemple | Comportement |
|------|---------|---------|--------------|
| **MEMORY SOLO** | Exactement 1 équipe sélectionnée | `["Red"]` | Permis → READY, sinon reste PREPARE |
| **MEMORY multi** (CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE) | Au moins 2 équipes sélectionnées | `["Red", "Blue"]` | ≥2 → READY ; <2 → PREPARE |
| **MEMOTION** | Au moins 1 équipe sélectionnée | `["Red"]` | ≥1 → READY ; 0 → PREPARE |
| **Type inconnu** | Permissif | N/A | Toujours permis (défaut sûr) |

### Retour Arrière READY → PREPARE (#172)

Si la sélection des participants cesse d'être conforme **alors que la phase est `READY`**, la question repasse automatiquement en `PREPARE` :
- **Quand cela arrive** : un administrateur retire une équipe de la sélection (ex. en MEMORY SOLO, passer de 1 équipe à 0, ou en MEMORY multi, de 2 équipes à 1)
- **Jamais depuis une phase de jeu** : `STARTED`, `PAUSED`, `COUNTDOWN`, `REVEALED` restent insensibles aux changements de sélection
- **Comportement** : le motif d'attente est affiché à la régie et sur la tablette animateur (`#172 Bloc C`)
- **Remise en conformité** : dès que la sélection redevient valide, repasse automatiquement en `READY` sans geste supplémentaire

## Type de jeu RAFALE (v8.0.0, #16)

### Vue d'ensemble

RAFALE n'ajoute **aucune phase** à la machine à états principale (STOPPED/PREPARE/READY/COUNTDOWN/STARTED/PAUSED/REVEALED). C'est un **nouveau `QuestionType`** qui suit le patron MEMOTION : il porte une configuration de manche et génère des **sous-phases** internes.

### Sous-phases RAFALE

Au lieu d'une seule question avec une seule réponse, RAFALE enchaîne les questions automatiquement jusqu'à expiration du timer. Deux sous-phases gouvernent ce comportement :

| Sous-phase | Description | Durée | Comportement |
|---|---|---|---|
| **QUESTION** | Question courante affichée, timer question actif (~3 s) | ~3 s | L'animateur valide/invalide, ou timer=0 → réponse invalide |
| **ROUND_END** | Manche terminée, équipes attendent attribution des points | indéfini | Admin/anim clique une équipe → `TEAM_POINTS`, revient à STOPPED |

```
Début manche RAFALE (phase=STARTED)
    ↓
[QUESTION] — timer manche actif (~120 s), timer question actif (~3 s)
    ↓
    ├─ VALIDE → compteur++, rotation si applicable, pioche suivante
    │
    ├─ INVALIDE → rotation si applicable, pioche suivante
    │
    ├─ Timer question → 0 (identique à INVALIDE)
    │
    └─ Timer manche → 0 OU pool épuisé OU plafond MAX_QUESTIONS
                         ↓
                    [ROUND_END]
                         ↓
                    Admin clique équipe (TEAM_POINTS) → fin manche

```

### Champs GameState RAFALE (tous présents, aucun `omitempty`)

```go
RAFALE_SUBPHASE          RafaleSubPhase  // "" | "QUESTION" | "ROUND_END"
RAFALE_CURRENT_QUESTION  RafaleCurrent   // {ID, QUESTION, CATEGORY, DIFFICULTY} (JAMAIS ANSWER)
RAFALE_QUESTION_TIME     int             // décompte timer question courant (ms)
RAFALE_TEAM_COUNTERS     map[string]int  // compteur réponses correctes par équipe
RAFALE_TEAM_BEST         map[string]int  // meilleur compteur atteint (MAILLON_FAIBLE only)
RAFALE_CURRENT_TEAM      string          // équipe jouant (multi-mode)
RAFALE_PARTICIPATING_TEAMS []string      // liste équipes (ordre de rotation)
RAFALE_CURRENT_TEAM_COLOR []int          // RGB couleur équipe active
RAFALE_ASKED_COUNT       int             // nombre total de questions tirées
RAFALE_POOL_REMAINING    int             // questions encore disponibles
RAFALE_EXHAUSTED         bool            // true si pool vide pendant manche
```

**Persistance** : tous ces champs sont **éphémères** (exclus de `PersistedGameState`). Seul `rafale_used.json` persiste.

### Actions WebSocket spécifiques RAFALE

**Client → Serveur** (allow-list fermée) :
- `RAFALE_VALIDATE` (admin+anim) : réponse jugée correcte
- `RAFALE_INVALIDATE` (admin+anim) : réponse jugée incorrecte
- `RAFALE_SET_TEAMS` (admin) : sélection équipes + ordre de rotation

**Serveur → Client** :
- `RAFALE_ANSWER` (admin+anim uniquement) : `{ID, ANSWER}` — réponse attendue, jamais via `GameState`
- `RAFALE_TICK` (tous) : `{QUESTION_TIME}` — décompte timer question (payload léger)
- `UPDATE_TIMER` (tous) : décompte timer manche (réutilise action existante, inchangée)

### Modes de jeu RAFALE

| Mode | Rotation | Compteur sur Mauvais |
|---|---|---|
| **SOLO** | Aucune | N/A (pas de rotation) |
| **CHACUN_SON_TOUR** | À chaque réponse (V ou I) | Gardé, équipe suivante |
| **TANT_QUE_JE_GAGNE** | Réponse I uniquement | Gardé, même équipe continue |
| **MAILLON_FAIBLE** | À chaque réponse | Remis à 0, meilleur mémorisé |

**Attribution de points** : aucun point réel pendant manche. Fin de manche → sous-phase `ROUND_END`, l'admin clique équipe → action `TEAM_POINTS` (valeur pré-remplie = compteur_retenu × barème_manche, ajustable).

### Épuisement du réservoir

Si la pool devient vide avant expiration du timer de manche :
- `RAFALE_EXHAUSTED = true`
- Sous-phase → `ROUND_END`
- Timer de manche arrêté, message explicite à l'animateur
- **Jamais de reproposition** d'une question déjà vue

### Pré-validations avant démarrage

Avant `START`, l'admin voit :
- **Nombre de questions disponibles** pour filtres catégories + difficulté actuels
- **Besoin estimé** = `ceil(durée_manche / temps_par_question)` (majorant)
- **Trois états d'alerte** :
  - 🔴 Bloquant : `disponibles == 0` → démarrage refusé
  - 🟠 Avertissement : `disponibles < besoin_estimé` → démarrage autorisé, risque fin anticipée
  - ✅ Neutre : `disponibles ≥ besoin_estimé` → OK

**Plafond dur** : `RAFALE_MAX_QUESTIONS` (défaut 100, maximum 100). Fin manche si atteint, même si timer tourne encore.

## Implémentation

### Backend (Go)
- Fichier: `server-go/internal/game/engine.go`
- Les phases sont définies comme constantes: `PhaseStopped`, `PhasePrepare`, `PhaseReady`, `PhaseStarted`, `PhasePaused`, `PhaseRevealed`
- Sous-phases RAFALE : `RafaleSubPhaseQuestion`, `RafaleSubPhaseRoundEnd` (constantes `internal/game/models.go`)

#### Verrou de phase sur `Engine.Start()` (#172)
La méthode `Engine.Start()` refuse toute phase autre que `READY` :
```go
if e.state.Phase != PhaseReady {
    // Log + return sans rien modifier
}
```
**Conséquence** : une partie démarrée est nécessairement conforme, et le reste (car `SetMemoryParticipatingTeams` et `SetMotionParticipatingTeams` n'acceptent que PREPARE/READY, interdites une fois `STARTED` atteint).

**`ForceReady()` (fonction de débogage, admin)** : saute l'attente des PONG mais **respecte toujours** la conformité des participants — voir arbitrage G du plan #172.

### Frontend Admin (React)
- Fichier: `server-go/web/src/pages/GamePage.jsx`
- Les états des boutons sont calculés en fonction de `gameState.phase`

### Frontend Joueur (React)
- Fichier: `server-go/web/src/pages/PlayerDisplay.jsx`
- L'affichage est conditionné par `gameState.phase`

### Frontend Animateur (React) — v6.2.x
- Fichier: `server-go/web/src/pages/AnimPage.jsx` (page `/anim`), zone conduite `AnimConductPanel.jsx`
- Conduite en direct des transitions de phase depuis une tablette (LANCER/PAUSE/CONTINUER/STOP/RÉPONSE), au même titre que `/admin` — voir `docs/REACT_INTERFACE.md` §"Layout AnimPage"
- **Depuis #159 (v6.2.0.27), l'animateur peut également agir directement sur l'état de jeu en mode MEMORY** : la grille (`AnimMemoryGrid.jsx`) permet de retourner les cartes du doigt sur sa tablette, action `FLIP_MEMORY_CARD` (auparavant réservée à `tv`/`vplayer`). C'est le premier mode où l'interface animateur modifie l'état de jeu (paires trouvées, erreurs, tour de l'équipe active) au-delà de la conduite de phase et du crédit de points — voir `docs/WEBSOCKET_PROTOCOL.md` §"Liste blanche entrante" pour le détail de cet élargissement de capacité. Le choix des équipes participantes (`MEMORY_SET_TEAMS`) reste réservé à `/admin`.

## États du Chronomètre (Timer)

Le chronomètre affiche différents états visuels :

| État | Affichage | Animation | Couleur |
|------|-----------|-----------|---------|
| **Normal** | `MM:SS` | Barre de progression verte | Vert |
| **Urgent** (≤10s) | `MM:SS` | Barre orange, shine rapide | Orange |
| **Critique** (≤5s) | `MM:SS` | Pulsation, ombre rouge | Rouge |
| **Pause** | `MM:SS` | Clignotement opacité | Bleu |

### Barre de progression

- **>50%** : Vert (`--success`)
- **25-50%** : Orange (`--warning`)
- **<25%** : Rouge (`--error`)

## ENTRACTE — État Transverse (v6.5.2, #119)

### Concept

`ENTRACTE` est un **booléen transverse** de `GameState`, indépendant du cycle de question. 
Il représente une **pause globale** de la partie (déjeuner, changement de salle, annonce) :

- Quand actif (`ENTRACTE = true`) :
  - Les **quatre surfaces** (TV, VJoueur, admin, animateur) affichent un **panneau 
    estompé** au-dessus du contenu existant.
  - Aucune action de jeu (`START`, `READY`, `REVEAL`, etc.) n'est acceptée.
  - Seules les actions de maintenance sont autorisées (`ENTRACTE_SET`, `HELLO`, etc.).
  - **LEDs des buzzers éteintes**.

- Quand inactif (`ENTRACTE = false`) :
  - Comportement normal — le cycle de question reprend.
  - LEDs restaurées à l'état correspondant à la phase courante.

### Phases autorisées pour l'entrée en entracte

| Phase | Autorisée | Motif |
|---|---|---|
| `STOPPED` | ✅ Oui | Aucune question en cours |
| `PREPARE` | ✅ Oui | Aucune question en cours |
| `READY` | ✅ Oui | Aucune question en cours |
| `NEW_GAME` | ✅ Oui | Transition avant question, pas de manche active |
| `REVEALED` | ✅ Oui | Question finie, scoring en cours — moment naturel pour annoncer une pause |
| `COUNTDOWN` | ❌ Non | Compte à rebours avant lancement — question imminente |
| `STARTED` | ❌ Non | Manche en cours — ne jamais couper une manche active |
| `PAUSED` | ❌ Non | Manche temporairement arrêtée — toujours en cours |
| `ENROLL` | ❌ Non | Écran d'inscription — QR code visible, le masquer serait contre-productif |

### Sortie de l'entracte

La **sortie est autorisée depuis n'importe quelle phase** — on ne doit jamais rester 
coincé en entracte.

### Configuration gelée à l'activation

À l'entrée en entracte, la configuration courante est **sauvegardée et gelée** dans 
`ENTRACTE_CONFIG_SAVED` — la configuration du panneau ne change plus pendant toute la 
durée de l'entracte. Les modifications apportées pendant une pause active (titre, 
animation, etc.) prennent effet au **prochain cycle d'entracte**, pas immédiatement.

Configuration persistée dans `game_state.json` (cycle de vie `QUIZ_*`), partie intégrante 
de la partie en cours — pas une section globale de réglages.

## Historique

| Version | Date | Changements |
|---------|------|-------------|
| 2.9.1 | 2026-01 | Fix deadlock callbacks, cohérence noms phases frontend/backend |
| 2.9.0 | 2026-01 | Ajout transitions PREPARE→PREPARE et READY→PREPARE pour changer de question |
| 2.8.0 | 2026-01 | Refonte complète de la machine à états |

---

## Phase NEW_GAME (v4.0.1 / v4.0.4)

**Phase NEW_GAME** : état transitoire déclenché par le bouton "NOUVELLE PARTIE" (admin, phase STOPPED uniquement).
- Reset : scores teams + joueurs à 0, historique vide, tous statuts questions → AVAILABLE, question sélectionnée → nil
- Transition : `NEW_GAME → PREPARE` automatique à la sélection de la première question
- TV (`/tv`) : affiche l'écran "NOUVELLE PARTIE À VENIR" avec métadonnées du quiz

**Métadonnées de quiz** (v4.0.1) :
- `quiz_name`, `quiz_theme`, `quiz_notes` — champs GameState, sans `omitempty`, persistés
- Mis à jour via `UPDATE_QUIZ_META` (payload : `NAME`, `THEME`, `NOTES`)

**Fonds d'écran NEW_GAME** (v4.0.4) :
- Stockés dans `data/files/new-game-backgrounds/` + `backgrounds.json` — `[]Background{Path, Duration, Opacity}`
- `POST /new-game-backgrounds` | `PUT /new-game-backgrounds` | `DELETE /new-game-backgrounds?file=xxx`
- Sérialisé en `new_game_backgrounds` dans GameState et CONFIG_UPDATE (jamais `omitempty`)
- Fallback : dégradé animé (violet→bleu→cyan→rose→ambre, 9s infini)

**Actions WebSocket** :
```json
{ "ACTION": "NEW_GAME", "MSG": {} }
{ "ACTION": "UPDATE_QUIZ_META", "MSG": { "NAME": "Mon Quiz", "THEME": "Science", "NOTES": "..." } }
```

Refus NEW_GAME si phase ≠ STOPPED : `{ "ACTION": "REMOTE", "MSG": { "error": "..." } }`

**Fichiers clés** : `engine.go` (`InitGame()`), `models.go` (champs QuizName/Theme/Notes/NewGameBackgrounds), `main.go` (handlers + loadNewGameBackgrounds), `PlayerDisplay.jsx` (écran TV NEW_GAME)

---

## Type de question ARDOISE (v5.6.0 / Milestone v5.6.x)

**Type de question** : `ARDOISE` — réponse libre au clavier (joueur virtuel VPlayer)

### Comportement de la machine d'états

La machine d'états ARDOISE suit le **même cycle que QUIZ** avec le discriminant `question.TYPE === "ARDOISE"`.

**Cycle ARDOISE** :
1. Admin sélectionne question ARDOISE → `PREPARE` : VPlayer envoie PING
2. Tous PONGs reçus → `READY`
3. Admin clique START → `STARTED` : clavier VPlayer **actif** (inputs traités + envoyés)
4. Admin clique STOP ou timer expire → `STOPPED`
5. Admin clique REPONSE → `REVEALED` : affiche réponses équipes + bonne réponse

### Affichage TV et clavier VPlayer

| Phase | Affichage question | Clavier VPlayer | État inputs |
|-------|-------------------|-----------------|------------|
| **PREPARE** | "PREPAREZ-VOUS" | Affiché | Verrouillé (phase ≠ STARTED) |
| **READY** | "PREPAREZ-VOUS" clignote | Affiché | Verrouillé (phase ≠ STARTED) |
| **STARTED** | Question ARDOISE texte + média | Affiché | **Actif** (inputs traités + envoyés) |
| **PAUSED** | Question + chronomètre clignotant | Affiché | Verrouillé (phase ≠ STARTED) |
| **STOPPED** | Question + chronomètre arrêté (SANS réponse) | Affiché | Verrouillé (phase ≠ STARTED) |
| **REVEALED** | Question + **BONNE RÉPONSE** + réponses équipes | Masqué | Verrouillé |

**Règle clavier VPlayer** :
- Clavier **toujours affiché** dès que `question.TYPE === "ARDOISE"` (y compris PREPARE/READY/countdown)
- Clavier **actif** (inputs acceptés + envoyés au serveur) **uniquement** lors de `phase === "STARTED"`
- Clavier **verrouillé** (inputs ignorés localement) dès que phase quitte STARTED (STOP, PAUSE, REVEALED)

### Nouveau champ GameState : ARDOISE_ANSWERS

Dictionnaire équipe → réponse saisie.

```go
ARDOISE_ANSWERS map[string]ArdoiseAnswer

type ArdoiseAnswer struct {
    TEXT        string // Texte saisi par l'équipe
    SUBMITTED_AT int64  // Timestamp microseconde (µs), moment du dernier input envoyé
}
```

**Sérialisation** : **Jamais `omitempty`** — toujours présent dans GameState (évite réinitialisations manquées côté frontend).

**Initialisation** : 
- À chaque nouvelle question (dans `Ready()`) : reset `ARDOISE_ANSWERS = {}`
- Rempli au fur et à mesure des `ARDOISE_INPUT` reçus du serveur

### Actions WebSocket ARDOISE

```json
{ "ACTION": "ARDOISE_INPUT", "MSG": { "TEXT": "Paris" } }
```

**Envoi VPlayer** (throttling + flush forcé) :
- **Throttlé 200ms** : texte complet envoyé toutes les 200ms
- **Flush forcé** sur STOP/PAUSE : dès que le serveur envoie STOP ou PAUSE, l'input en cours est envoyé immédiatement au serveur
- **Verrouillage** : aucun envoi post-REVEALED

Voir `WEBSOCKET_PROTOCOL.md` pour détails complets.

---

## Type de jeu MEMOTION (v5.0.0 et v7.0.0)

Jeu de cartes à 3 faces avec grille interactive, difficulté configurable et mode équipe. À partir de v7.0.0, les cartes MEMOTION supportent plusieurs types de questions imbriquées (QCM, SPEEDY, ARDOISE, MEMORY) avec un système de points configurable.

### Structure MotionCard (historique + v7.0.0)

```json
{
  "ID": "mc-1",
  "TYPE": "QCM",
  "RECTO_THEME": "Thème de la carte",
  "RECTO_IMAGE": "/files/questions/img_recto.jpg",
  "DIFFICULTY": 2,
  "POINTS_RULE": { "MODE": "STARS", "VALUE": 0 },
  "QUESTION_TEXT": "Question ou énigme",
  "QUESTION_IMAGE": "/files/questions/img_question.jpg",
  "ANSWER_TEXT": "Réponse ou explication",
  "ANSWER_IMAGE": "/files/questions/img_answer.jpg",
  "QCM_ANSWERS": {"1": "Opt A", "2": "Opt B", "3": "Opt C", "4": "Opt D"},
  "QCM_CORRECT": "1"
}
```

#### Champ `TYPE` (v7.0.0)
- **Absent ou `""`** → défaut `SPEEDY` (rétrocompatibilité)
- **Valeurs acceptées** : `SPEEDY` | `QCM` | `ARDOISE` | `MEMORY`
- **Interdit** : `MEMOTION` (profondeur 1 — une carte ne porte jamais une autre carte)
- **Validation** : l'enregistrement refuse les types non-imbriquables ou invalides

#### Champ `POINTS_RULE` (v7.0.0)
```jsonc
"POINTS_RULE": { "MODE": "STARS" | "FIXED" | "PER_UNIT", "VALUE": 10 }
```

| `MODE` | Points attribués | Usage |
|---|---|---|
| absent / `STARS` | Barème par étoiles existant : `MOTION_CONFIG.POINTS_<n>_STAR` si `> 0`, sinon `DIFFICULTY` → 1/3/5 | **Défaut** — comportement actuel, inchangé |
| `FIXED` | `VALUE` si équipe gagnante, sinon 0 | Carte dont la valeur ne dépend pas de sa difficulté |
| `PER_UNIT` | `VALUE × Units` | Types à progression (MEMORY au prorata, #187 v7.1.0) |

**Défaut** : absent ou `MODE="STARS"` → applique l'ancien barème par difficulté.
**Non-régression** : les 9 questions MEMOTION existantes ne portent pas ce champ, donc valent STARS → comportement inchangé.

### Champ `MEMOTION_ACTIVE` (v7.0.0)

État vivant unique du type imbriqué actif. Réinitialisé à chaque `MEMOTION_SELECT`, vidé au retour en `GRID`.

```jsonc
"MEMOTION_ACTIVE": {
  "CARD_ID": "mc-3",          // "" hors SELECTED/QUESTION/REVEAL
  "TYPE": "QCM",              // "" hors emplacement actif ; "SPEEDY" pour une carte historique
  "STATE": {                  // état vivant du type — forme libre, propre au type
    "QCM_INVALIDATED": ["RED", "YELLOW"]
  }
}
```

**Caractéristiques** :
- **Jamais `omitempty`** — règle projet, toujours sérialisé même vide (`{"CARD_ID":"","TYPE":"","STATE":{}}`)
- **Non persisté** — rejoint `Motion*` dans `state_persistence.go`
- **Un seul emplacement** — pas de dictionnaire par carte, une seule carte jouable à la fois

**`CARD_ID`** :
- `""` en sous-phases `MEMORIZE`, `GRID`, hors jeu MEMOTION
- ID de la carte (chaîne `"mc-3"`) en `SELECTED`, `QUESTION`, `REVEAL`
- Synchronisé avec `MEMOTION_SELECTED` toujours

**`TYPE`** :
- Type de la carte active (discriminant du contenu de `STATE`)
- `""` quand `CARD_ID` est vide

**`STATE`** :
- Structure propre au type — ex. `{"QCM_INVALIDATED": [...]}` pour un QCM
- Vide (`{}`) quand aucune carte n'est active
- Remis à zéro à chaque sélection de nouvelle carte

### Contexte d'hôte normalisé — `HostContext` (v7.0.0)

Une implémentation de type (composant QCM, SPEEDY, ARDOISE, MEMORY) ne lit **jamais** `PHASE` ou `MEMOTION_SUBPHASE` directement. Elle reçoit de son hôte un triplet normalisé.

```go
type HostContext struct {
    Playable     bool   // les entrées sont acceptées, le contenu est en jeu
    Revealed     bool   // la réponse est montrée
    TimerRunning bool   // un chronomètre décompte pour cette manche
    CardID       string // "" pour l'hôte question ; ID de carte pour l'hôte carte MEMOTION
}
```

**Table de dérivation — implémentée côté Go et JavaScript de manière identique** :

| Hôte | `Playable` | `Revealed` | `TimerRunning` |
|---|---|---|---|
| **Question** | `PHASE == STARTED` | `PHASE == REVEALED` | `PHASE == STARTED` **ET** `CURRENT_TIME > 0` |
| **Carte MEMOTION** | `MEMOTION_SUBPHASE == QUESTION` | `MEMOTION_SUBPHASE == REVEAL` | `MEMOTION_SUBPHASE == QUESTION` **ET** `CURRENT_TIME > 0` |
| **Aucun** (GRID, MEMORIZE, SELECTED, PREPARE, READY…) | `false` | `false` | `false` |

**`CardID`** — règle unique :
- Toujours `MEMOTION_SELECTED` (jamais de condition)
- `""` hors jeu MEMOTION ou en `GRID`/`MEMORIZE`/`SELECTED`
- ID de carte en `QUESTION` et `REVEAL`

**`TimerRunning` — jamais dérivé du ticker serveur** :
- Uniquement de `CURRENT_TIME > 0` dans l'état de jeu courant
- Un ticker serveur (`Engine.timer`) n'est jamais sérialisé ni observable côté client
- Convention en production depuis #160 : `AnimPage.jsx` utilise `gameState.timer > 0`

**Non sérialisé** — recalculé côté Go et JavaScript à partir de `PHASE` et `MEMOTION_SUBPHASE`, tous deux déjà présents. Économie de charge utile. Contrepartie : chaque côté porte un test dont les cas sont nommés à l'identique (voir tests de dérivation).

### Contrainte de profondeur 1

**Une carte MEMOTION ne peut pas porter le type `MEMOTION`.**

Cette contrainte est structurelle et validée à trois points :
1. **À l'enregistrement** : `handleUploadQuestion` refuse `TYPE="MEMOTION"` avec HTTP 400
2. **À la sélection** : `SelectMotionCard` ignore silencieusement les tentatives d'imbrication récursive (défense en profondeur)
3. **Dans le registre** : `MEMOTION` n'a pas `NestableInMotionCard = true`, marqué explicitement `false`

Motif : une grille de cartes imbriquées dans une carte imbriquée dans une grille crée une récursion sans fin. Une carte MEMOTION n'est jamais elle-même un hôte MEMOTION.

### Subphases MEMOTION

| Subphase | Description | `HostContext` (`Playable`/`Revealed`/`TimerRunning`) |
|----------|-------------|---|
| `MEMORIZE` | *(Secret Mode)* Toutes les cartes RECTO visibles + timer décompte. Sélection impossible. | `F / F / F` |
| `GRID` | Grille de cartes RECTO (ou coordonnées en Secret Mode), prêtes à sélectionner | `F / F / F` |
| `SELECTED` | Carte zoomée plein écran (RECTO thème + points). Pas de timer. | `F / F / F` |
| `QUESTION` | Face VERSO (question) plein écran, timer actif (si `CURRENT_TIME > 0`) | `T / F / T*` |
| `REVEAL` | Face REVEAL (réponse) plein écran, admin attribue points | `F / T / F` |

**Note** : `QUESTION` avec timer expiré (CURRENT_TIME = 0) → `TimerRunning = false` même si `MEMOTION_SUBPHASE == QUESTION`

### Secret Mode — Subphase MEMORIZE (v5.5.0)

Activé si `MOTION_MEMORIZE_DURATION > 0` sur la question MEMOTION.

**Machine d'états Secret Mode :**
```
STARTED → MEMORIZE (timer MOTION_MEMORIZE_DURATION s) → [timer expiré] → GRID → SELECTED → QUESTION → REVEAL → GRID
```

**Comportement :**
- `MEMORIZE` : toutes les cartes affichées face RECTO, timer `CURRENT_TIME` décrémente (même format que QUESTION), sélection de carte refusée (`NOT_IN_GRID_SUBPHASE`)
- Transition `MEMORIZE → GRID` : automatique à expiration, aucune action admin requise
- Phase `GRID` en Secret Mode : coordonnées (A1, B2…) remplacent les thèmes sur les cartes verso ; l'admin voit la liste coordonnée↔thème pour guider les joueurs
- Rétrocompatibilité : `MOTION_MEMORIZE_DURATION = 0` (ou absent) → démarrage direct en `GRID` (comportement v5.0+)

### États visuels des cards MEMOTION

Structure commune à tous les jeux :
- `zone-timer` : 10vh (chronomètre + barre de progression)
- Zone de jeu : 90vh restants

**État 1 — RECTO UNPLAYED/SELECTED** (carte sur la grille, non jouée ou sélectionnée)
- Layout grid : 1/6 titre | 4/6 image | 1/6 étoiles
- Titre (RECTO_THEME) : centré horizontalement, 1/6 hauteur carte
- Image (RECTO_IMAGE) : 4/6 hauteur carte, object-fit: contain
- Étoiles (DIFFICULTY) : centrées horizontalement, 1/6 hauteur carte
- Fond : dégradé violet foncé
- Carte selected : pulsing glow blanc

**État 2 — SELECTED fullscreen** (carte sélectionnée, zoom plein écran avant flip)
- Même layout que RECTO (1/6 | 4/6 | 1/6)
- Même contenu : RECTO_THEME + RECTO_IMAGE + étoiles
- Occupe 90vh sous le timer (top: 10vh → bottom: 0)
- Fond : même violet que la carte

**État 3 — VERSO QUESTION** (plein écran, après flip)
- Zone question (1/6 × 90vh = ~15vh) : QUESTION_TEXT
- Zone media (4/6 × 90vh = ~60vh) : QUESTION_IMAGE
- Zone réponses (1/6 × 90vh = ~15vh) : vide

**État 4 — VERSO RÉPONSE** (plein écran, après révélation)
- Zone question (1/6) : QUESTION_TEXT (rappel)
- Zone media (4/6) : ANSWER_IMAGE (ou ANSWER_TEXT si pas d'image)
- Zone réponses (1/6) : ANSWER_TEXT (si image présente) ou vide

**État 5 — RECTO DONE** (carte jouée, retour sur la grille)
- Même layout que RECTO (1/6 | 4/6 | 1/6)
- Fond : couleur de l'équipe gagnante (au lieu de violet)
- Footer : étoiles + nom de l'équipe côte à côte
- Taille : 80% (scale 0.8) pour indiquer que la carte a été jouée

### Champs GameState MEMOTION (sans omitempty)
- `MEMOTION_ACTIVE` : état vivant du type imbriqué — voir §"Champ `MEMOTION_ACTIVE`" ci-dessus (v7.0.0)
- `MEMOTION_SUBPHASE` : `""` | `"MEMORIZE"` | `"GRID"` | `"SELECTED"` | `"QUESTION"` | `"REVEAL"` — `MEMORIZE` ajouté en v5.5.0 (Secret Mode)
- `MEMOTION_SELECTED` : ID carte sélectionnée (`""` en GRID) — même valeur que `MEMOTION_ACTIVE.CARD_ID`
- `MEMOTION_CARD_STATES` : map[string]string → `"UNPLAYED"` | `"SELECTED"` | `"QUESTION"` | `"REVEALED"` | `"DONE"`
- `MEMOTION_CARD_TEAMS` : map[string]string → teamName quand DONE
- `MEMOTION_CURRENT_TEAM` : équipe active
- `MEMOTION_PARTICIPATING_TEAMS` : []string
- `MEMOTION_CURRENT_TEAM_COLOR` : [3]int RGB

### Flux de jeu (5 étapes + gestion `MEMOTION_ACTIVE`)

1. **GRID → SELECTED** : Admin sélectionne une carte depuis la preview TV (clic sur carte `UNPLAYED`)
   - Action : `MEMOTION_SELECT` `{"CARD_ID": "mc-3"}`
   - Transitions : `MEMOTION_SUBPHASE = "SELECTED"`, `MEMOTION_SELECTED = "mc-3"`
   - **`MEMOTION_ACTIVE` initialisé** : `{"CARD_ID": "mc-3", "TYPE": <type de la carte>, "STATE": {}}`

2. **SELECTED → QUESTION** : Admin clique "Démarrer"
   - Action : `MEMOTION_FLIP`
   - Transitions : `MEMOTION_SUBPHASE = "QUESTION"`, `CURRENT_TIME = QUESTION.TIME`, timer démarre
   - **`MEMOTION_ACTIVE.STATE` alimenté** selon le type (ex. QCM : indices en cours, réponses invalidées…)

3. **QUESTION contrôle-temps** : Timer décroît ou arrêt manuel
   - Action : `MEMOTION_STOP_TIMER` (optionnel, si arrêt avant expiration)
   - État : `MEMOTION_SUBPHASE` reste `"QUESTION"`, `CURRENT_TIME` s'arrête
   - **`MEMOTION_ACTIVE.STATE` figé** à l'instant de l'arrêt

4. **QUESTION → REVEAL** : Admin clique "RÉVÉLER"
   - Action : `MEMOTION_REVEAL`
   - Transitions : `MEMOTION_SUBPHASE = "REVEAL"`, timer arrêté (via `StopMotionCardTimer()`)
   - État : réponse affichée, `HostContext.Revealed = true`
   - **`MEMOTION_ACTIVE.STATE` inchangé** (contient l'historique de la manche)

5. **REVEAL → GRID** : Admin clique "Perdu" ou bouton équipe gagnante
   - Action : `MEMOTION_DONE` `{"CARD_ID": "mc-3", "WINNER_TEAM": "team_A", "UNITS": 1}`
   - Transitions : retour `MEMOTION_SUBPHASE = "GRID"`, `MEMOTION_SELECTED = ""`, carte marquée DONE
   - Points attribués selon `MotionCard.POINTS_RULE` du barème de la carte
   - **`MEMOTION_ACTIVE` réinitialisé** : `{"CARD_ID": "", "TYPE": "", "STATE": {}}`

**Annulation depuis SELECTED** : `MEMOTION_DONE` avec `WINNER_TEAM=""` → carte retourne UNPLAYED, `MEMOTION_ACTIVE` vidé.

### Modes de jeu
- `SOLO` : une seule équipe joue
- `CHACUN_SON_TOUR` : rotation après chaque carte
- `TANT_QUE_JE_GAGNE` : conserve le tour si victoire

### Actions WebSocket

```json
{ "ACTION": "MEMOTION_SELECT",    "MSG": { "CARD_ID": "mc-1" } }
{ "ACTION": "MEMOTION_FLIP",      "MSG": {} }
{ "ACTION": "MEMOTION_STOP_TIMER","MSG": {} }
{ "ACTION": "MEMOTION_REVEAL",    "MSG": {} }
{ "ACTION": "MEMOTION_DONE",      "MSG": { "CARD_ID": "mc-1", "WINNER_TEAM": "team_A", "UNITS": 1 } }
{ "ACTION": "MEMOTION_SET_TEAMS", "MSG": { "TEAMS": ["team_A", "team_B"] } }
```

**Champ `UNITS` (v7.0.0)** :
- **Optionnel**, défaut `1`
- Rapporte le nombre d'unités gagnées (pour types à progression : MEMORY au prorata, #187 v7.1.0)
- Utilisé par barème `POINTS_RULE.MODE == "PER_UNIT"` : points = `VALUE × UNITS`
- Types simples (QCM, SPEEDY, ARDOISE) : toujours `1` (gagné/perdu)

**Invariant de portée** (v7.0.0, #184) :
- Toute action typée **hors `MEMOTION_DONE`** ignore `MOTION_CARD_ID` (toutes les actions MEMOTION actuelles en dépendent)
- `MEMOTION_DONE` contient explicitement `CARD_ID` pour fermer la manche en cours
- Un `MEMOTION_DONE` avec `CARD_ID` différent de `MEMOTION_SELECTED` est refusé (error `CARD_SCOPE_MISMATCH`)

### Timer par carte

- **Démarrage** : `MEMOTION_FLIP` (durée = `CURRENT_TIME` du type imbriqué de la carte)
- **Arrêt manuel** : `MEMOTION_STOP_TIMER` → `CURRENT_TIME` reste figé, subphase reste `QUESTION`
- **Arrêt automatique** : `MEMOTION_REVEAL` ou expiration timer=0 → `StopMotionCardTimer()`
- **Impact `HostContext`** : `TimerRunning` dérivé uniquement de `MEMOTION_SUBPHASE == QUESTION` **ET** `CURRENT_TIME > 0`
  - Timer = 0 en QUESTION → `TimerRunning = false`, même si subphase n'a pas changé
  - Timer actif en SELECTED → `TimerRunning = false` (subphase seule contrôle)

### Animations TV (framer-motion)
- GRID→SELECTED : `layoutId` shared → zoom automatique depuis position grille
- Carte en grille masquée (`visibility: hidden`) quand sélectionnée (évite doublon)
- SELECTED→QUESTION : `AnimatePresence` `initial={{ rotateY: -90 }}`
- QUESTION→REVEAL : `AnimatePresence` `initial={{ rotateY: 90 }}` (direction opposée)
- DONE→GRID : `layoutId` anime le retour vers la grille

### Fichiers clés

**Backend (Go)**

- `engine.go` : `SelectMotionCard`, `FlipMotionCard`, `RevealMotionCard`, `DoneMotionCard`, `StopMotionCardTimer` + nouveau (v7.0.0) : gestion `MEMOTION_ACTIVE`, calcul points selon `POINTS_RULE`
- `models.go` : struct `MotionCard` (ajout champs `TYPE`, `POINTS_RULE` v7.0.0), `GameState.MEMOTION_ACTIVE` (v7.0.0), `TypedContent` embarqué (v7.0.0), `HostContext` (v7.0.0), `CardScope` (v7.0.0)
- `question_types.go` : registre des types (v7.0.0, #184), liste `TypeDescriptor`, validation imbrication
- `messages.go` : `ActionMotionSelect/Flip/StopTimer/Reveal/Done/SetTeams`, nouveau (v7.0.0) : parsing `UNITS` en `MEMOTION_DONE`, portée actions via `MOTION_CARD_ID`
- `http.go` : parser `MOTION_MEMORIZE_DURATION` (v5.5.0), nouveau (v7.0.0) : parser `TYPE` et `POINTS_RULE` de carte, valider non-régression JSON
- `main.go` : handlers `handleMotion*`, nouveau (v7.0.0) : calcul HostContext côté serveur (tests de dérivation)
- `state_persistence.go` : `MEMOTION_ACTIVE` **exclue** de la persistance (non-persisté, v7.0.0)

**Frontend (React)**

- `GamePage.jsx` : panneaux admin GRID/SELECTED/QUESTION/REVEAL, nouveau (v7.0.0) : affichage type carte, barème en leçon
- `PlayerDisplay.jsx` : vues TV, animations, nouveau (v7.0.0) : résolution HostContext local, pas de dépendance PHASE/MEMOTION_SUBPHASE directe
- `QuestionsPage.jsx` : éditeur carte, nouveau (v7.0.0) : champ `TYPE`, verrou type selon `OwnedFields`, barème `POINTS_RULE`
- `utils/hostContext.js` : **nouveau** (v7.0.0) — fonction `resolveHostContext(gameState)` partagée par tous les composants de type
- `utils/typeState.js` : **nouveau** (v7.0.0) — résolution `getTypeState(gameState, hostContext)` : question vs carte
- `utils/questionTypeMeta.js` : enregistrement client des types (v7.0.0, #184)
- `AnimPage.jsx` / `AnimConductPanel.jsx` / `AnimMotionGrid.jsx` : nouveau (v7.0.0) : utilisation HostContext au lieu de `phase` en prop
- `utils/motionRules.js` : matrice gestes MEMOTION, nouveau (v7.0.0) : dépendance HostContext.Playable plutôt que `phase === QUESTION`

**Tests (v7.0.0, #184)**

- `game_engine_test.go` : test dérivation HostContext (cas identiques côté Go et JS)
- `models_test.go` : test imbrication (profondeur 1), test verrou type, test non-régression JSON
- `typeState.test.js` : test dérivation HostContext JS (cas identiques côté Go)
- `motion_actions_test.go` : test portée actions, invariant `CARD_SCOPE_MISMATCH`
