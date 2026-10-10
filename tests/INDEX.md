# Index des tests

> Tests de specification ecrits par le test-writer, ranges en lots `<famille>/<theme>/<lot>/` ; statuts tenus par le CDP.
> Le chemin porte l'identite ; cet index ne porte que l'etat : une ligne par lot (chemin termine par `/`),
> plus des lignes fichier pour les exceptions (quarantaine). Lot sans ligne = `feature`.
> Convention : `context/COMMON.md` section 15. Statuts : `feature` | `regression` | `quarantaine`.
> Tags : `smoke`, `critical`, `slow`.
> Lignes #246 par fichier (exception COMMON §15.1) : Go exige les `_test.go` dans le dossier du paquet testé (`cmd/server`, `internal/*`), Vitest les colocalise avec le composant ; aucun dossier `<famille>/<theme>/<lot>/` n'est possible sans casser l'accès aux symboles non exportés / aux mocks relatifs. Un fichier = un lot (< 50 cas).

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
| server-go/web/src/pages/GamePage.ardoise-reveal-gate.246.test.jsx | unit | admin-game | #246 Batch 2 — maquette admin validée : pastilles, compteur n/N reçues, REPONSE désactivé + motif, « Révéler quand même… » + confirmation, « non confirmée » | feature | critical |
| server-go/web/src/pages/AnimPage.ardoise-reveal-gate.246.test.jsx | unit | anim | #246 Batch 2 — maquette anim validée : mêmes états et règles que l'admin | feature | critical |
| server-go/web/src/pages/VPlayerPage.overlay.246.test.jsx | unit | vplayer | #246 Batch 2 — maquette tablette validée : Envoi…, pause sans invitation, « Aucune réponse », connexion perdue, écrans de session | feature | critical |
| server-go/web/src/pages/VPlayerPage.live-retype.246.test.jsx | unit | vplayer | #246 cause reproduite : effacement puis ressaisie continue sans pause jusqu'au STOP — envoi en direct (throttle ≤ 200 ms), dernier envoi = texte courant | regression | critical |
| server-go/web/src/pages/VPlayerPage.restore.246.test.jsx | unit | vplayer | #246 rechargement en STARTED : texte restauré (sessionStorage par bumper+question) envoyé en final, jamais de final vide, sessionStorage indisponible | regression | critical |
| server-go/internal/server/slow_client_246_test.go | integration | websocket, logging | #246 diagnostic de diffusion : CLIENT_EVICTED_SLOW (file d'envoi pleine → coupé + journalisé), CLIENT_WRITE_TIMEOUT (écriture > 3 s), INCOMING_DROPPED (file d'entrée pleine) | feature | slow |
| server-go/cmd/server/teams_249_test.go | integration | teams, websocket, game-engine | #249 renommage/suppression : NAME = clé (reproduction harness246_rename_server), normalisation à l'enregistrement, refus pendant une question (TEAMS_UPDATE_REFUSED ciblé, atomique), ajout/couleur/affectation permis, TEAMS_LOCKED, report en REVEALED (ARDOISE, AWARDED_TEAMS, score) | regression | critical |
| server-go/internal/game/teams_249_test.go | unit | teams, game-engine | #249 NAME = clé (SetTeams, LoadTeams + réécriture), cycle de vie de TEAMS_LOCKED | regression | critical |
| server-go/internal/game/teams_rename_249_test.go | unit | game-engine | #249 report de la liste fermée des états indexés par équipe (RenameTeamState / PurgeTeamState) + garde-fou de la liste | feature | |
| server-go/web/src/pages/TeamsPage.lock.249.test.jsx | unit | teams, admin-game | #249 TeamsPage : NAME = nouvelle clé au renommage (reproduction), champs/✕ désactivés + infobulle + bandeau si TEAMS_LOCKED, refus affiché | regression | critical |
| server-go/web/src/pages/GamePage.rename.249.test.jsx | unit | admin-game | #249 reproduction harness246_rename_gamepage : réponse ARDOISE retrouvée sous la clé malgré un NAME périmé | regression | critical |
| server-go/web/src/utils/ardoiseOrder.249.test.js | unit | admin-game, anim | #249 identité = clé dans sortArdoiseEntries | regression | |
| server-go/web/src/hooks/useWebSocket.teams-lock.249.test.js | unit | websocket, teams | #249 TEAMS_LOCKED dans l'état reçu, TEAMS_UPDATE_REFUSED | feature | |
| tests/procedures/teams-249.md | manual | teams, admin-game, anim | #249 procédure manuelle (utilisateur uniquement) | feature | |
| server-go/web/src/pages/PlayerDisplay.ardoise-grid.250.test.jsx | unit | tv | #250 TV REVEAL ARDOISE (maquette adaptive_grid validée) : toutes les équipes (4→20), règle lignes/colonnes/lignes de texte, polices relatives, équipe par la clé, aucun défilement (CSS), coupe « … », non-régression < 8 équipes et autres jeux | feature | critical |
| server-go/cmd/server/new_game_243_test.go | integration | game-engine, websocket, logging | #243 NEW_GAME : sans options = v11.1, défaut popup, chaque option isolée, RESET_QUESTIONS, refus joueurs sans scores (état inchangé), minuteurs arrêtés, double lancement, TEAMS_LOCKED/AWARDED_TEAMS, journal NEW_GAME_PURGE/REFUSED | regression | critical, slow |
| server-go/internal/game/new_game_243_test.go | unit | game-engine | #243 InitGameWith : défauts v11.1, réservoir RAFALE et statuts selon RESET_QUESTIONS, minuteurs (aucun survivant, pas de décompte tardif) | regression | critical, slow |
| server-go/web/src/pages/GamePage.launch-game.243.test.jsx | unit | admin-game | #243 popup « Lancer la partie » (page de jeu) : cases par défaut, verrouillage Scores par Joueurs, phrases, avertissements, bouton désactivé, NEW_GAME_REFUSED, accessibilité | regression | critical |
| server-go/web/src/components/QuizMetaForm.launch-game.243.test.jsx | unit | admin-game | #243 second point d'entrée (Backstage) | regression | |
| server-go/web/src/hooks/useWebSocket.new-game.243.test.js | unit | websocket | #243 newGame(options) envoie les quatre champs ; NEW_GAME_REFUSED | regression | critical |
| tests/procedures/new-game-243.md | manual | admin-game, game-engine | #243 procédure manuelle (utilisateur uniquement) | feature | |
| server-go/internal/game/new_game_timers_243_test.go | unit | game-engine | #243 NEW_GAME : aucun minuteur ne survit (RAFALE classique, ticks RAFALE de carte / MEMOTION carte et mémorisation inertes, chrono différé de fin de son jamais relancé) + témoins positifs | regression | critical, slow |
| server-go/cmd/server/new_game_sound_243_test.go | integration | game-engine, sound | #243 NEW_GAME coupe le son de question, QUESTION_SOUND_STATE au repos, fin du son sans effet ; refus = son intact | regression | critical |
| server-go/cmd/server/new_game_review_243_test.go | integration | game-engine, websocket, buzzer-led | #243 revue : refus en entracte (ENTRACTE_ACTIVE ciblé, rien appliqué), déliaison des connexions des joueurs purgés (#246), LED_SET + ambiance après NEW_GAME | regression | critical |
