# Index des tests

> Tests de specification ecrits par le test-writer, ranges en lots `<famille>/<theme>/<lot>/` ; statuts tenus par le CDP.
> Le chemin porte l'identite ; cet index ne porte que l'etat : une ligne par lot (chemin termine par `/`),
> plus des lignes fichier pour les exceptions (quarantaine). Lot sans ligne = `feature`.
> Convention : `context/COMMON.md` section 15. Statuts : `feature` | `regression` | `quarantaine`.
> Tags : `smoke`, `critical`, `slow`.

| Chemin | Niveau | Composant | Feature | Statut | Tags |
|--------|--------|-----------|---------|--------|------|
| server-go/cmd/server/identity_binding_246_test.go (+ internal/game/identity_246_test.go) | integration | vplayer, websocket | #246 identité : liaison connexion↔bumper, refus UNBOUND/FOREIGN_ID, cas 1 sans renommage, unicité des noms, DUPLICATE_SESSION (reproductions harness246_identity_server / name_binding) | regression | critical |
| server-go/cmd/server/ardoise_concurrency_246_test.go | integration | websocket, game-engine, admin-game | #246 non-régression « le texte ARDOISE vu par l'admin ne recule jamais » (H1 10 équipes, H6 2 admins + 2 TV) | regression | critical, slow |
| server-go/cmd/server/ardoise_instrumentation_246_test.go | integration | logging, websocket | #246 instrumentation : ARDOISE_CLEARED, SEQ gap/stale, invariant ARDOISE_STATE_DIVERGED, CLIENT_TRACE (allow-list, limites) | feature | |
| server-go/internal/server/logger_file_246_test.go | unit | logging | #246 journalisation fichier : défauts config, niveau, format, rotation, erreur disque | feature | |
| server-go/cmd/server/ardoise_final_246_test.go | integration | websocket, game-engine, admin-game, anim | #246 Batch 2 : ARDOISE_FINAL/ACK, RECEIVED, garde REVEAL, FORCE, REVEAL_REFUSED (désactivé jusqu'à B-STOP : TEST_246_BATCH2=1) | feature | critical |
| server-go/web/src/pages/VPlayerPage.stale-tab.246.test.jsx | unit | vplayer | #246 onglet périmé + DUPLICATE_SESSION côté tablette (reproduction harness246_stale_tab inversée) | regression | critical |
| server-go/web/src/pages/VPlayerPage.binding.246.test.jsx | unit | vplayer | #246 PLAYER_CONNECT à chaque ouverture, mise en attente jusqu'à liaison, pas de liaison par nom (harness246_name_binding inversée), SEQ/REASON | regression | critical |
| server-go/web/src/hooks/useWebSocket.client-trace.246.test.js | unit | websocket, admin-game, anim | #246 CLIENT_TRACE émis par le hook : VISIBILITY, PAGESHOW, FREEZE/RESUME, WS_GAP, ARDOISE_CLEARED_SEEN | feature | |
| server-go/web/src/utils/fnv1a.246.test.js | unit | vplayer | #246 FNV-1a UTF-8 : 6 vecteurs du contrat | feature | |
| server-go/web/src/pages/VPlayerPage.final-ack.246.test.jsx | unit | vplayer | #246 Batch 2 : texte final au STOP, renvoi 1 s, gel du clavier (désactivé jusqu'à F-STOP : TEST_246_BATCH2=1) | feature | critical |
| tests/procedures/ardoise-246.md | manual | vplayer, websocket, logging, admin-game, anim | #246 procédure manuelle (utilisateur uniquement) : 10 tablettes, onglet périmé, double onglet, STOP en pleine frappe, mode avion, logs | feature | |
