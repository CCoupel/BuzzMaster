# Page Logs (/logs)

**Statut** : ✅ Implémenté v2.43.0 (WebSocket dédiée)

## Description

Une page `/logs` dans l'interface admin pour afficher les logs du serveur en temps réel. Cette page permet à l'animateur et aux administrateurs de surveiller l'activité du serveur, diagnostiquer les problèmes et comprendre le flux des événements.

## Objectifs

- [x] Afficher les logs du serveur Go en temps réel via WebSocket
- [x] Filtrer les logs par niveau (DEBUG, INFO, WARN, ERROR)
- [x] Filtrer les logs par composant (Engine, HTTP, WebSocket, TCP, UDP, App)
- [x] Permettre la recherche dans les logs
- [x] Auto-scroll avec pause au survol
- [x] Export des logs visibles

## Architecture

### Backend (Go)

Le serveur Go doit broadcaster les logs vers les clients WebSocket connectés.

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────────┐
│  Logger Go      │ ──► │  Log Buffer  │ ──► │  WebSocket      │
│  (CustomLogger) │     │  (ring 1000) │     │  Broadcast      │
└─────────────────┘     └──────────────┘     └─────────────────┘
                                                     │
                                                     ▼
                                             ┌─────────────────┐
                                             │  /admin/logs    │
                                             │  (React client) │
                                             └─────────────────┘
```

### Frontend (React)

Page admin avec affichage des logs en temps réel.

```
┌────────────────────────────────────────────────────────────┐
│ 🔍 [Recherche...        ]  [DEBUG] [INFO] [WARN] [ERROR]   │
│ Composant: [Tous ▼]        [ ] Auto-scroll   [Exporter]    │
├────────────────────────────────────────────────────────────┤
│ 22:15:03.123 INFO  [Engine]   Game started, delay=30s      │
│ 22:15:03.456 DEBUG [WebSocket] Client connected: admin_1   │
│ 22:15:05.789 INFO  [TCP]      Bumper AA:BB:CC:DD connected │
│ 22:15:06.012 WARN  [Engine]   Bumper not found: XX:YY:ZZ   │
│ 22:15:10.345 INFO  [Engine]   Button press: AA:BB:CC:DD    │
│ 22:15:10.567 DEBUG [Engine]   Processing buzz, time=342ms  │
│ ...                                                         │
│                                                             │
│                                                             │
└────────────────────────────────────────────────────────────┘
```

## Tâches

### Phase 1 - Backend (v2.42.0)

- [x] **LogBuffer** : Buffer circulaire pour stocker les derniers 1000 logs
  - Struct `LogEntry` : Timestamp, Level, Component, Message
  - Thread-safe avec mutex
  - Méthode `GetRecent(n int)` pour récupérer les n derniers logs

- [x] **LogBroadcaster** : Broadcast des logs vers les clients WebSocket
  - Canal Go pour recevoir les nouveaux logs
  - Action WebSocket `LOG_ENTRY` pour envoyer un log
  - Action WebSocket `LOG_HISTORY` pour envoyer l'historique initial

- [x] **Intégration CustomLogger** : Connecter le logger existant au buffer
  - Hook pour capturer chaque log
  - Parsing du niveau et du composant

- [x] **Action WebSocket SUBSCRIBE_LOGS** : Client demande à recevoir les logs
  - Envoie l'historique récent (tous les logs du buffer)
  - Ajoute le client à la liste des abonnés

- [x] **Action WebSocket UNSUBSCRIBE_LOGS** : Client arrête de recevoir les logs
  - Retire le client de la liste des abonnés

### Phase 2 - Frontend (v2.42.0)

- [x] **LogsPage.jsx** : Page principale d'affichage des logs
  - Route `/admin/logs` et `/anim/logs`
  - Connexion WebSocket pour recevoir les logs
  - État local pour stocker les logs reçus

- [x] **Composant LogEntry** : Affichage d'une ligne de log
  - Couleur selon le niveau (DEBUG=gris, INFO=blanc, WARN=orange, ERROR=rouge)
  - Badge coloré pour le composant
  - Timestamp formaté (HH:MM:SS.mmm)
  - Message avec highlight de la recherche

- [x] **Filtres de niveau** : Boutons toggle pour chaque niveau
  - DEBUG, INFO, WARN, ERROR
  - Filtrage côté client (tous les logs reçus, filtrés à l'affichage)

- [x] **Filtre de composant** : Boutons toggle pour filtrer par composant
  - Options : App, Engine, HTTP, WebSocket, TCP, UDP
  - Sélection multiple

- [x] **Recherche** : Input de recherche temps réel
  - Filtre sur le message du log
  - Highlight des termes trouvés
  - Debounce 300ms

- [x] **Auto-scroll** : Scroll automatique vers le bas
  - Bouton pour activer/désactiver
  - Pause automatique si l'utilisateur scroll manuellement
  - Reprise si scroll en bas
  - Indicateur visuel pour nouveaux logs

- [x] **Export** : Bouton pour exporter les logs visibles
  - Format texte avec timestamp ISO
  - Téléchargement fichier `.log`

### Phase 3 - WebSocket dédiée (v2.43.0) ✅

- [x] **Séparation des WebSockets** : `/ws` pour le jeu, `/ws/logs` pour les logs
  - Endpoint dédié `/ws/logs` (independant de `/ws`)
  - Fichier `logswebsocket.go` pour la gestion des connexions logs
  - Connexion automatique → envoi immédiat de `LOG_HISTORY`
  - Temps réel via `LOG_ENTRY` pour chaque nouveau log
  - Déconnexion = désabonnement automatique (pas d'action UNSUBSCRIBE)

- [x] **Hook dédié** : `useLogsWebSocket()` pour LogsPage
  - Connexion directe à `/ws/logs` au lieu de `/ws`
  - Pas de pollution de la WebSocket principale
  - Meilleure séparation des préoccupations

- [x] **Layout fixe** : Correction du scroll de la page Logs
  - Position fixed sur `.logs-page` (pas de scroll global)
  - Toolbar sticky en haut (z-index: 10)
  - Liste scrollable avec flex: 1

### Phase 4 - Améliorations futures (v2.44+)

- [ ] **Persistence logs** : Option pour sauvegarder les logs sur disque
  - Configuration dans config.json : `logs.persist`, `logs.max_size_mb`
  - Rotation automatique des fichiers

- [ ] **Niveaux de log configurables** : Changer le niveau minimum en temps réel
  - Action WebSocket `SET_LOG_LEVEL`
  - Dropdown dans la page logs

- [ ] **Logs structurés** : Ajouter des métadonnées aux logs
  - ID de requête, ID de bumper, ID de question
  - Filtrage avancé par métadonnée

## Modèle de données

### LogEntry (Backend)

```go
type LogEntry struct {
    Timestamp int64  `json:"timestamp"` // Unix milliseconds
    Level     string `json:"level"`     // DEBUG, INFO, WARN, ERROR
    Component string `json:"component"` // Engine, HTTP, WebSocket, TCP, UDP
    Message   string `json:"message"`   // Log message
}
```

### LogEntry (Frontend)

```typescript
interface LogEntry {
    timestamp: number;  // Unix milliseconds
    level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR';
    component: string;
    message: string;
}
```

## Actions WebSocket

| Action | Direction | Description |
|--------|-----------|-------------|
| `SUBSCRIBE_LOGS` | Client→Server | S'abonner aux logs temps réel |
| `UNSUBSCRIBE_LOGS` | Client→Server | Se désabonner des logs |
| `LOG_HISTORY` | Server→Client | Historique initial (100 derniers) |
| `LOG_ENTRY` | Server→Client | Nouveau log en temps réel |
| `SET_LOG_LEVEL` | Client→Server | Changer le niveau minimum (Phase 3) |

### Payloads

**LOG_HISTORY** :
```json
{
    "ACTION": "LOG_HISTORY",
    "MSG": {
        "entries": [
            {"timestamp": 1706000000000, "level": "INFO", "component": "Engine", "message": "Game started"},
            ...
        ]
    }
}
```

**LOG_ENTRY** :
```json
{
    "ACTION": "LOG_ENTRY",
    "MSG": {
        "timestamp": 1706000000123,
        "level": "DEBUG",
        "component": "WebSocket",
        "message": "Client connected: admin_1"
    }
}
```

## Styles CSS

### Couleurs par niveau

| Niveau | Couleur texte | Couleur badge |
|--------|---------------|---------------|
| DEBUG | `--text-secondary` (gris) | `--gray-600` |
| INFO | `--text-primary` (blanc) | `--primary-500` |
| WARN | `--warning` (orange) | `--warning` |
| ERROR | `--error` (rouge) | `--error` |

### Couleurs par composant

| Composant | Couleur badge |
|-----------|---------------|
| Engine | `--accent-purple` |
| HTTP | `--accent-cyan` |
| WebSocket | `--accent-green` |
| TCP | `--accent-orange` |
| UDP | `--accent-pink` |

## Navbar

Ajouter l'onglet "Logs" dans la navbar admin :

```jsx
{ path: '/admin/logs', label: 'Logs', icon: '📋' }
```

Position : Après "Palmarès", avant "Config"

## Version cible

- **Phase 1-2** : v2.42.0 (fonctionnalité complète de base) ✅
- **Phase 3** : v2.43.0 (WebSocket dédiée + layout fixe) ✅
- **Phase 4** : v2.44+ (améliorations optionnelles)

## Notes techniques

- Le buffer de logs doit être thread-safe (mutex)
- Limiter le nombre de logs côté client (5000 max) pour éviter les problèmes de mémoire
- Utiliser `requestAnimationFrame` pour le scroll auto (performance)
- Les logs sont transmis uniquement aux clients qui ont souscrit (pas de broadcast global)
- Déconnexion WebSocket = désabonnement automatique
