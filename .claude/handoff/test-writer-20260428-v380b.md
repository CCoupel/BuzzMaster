# Handoff — TEST-WRITER (pass 2)

**Feature** : v3.8.0 #41 — Sérialiseurs payload WS (SerializeForAdmin/WebClient/Buzzer) + BroadcastRaw*
**SHA** : f41aac8

## Ce qui a été fait

Ajout de 20 tests Go additifs couvrant les 3 sérialiseurs de payload (messages_test.go) et les 2 méthodes BroadcastRaw (websocket_test.go, websocket_buzzer_test.go). Tests écrits depuis le contrat `contracts/ws-payload-serialization.md` et l'implémentation des commits 387b502 + 3268f61.

## Décisions clés

1. **`config` absent dans SerializeForWebClient** : Le contrat (`contracts/ws-payload-serialization.md`) indique que `config` doit être absent du payload TV/VPlayer. L'implémentation (387b502) NE strip PAS `config`. Le test `TestSerializeForWebClient_StripsConfigFromMsg` reflète le contrat — si ce test échoue en QA, c'est un bug backend à corriger.

2. **Fixture `buildUpdateMsg`** : Helper partagé qui construit un UPDATE réaliste avec BUMPERS (FIRMWARE_VERSION, ACK_PENDING, etc.) et TEAMS (MEMBERS). Évite la duplication entre les tests.

3. **Immutabilité de la source** : Tests `SourceImmutable` pour SerializeForWebClient et SerializeForBuzzer — critique car les deux méthodes unmarshal+remarshal le payload et pourraient muter le msg.Msg si l'implémentation faisait une modification in-place.

4. **BroadcastRawToTypes vs BroadcastToTypes** : Les tests de BroadcastRawToTypes sont similaires à BroadcastToTypes mais utilisent `admin.ReadMessage()` directement (bytes bruts) au lieu de `readWSMsg()` — vérifient que les bytes sont transmis sans re-sérialisation.

5. **BroadcastRawIfRelevant** (buzzer hub) : Placé dans `websocket_buzzer_test.go` à côté des tests `BroadcastIfRelevant` existants. Couvre la liste complète des 12 actions whitelistées et les actions bloquées.

## Points d'attention pour QA

- **Test `TestSerializeForWebClient_StripsConfigFromMsg`** : Susceptible de FAIL car l'implémentation ne strip pas `config`. À investiguer avec le backend dev.
- **Tests d'immutabilité** : Nécessitent que SerializeForWebClient et SerializeForBuzzer créent une copie de `msg.Msg` avant modification. L'implémentation actuelle crée une copie via `out := *m; out.Msg = stripped` — OK.
- **Race detector** : Lancer `go test -race ./internal/protocol/ ./internal/server/` pour valider la sûreté concurrente.

## Fichiers modifiés

- `server-go/internal/protocol/messages_test.go` — +14 tests (3 sérialiseurs)
- `server-go/internal/server/websocket_test.go` — +4 tests (BroadcastRawToTypes)
- `server-go/internal/server/websocket_buzzer_test.go` — +2 tests (BroadcastRawIfRelevant)
- `.claude/handoff/test-writer-20260428-v380b.md` — ce fichier
