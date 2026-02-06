# Brief Dev-Backend Phase 6 Memory Multi-Teams

**Version** : 2.51.0
**Branche** : feature/memory-multi-teams
**Dépendances** : Aucune (premier agent)
**Timeline** : 2h estimées

---

## Contexte

Phase 6 du Memory Game : support des modes de jeu multi-équipes (SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE).

Frontend attend les contrats finalisés. Tu peux modifier les contrats si contrainte technique (documenter le changement).

---

## Tâches Backend

### 1. Models (server-go/internal/game/models.go)

Ajouter à `Question` :
```go
MEMORY_MODE string // "SOLO" | "CHACUN_SON_TOUR" | "TANT_QUE_JE_GAGNE"
```

Ajouter à `GameState` :
```go
MemoryCurrentTeam      string            // Équipe jouant actuellement
MemoryTeamPairs        map[string]int    // Paires trouvées par équipe
MemoryParticipatingTeams []string        // Équipes sélectionnées
```

---

### 2. Engine - Initialisation (server-go/internal/game/engine.go)

Fonction `startMemoryGame()` (ou modifier existante) :
- Récupérer équipes participantes depuis GameState
- Initialiser `MemoryCurrentTeam` = première équipe
- Initialiser `MemoryTeamPairs` = map vide pour chaque équipe
- Stocker mode de jeu depuis Question.MEMORY_MODE

---

### 3. Engine - Logique de retournement (FlipMemoryCard)

Modifier `FlipMemoryCard()` :

**Mode SOLO** :
- Comportement identique à Phase 5

**Mode CHACUN_SON_TOUR** :
- Après chaque tentative (2 cartes), appeler `rotateToNextTeam()`
- Que la paire soit trouvée ou non

**Mode TANT_QUE_JE_GAGNE** :
- Si paire trouvée : attribuer à l'équipe, elle continue
- Si non-match : passer à l'équipe suivante

---

### 4. Engine - Fonction rotation (nouveau)

Nouvelle fonction `rotateToNextTeam()` :
```go
func (gs *GameState) rotateToNextTeam() {
  // Trouver index équipe courante
  // Passer à l'index suivant (modulo)
  // Mettre à jour MemoryCurrentTeam
  // Broadcast MEMORY_TEAM_CHANGED
}
```

---

### 5. WebSocket Actions (server-go/internal/server/websocket.go)

**MEMORY_SET_TEAMS** (nouvelle) :
- Payload : `{"Teams": ["Équipe1", "Équipe2", ...]}`
- Valider : min 2 équipes en multi, 1 en solo
- Initialiser GameState.MemoryParticipatingTeams
- Initialiser GameState.MemoryCurrentTeam = première équipe
- Initialiser GameState.MemoryTeamPairs

**FLIP_MEMORY_CARD** (modifiée) :
- Valider : vérifier équipe courante en multi
- Rejeter si équipe incorrecte avec erreur `"INVALID_TEAM"`

---

### 6. Broadcasts (server-go/internal/server/websocket.go)

**MEMORY_TEAM_CHANGED** (nouveau broadcast) :
```json
{
  "ACTION": "MEMORY_TEAM_CHANGED",
  "MSG": {
    "CurrentTeam": "Équipe Bleue",
    "TeamIndex": 1,
    "PairsCount": {
      "Équipe Rouge": 2,
      "Équipe Bleue": 0,
      "Équipe Verte": 1
    }
  }
}
```

**GAME_STATE** (modifiée) :
- Ajouter champs Memory multi :
  - `MemoryCurrentTeam`
  - `MemoryTeamPairs`
  - `MemoryParticipatingTeams`

---

### 7. Historique (server-go/internal/game/history.go)

Modifier `MEMORY_COMPLETED` pour inclure :
```json
{
  "EventType": "MEMORY_COMPLETED",
  "Details": {
    "PairsFound": 6,
    "TotalPairs": 6,
    "Errors": 2,
    "TimeElapsed": 180000,
    "Mode": "CHACUN_SON_TOUR",
    "TeamResults": {
      "Équipe Rouge": {
        "Pairs": 2,
        "Points": 20
      },
      "Équipe Bleue": {
        "Pairs": 2,
        "Points": 20
      }
    }
  }
}
```

---

## Contrats API

Fichier : `contracts/memory-phase6.md` (déjà créé)

Tu peux modifier si contrainte technique (noter les changements).

---

## Validation Critique

- Rotation correcte (modulo)
- Scores par équipe mis à jour
- Historique inclut résultats multi
- Rétrocompatibilité Phase 5 (SOLO par défaut)
- Questions sans MEMORY_MODE = SOLO

---

## Livrables

- Code compilé et buildable
- Commits atomiques
- Version incrémentée ✅ (déjà fait : 2.51.0)
- Contrats API finalisés (ou documenté changements)

---

## Notes

- Phase 5 (SOLO) doit continuer de fonctionner sans modification
- Frontend attend ces contrats finalisés
- Push sur branche feature/memory-multi-teams après implémentation

