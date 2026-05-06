# Revue de Code — Cycle 2 — v3.8.0 (commit aa2ee6e)

**Date** : 2026-04-28  
**Branche** : `feature/ws-broadcast-ack-v380`  
**Commit** : `aa2ee6e` — `fix(v3.8.0): 3 race/cascade/broadcast correctness fixes + 2 cleanups`  
**Rapport précédent** : `.claude/reports/code-review-20260428-v380.md`  
**Scope** : Revue ciblée sur les 5 corrections (3 majeures + 2 mineures)

---

## Verdict : ✅ APPROUVE

Toutes les 5 corrections sont correctement appliquées. Aucun nouveau problème introduit.

---

## Vérification point par point

---

### MAJEUR-1 — BroadcastToTypes : race condition double-close ✅ CORRIGÉ

**Fichier** : `server-go/internal/server/websocket.go`

**Avant** : `RLock` pour la lecture + fermeture de canal, suivi d'un `WLock` séparé pour la suppression de la map. Fenêtre de race entre les deux locks.

**Après** :
```go
h.mu.Lock()  // WLock unique couvrant close + delete
for client := range h.clients {
    if !typeSet[client.Type] { continue }
    select {
    case client.Send <- data:
    default:
        close(client.Send)
        delete(h.clients, client)  // atomic avec le close
    }
}
h.mu.Unlock()
```

**Vérifications** :
- ✅ `RLock` remplacé par `WLock` unique — plus aucune fenêtre entre close et delete
- ✅ `close` + `delete` se font dans la même section critique — pattern cohérent avec `Run()`
- ✅ `delete` pendant un `for range` sur une map : comportement bien défini en Go (safe)
- ✅ Commentaire explicatif ajouté sur la raison du WLock
- ℹ️ Contention légèrement plus élevée (WLock vs RLock) mais négligeable — `BroadcastToTypes` est O(n_clients) et `close` est rarement déclenché (canal plein à 256 items)

---

### MAJEUR-2 — OnRetry : MSG_ID original réutilisé ✅ CORRIGÉ

**Fichier** : `server-go/cmd/server/main.go` — câblage `ackManager.OnRetry`

**Avant** : `a.resendLEDOnReconnect(mac)` → `sendLEDSet()` → nouveau `msgID` → nouvelle entrée AckManager → accumulation AckMaxRetries².

**Après** :
```go
a.ackManager.OnRetry = func(mac, msgID string) {
    if payload, ok := a.bumperLEDState[mac]; ok {
        msg, err := protocol.NewMessage(protocol.ActionLEDSet, payload)
        if err != nil { ... return }
        msg.MsgID = msgID  // msgID original — le buzzer confirmera le bon slot
        a.buzzerHub.SendToClient(mac, msg)
    }
}
```

**Vérifications** :
- ✅ Aucun appel à `sendLEDSet()` ni `resendLEDOnReconnect()` dans le callback
- ✅ `msg.MsgID = msgID` — le msgID original est réutilisé
- ✅ Envoi via `buzzerHub.SendToClient` direct — aucune nouvelle entrée créée dans AckManager
- ✅ Gestion des erreurs ajoutée (LogError + LogWarn)
- ✅ Guard `if payload, ok := a.bumperLEDState[mac]; ok` — silence si payload inconnu (correct)
- ℹ️ Accès à `bumperLEDState` depuis la goroutine AckManager sans mutex : comportement identique à l'ancien `resendLEDOnReconnect` — pré-existant, hors scope de ce fix

---

### MAJEUR-3 — broadcastUpdate() : 1 seul appel post-boucle ✅ CORRIGÉ

**Fichier** : `server-go/cmd/server/main.go` — `sendLEDSet()` + 9 fonctions LED en boucle

**Vérifications** :

| Fonction | `broadcastUpdate()` supprimé de `sendLEDSet` | `broadcastUpdate()` ajouté post-boucle |
|----------|----------------------------------------------|----------------------------------------|
| `sendLEDSet` | ✅ supprimé + commentaire explicatif | N/A — fonction unitaire |
| `sendLEDSetAllBuzzers` | N/A | ✅ ajouté |
| `sendLEDSetStop` | N/A | ✅ ajouté |
| `sendLEDSetPause` | N/A | ✅ ajouté |
| `sendLEDSetPauseAll` | N/A | ✅ délègue à `sendLEDSetPause("")` → hérite du `broadcastUpdate` |
| `sendLEDSetContinue` | N/A | ✅ délègue à `sendLEDSetAllBuzzers()` → hérite du `broadcastUpdate` |
| `sendLEDSetReveal` | N/A | ✅ ajouté |
| `sendLEDSetToTeam` | N/A | ✅ ajouté |
| `sendLEDSetComet` | N/A | ✅ ajouté |
| `broadcastLEDSet` | N/A | ✅ ajouté |

**Observation clé** : `sendLEDSetPauseAll` et `sendLEDSetContinue` sont des wrappers délégants — elles héritent correctement du `broadcastUpdate` de leur fonction délégataire. La chaîne est complète. ✅

Le commentaire dans `sendLEDSet` est explicite : `// NOTE: broadcastUpdate() is intentionally NOT called here...` — réduction documentée.

---

### MINEUR-1 — Duplication `generateMsgID` ✅ CORRIGÉ

**Fichier** : `server-go/cmd/server/main.go`

**Vérifications** :
- ✅ Fonction locale `generateMsgID()` supprimée (diff confirme le `-` sur la fonction)
- ✅ Import `math/rand` supprimé (devenu inutile)
- ✅ Tous les call sites utilisent `server.GenerateMsgID()` : `sendLEDSet`, `sendWifiConfigToBuzzer`, `broadcastWifiConfig` (3 occurrences)

---

### MINEUR-2 — Context annulable pour AckManager ✅ CORRIGÉ

**Fichier** : `server-go/cmd/server/main.go`

**Vérifications** :
- ✅ `App` enrichi de `ctx context.Context` et `cancelCtx context.CancelFunc`
- ✅ Créés dans `main()` via `context.WithCancel(context.Background())`
- ✅ `go a.ackManager.Start(a.ctx)` — goroutine annulable à l'arrêt
- ✅ `a.cancelCtx()` appelé en premier dans `stop()` avant les autres `Stop()` — ordre correct
- ✅ Guard `if a.cancelCtx != nil` — safe même si `stop()` est appelé avant `start()`

---

### Bonus non demandés ✅

Deux corrections supplémentaires bienvenues dans le même commit :

1. **Nil guard dans `broadcastUpdate()`** :
   ```go
   if a.wsHub == nil {
       return // Guard for unit tests that construct a minimal App without a wsHub
   }
   ```
   Permet aux tests unitaires utilisant un `App` minimal de ne pas paniquer. Défensive, propre.

2. **Fix TestReleasesCache** (`github_client_test.go`) : TTL 100 ns → 1 minute pour un test de cache. Élimine un test flaky pré-existant sur les machines lentes.

---

## Résumé des corrections

| Réserve | Statut | Qualité de la correction |
|---------|--------|--------------------------|
| MAJEUR-1 — race `BroadcastToTypes` | ✅ Corrigé | Exacte — pattern identique à `Run()` |
| MAJEUR-2 — `OnRetry` cascade MSG_ID | ✅ Corrigé | Exacte — msgID original préservé |
| MAJEUR-3 — N `broadcastUpdate` | ✅ Corrigé | Complète — 9/9 fonctions + chaîne de délégation |
| MINEUR-1 — `generateMsgID` dupliquée | ✅ Corrigé | Exacte — import nettoyé également |
| MINEUR-2 — `context.Background()` | ✅ Corrigé | Propre — ctx en champ d'App avec cancelCtx |

---

## Verdict Final

```
[X] APPROUVE — Prêt pour merge / QA
[ ] APPROUVE AVEC RESERVES
[ ] REFUSE
```

Toutes les réserves du cycle 1 sont levées. Aucun nouveau problème détecté dans ce commit. Le code est prêt pour la phase QA.
