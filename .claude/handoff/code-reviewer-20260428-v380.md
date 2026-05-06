# Handoff — CODE-REVIEWER

**Feature** : v3.8.0 — #11 WS endpoints dédiés + #41 payload buzzers + #54 ACK buzzer
**SHA** : N/A (phase review, pas de code modifié)

## Ce qui a été fait

Revue complète de 29 fichiers (1 824 insertions) sur `feature/ws-broadcast-ack-v380`.
Lecture des 5 handoffs (planner, backend, firmware, frontend, test-writer) et des 3 contrats API.
Analyse du diff Git commit par commit (8f9b665 → 044d74f).
Vérification de conformité à la table de filtrage des 25 actions du contrat websocket-endpoints.md.

## Décisions clés

Verdict **APPROUVE AVEC RESERVES** : qualité globale bonne, backward compat exemplaire, tests solides. 3 corrections majeures identifiées avant merge.

## Points d'attention

### Pour le dev-backend (corrections requises)

**MAJEUR-1** — `BroadcastToTypes` dans `websocket.go` : race condition latente.
- `close(client.Send)` appelé sous `RLock` → si deux goroutines concurrentes (ex: ticker + handler), double-close → panic.
- Fix : effectuer close + delete sous `WLock` unique, comme dans `Run()`.

**MAJEUR-2** — `OnRetry` câblage dans `main.go:init()`.
- Appelle `resendLEDOnReconnect()` → `sendLEDSet()` → nouveau `msgID` → nouvelle entrée `pendingAcks`.
- L'entrée originale ne peut jamais être confirmée, elle expire en accumulant des retries.
- Fix : dans `OnRetry`, resend avec le `msgID` original (stocker payload dans `AckEntry` ou `bumperLEDState`).

**MAJEUR-3** — `sendLEDSet()` appelle `broadcastUpdate()` pour chaque buzzer.
- `sendLEDSetAllBuzzers()` → N × `broadcastUpdate()` au lieu de 1 seul.
- Fix : supprimer `broadcastUpdate()` de `sendLEDSet()`, l'appeler une fois après la boucle dans chaque caller.

### Pour QA (informations utiles)

- Test `-race` recommandé sur `BroadcastToTypes` concurrent (non couvert par tests actuels).
- Le bug MAJEUR-3 peut être observé dans les logs : N lignes "broadcastUpdate" successives lors d'un START/STOP.
- Le bug MAJEUR-2 se voit dans les logs : "AckManager: retry 1/3" répété pour le même MAC + nouvelles registrations.

## Fichiers modifiés

Aucun (phase review).
