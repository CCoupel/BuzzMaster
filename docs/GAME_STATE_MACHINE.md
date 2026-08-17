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

## Implémentation

### Backend (Go)
- Fichier: `server-go/internal/game/engine.go`
- Les phases sont définies comme constantes: `PhaseStopped`, `PhasePrepare`, `PhaseReady`, `PhaseStarted`, `PhasePaused`, `PhaseRevealed`

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

## Type de jeu MEMOTION (v5.0.0)

Jeu de cartes à 3 faces avec grille interactive, difficulté configurable et mode équipe.

### Structure MotionCard
```json
{
  "ID": "mc-1",
  "RECTO_THEME": "Thème de la carte",
  "RECTO_IMAGE": "/files/questions/img_recto.jpg",
  "DIFFICULTY": 2,
  "QUESTION_TEXT": "Question ou énigme",
  "QUESTION_IMAGE": "/files/questions/img_question.jpg",
  "ANSWER_TEXT": "Réponse ou explication",
  "ANSWER_IMAGE": "/files/questions/img_answer.jpg"
}
```

Points par difficulté : ★ (1) → 1 pt | ★★ (2) → 3 pts | ★★★ (3) → 5 pts

### Subphases MEMOTION
| Subphase | Description |
|----------|-------------|
| `MEMORIZE` | *(Secret Mode)* Toutes les cartes RECTO visibles + timer décompte. Sélection impossible. |
| `GRID` | Grille de cartes RECTO (ou coordonnées en Secret Mode), prêtes à sélectionner |
| `SELECTED` | Carte zoomée plein écran (RECTO thème + points). Pas de timer. |
| `QUESTION` | Face VERSO (question) plein écran, timer actif |
| `REVEAL` | Face REVEAL (réponse) plein écran, admin attribue points |

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
- `MEMOTION_SUBPHASE` : `""` | `"MEMORIZE"` | `"GRID"` | `"SELECTED"` | `"QUESTION"` | `"REVEAL"` — `MEMORIZE` ajouté en v5.5.0 (Secret Mode)
- `MEMOTION_SELECTED` : ID carte sélectionnée (`""` en GRID)
- `MEMOTION_CARD_STATES` : map[string]string → `"UNPLAYED"` | `"SELECTED"` | `"QUESTION"` | `"REVEALED"` | `"DONE"`
- `MEMOTION_CARD_TEAMS` : map[string]string → teamName quand DONE
- `MEMOTION_CURRENT_TEAM` : équipe active
- `MEMOTION_PARTICIPATING_TEAMS` : []string
- `MEMOTION_CURRENT_TEAM_COLOR` : [3]int RGB

### Flux de jeu (9 étapes)
1. Admin sélectionne une carte depuis la preview TV (clic sur carte `UNPLAYED` en subphase GRID) → `MEMOTION_SELECT` → SELECTED (zoom plein écran, RECTO)
2. Admin click "Démarrer" → `MEMOTION_FLIP` → QUESTION + timer démarre
3. Timer expire ou admin "STOP TIMER" → `MEMOTION_STOP_TIMER` → timer stop, reste QUESTION
4. Admin "RÉVÉLER" → `MEMOTION_REVEAL` → REVEAL (timer arrêté)
5. Admin click "Perdu" ou bouton équipe courante → `MEMOTION_DONE` → retour GRID + carte DONE colorée (seule l'équipe courante peut gagner)

**Annulation depuis SELECTED** : `MEMOTION_DONE` avec `WINNER_TEAM=""` → carte retourne UNPLAYED.

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
{ "ACTION": "MEMOTION_DONE",      "MSG": { "CARD_ID": "mc-1", "WINNER_TEAM": "team_A" } }
{ "ACTION": "MEMOTION_SET_TEAMS", "MSG": { "TEAMS": ["team_A", "team_B"] } }
```

### Timer par carte
- Démarre au `MEMOTION_FLIP` (durée = champ `Time` de la Question)
- `MEMOTION_STOP_TIMER` : arrêt manuel, subphase reste QUESTION
- `MEMOTION_REVEAL` : arrête aussi le timer via `StopMotionCardTimer()`

### Animations TV (framer-motion)
- GRID→SELECTED : `layoutId` shared → zoom automatique depuis position grille
- Carte en grille masquée (`visibility: hidden`) quand sélectionnée (évite doublon)
- SELECTED→QUESTION : `AnimatePresence` `initial={{ rotateY: -90 }}`
- QUESTION→REVEAL : `AnimatePresence` `initial={{ rotateY: 90 }}` (direction opposée)
- DONE→GRID : `layoutId` anime le retour vers la grille

### Fichiers clés
- `engine.go` : `SelectMotionCard`, `FlipMotionCard`, `RevealMotionCard`, `DoneMotionCard`, `StopMotionCardTimer`, `StartMotionMemorizeTimer` (v5.5.0), `StopMotionMemorizeTimer` (v5.5.0)
- `models.go` : struct `MotionCard`, champs `GameState.Motion*`, `QuestionTypeMemotion`, `Question.MOTION_MEMORIZE_DURATION` (v5.5.0)
- `messages.go` : `ActionMotionSelect/Flip/StopTimer/Reveal/Done/SetTeams`
- `http.go` : parser `MOTION_MEMORIZE_DURATION` (v5.5.0)
- `main.go` : handlers `handleMotion*`
- `GamePage.jsx` : panneaux admin GRID/SELECTED/QUESTION/REVEAL + MEMORIZE status + liste coordonnées (v5.5.0)
- `PlayerDisplay.jsx` : vues TV + framer-motion layoutId + AnimatePresence flip + bannière MEMORIZE + coordonnées Secret Mode (v5.5.0)
- `PlayerDisplay.css` : classes `.memotion-*`, `.memotion-memorize-active`, `.memotion-memorize-banner` (v5.5.0)
- `QuestionsPage.jsx` : champ "Durée mémorisation (s)" dans éditeur MEMOTION (v5.5.0)
- `AnimPage.jsx` : interface animateur tablette (v6.2.0, #160 — conduite MEMOTION complète depuis `/anim`)
- `AnimConductPanel.jsx` : conduite 5 lignes, L2 occupée par `AnimMotionActions`, L3 4 voies avec MEMOTION grille/carte (v6.2.0, #160)
- `AnimMotionGrid.jsx` / `AnimMotionCard.jsx` / `AnimMotionActions.jsx` : composants MEMOTION tablette (v6.2.0, #160)
- `utils/motionGrid.js` : règles disposition MEMOTION partagées avec TV (v6.2.0, #160)
- `utils/motionRules.js` : matrice gestes MEMOTION par sous-phase (v6.2.0, #160)
