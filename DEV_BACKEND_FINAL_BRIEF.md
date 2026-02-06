# Dev-Backend Phase 6 - Brief Final d'Implémentation

**Version** : 2.51.0
**Branche** : feature/memory-multi-teams
**Date** : 2026-02-02
**Timeline** : 2h estimées

---

## Tâche

Implémenter le support des modes de jeu multi-équipes pour Memory Game.

3 modes à supporter :
- **SOLO** (défaut, existant) - Une équipe joue
- **CHACUN_SON_TOUR** (nouveau) - Rotation stricte après chaque paire
- **TANT_QUE_JE_GAGNE** (nouveau) - Équipe continue si match, change si erreur

---

## Fichiers à Modifier (5)

### 1. server-go/internal/game/models.go

Ajouter constantes pour MemoryMode (après QuestionType constantes) :
- MemoryModeSolo = "SOLO"
- MemoryModeChacunSonTour = "CHACUN_SON_TOUR"
- MemoryModeTantQueJeGagne = "TANT_QUE_JE_GAGNE"

Ajouter dans Question struct :
- MemoryMode MemoryMode (json: "MEMORY_MODE,omitempty")

Ajouter dans GameState struct (après MemoryErrors) :
- MemoryCurrentTeam string (json: "MEMORY_CURRENT_TEAM,omitempty")
- MemoryTeamPairs map[string]int (json: "MEMORY_TEAM_PAIRS,omitempty")
- MemoryParticipatingTeams []string (json: "MEMORY_PARTICIPATING_TEAMS,omitempty")

---

### 2. server-go/internal/protocol/messages.go

Ajouter constante :
- ActionMemorySetTeams = "MEMORY_SET_TEAMS"

Ajouter struct MemorySetTeamsPayload :
- Teams []string (json: "Teams")

---

### 3. server-go/internal/game/engine.go

**Ajouter fonction rotateToNextTeam()** :
- Trouve index équipe courante dans MemoryParticipatingTeams
- Passe à l'index suivant avec modulo
- Gère cas équipe introuvable (reset à première)

**Modifier FlipMemoryCard()** :

Après révélation 2ème carte :
- Mode SOLO : comportement Phase 5 identique
- Mode CHACUN_SON_TOUR : rotateToNextTeam() après chaque paire (match ou non)
- Mode TANT_QUE_JE_GAGNE : rotateToNextTeam() seulement si non-match
- Incrémenter MemoryTeamPairs[MemoryCurrentTeam] quand match trouvé
- Incrémenter MemoryErrors quand non-match

**Initialiser au démarrage Memory** :
- Si MemoryParticipatingTeams vide, affecter première équipe
- Initialiser MemoryCurrentTeam = première équipe
- Créer MemoryTeamPairs map avec 0 pour chaque équipe

---

### 4. server-go/internal/server/websocket.go ou http.go

**Handler MEMORY_SET_TEAMS** :
- Valider payload Teams
- Vérifier question est MEMORY
- Vérifier min 2 équipes en multi (min 1 en SOLO)
- Initialiser GameState.MemoryParticipatingTeams
- Initialiser GameState.MemoryCurrentTeam (première)
- Initialiser GameState.MemoryTeamPairs (map)
- Broadcaster UPDATE_GAME_STATE

**Broadcaster MEMORY_TEAM_CHANGED** :
- Après appel rotateToNextTeam()
- Payload : CurrentTeam, TeamIndex, PairsCount
- Uniquement en multi-équipes

---

### 5. server-go/internal/game/history.go

**Modifier MEMORY_COMPLETED** :
- Ajouter champ Mode (valeur de MemoryMode)
- Ajouter TeamResults si multi-équipes
  - TeamResults[team] = {Pairs: N, Points: N*POINTS_PER_PAIR}
  - Ajouter COMPLETION_BONUS à équipe qui trouve dernière paire

---

## Validation Critique

Avant de push :

- go build -o server.exe ./cmd/server doit compiler
- go test ./... -v doit passer
- Rétrocompatibilité : Questions sans MemoryMode jouent en SOLO
- Rotation : modulo fonctionne (2, 3, 4+ équipes)
- Scores : MemoryTeamPairs mis à jour
- Historique : MEMORY_COMPLETED complet

---

## Commits Recommandés

1. feat(models): Add MEMORY_MODE and multi-team fields
2. feat(protocol): Add MEMORY_SET_TEAMS action
3. feat(engine): Implement rotateToNextTeam and flip logic
4. feat(websocket): Handle MEMORY_SET_TEAMS and broadcast
5. feat(history): Enrich MEMORY_COMPLETED

---

## Ressources

- contracts/memory-phase6.md (contrats API)
- backlog/En-Cours/memory-game.md (Phase 6)
- IMPLEMENTATION_PLAN_PHASE6.md (plan global)

---

## Durée

2 heures estimées
