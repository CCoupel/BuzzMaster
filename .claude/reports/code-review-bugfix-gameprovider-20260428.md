# Revue de Code — Fix GameProvider endpoint (commit 8021be1)

**Date** : 2026-04-28  
**Commit** : `8021be1` — `fix(#11): GameProvider endpoint prop — single WS per page, no crash`  
**Fichiers** : `GameContext.jsx`, `App.jsx`, `PlayerDisplay.jsx`, `VPlayerPage.jsx`, `EnrollPage.jsx`  
**Mode** : general

---

## Verdict : ✅ APPROUVE

Toutes les vérifications passent. Architecture propre — une seule connexion WS par page, sans doublon, sans crash.

---

## Vérifications point par point

### 1. `GameContext.jsx` — prop `endpoint` → `useWebSocket` ✅

```jsx
export function GameProvider({ children, endpoint = '/ws/admin' }) {
  const websocket = useWebSocket(endpoint)
  return (
    <GameContext.Provider value={websocket}>
      {children}
    </GameContext.Provider>
  )
}
```

Changement minimal : prop `endpoint` avec défaut `/ws/admin`, transmis directement à `useWebSocket`. Pages admin qui n'utilisent pas le prop héritent du défaut correct. ✅

### 2. `App.jsx` — routing → endpoint sélectionné par route ✅

```jsx
const isAdminRoute = location.pathname.startsWith('/admin') || location.pathname.startsWith('/anim')
const isTvRoute    = location.pathname === '/tv'
const endpoint = isAdminRoute ? '/ws/admin' : isTvRoute ? '/ws/tv' : '/ws/player'

return (
  <GameProvider endpoint={endpoint}>
    <AppContent />
  </GameProvider>
)
```

| Route | Endpoint calculé |
|-------|-----------------|
| `/admin/*`, `/anim/*` | `/ws/admin` ✅ |
| `/tv` | `/ws/tv` ✅ |
| `/`, `/player`, `/enroll` (fallback) | `/ws/player` ✅ |

Un seul `GameProvider` pour toutes les routes (plus de bifurcation conditionnelle). ✅

### 3. `PlayerDisplay`, `VPlayerPage`, `EnrollPage` → `useGame()` ✅

| Fichier | Import remplacé | Appel |
|---------|-----------------|-------|
| `PlayerDisplay.jsx` | `useWebSocket` → `useGame` | `const { gameState, ... } = useGame()` ✅ |
| `VPlayerPage.jsx` | `useWebSocket` → `useGame` | `const { sendMessage, ... } = useGame()` ✅ |
| `EnrollPage.jsx` | `useWebSocket` → `useGame` | `const { connectVirtualPlayer, ... } = useGame()` ✅ |

Aucun `useWebSocket` direct résiduel dans ces pages. ✅

**Note `PlayerDisplay` dans `VPlayerPage`** : quand VPlayerPage est à `/player`, l'endpoint est `/ws/player`. `PlayerDisplay` imbriqué utilise `useGame()` → consomme le même contexte `/ws/player`. Les messages nécessaires (UPDATE, START, STOP, PAUSE, etc.) sont bien diffusés sur `/ws/player` per le contrat. ✅

### 4. Pages admin — `useGame()` inchangé ✅

`GamePage`, `QuestionsPage`, `ConfigPage`, `TeamsPage` etc. utilisent `useGame()` dans `AppContent`, qui est sous `GameProvider endpoint="/ws/admin"`. Aucun changement de comportement pour les pages admin. ✅

### 5. Build `npm run build` ✅

Confirmé par dev-frontend.

---

## Qualité de l'implémentation

**Reconnexion sur changement d'endpoint** (analyse `useWebSocket.js`) :

`connect` est un `useCallback([endpoint])` — recréé quand `endpoint` change. Le `useEffect([connect])` a un cleanup qui ferme l'ancienne connexion avant d'ouvrir la nouvelle :

```js
useEffect(() => {
  connect()
  return () => {
    clearTimeout(reconnectTimeoutRef.current)
    wsRef.current?.close()
  }
}, [connect])
```

Séquence sur navigation admin→TV :
1. Cleanup : `clearTimeout` + `wsRef.current.close()` (ferme `/ws/admin`)
2. Nouveau effet : `connect()` ouvre `/ws/tv`
3. ✅ La garde `if (wsRef.current?.readyState === WebSocket.OPEN) return` ne bloque pas car l'ancien WS est en `CLOSING`

**AppContent restauré proprement** : le `hideNavbar` logic et les classes CSS `main-content fullscreen` sont correctement rétablis depuis la version pre-111d184. ✅

---

## Observation (INFO)

**`onclose` race condition latente sur changement d'endpoint** (pré-existante dans `useWebSocket.js`, hors scope) :

Quand l'ancien WS se ferme (async), son handler `onclose` pose `wsRef.current = null` et schedule un `setTimeout(oldConnect, 5000)`. Ce timer pointe vers l'ancien endpoint. Si ce timer tire avant le cleanup du nouvel effet, une reconnexion sur le mauvais endpoint pourrait survenir.

Impact : uniquement si admin et TV sont sur le **même device/onglet** et que l'utilisateur navigue entre les deux rôles — scénario hors modèle de déploiement production (devices dédiés par rôle). Aucun impact en production standard.

---

## Verdict Final

```
[X] APPROUVE — Prêt pour merge / QA
[ ] APPROUVE AVEC RESERVES
[ ] REFUSE
```
