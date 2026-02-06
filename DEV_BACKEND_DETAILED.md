# Dev-Backend Phase 6 - Brief Complet

**Version** : 2.51.0
**Branche** : feature/memory-multi-teams
**Timeline** : 2h estimées
**Dépendances** : Aucune

---

## Structure du Code (Important pour l'implémentation)

### Packages

- `internal/game/` : Modèles (models.go), moteur (engine.go)
- `internal/server/` : HTTP, WebSocket, TCP/UDP
- `internal/protocol/` : Définition des actions

### Fichiers Clés

**models.go** (L172) :
- GameState struct avec 15+ champs existants
- À AJOUTER 3 champs : MemoryCurrentTeam, MemoryTeamPairs, MemoryParticipatingTeams

**Question struct** (L140+) :
- À AJOUTER champ : MEMORY_MODE string

**protocol/messages.go** :
- ActionFlipMemoryCard = "FLIP_MEMORY_CARD" (existant)
- À AJOUTER : ActionMemorySetTeams = "MEMORY_SET_TEAMS"

**engine.go** :
- FlipMemoryCard(cardID string) error (existant, Phase 5)
- À AJOUTER : rotateToNextTeam() fonction
- À MODIFIER : logique FlipMemoryCard()

**http.go / websocket.go** :
- Handlers WebSocket pour traiter les actions

---

## Tâches Détaillées

### TÂCHE 1 : Ajouter champs Question (models.go)

Ajouter après les champs QCM :

```go
MEMORY_MODE string  `json:"MEMORY_MODE,omitempty"`
```

Notes:
- Absence de champ = défaut "SOLO" (rétrocompatibilité)
- Valeurs : "SOLO" | "CHACUN_SON_TOUR" | "TANT_QUE_JE_GAGNE"

---

### TÂCHE 2 : Ajouter champs GameState (models.go)

Après MemoryErrors int (L184), ajouter :

```go
MemoryCurrentTeam       string            `json:"MEMORY_CURRENT_TEAM,omitempty"`
MemoryTeamPairs         map[string]int    `json:"MEMORY_TEAM_PAIRS,omitempty"`
MemoryParticipatingTeams []string         `json:"MEMORY_PARTICIPATING_TEAMS,omitempty"`
```

Notes:
- Initialiser maps et slices proprement en fonction
- JSON tags avec omitempty pour optionalité

---

### TÂCHE 3 : Ajouter action dans protocol/messages.go

```go
const (
  ActionMemorySetTeams = "MEMORY_SET_TEAMS"
)

type MemorySetTeamsPayload struct {
  Teams []string `json:"Teams"`
}
```

---

### TÂCHE 4 : Implémenter handler MEMORY_SET_TEAMS

Dans le handler WebSocket (http.go ou websocket.go), ajouter case:

```go
case protocol.ActionMemorySetTeams:
  // Valider payload
  // Vérifier question est MEMORY
  // Valider min 2 équipes en multi-équipes
  // Initialiser GameState.MemoryParticipatingTeams
  // Initialiser GameState.MemoryCurrentTeam = premier
  // Initialiser GameState.MemoryTeamPairs (map vide)
  // Broadcast GAME_STATE mis à jour
```

---

### TÂCHE 5 : Implémenter rotateToNextTeam() dans engine.go

```go
func (gs *GameState) rotateToNextTeam() {
  // Trouver index équipe courante
  // Passer à l'index suivant (modulo)
  // Mettre à jour MemoryCurrentTeam
}
```

---

### TÂCHE 6 : Modifier FlipMemoryCard() dans engine.go

Ajouter après logique flip :

**Pour Mode CHACUN_SON_TOUR :**
- Après 2 cartes révélées, appeler rotateToNextTeam()
- Peu importe match ou non

**Pour Mode TANT_QUE_JE_GAGNE :**
- Si match : incrémenter MemoryTeamPairs, continuer
- Si non-match : incrémenter erreurs, appeler rotateToNextTeam()

**Pour Mode SOLO :**
- Comportement identique Phase 5

---

### TÂCHE 7 : Broadcast MEMORY_TEAM_CHANGED

Après appel rotateToNextTeam(), broadcaster :

```json
{
  "ACTION": "MEMORY_TEAM_CHANGED",
  "MSG": {
    "CurrentTeam": "...",
    "TeamIndex": 0,
    "PairsCount": {...}
  }
}
```

---

### TÂCHE 8 : Enrichir MEMORY_COMPLETED dans history.go

Ajouter champs à l'événement :

```json
"Mode": "CHACUN_SON_TOUR",
"TeamResults": {
  "Équipe1": {"Pairs": 2, "Points": 20},
  "Équipe2": {"Pairs": 1, "Points": 10}
}
```

---

## Validation Critique

Avant de push :

1. go build doit passer (compilation sans erreur)
2. go test ./... doit passer (tous les tests existants)
3. Rétrocompatibilité : Questions Phase 5 sans MEMORY_MODE doivent marcher
4. Logique rotation : modulo fonctionne correctement
5. Scores : MemoryTeamPairs mis à jour et broadcaté

---

## Commits Recommandés (Atomiques)

1. feat(models): Add MEMORY_MODE to Question and multi-team fields to GameState
2. feat(protocol): Add MEMORY_SET_TEAMS action definition
3. feat(engine): Implement rotateToNextTeam and multi-team flip logic
4. feat(websocket): Handle MEMORY_SET_TEAMS action and broadcast MEMORY_TEAM_CHANGED
5. feat(history): Enrich MEMORY_COMPLETED with multi-team results

---

## Ressources

- Contrats API : contracts/memory-phase6.md
- Plan : IMPLEMENTATION_PLAN_PHASE6.md
- Backlog Phase 6 : backlog/En-Cours/memory-game.md (section Phase 6)

EOF
