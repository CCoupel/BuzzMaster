# Plan d'Implémentation Phase 6 : Memory Multi-Teams

**Version** : 2.51.0
**Branche** : feature/memory-multi-teams
**Date** : 2026-02-02

## Vue d'ensemble

La Phase 6 ajoute le support des modes de jeu multi-équipes au Memory Game :
- Mode SOLO (défaut, existant)
- Mode CHACUN_SON_TOUR (rotation stricte après chaque tentative)
- Mode TANT_QUE_JE_GAGNE (équipe continue si match, passe si erreur)

## Tâches Backend

### 1. Modèles Go (models.go)

Ajouter les nouveaux champs à la structure `Question` :
```go
type Question struct {
  // ... champs existants
  MEMORY_MODE string // "SOLO" | "CHACUN_SON_TOUR" | "TANT_QUE_JE_GAGNE"
}
```

Ajouter les champs à `GameState` :
```go
type GameState struct {
  // ... champs existants
  MemoryCurrentTeam      string            // Équipe jouant actuellement
  MemoryTeamPairs        map[string]int    // Paires trouvées par équipe
  MemoryParticipatingTeams []string        // Équipes sélectionnées pour le jeu
}
```

### 2. Engine - Initialisation du jeu (engine.go)

Fonction `StartMemoryGame()` :
- Récupérer la liste des équipes participantes depuis GameState
- Initialiser `MemoryCurrentTeam` avec la première équipe
- Initialiser `MemoryTeamPairs` avec 0 pour chaque équipe
- Définir le mode de jeu depuis la Question

### 3. Engine - Logique de retournement (FlipMemoryCard)

Modifier `FlipMemoryCard()` pour supporter multi-équipes :

**Mode SOLO** :
- Comportement identique à la Phase 5

**Mode CHACUN_SON_TOUR** :
- Après chaque tentative (2 cartes révélées), appeler `rotateToNextTeam()`
- Que la paire soit trouvée ou non

**Mode TANT_QUE_JE_GAGNE** :
- Si paire trouvée : attribuer à l'équipe courante, elle continue
- Si non-match : passer à l'équipe suivante via `rotateToNextTeam()`

### 4. Engine - Changement d'équipe (nouveau)

Nouvelle fonction `rotateToNextTeam()` :
- Trouver l'index de l'équipe courante dans `MemoryParticipatingTeams`
- Passer à l'index suivant (modulo pour revenir au début)
- Broadcast du changement d'équipe aux clients

### 5. WebSocket Actions

Ajouter/modifier les actions :

`FLIP_MEMORY_CARD` (existante) :
- Validation : vérifier que c'est l'équipe courante qui clique (mode multi)

`MEMORY_SET_TEAMS` (nouvelle) :
- Payload : `{"Teams": ["Équipe1", "Équipe2", "Équipe3"]}`
- Appelée lors du "Prepare" avant de démarrer
- Valide : minimum 2 équipes en mode multi, 1 équipe en mode SOLO

`MEMORY_GAME_STATE` (broadcast) :
- Ajouter les champs : `CurrentTeam`, `TeamPairs`, `ParticipatingTeams`

### 6. Persistance - Historique

Modifier l'enregistrement des scores :
- EventType : `"MEMORY_COMPLETED"` (remplace `"POINTS_AWARDED"`)
- Détails inclus : mode de jeu, résultats par équipe

## Tâches Frontend

### 1. GamePage - Sélection des équipes

Composant dépliable "Équipes participantes" dans la section Prepare :
- Récupérer la liste des équipes depuis le state
- Checkboxes pour sélectionner les équipes
- Validation : minimum 2 équipes requises en mode multi
- Bouton "Start" désactivé si conditions non remplies
- Envoyer `MEMORY_SET_TEAMS` au serveur avant `START_GAME`

### 2. GamePage - Sélection du mode

Lors de la création de la question Memory :
- Radio buttons ou dropdown dans QuestionsPage
- Options : SOLO / CHACUN_SON_TOUR / TANT_QUE_JE_GAGNE
- Afficher description courte de chaque mode

### 3. PlayerDisplay - Affichage équipe courante

Sur `/tv` (affichage grille Memory) :
- Ajouter un badge au-dessus de la grille
- Texte : "🎮 Au tour de : [Équipe]"
- Couleur dynamique selon l'équipe
- Animation de transition lors du changement

### 4. PlayerDisplay - Contrôle des clics

En mode multi-équipes :
- Seule l'équipe courante peut cliquer sur les cartes
- Les autres voient la grille mais cartes non-cliquables (pointer-events: none)
- Grisage visuel des cartes pour les équipes en attente

### 5. PlayerDisplay - Tableau des scores

Affichage en temps réel pendant le jeu :
- Tableau avec une ligne par équipe
- Colonnes : Équipe | Paires | Points
- Mise à jour à chaque paire trouvée
- Classement en direct

### 6. QuestionsPage - Éditeur Memory

Ajouter sélecteur de mode :
- Radio buttons : SOLO / CHACUN_SON_TOUR / TANT_QUE_JE_GAGNE
- Description courte : "Une seule équipe", "Rotation après chaque tour", "Continue si match"
- Mode par défaut : SOLO

## Contrats API à créer/modifier

Fichier : `contracts/websocket-actions.md`

Ajouter les actions :
```markdown
### MEMORY_SET_TEAMS
Direction : Client → Serveur (Admin)
Payload : {"Teams": ["Équipe1", "Équipe2"]}
Description : Définit les équipes participantes avant de démarrer
Validation : min 2 équipes en mode multi, 1 en mode SOLO

### MEMORY_TEAM_CHANGED
Direction : Serveur → Clients (Broadcast)
Payload : {"CurrentTeam": "Équipe1", "Index": 0}
Description : Notifie du changement d'équipe active
```

Modifier :
```markdown
### FLIP_MEMORY_CARD
... (existant)
Validation : Vérifier l'équipe courante en mode multi
```

## Tâches de Tests

### Tests unitaires Go

Fichier : `server-go/internal/game/memory_test.go`

```go
func TestMemoryMultiTeamSOLO(t *testing.T) {}
func TestMemoryMultiTeamRotation(t *testing.T) {}
func TestMemoryMultiTeamKeeperMode(t *testing.T) {}
func TestMemoryTeamValidation(t *testing.T) {}
```

### Scénarios E2E Chrome

Fichier : `tests/e2e/memory-phase6.md`

- Scénario SOLO avec 1 équipe
- Scénario CHACUN_SON_TOUR avec 3 équipes
- Scénario TANT_QUE_JE_GAGNE avec 2 équipes
- Validation du changement d'équipe
- Validation du calcul des points par équipe

## Validation Fonctionnelle

Points clés à valider :

1. Sélection des équipes en mode Prepare
2. Badge "Équipe courante" s'affiche correctement
3. Cartes non-cliquables pour équipes en attente
4. Changement d'équipe aux moments corrects
5. Scores par équipe augmentent correctement
6. Historique MEMORY_COMPLETED inclut les résultats multi-équipes
7. Compatibilité rétroactive : Questions sans MEMORY_MODE = SOLO

## Dépendances

- Backend → Frontend (nouvelles actions WS, champs GameState)
- Frontend dépend du backend pour les contrats finalisés

## Timeline estimée

- Backend : 2h
- Frontend : 1.5h
- Tests : 1h
- Revue : 1h
- QA : 1.5h
- Documentation : 30min
- Déploiement : 30min

**Total estimé : 8h**

