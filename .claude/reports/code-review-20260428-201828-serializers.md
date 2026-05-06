# Revue de Code — Whitelist + Sérialiseurs #41

**Date** : 2026-04-28  
**Branche** : `feature/ws-broadcast-ack-v380`  
**Commits** :
- `3268f61` — `fix(#41): correct buzzer action whitelist`
- `387b502` — `feat(#41): implement payload serializers for admin/webclient/buzzer`

**Fichiers analysés** :
- `server-go/internal/server/websocket_buzzer.go`
- `server-go/internal/server/websocket.go`
- `server-go/internal/protocol/messages.go`
- `server-go/cmd/server/main.go`
- `server-go/internal/protocol/messages_test.go`
- `server-go/internal/server/websocket_buzzer_test.go`

---

## Résumé
- Fichiers analysés : 6
- Problèmes trouvés : 3 (2 critiques, 1 majeur)
- Verdict : **CORRECTIONS REQUISES**

---

## Problèmes Critiques

### [CRITIQUE-1] `go test ./...` FAIL — `TestSerializeForWebClient_StripsConfigFromMsg` échoue

- **Fichier** : `server-go/internal/protocol/messages_test.go:435` / `messages.go:395`
- **Description** : La suite complète retourne `FAIL` — un test dans le package `internal/protocol` échoue :

```
--- FAIL: TestSerializeForWebClient_StripsConfigFromMsg (0.00s)
    messages_test.go:445: SerializeForWebClient: 'config' key should be absent from MSG on UPDATE (TV/VPlayer don't need server config)
FAIL    buzzcontrol/internal/protocol    0.011s
```

`SerializeForWebClient()` ne supprime jamais la clé `"config"` du payload MSG — il n'y a aucun `delete(raw, "config")` dans l'implémentation. Le test, écrit dans le commit `f41aac8`, vérifie ce comportement contractuel et échoue.

- **Suggestion** : Ajouter `delete(raw, "config")` dans `SerializeForWebClient()` après le bloc bumpers :

```go
// Dans SerializeForWebClient(), après le bloc bumpers
delete(raw, "config")  // config server-side n'est pas nécessaire pour TV/VPlayer
```

---

### [CRITIQUE-2] Serializers incompatibles avec le format réel de `GetGameJSON()`

- **Fichier** : `server-go/internal/protocol/messages.go:395` (`SerializeForWebClient`) et `:444` (`SerializeForBuzzer`)
- **Description** : Les deux sérialiseurs supposent un format de payload qui ne correspond PAS au JSON produit par `engine.GetGameJSON()` en production.

**Format réel de `GetGameJSON()`** (`GameData` struct) :
```json
{
  "GAME": { "PHASE": "STARTED", "TIME": 1234, ... },
  "teams":   { "MAC_TEAM": { "NAME": "...", "COLOR": [...], ... } },
  "bumpers": { "AA:BB:CC:EE:01": { "NAME": "...", "FIRMWARE_VERSION": "3.8.0", ... } }
}
```

**Ce que les sérialiseurs cherchent** :
```go
// SerializeForWebClient
raw["BUMPERS"].([]interface{})    // clé uppercase + SLICE

// SerializeForBuzzer
full["BUMPERS"].([]interface{})   // clé uppercase + SLICE
full["TEAMS"].([]interface{})     // clé uppercase + SLICE
full["PHASE"], full["TIMER"], full["TIME"]  // champs top-level, hors du nœud "GAME"
```

**Impact en production** :

| Sérialiseur | Assertion qui échoue | Résultat MSG | Impact |
|---|---|---|---|
| `SerializeForWebClient()` | `raw["BUMPERS"]` → nil (clé lowercase "bumpers") | Payload NON strippé, identique à SerializeForAdmin | TV/VPlayer reçoivent FIRMWARE_VERSION, IS_OUTDATED, OTA_STATUS, ACK_PENDING |
| `SerializeForBuzzer()` | Toutes les assertions → nil | `MSG: {}` (vide) | Buzzers reçoivent UPDATE avec état vide |

Preuve d'impact pour `SerializeForBuzzer()` :
```go
// full = {"GAME":{...}, "teams":{...}, "bumpers":{...}}
full["PHASE"] → nil (inexistant au top level — c'est full["GAME"]["PHASE"])
full["BUMPERS"] → nil (la vraie clé est "bumpers", minuscule, et c'est une MAP pas un slice)
full["TEAMS"]   → nil (idem)
// → minimal = {} → msg.Msg = "{}" → UPDATE avec MSG vide envoyé aux buzzers
```

Les tests passent parce que la fixture `buildUpdateMsg()` utilise une forme normalisée (uppercase, slices, PHASE top-level) qui correspond aux sérialiseurs mais pas aux données réelles.

- **Suggestion** : Mettre à jour les sérialiseurs pour utiliser le format réel :

```go
// SerializeForWebClient — bumpers en map lowercase
if bumpers, ok := raw["bumpers"].(map[string]interface{}); ok {
    for _, b := range bumpers {
        if bumper, ok := b.(map[string]interface{}); ok {
            delete(bumper, "FIRMWARE_VERSION")
            delete(bumper, "IS_OUTDATED")
            delete(bumper, "OTA_STATUS")
            delete(bumper, "OTA_PERCENT")
            delete(bumper, "ACK_PENDING")
        }
    }
}
delete(raw, "config")
```

```go
// SerializeForBuzzer — accès via nœud "GAME" + maps lowercase
if game, ok := full["GAME"].(map[string]interface{}); ok {
    for _, key := range []string{"PHASE", "TIME", "CURRENT_TIME"} {
        if v, ok := game[key]; ok {
            minimal[key] = v
        }
    }
}
if bumpers, ok := full["bumpers"].(map[string]interface{}); ok {
    minBumpers := make(map[string]interface{}, len(bumpers))
    for mac, b := range bumpers {
        if bumper, ok := b.(map[string]interface{}); ok {
            minBumper := make(map[string]interface{}, len(buzzerBumperKeys))
            for _, key := range buzzerBumperKeys {
                if v, ok := bumper[key]; ok {
                    minBumper[key] = v
                }
            }
            minBumpers[mac] = minBumper
        }
    }
    minimal["bumpers"] = minBumpers
}
if teams, ok := full["teams"].(map[string]interface{}); ok {
    // idem pour teams
}
```

Mettre à jour la fixture `buildUpdateMsg()` pour refléter le vrai format `GetGameJSON()`.

---

## Problèmes Majeurs

### [MAJEUR-1] Fixture `buildUpdateMsg()` misalignée avec `GetGameJSON()`

- **Fichier** : `server-go/internal/protocol/messages_test.go:231`
- **Description** : La fixture utilise une forme contractuelle (uppercase, slices, PHASE top-level) qui correspond aux sérialiseurs mais pas au payload réel. Les tests passent sur la fixture mais valident un comportement qui n'existe pas en production. Aucun test ne couvre le chemin réel `GetGameJSON()` → sérialiseur → client.

```go
// Fixture (test) — format qui ne correspond pas à GetGameJSON()
"BUMPERS": []interface{}{map[string]interface{}{...}}  // uppercase + slice
"TEAMS":   []interface{}{map[string]interface{}{...}}  // uppercase + slice
"PHASE": "STARTED"  // top-level, hors du nœud GAME

// Réalité — GameData struct json tags
"bumpers": map[string]*Bumper{}  // lowercase + map
"teams":   map[string]*Team{}    // lowercase + map
"GAME": {"PHASE": "STARTED", "TIME": ...}  // PHASE sous GAME
```

- **Suggestion** : Après correction de CRITIQUE-2, mettre à jour `buildUpdateMsg()` pour générer un JSON via `json.Marshal(GameData{...})` ou construire la map avec les clés et types exacts du format runtime.

---

## Points Positifs

### Commit 3268f61 — Whitelist ✅

- Whitelist 12 actions correcte et complète :
  - 8 actions de synchronisation état-jeu (UPDATE, UPDATE_TIMER, START, CONTINUE, STOP, PAUSE, READY, RESET)
  - 4 actions de contrôle (HELLO, LED_SET, OTA_UPDATE, WIFI_CONFIG)
- Rationale documentée en commentaire — buzzer besoin état-jeu pour machine d'état firmware ✅
- `BroadcastRawIfRelevant` : thread-safety correcte — channel send goroutine-safe, whitelist map immutable après init ✅
- Tests `TestBuzzerActionWhitelist`, `TestBroadcastIfRelevant_DropsNonWhitelisted`, `TestBroadcastIfRelevant_AllowsWhitelisted` : complets et correctement vérifiés ✅

### Commit 387b502 — Parties correctes ✅

- `SerializeForAdmin()` : trivial et correct — alias explicite de `SerializeForWebSocket()` ✅
- `BroadcastRawToTypes` : WLock unique couvrant `close(client.Send)` + `delete(h.clients, client)` — pattern identique à `BroadcastToTypes` validé en cycle 2 ✅
- Immutabilité du message original : `out := *m; out.Msg = stripped` — shallow copy propre, `m.Msg` inchangé ✅
- Structure `broadcastUpdate()` à 3 chemins (Admin / TV+VPlayer / Buzzer) : architecture propre et documentée ✅
- Gestion d'erreur défensive sur les sérialiseurs (fallback silencieux sur échec marshal) ✅
- Commit messages propres avec références issues ✅

---

## Verdict Final

```
[ ] APPROUVE — Prêt pour merge / QA
[ ] APPROUVE AVEC RESERVES
[X] CORRECTIONS REQUISES — Voir CRITIQUE-1 et CRITIQUE-2
```

### Corrections requises (bloquantes)

1. **CRITIQUE-1** : Ajouter `delete(raw, "config")` dans `SerializeForWebClient()` — 1 ligne
2. **CRITIQUE-2** : Corriger les assertions de type dans `SerializeForWebClient()` et `SerializeForBuzzer()` pour utiliser les clés lowercase (`"bumpers"`, `"teams"`) et le type `map[string]interface{}` correspondant au format `GetGameJSON()` ; accéder à PHASE/TIME via le nœud `"GAME"` dans `SerializeForBuzzer()`
3. **MAJEUR-1** : Mettre à jour `buildUpdateMsg()` pour refléter le vrai format `GetGameJSON()`

### Périmètre non impacté

Le reste de la feature v3.8.0 (whitelist, BroadcastRawToTypes, BroadcastRawIfRelevant, broadcastUpdate() structure, commits précédents aa2ee6e, 8021be1) reste approuvé.
