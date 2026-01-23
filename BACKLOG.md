# BuzzControl - Backlog

---

## ✅ Fonctionnalités terminées

### Gestion des scores

- [x] **Points d'équipe dissociés des points joueurs** *(v2.18.0)*
  - Champ `TEAM_POINTS` sur les équipes
  - Score total = TEAM_POINTS + sum(player scores)
  - Clic sur header équipe = points à l'équipe
  - Clic sur ligne joueur = points au joueur

### Catégories de questions

- [x] **Champ CATEGORY pour les questions** *(v2.22.0)*
  - Champ `CATEGORY` au modèle Question
  - UI pour sélectionner/créer une catégorie lors de l'ajout de question
  - Filtrage des questions par catégorie dans QuestionsPage

- [x] **Palmarès par catégorie** *(v2.34.0)*
  - Page admin `/palmares` avec classement par catégorie
  - Vue TV Palmares avec grille 3x2 des catégories
  - Classement séparé équipes/joueurs avec médailles

### Timer et gameplay

- [x] **Décompte de 3 secondes avant le timer** *(v2.29.0)*
  - Décompte visuel "3... 2... 1..." avant le timer principal
  - Phase COUNTDOWN distincte avec badge orange "DECOMPTE"
  - Les buzzers restent bloqués pendant le décompte
  - Le timer démarre automatiquement après le décompte

### QCM - Indices et pénalités *(v2.38.0)*

- [x] **Option activable par question QCM**
  - Champ `QCM_HINTS_ENABLED` (boolean, défaut: false)
  - Toggle dans le formulaire de création/édition de question
  - Seuils configurables : `QCM_HINT_THRESHOLD_1`, `QCM_HINT_THRESHOLD_2`

- [x] **Logique d'invalidation (Backend)**
  - Invalidation aléatoire des mauvaises réponses aux seuils configurés
  - Seuils par défaut : 25% et 12.5% du temps restant
  - Contraintes de sécurité (min 1s entre indices, seuil 2 ≥ 1s avant fin)
  - Fichiers : `engine.go` (`shouldTriggerQCMHint`, `invalidateRandomWrongAnswer`)

- [x] **Affichage TV (Frontend)**
  - Réponse invalidée : visuellement grisée avec opacité réduite
  - État `QCM_INVALIDATED` dans GameState
  - Fichiers : `PlayerDisplay.jsx`, `PlayerDisplay.css` (`.invalidated`)

- [x] **Broadcast WebSocket**
  - Action `QCM_HINT` : `{COLOR, REMAINING}`
  - Fichiers : `messages.go`, `main.go` (`broadcastQCMHint`)

- [x] **Pénalités de points**
  - Champ `HintsAtBuzz` sur Bumper
  - Ratio : 100% (0 indice), 67% (1 indice), 33% (2 indices)
  - Badge pénalité sur GamePage

### Debug et tests

- [x] **Ctrl+Click sur joueur en PREPARE simule PONG** *(v2.28.0)*
  - Permet de tester sans buzzers physiques connectés

### Affichage TV

- [x] **Synchronisation des changements d'image de fond** *(v2.30.0)*
  - Le serveur centralise le timing et notifie tous les clients
  - Action `BACKGROUND_CHANGE` dans le protocole WebSocket
  - Transitions simultanées sur tous les écrans

- [x] **Affichage des phases de jeu** *(v2.40.0)*
  - PREPARATION : Affichage centré avec 🔔 + "NOUVELLE QUESTION"
  - PRET : Icône de catégorie + nom avec couleur de fond
  - DECOMPTE : Animation de la catégorie vers la zone question
  - MEMORY PRET : Badge catégorie avec icône

### Type de jeu : Memory *(v2.33.0)*

- [x] **Nouveau type de question `MEMORY`**
  - Structure `MEMORY_PAIRS` : tableau de paires `[{id, card1, card2}]`
  - Chaque carte peut être : texte OU image
  - Paramètres configurables (FLIP_DELAY, POINTS_PER_PAIR, ERROR_PENALTY, COMPLETION_BONUS)

- [x] **Interface de création de paires (QuestionsPage)**
  - Éditeur de paires avec texte ou upload image
  - Preview de la grille générée
  - Validation : 2-12 paires

- [x] **État du jeu Memory et Affichage TV**
  - `MemoryFlippedCards`, `MemoryMatchedPairs`, `MemoryErrors`
  - Grille responsive avec Container Queries
  - Animation flip 3D CSS

- [x] **Gameplay interactif**
  - Action `FLIP_MEMORY_CARD`
  - Logique de révélation et matching
  - Détection de fin de partie

- [x] **Interface Admin (GamePage)**
  - Indicateurs en temps réel (paires trouvées, erreurs)
  - Bouton "Révéler tout"

- [x] **Calcul des points Memory**
  - Score = (paires × POINTS_PER_PAIR) + COMPLETION_BONUS - (erreurs × ERROR_PENALTY)

---

## 🔄 Améliorations futures

### Historique

- [ ] **Enregistrement de la pénalité QCM dans l'historique**
  - Champ optionnel : `PenaltyApplied` (pourcentage de réduction)

- [ ] **Enregistrement spécifique Memory dans l'historique**
  - EventType: "MEMORY_COMPLETED"
  - Détails: paires trouvées, erreurs, temps total

### Memory - Améliorations

- [ ] **Mode Équipes** : les équipes buzzent pour désigner les cartes
- [ ] **Mode Chrono** : temps limité, max de paires en un temps donné
- [ ] **Thèmes de cartes** : dos de carte personnalisable
- [ ] **Types de paires mixtes** : Image ↔ Texte (association)
- [ ] **Niveaux de difficulté** : délai de retournement variable
