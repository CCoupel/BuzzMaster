# QCM - Indices et pénalités

**Statut** : ⏳ En cours d'implémentation (v2.38.0)

## Description

Système d'indices automatiques pour les questions QCM avec pénalités de points progressives.

## Configuration

- [ ] **Option activable par question QCM**
  - Champ `QCM_HINTS_ENABLED` (boolean, défaut: false)
  - Visible uniquement pour les questions de type QCM
  - Toggle dans le formulaire de création/édition de question

## Invalidation automatique des mauvaises réponses

- [ ] **Logique d'invalidation (Backend)**
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

- [ ] **Affichage TV (Frontend)**
  - Réponse invalidée : visuellement barrée/grisée
  - Animation de transition lors de l'invalidation
  - État `QCM_INVALIDATED` dans GameState : liste des couleurs invalidées

- [ ] **Broadcast WebSocket**
  - Action `QCM_HINT` : notifie les clients d'une invalidation
  - Payload : `{COLOR: "RED|GREEN|YELLOW|BLUE"}`

## Pénalités de points

- [ ] **Calcul des pénalités (Backend)**
  - Si un joueur buzz après invalidation(s), ses points sont réduits
  - **Ratio de pénalité :**
    - 4 réponses (aucune invalidée) → 100% des points
    - 3 réponses (1 invalidée) → 67% des points
    - 2 réponses (2 invalidées) → 33% des points
  - Calcul : `points_effectifs = points_base × (réponses_restantes / 4)`

- [ ] **Affichage admin (Frontend)**
  - Indicateur de pénalité applicable sur GamePage
  - Badge "67%" ou "33%" à côté des points si pénalité active

- [ ] **Historique**
  - L'historique enregistre les points effectivement attribués (après pénalité)
  - Champ optionnel : `PenaltyApplied` (pourcentage de réduction)

## Version cible

v2.38.0
