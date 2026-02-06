# Contrats API - Memory Phase 6 (Multi-Teams)

> **Version** : 2.51.0
> **Statut** : À implémenter
> **Date** : 2026-02-02

---

## Actions WebSocket

### MEMORY_SET_TEAMS

Définit les équipes participantes avant de démarrer une partie Memory multi-équipes.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server (Admin)` |
| Phase     | PREPARE |
| Type      | MEMORY |
| Trigger   | Clic bouton "Start" en mode multi-équipes |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| Teams | []string | ✅ | Liste des noms d'équipes participantes |

#### Validation

- **Minimum 2 équipes** requises en mode `CHACUN_SON_TOUR` ou `TANT_QUE_JE_GAGNE`
- **1 équipe minimum** en mode `SOLO`
- Les équipes doivent exister dans la configuration

#### Exemple

```json
{
  "ACTION": "MEMORY_SET_TEAMS",
  "MSG": {
    "Teams": ["Équipe Rouge", "Équipe Bleue", "Équipe Verte"]
  }
}
```

#### Notes

- Action envoyée **avant** `START_GAME`
- Initialise `GameState.MemoryParticipatingTeams`
- Initialise `GameState.MemoryCurrentTeam` = première équipe
- Initialise `GameState.MemoryTeamPairs` = map vide pour chaque équipe

---

### FLIP_MEMORY_CARD (Modification)

Retourne une carte Memory. Comportement identique en Phase 5, mais avec validation supplémentaire en multi-équipes.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STARTED |
| Type      | MEMORY |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| CARD_ID | string | ID de la carte (ex: "1-1", "2-2") |

#### Validation (Mode Multi-Équipes)

- **Vérifier l'équipe courante** : `gs.MemoryCurrentTeam`
- **Rejeter si équipe non-correspondante** (mode CHACUN_SON_TOUR et TANT_QUE_JE_GAGNE)
- Envoyer erreur : `"INVALID_TEAM"` si équipe incorrecte

#### Exemple

```json
{
  "ACTION": "FLIP_MEMORY_CARD",
  "MSG": {
    "CARD_ID": "1-1"
  }
}
```

---

## Broadcasts Server → Clients

### MEMORY_TEAM_CHANGED

Notifie du changement d'équipe active.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Clients (Broadcast)` |
| Phase     | STARTED |
| Type      | MEMORY |
| Trigger   | Fin de tentative (mode CHACUN_SON_TOUR) ou erreur (mode TANT_QUE_JE_GAGNE) |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| CurrentTeam | string | Nom de l'équipe qui joue |
| TeamIndex | int | Index dans `MemoryParticipatingTeams` |
| PairsCount | map[string]int | Nombre de paires trouvées par équipe |

#### Exemple

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

#### Réception Frontend

- Mettre à jour le badge "Au tour de : [Équipe]"
- Mettre à jour le tableau des scores
- Animer la transition d'équipe
- Désactiver/réactiver les clics selon l'équipe courante

---

### GAME_STATE (Modification existante)

Broadcast de l'état du jeu. Ajouter les champs Memory multi-équipes.

#### Champs ajoutés

```json
"Memory": {
  "MemoryCurrentTeam": "Équipe Bleue",
  "MemoryTeamPairs": {
    "Équipe Rouge": 2,
    "Équipe Bleue": 1,
    "Équipe Verte": 3
  },
  "MemoryParticipatingTeams": ["Équipe Rouge", "Équipe Bleue", "Équipe Verte"],
  // ... champs Phase 5 existants
}
```

---

## Modèles de Données

### Question (models.go)

```go
type Question struct {
  // ... champs existants
  MEMORY_MODE string  // "SOLO" | "CHACUN_SON_TOUR" | "TANT_QUE_JE_GAGNE"
}
```

**Compatibilité** : Absence de `MEMORY_MODE` = défaut `"SOLO"`

---

### GameState (models.go)

```go
type GameState struct {
  // ... champs existants
  MemoryCurrentTeam         string            // Équipe jouant actuellement
  MemoryTeamPairs           map[string]int    // Paires trouvées par équipe
  MemoryParticipatingTeams  []string          // Équipes sélectionnées pour le jeu
}
```

**Initialisation** :
- `MemoryCurrentTeam` = première équipe de `MemoryParticipatingTeams`
- `MemoryTeamPairs` = map avec 0 pour chaque équipe
- `MemoryParticipatingTeams` = reçu via `MEMORY_SET_TEAMS`

---

## Historique des Événements

### MEMORY_COMPLETED (Modification existante)

EventType : `"MEMORY_COMPLETED"`

Ajouter les champs multi-équipes :

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
      },
      "Équipe Verte": {
        "Pairs": 2,
        "Points": 20
      }
    }
  }
}
```

---

## Flux de Communication

### Initialization Sequence (Mode Multi)

```
1. Admin sélectionne les équipes en "Prepare"
   ↓
2. Admin clique "Start"
   ↓
3. Client envoie MEMORY_SET_TEAMS
   ↓
4. Serveur initialise : CurrentTeam, TeamPairs, ParticipatingTeams
   ↓
5. Client envoie START_GAME
   ↓
6. Serveur broadcast GAME_STATE (inclus champs Memory)
   ↓
7. TV affiche badge + grille + tableau scores
```

### Gameplay Sequence (CHACUN_SON_TOUR)

```
1. Équipe 1 clique cartes
   ↓
2. Serveur reçoit FLIP_MEMORY_CARD
   ↓
3. Validation OK (c'est l'équipe courante)
   ↓
4. Après 2 cartes révélées
   ↓
5. Serveur broadcast MEMORY_TEAM_CHANGED
   ↓
6. CurrentTeam = Équipe 2
   ↓
7. TV met à jour badge + désactive clics Équipe 1
   ↓
8. Équipe 2 peut cliquer
```

---

## Points de Validation

1. ✅ Phase 5 (SOLO) continue de fonctionner sans modification
2. ✅ Questions sans `MEMORY_MODE` défautent à SOLO
3. ✅ Validation équipes : min 2 en multi, 1 en solo
4. ✅ Rotation correcte des équipes (modulo)
5. ✅ Scores par équipe mis à jour correctement
6. ✅ Historique inclut résultats multi-équipes
7. ✅ Badge équipe courante affichée en temps réel
8. ✅ Cartes non-cliquables pour équipes en attente

---

## Rétrocompatibilité

| Cas | Comportement |
|-----|-------------|
| Question sans MEMORY_MODE | Défaut SOLO (Phase 5) |
| MEMORY_SET_TEAMS avec 1 équipe | Accepté (mode SOLO) |
| Client ancien (pas de set teams) | Défaut à première équipe |
| GameState sans champs multi | Initialisation automatique |

