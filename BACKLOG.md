# BuzzControl - Backlog

Fonctionnalités à implémenter.

---

## Gestion des scores

- [x] **Points d'équipe dissociés des points joueurs** *(v2.18.0)*
  - Champ `TEAM_POINTS` sur les équipes
  - Score total = TEAM_POINTS + sum(player scores)
  - Clic sur header équipe = points à l'équipe
  - Clic sur ligne joueur = points au joueur

---

## Catégories de questions

- [x] **Champ CATEGORY pour les questions** *(implémenté)*
  - Ajouter un champ `CATEGORY` au modèle Question
  - UI pour sélectionner/créer une catégorie lors de l'ajout de question
  - Filtrage des questions par catégorie dans QuestionsPage

- [x] **Palmarès par catégorie** *(v2.34.0)*
  - Page admin `/palmares` avec classement par catégorie
  - Vue TV Palmares avec grille 3x2 des catégories
  - Classement séparé équipes/joueurs avec médailles

---

## Timer et gameplay

- [x] **Décompte de 3 secondes avant le timer** *(v2.29.0)*
  - Décompte visuel "3... 2... 1..." avant le timer principal
  - Phase COUNTDOWN distincte avec badge orange "DECOMPTE"
  - Les buzzers restent bloqués pendant le décompte
  - Le timer démarre automatiquement après le décompte

---

## QCM - Indices et pénalités

> **En cours d'implémentation** - v2.38.0

### Configuration

- [ ] **Option activable par question QCM**
  - Champ `QCM_HINTS_ENABLED` (boolean, défaut: false)
  - Visible uniquement pour les questions de type QCM
  - Toggle dans le formulaire de création/édition de question

### Invalidation automatique des mauvaises réponses

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

### Pénalités de points

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

---

## Debug et tests

- [x] **Ctrl+Click sur joueur en PREPARE simule PONG** *(v2.28.0)*
  - En état PREPARE, Ctrl+Click sur un joueur simule la réponse au PING
  - Permet de tester sans buzzers physiques connectés
  - Le joueur passe de "en attente" à "prêt"

---

## Affichage TV

- [x] **Synchronisation des changements d'image de fond** *(v2.30.0)*
  - Le serveur centralise le timing et notifie tous les clients
  - `CurrentBackgroundIndex` dans GameState (backend)
  - Goroutine de cycling dans main.go
  - Action `BACKGROUND_CHANGE` dans le protocole WebSocket
  - Tous les clients TV reçoivent l'index synchronisé
  - Transitions simultanées sur tous les écrans

---

## Type de jeu : Memory

Jeu de mémoire avec paires de cartes à retrouver.

### Phase 1 - Modèle et création de question ✅

- [x] **Nouveau type de question `MEMORY`**
  - Champ `TYPE: "MEMORY"` dans le modèle Question
  - Structure `MEMORY_PAIRS` : tableau de paires `[{id, card1, card2}]`
  - Chaque carte peut être : texte OU image (chemin)
  - Paramètres configurables :
    - `MEMORY_FLIP_DELAY` : délai avant retournement si non-match (défaut: 3s)
    - `MEMORY_POINTS_PER_PAIR` : points par paire trouvée (défaut: 10)
    - `MEMORY_ERROR_PENALTY` : pénalité par erreur (défaut: 0)
    - `MEMORY_COMPLETION_BONUS` : bonus si toutes les paires trouvées (défaut: 0)

- [x] **Interface de création de paires (QuestionsPage)**
  - Sélecteur type "MEMORY" affiche l'éditeur de paires
  - Liste des paires avec boutons +/- pour ajouter/supprimer
  - Chaque paire : 2 inputs (texte ou upload image)
  - Preview de la grille générée automatiquement
  - Validation : minimum 2 paires, maximum 12 paires

### Phase 2 - État du jeu Memory et Affichage TV ✅

- [x] **Structure Memory dans GameState**
  - `MemoryFlippedCards []string` : IDs des cartes retournées (max 2)
  - `MemoryMatchedPairs []int` : IDs des paires trouvées
  - `MemoryErrors int` : compteur d'erreurs (non-matches)

- [x] **Affichage TV (PlayerDisplay)**
  - Grille responsive avec Container Queries (cqw, cqh, cqmin)
  - Animation flip 3D CSS sur les cartes
  - Colonnes automatiques selon nombre de cartes (2-6 colonnes)
  - États visuels : dos (violet), révélée, matched (vert)
  - Mélange Fisher-Yates avec seed basé sur question ID

### Phase 3 - Gameplay interactif ✅

- [x] **Action `FLIP_MEMORY_CARD` (Admin/TV → Serveur)**
  - Payload : `{CARD_ID: string}` (format "pairID-cardNum")
  - Le serveur valide et met à jour l'état
  - Broadcast de l'état aux clients TV

- [x] **Logique de révélation (engine.go:FlipMemoryCard)**
  - Si 0 carte révélée → révéler la carte, attendre la 2ème
  - Si 1 carte révélée → révéler la 2ème, vérifier le match
  - Si match → marquer les 2 cartes comme MATCHED, incrémenter compteur
  - Si non-match → incrémenter erreurs, démarrer timer (FLIP_DELAY), puis cacher

- [x] **Détection de fin de partie**
  - Toutes les paires trouvées → auto-stop game, transition vers STOPPED
  - Timer global épuisé (si configuré) → fin avec points partiels

- [x] **Affichage statistiques pendant le jeu**
  - Paires trouvées X/Y
  - Erreurs Z (si penalty ou erreurs > 0)

### Phase 4 - Interface Admin (GamePage)

- [x] **Indicateurs Memory en temps réel dans GamePage** *(implémenté)*
  - Paires trouvées X/Y, compteur d'erreurs
  - Badge de succès si toutes les paires sont trouvées

- [x] **Bouton "Révéler tout" pour Memory** *(implémenté)*
  - Le bouton "REPONSE" passe en phase REVEALED
  - Révèle toutes les cartes en cascade avec REVEAL_DELAY

### Phase 5 - Scoring et historique

- [x] **Calcul des points Memory** *(implémenté dans GamePage.jsx + engine.go)*
  ```
  Score = (paires_trouvées × POINTS_PER_PAIR)
        + (COMPLETION_BONUS si toutes trouvées)
        - (erreurs × ERROR_PENALTY)
  ```

- [ ] **Enregistrement spécifique dans l'historique**
  - EventType: "MEMORY_COMPLETED" (actuellement "POINTS_AWARDED")
  - Détails: paires trouvées, erreurs, temps total

### Améliorations futures (hors scope initial)

- [ ] **Mode Équipes** : les équipes buzzent pour désigner les cartes
- [ ] **Mode Chrono** : temps limité, max de paires en un temps donné
- [ ] **Thèmes de cartes** : dos de carte personnalisable
- [ ] **Types de paires mixtes** : Image ↔ Texte (association)
- [ ] **Niveaux de difficulté** : délai de retournement variable
