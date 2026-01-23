# QCM - Indices et pénalités

**Statut** : ✅ Terminé (v2.38.0)

## Description

Système d'indices automatiques pour les questions QCM avec pénalités de points progressives.

## Configuration

- [x] **Option activable par question QCM** *(v2.38.0)*
  - Champ `QCM_HINTS_ENABLED` (boolean, défaut: false)
  - Visible uniquement pour les questions de type QCM
  - Toggle dans le formulaire de création/édition de question
  - Seuils configurables : `QCM_HINT_THRESHOLD_1`, `QCM_HINT_THRESHOLD_2`

## Invalidation automatique des mauvaises réponses

- [x] **Logique d'invalidation (Backend)** *(v2.38.0)*
  - Si aucun joueur n'a buzzé, invalider une mauvaise réponse aux seuils configurés
  - L'invalidation est aléatoire parmi les mauvaises réponses restantes
  - **Seuils par défaut (proportionnels au timer) :**
    - Seuil 1 (1er indice) : 25% du temps restant
    - Seuil 2 (2ème indice) : 12.5% du temps restant
  - **Contraintes de sécurité :**
    - Minimum 1s entre les deux indices
    - Seuil 2 ≥ 1s avant la fin du jeu
    - Si ces contraintes ne peuvent être respectées → pas d'indices
  - **Exemples :**
    | Timer | Seuil 1 | Seuil 2 | Notes |
    |-------|---------|---------|-------|
    | 30s   | 7.5s    | 3.75s   | OK |
    | 20s   | 5s      | 2.5s    | OK |
    | 10s   | 2.5s    | 1.25s   | OK |
    | 4s    | 1s      | —       | 1 seul indice possible |
    | 2s    | —       | —       | Pas d'indices |

- [x] **Affichage TV (Frontend)** *(v2.38.0)*
  - Réponse invalidée : visuellement grisée avec opacité réduite
  - État `QCM_INVALIDATED` dans GameState : liste des couleurs invalidées
  - Fichiers : `PlayerDisplay.jsx`, `PlayerDisplay.css` (`.invalidated`)

- [x] **Broadcast WebSocket** *(v2.38.0)*
  - Action `QCM_HINT` : `{COLOR, REMAINING}`
  - Fichiers : `messages.go`, `main.go` (`broadcastQCMHint`)

## Pénalités de points

- [x] **Calcul des pénalités (Backend)** *(v2.38.0)*
  - Champ `HintsAtBuzz` sur Bumper
  - **Ratio de pénalité :**
    - 4 réponses (aucune invalidée) → 100% des points
    - 3 réponses (1 invalidée) → 67% des points
    - 2 réponses (2 invalidées) → 33% des points
  - Calcul : `points_effectifs = points_base × (réponses_restantes / 4)`
  - Fichiers : `engine.go`, `models.go`

- [x] **Affichage admin (Frontend)** *(v2.38.0)*
  - Badge pénalité sur GamePage
  - Indicateur visuel sur TeamCard avec anneau de pénalité
  - Fichiers : `GamePage.jsx`, `TeamCard.jsx`, `TeamCard.css`

- [x] **Historique** *(v2.38.0)*
  - L'historique enregistre les points effectivement attribués (après pénalité)
  - Champ `HintsAtBuzz` capturé au moment du buzz

## Version cible

v2.38.0
