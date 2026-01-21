# Type de jeu : Memory

**Statut** : ✅ Complété (Phases 1-5), ⏳ Une tâche restante

## Description

Jeu de mémoire avec paires de cartes à retrouver.

## Phase 1 - Modèle et création de question ✅

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

## Phase 2 - État du jeu Memory et Affichage TV ✅

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

## Phase 3 - Gameplay interactif ✅

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

## Phase 4 - Interface Admin (GamePage) ✅

- [x] **Indicateurs Memory en temps réel dans GamePage**
  - Paires trouvées X/Y, compteur d'erreurs
  - Badge de succès si toutes les paires sont trouvées

- [x] **Bouton "Révéler tout" pour Memory**
  - Le bouton "REPONSE" passe en phase REVEALED
  - Révèle toutes les cartes en cascade avec REVEAL_DELAY

## Phase 5 - Scoring et historique

- [x] **Calcul des points Memory**
  ```
  Score = (paires_trouvées × POINTS_PER_PAIR)
        + (COMPLETION_BONUS si toutes trouvées)
        - (erreurs × ERROR_PENALTY)
  ```

- [ ] **Enregistrement spécifique dans l'historique**
  - EventType: "MEMORY_COMPLETED" (actuellement "POINTS_AWARDED")
  - Détails: paires trouvées, erreurs, temps total

## Phase 6 - Modes de jeu multi-équipes

### Concept

Ajout de plusieurs modes de jeu pour le Memory, permettant de faire jouer plusieurs équipes avec des règles différentes.

### Modes disponibles

- [ ] **Mode SOLO** (mode actuel - par défaut)
  - Une seule équipe joue
  - Tous les joueurs d'une équipe peuvent retourner les cartes
  - Points attribués à l'équipe à la fin du jeu
  - C'est le mode actuellement implémenté

- [ ] **Mode CHACUN_SON_TOUR**
  - Multi-équipes
  - On change d'équipe à chaque retournement de paire (2 cartes)
  - Que la paire soit valide ou non, on passe à l'équipe suivante
  - Rotation : Équipe 1 → Équipe 2 → Équipe 3 → ... → Équipe 1
  - Chaque équipe accumule ses propres paires trouvées
  - Points par paire attribués à l'équipe qui la trouve

- [ ] **Mode TANT_QUE_JE_GAGNE**
  - Multi-équipes
  - Une équipe continue de jouer tant qu'elle trouve des paires valides
  - Dès qu'une paire n'est pas valide (non-match), on passe à l'équipe suivante
  - L'équipe suivante joue jusqu'à ce qu'elle fasse une erreur, etc.
  - Chaque équipe accumule ses propres paires trouvées
  - Points par paire attribués à l'équipe qui la trouve
  - Mode "hot potato" : celui qui se trompe perd la main

### Implémentation

- [ ] **Nouveau champ dans Question MEMORY**
  ```json
  {
    "TYPE": "MEMORY",
    "MEMORY_MODE": "SOLO" | "CHACUN_SON_TOUR" | "TANT_QUE_JE_GAGNE",
    "MEMORY_PAIRS": [...],
    "MEMORY_CONFIG": {...}
  }
  ```

- [ ] **Structure GameState étendue**
  ```go
  type GameState struct {
    // ... champs existants
    MemoryCurrentTeam   string  // Nom de l'équipe qui joue actuellement
    MemoryTeamPairs     map[string]int  // Nombre de paires trouvées par équipe
    MemoryConsecutiveSuccess bool  // Pour mode TANT_QUE_JE_GAGNE
  }
  ```

- [ ] **Logique de changement d'équipe (engine.go)**

  **Mode CHACUN_SON_TOUR** :
  - Après chaque tentative (2 cartes révélées), incrémenter vers l'équipe suivante
  - Si paire trouvée : attribuer à l'équipe courante, puis changer
  - Si non-match : incrémenter erreur globale, puis changer

  **Mode TANT_QUE_JE_GAGNE** :
  - Si paire trouvée : attribuer à l'équipe courante, elle continue
  - Si non-match : incrémenter erreur globale, passer à l'équipe suivante

- [ ] **Indicateur visuel équipe courante**
  - Sur `/tv` : Afficher le nom de l'équipe qui joue en cours
  - Badge coloré avec la couleur de l'équipe
  - Position : Au-dessus de la grille Memory
  - Message : "🎮 Au tour de : [Équipe]"
  - Animation de transition lors du changement d'équipe

- [ ] **Contrôle des cartes cliquables**
  - En mode multi-équipes, seule l'équipe courante peut cliquer sur les cartes
  - Les autres équipes voient la grille mais les cartes sont non-cliquables
  - Grisage des cartes pour les équipes en attente

- [ ] **Interface admin (QuestionsPage)**
  - Sélecteur de mode de jeu Memory :
    - Radio buttons : SOLO / CHACUN SON TOUR / TANT QUE JE GAGNE
    - Description courte de chaque mode
    - Mode SOLO par défaut pour compatibilité

- [ ] **Calcul des points par équipe**
  - Chaque équipe a son propre compteur de paires trouvées
  - Points = `paires_trouvées_équipe × POINTS_PER_PAIR`
  - COMPLETION_BONUS : attribué à l'équipe qui trouve la dernière paire
  - ERROR_PENALTY : global ou par équipe ? (à décider)

- [ ] **Affichage scores en temps réel**
  - Tableau des scores pendant le jeu
  - Afficher le nombre de paires par équipe
  - Classement en direct
  - Mise à jour à chaque paire trouvée

### Scénario d'usage : Mode CHACUN_SON_TOUR

```
Initial : 6 paires à trouver, 3 équipes (Rouge, Bleu, Vert)

Tour 1 - Équipe Rouge :
  - Retourne carte 1 (chat)
  - Retourne carte 5 (chien)
  - Non-match → 0 paire pour Rouge
  → Passe à Équipe Bleu

Tour 2 - Équipe Bleu :
  - Retourne carte 3 (chat)
  - Retourne carte 1 (chat)
  - Match ! → +1 paire pour Bleu
  → Passe à Équipe Vert (même si match)

Tour 3 - Équipe Vert :
  - Retourne carte 2 (oiseau)
  - Retourne carte 4 (oiseau)
  - Match ! → +1 paire pour Vert
  → Passe à Équipe Rouge

... et ainsi de suite
```

### Scénario d'usage : Mode TANT_QUE_JE_GAGNE

```
Initial : 6 paires à trouver, 3 équipes (Rouge, Bleu, Vert)

Tour 1 - Équipe Rouge :
  - Retourne carte 1 (chat) + carte 3 (chat)
  - Match ! → +1 paire pour Rouge
  - Rouge continue (car match)

Tour 2 - Équipe Rouge (encore) :
  - Retourne carte 2 (oiseau) + carte 5 (chien)
  - Non-match → Rouge s'arrête
  → Passe à Équipe Bleu

Tour 3 - Équipe Bleu :
  - Retourne carte 2 (oiseau) + carte 4 (oiseau)
  - Match ! → +1 paire pour Bleu
  - Bleu continue

Tour 4 - Équipe Bleu (encore) :
  - Retourne carte 6 (poisson) + carte 7 (poisson)
  - Match ! → +1 paire pour Bleu
  - Bleu continue

Tour 5 - Équipe Bleu (encore) :
  - Retourne carte 8 (souris) + carte 5 (chien)
  - Non-match → Bleu s'arrête
  → Passe à Équipe Vert

... et ainsi de suite
```

### Questions ouvertes

- [ ] **ERROR_PENALTY** : Global (toutes équipes) ou par équipe ?
  - **Proposition** : Par équipe (chaque équipe a son compteur d'erreurs)

- [ ] **Timer** : Continue ou par tour d'équipe ?
  - **Option 1** : Timer global pour toute la partie
  - **Option 2** : Timer par tour d'équipe (ex: 30s par tour)
  - **Proposition** : Option 1 (timer global) pour garder la simplicité

- [ ] **Ordre des équipes** : Comment déterminer l'ordre ?
  - **Proposition** : Ordre d'affichage dans `/admin/teams` (de haut en bas)

- [ ] **Équipe absente/déconnectée** : Comment gérer ?
  - **Proposition** : Skip automatiquement, passer à l'équipe suivante

### Compatibilité

- ✅ Rétrocompatible : Questions Memory existantes sans `MEMORY_MODE` utilisent "SOLO" par défaut
- ✅ Mode SOLO identique au comportement actuel
- ✅ Pas de modification nécessaire des questions existantes

## Améliorations futures (hors scope initial)

- [ ] **Mode Équipes** : les équipes buzzent pour désigner les cartes
- [ ] **Mode Chrono** : temps limité, max de paires en un temps donné
- [ ] **Thèmes de cartes** : dos de carte personnalisable
- [ ] **Types de paires mixtes** : Image ↔ Texte (association)
- [ ] **Niveaux de difficulté** : délai de retournement variable

## Version

Phases 1-5 implémentées (v2.33.0)
