# Filtrage des broadcasts WebSocket par type de client

**Statut** : 📋 Planifié

## Description

Actuellement, tous les messages WebSocket sont envoyés à tous les clients connectés (admin, TV, VJoueur) sans distinction. Cette amélioration ajoute un filtrage intelligent pour n'envoyer que les messages pertinents à chaque type de client.

## Contexte

### Situation actuelle

```go
// Broadcast aveugle à tous les clients
for _, client := range a.wsClients {
    client.Send(msg)  // Tous reçoivent tout
}
```

### Problème

| Message | Admin | TV | VJoueur | Actuellement |
|---------|:-----:|:--:|:-------:|--------------|
| `UPDATE` | ✅ | ✅ | ✅ | Tous ✅ |
| `QUESTIONS` | ✅ | ❌ | ❌ | Tous ❌ |
| `CLIENTS` | ✅ | ❌ | ❌ | Tous ❌ |
| `BACKGROUND_CHANGE` | ❌ | ✅ | ✅ | Tous ❌ |
| `ENROLLMENT_UPDATE` | ✅ | ✅ | ❌ | Tous ❌ |

Les clients reçoivent des messages qu'ils n'utilisent pas → bande passante gaspillée.

## Objectifs

- [ ] Définir les types de clients (Admin, TV, VJoueur)
- [ ] Enrichir la structure WSClient avec le type
- [ ] Créer une fonction de broadcast avec filtre
- [ ] Mapper chaque action WebSocket à ses destinataires

## Solution proposée

### Structure client enrichie

```go
type ClientType string

const (
    ClientAdmin  ClientType = "admin"
    ClientTV     ClientType = "tv"
    ClientPlayer ClientType = "player"
)

type WSClient struct {
    Conn *websocket.Conn
    Type ClientType
}
```

### Fonction de broadcast filtré

```go
func (a *App) broadcast(msg Message, targets ...ClientType) {
    for _, client := range a.wsClients {
        if len(targets) == 0 || contains(targets, client.Type) {
            client.Send(msg)
        }
    }
}
```

### Mapping des messages

| Message | Cibles | Appel |
|---------|--------|-------|
| `UPDATE` | Tous | `broadcast(msg)` |
| `QUESTIONS` | Admin | `broadcast(msg, ClientAdmin)` |
| `CLIENTS` | Admin | `broadcast(msg, ClientAdmin)` |
| `BACKGROUND_CHANGE` | TV, VJoueur | `broadcast(msg, ClientTV, ClientPlayer)` |
| `QCM_HINT` | Tous | `broadcast(msg)` |
| `CONFIG_UPDATE` | Tous | `broadcast(msg)` |
| `ENROLLMENT_UPDATE` | Admin, TV | `broadcast(msg, ClientAdmin, ClientTV)` |

## Tâches

### Phase 1 - Backend

- [ ] Ajouter `ClientType` dans `internal/server/websocket.go`
- [ ] Modifier `WSClient` pour inclure le type
- [ ] Parser `SET_CLIENT_TYPE` pour définir le type (existant, à exploiter)
- [ ] Créer `broadcastFiltered(msg, targets...)` dans `main.go`
- [ ] Remplacer les appels `broadcast()` par `broadcastFiltered()` avec les bons filtres

### Phase 2 - Frontend (optionnel)

- [ ] Ajouter `ClientPlayer` pour les VJoueurs (actuellement non typés)
- [ ] Envoyer `SET_CLIENT_TYPE` depuis VPlayerPage

### Phase 3 - Tests

- [ ] Test unitaire : broadcast sans filtre → tous reçoivent
- [ ] Test unitaire : broadcast avec filtre → seuls les ciblés reçoivent
- [ ] Test E2E : vérifier qu'un VJoueur ne reçoit pas `QUESTIONS`

## Fichiers concernés

| Fichier | Modification |
|---------|--------------|
| `internal/server/websocket.go` | Ajouter `ClientType`, modifier `WSClient` |
| `cmd/server/main.go` | `broadcastFiltered()`, mise à jour des appels |
| `web/src/pages/VPlayerPage.jsx` | Envoyer `SET_CLIENT_TYPE: "player"` |
| `web/src/hooks/useWebSocket.js` | Ajouter type "player" pour VJoueur |

## Avantages

- Réduction du trafic WebSocket inutile
- Code plus explicite (on sait qui reçoit quoi)
- Préparation pour d'éventuelles restrictions de sécurité
- Pas de refactoring majeur (amélioration incrémentale)

## Version cible

v2.47.0
