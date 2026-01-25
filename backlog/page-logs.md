# Page Logs Serveur (/logs)

**Statut** : 📋 Planifié

## Concept

Page web affichant les logs du serveur en temps réel, accessible via `/logs`. Utile pour le debug et le monitoring sans avoir accès à la console serveur.

---

## Spécifications

### Route

| Route | Composant | Description |
|-------|-----------|-------------|
| `/logs` | `LogsPage` | Affichage des logs serveur en temps réel |

### Fonctionnalités

- [ ] **Affichage temps réel**
  - Logs streamés via WebSocket
  - Auto-scroll vers le bas (désactivable)
  - Limite d'affichage : 1000 lignes (configurable)

- [ ] **Filtrage**
  - Par niveau : DEBUG, INFO, WARN, ERROR
  - Par composant : Engine, HTTP, WebSocket, TCP, UDP
  - Recherche textuelle (filtre local)

- [ ] **Actions**
  - Pause/Resume du stream
  - Effacer l'affichage
  - Télécharger les logs visibles (.txt)
  - Copier une ligne au clic

- [ ] **Formatage**
  - Coloration syntaxique par niveau
  - Timestamp lisible
  - Composant en badge coloré

### Maquette

```
┌────────────────────────────────────────────────────────────┐
│ 📋 Logs Serveur                           [⏸ Pause] [🗑️]  │
├────────────────────────────────────────────────────────────┤
│ Niveau: [x]DEBUG [x]INFO [x]WARN [x]ERROR                  │
│ Composant: [x]All [x]Engine [x]HTTP [x]WS [x]TCP           │
│ Recherche: [________________________] 🔍                    │
├────────────────────────────────────────────────────────────┤
│ 10:24:01.234 [INFO]  [Engine] Game started with delay 30   │
│ 10:24:01.456 [DEBUG] [TCP]    Bumper b1 connected          │
│ 10:24:02.789 [INFO]  [Engine] Button press: b1, team=red   │
│ 10:24:03.012 [WARN]  [WS]     Client disconnected          │
│ 10:24:05.345 [ERROR] [HTTP]   Failed to parse request      │
│ ...                                                         │
│                                                    [v Auto] │
└────────────────────────────────────────────────────────────┘
```

### Couleurs par niveau

| Niveau | Couleur | Badge |
|--------|---------|-------|
| DEBUG | Gris | `#6b7280` |
| INFO | Bleu | `#3b82f6` |
| WARN | Orange | `#f59e0b` |
| ERROR | Rouge | `#ef4444` |

### Couleurs par composant

| Composant | Couleur |
|-----------|---------|
| Engine | Violet |
| HTTP | Vert |
| WebSocket | Cyan |
| TCP | Jaune |
| UDP | Orange |

---

## Implémentation Backend

### Action WebSocket

| Action | Direction | Description |
|--------|-----------|-------------|
| `SUBSCRIBE_LOGS` | Client→Server | S'abonner aux logs |
| `UNSUBSCRIBE_LOGS` | Client→Server | Se désabonner |
| `LOG_ENTRY` | Server→Client | Nouvelle entrée de log |

**Payload LOG_ENTRY :**
```json
{
  "ACTION": "LOG_ENTRY",
  "MSG": {
    "TIMESTAMP": 1706234567890,
    "LEVEL": "INFO",
    "COMPONENT": "Engine",
    "MESSAGE": "Game started with delay 30"
  }
}
```

### Système de logging Go

- [ ] **Ring buffer** pour stocker les N derniers logs (défaut: 1000)
- [ ] **Broadcast** aux clients abonnés
- [ ] **Historique initial** : envoyer les 100 derniers logs à la connexion

```go
type LogEntry struct {
    Timestamp int64  `json:"TIMESTAMP"`
    Level     string `json:"LEVEL"`
    Component string `json:"COMPONENT"`
    Message   string `json:"MESSAGE"`
}

type LogBuffer struct {
    entries []LogEntry
    maxSize int
    mu      sync.RWMutex
}

func (lb *LogBuffer) Add(entry LogEntry) {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    if len(lb.entries) >= lb.maxSize {
        lb.entries = lb.entries[1:]
    }
    lb.entries = append(lb.entries, entry)

    // Broadcast to subscribers
    broadcastLogEntry(entry)
}
```

---

## Implémentation Frontend

### Composants

| Composant | Fichier | Description |
|-----------|---------|-------------|
| `LogsPage` | `pages/LogsPage.jsx` | Page principale |
| `LogEntry` | `components/LogEntry.jsx` | Ligne de log formatée |
| `LogFilters` | `components/LogFilters.jsx` | Barre de filtres |

### État React

```javascript
const [logs, setLogs] = useState([])
const [paused, setPaused] = useState(false)
const [autoScroll, setAutoScroll] = useState(true)
const [filters, setFilters] = useState({
  levels: ['DEBUG', 'INFO', 'WARN', 'ERROR'],
  components: ['Engine', 'HTTP', 'WebSocket', 'TCP', 'UDP'],
  search: ''
})
```

### Gestion mémoire

- Limite locale : 1000 entrées affichées
- Suppression FIFO quand limite atteinte
- Virtualisation si performance nécessaire (react-window)

---

## Sécurité

- [ ] **Accès restreint** : Route accessible uniquement depuis réseau local
- [ ] **Pas de données sensibles** : Ne jamais logger mots de passe, tokens
- [ ] **Rate limiting** : Max 100 logs/seconde broadcastés

---

## Configuration

```json
{
  "logging": {
    "buffer_size": 1000,
    "broadcast_enabled": true,
    "min_level": "DEBUG"
  }
}
```

---

## Priorité

**Basse** - Feature de debug/monitoring, pas critique pour le gameplay.

À implémenter après les features principales (VJoueur, QCM interactif, etc.).
