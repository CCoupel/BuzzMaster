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
| **PREPARE** | READY | Tous PONG reçus | Bouton START devient actif. PAUSE/REPONSE restent désactivés | "PREPAREZ-VOUS" clignote |
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
- Se déclenche automatiquement quand tous les buzzers connectés ont répondu PONG
- Si aucun buzzer n'est connecté, la transition est immédiate

## Implémentation

### Backend (Go)
- Fichier: `server-go/internal/game/engine.go`
- Les phases sont définies comme constantes: `PhaseStopped`, `PhasePrepare`, `PhaseReady`, `PhaseStarted`, `PhasePaused`, `PhaseRevealed`

### Frontend Admin (React)
- Fichier: `server-go/web/src/pages/GamePage.jsx`
- Les états des boutons sont calculés en fonction de `gameState.phase`

### Frontend Joueur (React)
- Fichier: `server-go/web/src/pages/PlayerDisplay.jsx`
- L'affichage est conditionné par `gameState.phase`

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
