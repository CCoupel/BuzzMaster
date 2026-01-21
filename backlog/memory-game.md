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

### Modes de jeu (gameplay)

Définissent **comment les équipes jouent** (ordre, tour, rotation).

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

---

## Phase 7 - Modes de points (scoring)

### Concept

Les modes de points définissent **comment les points sont calculés** et **ce qui se passe en cas d'erreur**. Ils sont **combinables** avec les modes de jeu (Phase 6).

### Modes de points disponibles

- [ ] **Mode TO_THE_END** (mode actuel - par défaut)
  - Les paires trouvées restent visibles jusqu'à la fin
  - Cartes matched ne se retournent jamais
  - Calcul de points classique : `(paires × POINTS) + BONUS - (erreurs × PENALTY)`
  - C'est le mode actuellement implémenté

- [ ] **Mode MORT_SUBITE** (hardcore)
  - En cas de **mauvaise paire** (non-match), **RESET complet** :
    - ❌ Toutes les cartes sont remises face cachée (même les paires trouvées)
    - ❌ Les points de toutes les équipes sont remis à zéro
    - ✅ On garde une trace du **meilleur score** atteint avant le reset
  - La partie continue avec les cartes réinitialisées
  - Affichage permanent du "High Score" pendant le jeu
  - Mode très difficile : une seule erreur = tout recommencer
  - **Note** : En mode multi-équipes, c'est l'erreur de n'importe quelle équipe qui déclenche le reset global

- [ ] **Mode PERFECT** (bonus perfectionniste)
  - Identique à TO_THE_END, mais avec un gros bonus si **aucune erreur**
  - `PERFECT_BONUS` : points supplémentaires si erreurs = 0
  - Encourage la concentration et la stratégie
  - Exemple : +50 points si toutes les paires trouvées sans erreur

- [ ] **Mode CASCADE** (multiplicateur progressif)
  - Les paires trouvées **consécutivement** sans erreur augmentent le multiplicateur
  - Multiplicateur : ×1, ×1.5, ×2, ×2.5, ×3... (plafond : ×5)
  - Une erreur **reset le multiplicateur** à ×1 (mais garde les paires trouvées)
  - Encourage les séries de réussite
  - Affichage du multiplicateur actuel à côté du score

- [ ] **Mode TIME_BONUS** (course contre la montre)
  - Bonus proportionnel au **temps restant** à la fin
  - Calcul : `BONUS = temps_restant / temps_total × MAX_TIME_BONUS`
  - Exemple : Si complété avec 50% du temps restant → +50% du TIME_BONUS max
  - Encourage la vitesse en plus de la mémoire

- [ ] **Mode ZERO_SUM** (risque élevé)
  - Score peut devenir **négatif**
  - Pénalités d'erreur élevées (ex: -20 par erreur)
  - Points par paire modérés (ex: +15)
  - Score final peut être négatif (affiché en rouge)
  - Mode punitif pour experts

### Implémentation

- [ ] **Nouveau champ dans Question MEMORY**
  ```json
  {
    "TYPE": "MEMORY",
    "MEMORY_MODE": "SOLO" | "CHACUN_SON_TOUR" | "TANT_QUE_JE_GAGNE" | "MAILLON_FAIBLE",
    "MEMORY_SCORING_MODE": "TO_THE_END" | "MORT_SUBITE" | "PERFECT" | "CASCADE" | "TIME_BONUS" | "ZERO_SUM",
    "MEMORY_PAIRS": [...],
    "MEMORY_CONFIG": {
      "POINTS_PER_PAIR": 10,
      "ERROR_PENALTY": 0,
      "COMPLETION_BONUS": 20,
      "PERFECT_BONUS": 50,         // Pour mode PERFECT
      "CASCADE_MAX_MULTIPLIER": 5, // Pour mode CASCADE
      "MAX_TIME_BONUS": 100,       // Pour mode TIME_BONUS
      "CHAIN_BONUS_ENABLED": false,   // Pour mode MAILLON_FAIBLE
      "ELIMINATION_ENABLED": false,   // Pour mode MAILLON_FAIBLE
      "ERROR_QUOTA": 3,               // Pour mode MAILLON_FAIBLE + ELIMINATION
      // ...
    }
  }
  ```

- [ ] **Structure GameState étendue pour MORT_SUBITE**
  ```go
  type GameState struct {
    // ... champs existants
    MemoryHighScore     int     // Meilleur score avant reset (mode MORT_SUBITE)
    MemoryResetCount    int     // Nombre de resets (mode MORT_SUBITE)
  }
  ```

- [ ] **Structure GameState étendue pour CASCADE**
  ```go
  type GameState struct {
    // ... champs existants
    MemoryMultiplier    float64 // Multiplicateur actuel (mode CASCADE)
    MemoryStreak        int     // Nombre de paires consécutives sans erreur
  }
  ```

- [ ] **Logique de reset MORT_SUBITE (engine.go)**
  ```go
  // Lors d'un non-match
  if scoringMode == "MORT_SUBITE" {
    // Sauvegarder le high score
    currentScore := calculateCurrentScore()
    if currentScore > gameState.MemoryHighScore {
      gameState.MemoryHighScore = currentScore
    }

    // Reset complet
    gameState.MemoryMatchedPairs = []int{}
    gameState.MemoryErrors = 0
    gameState.MemoryResetCount++

    // Reset scores équipes
    for teamName := range gameState.MemoryTeamPairs {
      gameState.MemoryTeamPairs[teamName] = 0
    }

    // Broadcast reset aux clients
    broadcastMemoryReset()
  }
  ```

- [ ] **Calcul multiplicateur CASCADE**
  ```go
  // Lors d'un match
  if scoringMode == "CASCADE" {
    gameState.MemoryStreak++
    multiplier := min(1.0 + float64(gameState.MemoryStreak) * 0.5, cascadeMaxMultiplier)
    gameState.MemoryMultiplier = multiplier
  }

  // Lors d'un non-match
  if scoringMode == "CASCADE" {
    gameState.MemoryStreak = 0
    gameState.MemoryMultiplier = 1.0
  }
  ```

- [ ] **Logique MAILLON_FAIBLE (engine.go)**
  ```go
  // Extension GameState pour MAILLON_FAIBLE
  type GameState struct {
    // ... champs existants
    MemoryTeamErrors    map[string]int  // Nombre d'erreurs par équipe (pour élimination)
    MemoryEliminatedTeams []string       // Équipes éliminées
  }

  // Lors d'un match (équipe continue)
  if gameMode == "MAILLON_FAIBLE" {
    // Attribuer la paire à l'équipe courante
    gameState.MemoryTeamPairs[currentTeam]++

    // Si bonus chaîne activé
    if config.ChainBonusEnabled {
      gameState.MemoryStreak++
      gameState.MemoryMultiplier = min(1.0 + float64(gameState.MemoryStreak) * 0.5, config.CascadeMaxMultiplier)
    }

    // L'équipe continue (pas de changement d'équipe)
  }

  // Lors d'un non-match (reset + élimination optionnelle)
  if gameMode == "MAILLON_FAIBLE" {
    // Sauvegarder high score
    currentScore := calculateCurrentScore()
    if currentScore > gameState.MemoryHighScore {
      gameState.MemoryHighScore = currentScore
    }

    // Incrémenter erreurs de l'équipe courante
    gameState.MemoryTeamErrors[currentTeam]++

    // Élimination si quota dépassé
    if config.EliminationEnabled && gameState.MemoryTeamErrors[currentTeam] >= config.ErrorQuota {
      gameState.MemoryEliminatedTeams = append(gameState.MemoryEliminatedTeams, currentTeam)
      broadcastTeamEliminated(currentTeam)
    }

    // Reset complet (MORT_SUBITE)
    gameState.MemoryMatchedPairs = []int{}
    gameState.MemoryFlippedCards = []string{}
    for teamName := range gameState.MemoryTeamPairs {
      gameState.MemoryTeamPairs[teamName] = 0
    }

    // Reset multiplicateur
    gameState.MemoryStreak = 0
    gameState.MemoryMultiplier = 1.0
    gameState.MemoryResetCount++

    // Passer à l'équipe suivante (en sautant les éliminées)
    nextTeam := getNextNonEliminatedTeam()
    gameState.MemoryCurrentTeam = nextTeam

    // Broadcast reset dramatique
    broadcastMemoryReset()
  }

  // Vérifier fin de partie
  if len(gameState.MemoryEliminatedTeams) >= totalTeams - 1 {
    // Une seule équipe reste → game over
    winningTeam := getLastStandingTeam()
    endGameWithWinner(winningTeam)
  }
  ```

- [ ] **Interface admin (QuestionsPage)**
  - Sélecteur de mode de scoring Memory :
    - Radio buttons ou dropdown : TO_THE_END / MORT_SUBITE / PERFECT / CASCADE / TIME_BONUS / ZERO_SUM
    - Description courte + icône pour chaque mode
    - Inputs conditionnels selon le mode (PERFECT_BONUS, CASCADE_MAX_MULTIPLIER, etc.)

- [ ] **Affichages spécifiques par mode**

  **MORT_SUBITE** :
  - Badge "💀 MORT SUBITE" rouge permanent
  - Affichage High Score : "🏆 Meilleur : 45 pts"
  - Compteur de resets : "🔄 Resets : 2"
  - Animation dramatique lors du reset (écran rouge, son, shake)

  **CASCADE** :
  - Badge multiplicateur dynamique : "×2.5"
  - Couleur du badge selon le niveau (×1 blanc, ×3 jaune, ×5 or)
  - Animation d'augmentation du multiplicateur
  - Streak visible : "🔥 Série : 5"

  **TIME_BONUS** :
  - Indicateur temps restant avec projection du bonus
  - "⏱️ Bonus temps : +34 pts"
  - Barre de progression temps avec couleur du bonus

  **MAILLON_FAIBLE** :
  - Badge mode permanent : "⚡ MAILLON FAIBLE" avec couleur équipe courante
  - Indicateur équipe qui joue : "🎮 Tour de : [Équipe]" (grande bannière colorée)
  - High Score visible en permanence : "🏆 Meilleur : 42 pts"
  - Compteur de resets global : "🔄 Resets : 5"
  - Si bonus chaîne activé : Badge multiplicateur "×3.5" + "🔥 Série : 7"
  - Si élimination activée :
    - Cœurs/vies par équipe : "❤️❤️🖤" (ex: 2/3 vies restantes)
    - Liste équipes éliminées (grisées, barrées) : "~~Les Rouges~~"
    - Badge "ÉLIMINÉ" rouge sur équipe éliminée
  - Animation dramatique lors du reset :
    - Écran rouge clignotant
    - Shake de toute la grille
    - Son de buzzer négatif
    - Affichage temporaire "❌ ERREUR ! RESET COMPLET !"
  - Animation lors d'élimination :
    - Équipe qui disparaît en fondu
    - Badge "💀 ÉLIMINÉE" qui apparaît
    - Son dramatique

### Modes de jeu supplémentaires (propositions)

- [ ] **Mode ELIMINATION** (battle royale)
  - Multi-équipes uniquement
  - Chaque équipe a un quota d'erreurs (ex: 3 erreurs max)
  - Si une équipe dépasse le quota → éliminée
  - La dernière équipe en jeu gagne
  - Affichage des cœurs/vies par équipe

- [ ] **Mode SPEED_RUN** (timer par tour)
  - Multi-équipes avec timer court par tour (ex: 10s)
  - Si temps écoulé sans retourner 2 cartes → erreur + passe au suivant
  - Encourage la prise de décision rapide
  - Affichage d'un petit timer par tour

- [ ] **Mode BLITZ** (cartes éphémères)
  - Les cartes révélées se cachent plus rapidement (ex: 1.5s au lieu de 3s)
  - Nécessite mémorisation rapide
  - Peut être combiné avec d'autres modes
  - Paramètre : `BLITZ_FLIP_DELAY` (défaut: 1.5s)

- [ ] **Mode MAILLON_FAIBLE** (hybride tour par tour + reset)
  - **Hybride** : Combine CHACUN_SON_TOUR + MORT_SUBITE + options CASCADE/ELIMINATION
  - Multi-équipes uniquement
  - **Règles de base** :
    - Les équipes jouent à tour de rôle (rotation stricte)
    - Tant que l'équipe trouve des paires valides, elle continue de jouer
    - Si une paire est invalide → **RESET COMPLET** pour toutes les équipes :
      - ❌ Toutes les cartes remises face cachée
      - ❌ Tous les points retombent à zéro
      - ✅ High Score conservé
    - Passage à l'équipe suivante après le reset
  - **Option bonus chaîne** (activable) :
    - Multiplicateur CASCADE pendant la série de l'équipe
    - Reset du multiplicateur lors de l'erreur (en plus du reset global)
  - **Option élimination** (activable) :
    - Quota d'erreurs par équipe (ex: 3 erreurs max)
    - L'équipe qui fait l'erreur est éliminée (ne joue plus)
    - Les autres équipes continuent avec reset des cartes
    - Dernière équipe en jeu gagne
  - **Paramètres configurables** :
    - `CHAIN_BONUS_ENABLED` : activer le multiplicateur (bool)
    - `ELIMINATION_ENABLED` : activer l'élimination (bool)
    - `ERROR_QUOTA` : nombre d'erreurs avant élimination (int, si ELIMINATION_ENABLED)
  - Mode extrêmement tendu : combinaison la plus difficile
  - Référence au jeu TV "Le Maillon Faible"

### Scénario d'usage : Mode MAILLON_FAIBLE (avec bonus chaîne + élimination)

```
Initial : 6 paires à trouver, 3 équipes (Rouge, Bleu, Vert)
Quota d'erreurs : 2 max par équipe
Bonus chaîne : activé (multiplicateur)

Tour 1 - Équipe Rouge (×1.0) :
  - Retourne carte 1 (chat) + carte 3 (chat)
  - Match ! → +10 pts (×1.0) = 10 pts pour Rouge
  - Multiplicateur → ×1.5
  - Rouge continue (car match)

Tour 2 - Équipe Rouge (×1.5) :
  - Retourne carte 2 (oiseau) + carte 4 (oiseau)
  - Match ! → +10 pts (×1.5) = 15 pts pour Rouge
  - Total Rouge : 25 pts
  - Multiplicateur → ×2.0
  - Rouge continue

Tour 3 - Équipe Rouge (×2.0) :
  - Retourne carte 5 (chien) + carte 7 (poisson)
  - Non-match ! ❌
  - Rouge Erreur #1 (sur 2 max)
  → RESET COMPLET :
    - Toutes les cartes face cachée
    - Scores : Rouge 0, Bleu 0, Vert 0
    - High Score : 25 pts (conservé)
    - Multiplicateur reset → ×1.0
    - Compteur resets : 1
  → Passe à Équipe Bleu

Tour 4 - Équipe Bleu (×1.0) :
  - Retourne carte 1 (chat) + carte 3 (chat)
  - Match ! → +10 pts pour Bleu
  - Multiplicateur → ×1.5
  - Bleu continue

Tour 5 - Équipe Bleu (×1.5) :
  - Retourne carte 6 (souris) + carte 8 (lapin)
  - Non-match ! ❌
  - Bleu Erreur #1 (sur 2 max)
  → RESET COMPLET (compteur resets : 2)
  → Passe à Équipe Vert

Tour 6 - Équipe Vert (×1.0) :
  - Retourne carte 2 (oiseau) + carte 8 (lapin)
  - Non-match ! ❌
  - Vert Erreur #1 (sur 2 max)
  → RESET COMPLET (compteur resets : 3)
  → Passe à Équipe Rouge

Tour 7 - Équipe Rouge (×1.0) :
  - Retourne carte 5 (chien) + carte 1 (chat)
  - Non-match ! ❌
  - Rouge Erreur #2 (quota atteint)
  → 💀 ÉQUIPE ROUGE ÉLIMINÉE
  → RESET COMPLET (compteur resets : 4)
  → Passe à Équipe Bleu

Tour 8 - Équipe Bleu (×1.0) :
  - Retourne carte 1 (chat) + carte 3 (chat)
  - Match ! → +10 pts
  - Bleu continue...
  - (série de 6 matches consécutifs avec multiplicateurs croissants)
  - Bleu complète toutes les paires !
  → High Score final : 95 pts (avec multiplicateurs)
  → ÉQUIPE BLEU GAGNE

Équipes finales :
  🥇 Bleu : 95 pts (gagnant)
  🥈 Vert : 0 pts (éliminé)
  🥉 Rouge : 0 pts (éliminé)

Statistiques :
  - High Score de la partie : 95 pts
  - Nombre de resets : 7
  - Équipe la plus performante : Bleu (aucune erreur fatale)
```

### Tableau de synthèse des caractéristiques

#### Tableau 1 : Options activables par mode de jeu

Ce tableau montre quelles options de scoring sont **combinables** avec chaque mode de jeu.

| Mode de jeu | Solo/Multi | TO_THE_END | PERFECT | CASCADE | MORT_SUBITE | TIME_BONUS | ZERO_SUM | CHAIN_BONUS | ELIMINATION | Notes |
|-------------|------------|------------|---------|---------|-------------|------------|----------|-------------|-------------|-------|
| **SOLO** | Solo | ✅ Défaut | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | Tous les modes de scoring sauf options multi-équipes |
| **CHACUN_SON_TOUR** | Multi | ✅ Défaut | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | Rotation stricte après chaque tour |
| **TANT_QUE_JE_GAGNE** | Multi | ✅ Défaut | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | Équipe continue si match, change si erreur |
| **MAILLON_FAIBLE** | Multi | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ Option | ✅ Option | Mode **hybride autonome**, non combinable avec modes de scoring |
| **ELIMINATION** | Multi | ✅ Défaut | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ Intégré | Mode de jeu avec élimination intégrée |
| **SPEED_RUN** | Multi | ✅ Défaut | ✅ | ✅ | ⚠️ Difficile | ✅ | ✅ | ❌ | ✅ | Timer par tour, combinable avec scoring |
| **BLITZ** | Solo/Multi | ✅ | ✅ | ✅ | ⚠️ Extrême | ✅ | ✅ | ❌ | ✅ | **Modificateur** applicable à tous les modes |

**Légende :**
- ✅ **Compatible** : L'option peut être activée avec ce mode de jeu
- ✅ **Défaut** : Mode de scoring par défaut si non spécifié
- ✅ **Option** : Option activable dans la configuration du mode (pas un mode de scoring séparé)
- ✅ **Intégré** : Fonctionnalité intégrée dans le mode de jeu
- ❌ **Non compatible** : Impossible de combiner
- ⚠️ **Difficile/Extrême** : Techniquement possible mais très difficile à jouer

**Colonnes expliquées :**
- **TO_THE_END** : Paires restent visibles, scoring classique
- **PERFECT** : Bonus si aucune erreur
- **CASCADE** : Multiplicateur progressif (×1 à ×5)
- **MORT_SUBITE** : Reset complet si erreur (cartes + scores)
- **TIME_BONUS** : Bonus proportionnel au temps restant
- **ZERO_SUM** : Score peut être négatif
- **CHAIN_BONUS** : Multiplicateur pendant la série (spécifique MAILLON_FAIBLE)
- **ELIMINATION** : Quota d'erreurs, équipes éliminées

**Cas particuliers :**

1. **MAILLON_FAIBLE** :
   - **NE SE COMBINE PAS** avec les modes de scoring (TO_THE_END, PERFECT, etc.)
   - A son propre système de scoring intégré (reset si erreur)
   - 2 options activables indépendantes :
     - `CHAIN_BONUS_ENABLED: true/false` → Active le multiplicateur CASCADE
     - `ELIMINATION_ENABLED: true/false` → Active le quota d'erreurs + élimination

2. **BLITZ** :
   - N'est **PAS un mode de jeu**, c'est un **modificateur**
   - S'applique à n'importe quel mode (SOLO, CHACUN_SON_TOUR, etc.)
   - Change uniquement le `FLIP_DELAY` (1.5s au lieu de 3s)

3. **ELIMINATION** :
   - Peut être un mode de jeu à part entière (ligne ELIMINATION)
   - OU une option dans MAILLON_FAIBLE (colonne ELIMINATION)
   - OU une option combinée avec CHACUN_SON_TOUR/TANT_QUE_JE_GAGNE

#### Tableau 2 : Caractéristiques détaillées par combinaison

Ce tableau synthétise toutes les dimensions des modes Memory (jeu + scoring).

| Mode | Solo/Multi | Changement équipe | Reset cartes | Reset scores | Élimination | Multiplicateur | Bonus temps | High Score | Difficulté |
|------|------------|-------------------|--------------|--------------|-------------|----------------|-------------|------------|------------|
| **SOLO + TO_THE_END** | Solo | - | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ⭐ Facile |
| **SOLO + PERFECT** | Solo | - | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ⭐⭐ Moyen |
| **SOLO + CASCADE** | Solo | - | ❌ Non | ❌ Non | ❌ Non | ✅ Progressif | ❌ Non | ❌ Non | ⭐⭐⭐ Difficile |
| **SOLO + MORT_SUBITE** | Solo | - | ✅ Si erreur | ✅ Si erreur | ❌ Non | ❌ Non | ❌ Non | ✅ Oui | ⭐⭐⭐⭐⭐ Extrême |
| **SOLO + TIME_BONUS** | Solo | - | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ✅ Fin partie | ❌ Non | ⭐⭐ Moyen |
| **SOLO + ZERO_SUM** | Solo | - | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ⭐⭐⭐⭐ Très difficile |
| **CHACUN_SON_TOUR + TO_THE_END** | Multi | Après chaque tour | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ⭐⭐ Moyen |
| **CHACUN_SON_TOUR + PERFECT** | Multi | Après chaque tour | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ⭐⭐⭐ Difficile |
| **CHACUN_SON_TOUR + CASCADE** | Multi | Après chaque tour | ❌ Non | ❌ Non | ❌ Non | ✅ Par équipe | ❌ Non | ❌ Non | ⭐⭐⭐⭐ Très difficile |
| **CHACUN_SON_TOUR + ELIMINATION** | Multi | Après chaque tour | ❌ Non | ❌ Non | ✅ Quota erreurs | ❌ Non | ❌ Non | ❌ Non | ⭐⭐⭐ Difficile |
| **TANT_QUE_JE_GAGNE + TO_THE_END** | Multi | Si erreur | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ⭐⭐⭐ Difficile |
| **TANT_QUE_JE_GAGNE + CASCADE** | Multi | Si erreur | ❌ Non | ❌ Non | ❌ Non | ✅ Par équipe | ❌ Non | ❌ Non | ⭐⭐⭐⭐ Très difficile |
| **TANT_QUE_JE_GAGNE + MORT_SUBITE** | Multi | Si erreur | ✅ Si erreur | ✅ Si erreur | ❌ Non | ❌ Non | ❌ Non | ✅ Oui | ⭐⭐⭐⭐⭐ Extrême |
| **TANT_QUE_JE_GAGNE + ZERO_SUM** | Multi | Si erreur | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ⭐⭐⭐⭐ Très difficile |
| **MAILLON_FAIBLE** | Multi | Si erreur | ✅ Si erreur | ✅ Si erreur | ❌ Non | ❌ Non | ❌ Non | ✅ Oui | ⭐⭐⭐⭐⭐ Extrême |
| **MAILLON_FAIBLE + bonus chaîne** | Multi | Si erreur | ✅ Si erreur | ✅ Si erreur | ❌ Non | ✅ Par équipe | ❌ Non | ✅ Oui | ⭐⭐⭐⭐⭐ Extrême |
| **MAILLON_FAIBLE + élimination** | Multi | Si erreur | ✅ Si erreur | ✅ Si erreur | ✅ Quota erreurs | ❌ Non | ❌ Non | ✅ Oui | ⭐⭐⭐⭐⭐ Extrême |
| **MAILLON_FAIBLE + chaîne + élim** | Multi | Si erreur | ✅ Si erreur | ✅ Si erreur | ✅ Quota erreurs | ✅ Par équipe | ❌ Non | ✅ Oui | ⭐⭐⭐⭐⭐ Extrême |
| **ELIMINATION + TO_THE_END** | Multi | Après chaque tour | ❌ Non | ❌ Non | ✅ Quota erreurs | ❌ Non | ❌ Non | ❌ Non | ⭐⭐⭐ Difficile |
| **SPEED_RUN + TO_THE_END** | Multi | Si timeout | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ⭐⭐⭐ Difficile |
| **BLITZ + TIME_BONUS** | Solo/Multi | Selon mode | ❌ Non | ❌ Non | ❌ Non | ❌ Non | ✅ Fin partie | ❌ Non | ⭐⭐⭐ Difficile |

**Légende :**
- **Solo/Multi** : Nombre d'équipes qui jouent
- **Changement équipe** : Quand passe-t-on à l'équipe suivante ? (`-` = Solo)
- **Reset cartes** : Les cartes matchées sont-elles remises face cachée ?
- **Reset scores** : Les scores sont-ils remis à zéro ?
- **Élimination** : Des équipes peuvent-elles être éliminées ?
- **Multiplicateur** : Y a-t-il un multiplicateur de points progressif ?
- **Bonus temps** : Y a-t-il un bonus lié au temps restant ?
- **High Score** : Y a-t-il un high score conservé après reset ?
- **Difficulté** : Niveau de difficulté global (⭐ à ⭐⭐⭐⭐⭐)

**Notes importantes :**
1. **MAILLON_FAIBLE** est un mode **hybride autonome** qui :
   - Combine tour par tour (continue si match) + reset complet si erreur
   - Ne se combine PAS avec d'autres modes de points (il a son propre système)
   - Peut avoir 2 options : bonus chaîne (multiplicateur) et/ou élimination

2. **BLITZ** est un **modificateur** applicable à n'importe quel mode (cartes se cachent plus vite)

3. **SPEED_RUN** et **ELIMINATION** sont des **modes de jeu** à part entière, combinables avec modes de points

### Tableau des combinaisons pertinentes (cas d'usage)

| Mode de jeu | Mode de points | Difficulté | Description | Cas d'usage |
|-------------|----------------|------------|-------------|-------------|
| **SOLO** | TO_THE_END | ⭐ Facile | Une équipe, paires restent visibles | Apprentissage, débutants |
| **SOLO** | PERFECT | ⭐⭐ Moyen | Une équipe, bonus si aucune erreur | Entraînement concentration |
| **SOLO** | MORT_SUBITE | ⭐⭐⭐⭐⭐ Extrême | Une équipe, reset complet si erreur | Challenge hardcore |
| **SOLO** | CASCADE | ⭐⭐⭐ Difficile | Une équipe, multiplicateur progressif | Récompenser les séries |
| **SOLO** | TIME_BONUS | ⭐⭐ Moyen | Une équipe, bonus temps | Course contre la montre |
| **CHACUN_SON_TOUR** | TO_THE_END | ⭐⭐ Moyen | Multi-équipes, tour par tour classique | Jeu équitable multi-équipes |
| **CHACUN_SON_TOUR** | PERFECT | ⭐⭐⭐ Difficile | Multi-équipes, bonus si équipe parfaite | Compétition précision |
| **CHACUN_SON_TOUR** | CASCADE | ⭐⭐⭐⭐ Très difficile | Multi-équipes, multiplicateur par équipe | Compétition séries |
| **CHACUN_SON_TOUR** | ELIMINATION | ⭐⭐⭐ Difficile | Multi-équipes, quota d'erreurs | Battle royale |
| **TANT_QUE_JE_GAGNE** | TO_THE_END | ⭐⭐⭐ Difficile | Multi-équipes, garde la main si match | Récompenser la réussite |
| **TANT_QUE_JE_GAGNE** | CASCADE | ⭐⭐⭐⭐ Très difficile | Multi-équipes, multiplicateur + garde main | Pro, très compétitif |
| **TANT_QUE_JE_GAGNE** | MORT_SUBITE | ⭐⭐⭐⭐⭐ Extrême | Multi-équipes, reset global si erreur | Tension maximale |
| **TANT_QUE_JE_GAGNE** | ZERO_SUM | ⭐⭐⭐⭐ Très difficile | Multi-équipes, score négatif possible | Experts, risque élevé |
| **SOLO + BLITZ** | TIME_BONUS | ⭐⭐⭐ Difficile | Cartes rapides + bonus temps | Speed run |
| **CHACUN_SON_TOUR + SPEED_RUN** | TO_THE_END | ⭐⭐⭐ Difficile | Timer par tour, tour par tour | Décisions rapides |
| **ELIMINATION** | TO_THE_END | ⭐⭐⭐ Difficile | Multi-équipes, élimination progressive | Battle royale memory |
| **MAILLON_FAIBLE** | - | ⭐⭐⭐⭐⭐ Extrême | Tour par tour + reset global si erreur | "Le Maillon Faible" TV |
| **MAILLON_FAIBLE + bonus chaîne** | - | ⭐⭐⭐⭐⭐ Extrême | + multiplicateur CASCADE pendant série | Très compétitif, risque max |
| **MAILLON_FAIBLE + élimination** | - | ⭐⭐⭐⭐⭐ Extrême | + quota erreurs, élimination équipes | Survie, tension maximale |

### Combinaisons NON recommandées

| Mode de jeu | Mode de points | Raison |
|-------------|----------------|--------|
| SOLO | ELIMINATION | Pas de sens (une seule équipe) |
| MORT_SUBITE | ZERO_SUM | Trop punitif (double pénalité) |
| BLITZ | MORT_SUBITE | Quasi impossible (cartes trop rapides + reset) |

### Compatibilité

- ✅ Rétrocompatible : Questions Memory sans `MEMORY_SCORING_MODE` utilisent "TO_THE_END" par défaut
- ✅ Rétrocompatible : Questions Memory sans `MEMORY_MODE` utilisent "SOLO" par défaut
- ✅ Combinaisons infinies : 4 modes jeu × 6 modes points = 24 variantes de base (+ modes hybrides)
- ✅ MAILLON_FAIBLE est un mode hybride autonome (pas combinable avec d'autres modes de points)
- ✅ Extension future facile : ajouter de nouveaux modes sans casser l'existant

## Améliorations futures (hors scope initial)

- [ ] **Mode Équipes** : les équipes buzzent pour désigner les cartes
- [ ] **Mode Chrono** : temps limité, max de paires en un temps donné
- [ ] **Thèmes de cartes** : dos de carte personnalisable
- [ ] **Types de paires mixtes** : Image ↔ Texte (association)
- [ ] **Niveaux de difficulté** : délai de retournement variable

## Version

Phases 1-5 implémentées (v2.33.0)
