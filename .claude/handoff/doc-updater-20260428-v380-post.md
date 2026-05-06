# Handoff — DOC-UPDATER v3.8.0 Post-Documentation Update

**Date**: 2026-04-28  
**Branch**: `feature/ws-broadcast-ack-v380`  
**Status**: ✅ DONE

---

## Summary

Documentation augmented to reflect 4 post-initial commits:
- `3268f61` — whitelist buzzer extension (4 → 12 actions)
- `387b502` + `eb62458` — 3 payload serializers (ForAdmin/ForWebClient/ForBuzzer)
- `8021be1` — GameProvider endpoint prop (frontend)

---

## Commit

| SHA | Message |
|-----|---------|
| `ef751fc` | `docs(v3.8.0): add serializers, whitelist extension, GameProvider endpoint` |

---

## Files Modified

### CHANGELOG.md — [3.8.0] section

**Added subsection** — NEW content:
- Sérialiseurs payload différenciés (`SerializeForAdmin()` full, `SerializeForWebClient()` réduit, `SerializeForBuzzer()` minimal)
- GameProvider prop `endpoint` pour routing multi-rôle (v3.8.0)

**Changed subsection** — UPDATED:
- Whitelist buzzer extended: 4 actions → 12 actions (UPDATE, UPDATE_TIMER, START, CONTINUE, STOP, PAUSE, READY, RESET, HELLO, LED_SET, OTA_UPDATE, WIFI_CONFIG)
- Routing table now specifies serialization: Admin (full) vs TV/VPlayer (web) vs Buzzer (buzzer)
- `GameProvider` added to frontend notes

### CLAUDE.md

**Section "WebSocket Endpoints dédiés (v3.8.0)"** — EXPANDED:
- Added GameProvider endpoint prop documentation
- App.jsx routing: `/admin/*` (default), `/tv` → `/ws/tv`, `/player`/`/enroll` → `/ws/player`
- Future evolution note (multiple GameProvider instances with different endpoints)

**Architecture block** — UPDATED:
- Whitelist expanded: 4 → 12 actions
- Serialization notes added per endpoint (SerializeForAdmin/WebClient/Buzzer)

**Table de routage** — UPDATED:
- Added column for serialization type (full/web/buzzer)
- Actions now grouped with serialization hints
- UPDATE_TIMER added to routing

**NEW Section "Payload Serializers — Client-Specific Reduction (v3.8.0)"** — CREATED:
- Detailed explanation of 3 serializers (go code examples)
- Allocation table: which serializer per endpoint
- Broadcast implementation note (BroadcastRawToTypes in main.go)
- Volume reduction estimates (~40-60% for TV/VPlayer)
- Fichiers cles listing (messages.go, websocket.go, main.go)

**Section "Key Files"** — UPDATED:
- WebSocket Buzzer: whitelist extended (12 actions), BroadcastIfRelevant() noted
- WebSocket (clients web): added BroadcastRawToTypes(), serializers listed
- http.go: routes expanded (5 WebSocket routes)
- internal/protocol/messages.go: added 3 serializers + SerializeForWebSocket/Serialize
- Frontend hooks: GameContext.jsx added with endpoint prop routing details

---

## Design Decisions

1. **Table layout for routing** — easier to see all 3 serializer variants vs admin-only format
2. **UPDATE_TIMER inclusion** — important for buzzer state display (game timer sync)
3. **SerializeForWebClient reduction** — TV/VPlayer don't need firmware/OTA/ACK state (reduces volume)
4. **GameProvider endpoint prop** — enables future multi-instance scenarios (TV + admin simultaneous)
5. **BroadcastRawToTypes** — centralized serialization dispatch vs per-message logic

---

## Notes for Next Agent

- Documentation now fully reflects the 4 post-initial commits
- Backward compat preserved: GameProvider default endpoint=`/ws/admin`
- Serializers are internal (protocol/messages.go) — no external contract files needed
- Whitelist extension (4→12) significantly broadens buzzer state visibility (game progress now visible on physical buzzers)
- TV/VPlayer payload reduction improves browser performance (fewer data updates)

---

## Verification Checklist

- ✅ CHANGELOG.md [3.8.0] Added: serializers + GameProvider endpoint
- ✅ CHANGELOG.md [3.8.0] Changed: whitelist extended 4→12, serialization notes
- ✅ CLAUDE.md WebSocket Endpoints section: GameProvider endpoint prop routing documented
- ✅ CLAUDE.md Whitelist: 4→12 actions (UPDATE, UPDATE_TIMER, START, CONTINUE, STOP, PAUSE, READY, RESET, HELLO, LED_SET, OTA_UPDATE, WIFI_CONFIG)
- ✅ CLAUDE.md NEW section Payload Serializers: full explanation + 3 serializer types
- ✅ CLAUDE.md Key Files: serializers, BroadcastRawToTypes, GameContext.jsx endpoint routing
- ✅ Backward compat notes present (GameProvider default, Message.MsgID omitempty, etc.)
- ✅ Commit created on feature/ws-broadcast-ack-v380
