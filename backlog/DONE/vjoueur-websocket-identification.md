# VJoueur - Identification WebSocket

**Statut** : ✅ Complété (v2.47.0)

## Description

Vérifier et corriger l'identification des VJoueurs lors de leur connexion WebSocket. Les VJoueurs doivent s'identifier correctement en tant que clients virtuels (pas admin, pas tv).

## Contexte

Les VJoueurs utilisent la WebSocket `/ws` pour communiquer avec le serveur. Il faut vérifier :
1. Qu'ils envoient bien une action d'identification spécifique
2. Que le serveur les comptabilise correctement (séparément des admins et TV)
3. Que le type de client est bien `vplayer` et non `admin`

## Objectifs

- [ ] Vérifier le flux d'identification actuel des VJoueurs
- [ ] S'assurer que les VJoueurs ne sont pas comptés comme "admin"
- [ ] Ajouter un type de client `vplayer` si nécessaire

## Investigation

### Questions à vérifier

1. **Connexion WebSocket VJoueur** :
   - Quelle action est envoyée à la connexion ?
   - Le VJoueur envoie-t-il `SET_CLIENT_TYPE` ?
   - Avec quelle valeur ? (`admin`, `tv`, ou autre ?)

2. **Côté serveur** :
   - Comment `websocket.go` gère-t-il les connexions VJoueur ?
   - Y a-t-il un type `vplayer` distinct ?
   - Les VJoueurs sont-ils inclus dans `ADMIN_COUNT` ou `TV_COUNT` ?

3. **Côté frontend** :
   - `VPlayerPage.jsx` envoie-t-il `SET_CLIENT_TYPE` ?
   - `useWebSocket.js` a-t-il une logique spécifique pour VJoueur ?

### Fichiers à examiner

| Fichier | Vérification |
|---------|--------------|
| `web/src/pages/VPlayerPage.jsx` | Action envoyée à la connexion |
| `web/src/hooks/useWebSocket.js` | Gestion du type de client |
| `internal/server/websocket.go` | Comptage des types de clients |
| `cmd/server/main.go` | Handler `SET_CLIENT_TYPE` |

## Tâches

### Phase 1 - Investigation
- [ ] Lire le code de connexion WebSocket VJoueur
- [ ] Tracer le flux d'identification complet
- [ ] Documenter le comportement actuel

### Phase 2 - Correction (si nécessaire)
- [ ] Ajouter type de client `vplayer` dans `websocket.go`
- [ ] Modifier `VPlayerPage.jsx` pour envoyer `SET_CLIENT_TYPE: vplayer`
- [ ] Ajouter compteur `VPLAYER_COUNT` dans action `CLIENTS`
- [ ] Mettre à jour l'affichage admin si nécessaire

## Comportement attendu

```
VJoueur se connecte à /ws
  ↓
Envoie: SET_CLIENT_TYPE { TYPE: "vplayer" }
  ↓
Serveur: Incrémente vplayerCount
  ↓
Broadcast: CLIENTS { ADMIN_COUNT: 1, TV_COUNT: 1, VPLAYER_COUNT: 3 }
```

## Version cible

v2.47.0 (ou correction dans la prochaine release)
