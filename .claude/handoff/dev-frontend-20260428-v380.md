# Handoff — DEV-FRONTEND

**Feature** : v3.8.0 — #11 WS endpoints migration + #54 ACK_PENDING badge
**SHA** : acd2233 (latest on feature/ws-broadcast-ack-v380)

## Ce qui a été fait

### Phase #11 — Migration endpoints WebSocket frontend
- `useWebSocket.js` : ajout paramètre `endpoint = '/ws/admin'`. L'URL WS est construite avec `endpoint` au lieu de `/ws` en dur.
- `GameContext.jsx` : passe explicitement `/ws/admin` à `useWebSocket`.
- `PlayerDisplay.jsx` : remplacé `useGame()` par `useWebSocket('/ws/tv')` — la page TV a sa propre connexion sur le bon endpoint.
- `EnrollPage.jsx` : remplacé `useGame()` par `useWebSocket('/ws/player')`, supprimé l'effet `setClientType('vplayer')`.
- `VPlayerPage.jsx` : remplacé `useGame()` par `useWebSocket('/ws/player')`, supprimé l'effet `setClientType('vplayer')`.
- `App.jsx` : supprimé l'effet qui appelait `setClientType` selon la route (désormais inutile, le type est fixé par l'URL).

### Phase #54 — Badge ACK_PENDING
- `GamePage.jsx` : ajout de `ackPending: bumper.ACK_PENDING === true` dans le mapping des buzzers.
- `TeamCard.jsx` : badge ⚠ icône horloge (inline React, fond `#f59e0b`, tooltip "En attente de confirmation") quand `buzzer.ackPending === true && !isVPlayer && !isVirtual`.
- `TeamsPage.jsx` : même badge dans les 2 sections (buzzers non assignés et buzzers de team).

## Décisions clés

- **Architecture**: Plutôt qu'une refonte du GameContext, chaque page TV/VPlayer instancie son propre `useWebSocket` avec l'endpoint approprié. Cela crée 2 connexions WS quand VPlayerPage est actif (player + tv via PlayerDisplay), ce qui est intentionnel et correct — chaque endpoint reçoit exactement les messages filtrés pour son rôle.
- **Rétrocompatibilité**: La connexion admin via `/ws` (legacy) reste fonctionnelle — le backend maintient l'alias. `setClientType` reste dans le hook mais n'est plus appelé depuis les pages TV/VPlayer.
- **Badges**: Style inline React identique au badge CONNECTED=false (v3.6.8) — icône SVG horloge au lieu de wifi-off.

## Points d'attention

- Le build npm (`npm run build`) a réussi sans erreurs — ✅
- `showQRCode` destructuré dans PlayerDisplay.jsx est la fonction action (non utilisée dans le JSX — seul `gameState.showQRCode` y est référencé). Pas de breaking change.
- Les pages admin (GamePage, ConfigPage, etc.) utilisent `useGame()` via `GameContext`, qui est sur `/ws/admin` — aucun changement fonctionnel pour elles.
- Push OK : `feature/ws-broadcast-ack-v380` (SHAs `6f2782e` et `acd2233`).

## Fichiers modifiés

- `server-go/web/src/hooks/useWebSocket.js`
- `server-go/web/src/hooks/GameContext.jsx`
- `server-go/web/src/pages/PlayerDisplay.jsx`
- `server-go/web/src/pages/EnrollPage.jsx`
- `server-go/web/src/pages/VPlayerPage.jsx`
- `server-go/web/src/App.jsx`
- `server-go/web/src/pages/GamePage.jsx`
- `server-go/web/src/components/TeamCard.jsx`
- `server-go/web/src/pages/TeamsPage.jsx`
