# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## Démarrage de session

Utiliser `/start-session` pour démarrer chaque session : crée la TEAM de travail.
Source de vérité MEMORY : `.claude/memory/MEMORY.md` uniquement (versionné Git).
Le hook SessionStart a été supprimé — plus de démarrage automatique.

## État du projet (2026-08-07)

- **PROD** : v6.1.1 déployée (tag + release GitHub + CI verte, squash-merge sécurisé), milestone `v6.0` — Générateur de Jeu IA (4 issues encore ouvertes : #8 parente, #138, #139, #140)
- **Chantier "Générateur de Jeu IA" (#137)** : dédoublonnage config partie / popup IA, publics/difficultés multiples, objectif de partie (jamais diffusé aux joueurs), visibilité TV par champ (`QUIZ_HIDDEN_FIELDS`), fix bug de fraîcheur `QuestionsPage`. Suivi de 4 correctifs post-QUALIF : bug critique #142 (génération Groq 100% cassée — schéma `anyOf` discriminator ambigu `CATEGORY`/`DIFFICULTY`/`TYPE`, résolu et validé en conditions réelles sur les 5 types), verbosité des erreurs IA (`ERROR_MESSAGE` admin-only), couleur des sliders MEMORY/ARDOISE, focus clavier (a11y), bouton "Nouvelle génération". QA finale VALIDATED sans réserve (Chrome headless piloté via CDP, extension navigateur `mcp__claude-in-chrome` indisponible dans cet environnement tout du long de la session).
- **Incident sécurité traité** : clé API Groq en clair détectée dans `server-go/config.json` tracké (repo public) juste avant le push PROD — jamais fuité (branche jamais poussée), remédiée par lecture via variable d'environnement (`BUZZCONTROL_GROQ_API_KEY`/`BUZZCONTROL_ANTHROPIC_API_KEY`) + squash-merge pour ne jamais exposer l'historique contaminé. **Clé à révoquer/régénérer par l'utilisateur côté console Groq — vérifier au démarrage de la prochaine session si pas encore fait.**
- **Versionnement harmonisé** : adoption du schéma `X.Y.Z.a` du template, avec une parité **inversée par rapport au template** assumée et documentée (`docs/RELEASE_PROCEDURE.md`) : **pair = dev/milestone, impair = release**. Milestone `#8` renommé `v6.0` en conséquence. Baseline dev actuelle : `6.0.1.0`. Écart avec `COMMON.md` §5 (qui dit l'inverse) signalé explicitement dans la doc — ne pas le "corriger" silencieusement lors d'un futur sync template.
- **Contexte-audit exécuté** (fin de session) : 5 références cassées corrigées (suffixe `.template.md` manquant sur `qa`/`code-reviewer`/`security`/`marketing-release`/`cdp` dans les commandes/agents — **bug récurrent, réintroduit à chaque sync car il vient de `TEMPLATE_claude/` lui-même**, déjà corrigé une fois le 2026-08-05), 7 règles migrées de `MEMORY.md` vers leurs docs source de vérité (`context/GITHUB.md`, `deploy.md`, `TEAMMATES_PROTOCOL.md`, `marketing.md` ×2, `docs/FIRMWARE_UPDATE.md`), 3 doublons retirés.
- **Backlog restant** : #138 (LLM local Windows, non traité), #139 (backoff 429/413 court-circuité, non traité), #140 (test flaky TempDir, non traité), #141 (persistance GameState, backlog volontaire), #143 (pollution récurrente du fixture `internal/server/config.json` par `go test` — **reproduite 3 fois sur cette session**, cause probable : `config.Get()` résout un chemin relatif au répertoire du paquet en test), #121 (race `TestE2E_GameStateMachine`, diagnostic précis fourni par dev-backend mais pas encore posté en commentaire GitHub).
- **Point d'attention nommage** : milestone `#19 "v6.1.x - amelioration visuelle"` coexiste maintenant avec le nouveau `v6.0` — pas de collision actuelle mais à surveiller si `v6.1.x` doit un jour être renommé selon la nouvelle convention.
- **Fichier mémoire non harmonisé signalé (non corrigé)** : `.claude/memory/feedback_version_reconciliation.md` mentionne encore "reset Z à 0 pour une release vX.Y.0", en tension avec le nouveau schéma.

## État du projet (2026-08-05) — archivé

- **PROD** : v5.10.0 déployée (tag + release GitHub + CI verte 5/5), milestone `v5.10.x - Stabilité VJoueur` fermé (5/5 : #127, #129, #130, #132, #134)
- **Chantier "Stabilité VJoueur"** (déconnexions résiduelles) : investigation multi-rounds partie d'une hypothèse initiale infirmée (JSON incomplet bloquant un ping) ayant mené à 4 causes racines réelles distinctes, toutes corrigées et déployées en v5.10.0 :
  - #127 : rafale de broadcasts non groupée à la transition PREPARE→READY (12→2 messages)
  - #129 : rafales hors PREPARE/READY (connexions/déconnexions en masse, saisie ARDOISE soutenue) — ciblage plutôt que filtrage/regroupement (0 message inutile + écho ciblé)
  - #130 : marge de tolérance ping/pong qui ne tolérait en réalité AUCUNE perte (pas juste "trop juste") — recalibrée (ping 2s, tolérance 2 pertes, seuil client transmis par le serveur via HEARTBEAT au lieu d'une constante dupliquée)
  - #134 : nouvelle fonctionnalité — élargir "Réinscription" pour évincer un joueur connecté et libérer sa place sans perdre le score (le bouton existant, #122, ne touchait jamais une session active)
  - #132 : audit préventif de 5 fonctions LED restantes avec le même motif de bug (broadcast non ciblé) — toutes corrigées
- **Backlog issu de ce chantier, encore ouvert** : #128 (fuite ARDOISE_ANSWERS côté VJoueur, désormais dans milestone v5.11.x), #131 (aucun `recover()` — une panique sur un client fait tomber tout le serveur, pas de milestone), #133 (race pré-existante `client.Type` sous `-race`, pas de milestone)
- **Milestone renommé** : `v5.10.x - amelioration visuelle` → `v5.11.x - amelioration visuelle` (#111, #114, #119, #121, #128) pour laisser la place au chantier stabilité sur le numéro 5.10
- **Backlog hors chantier** : #83, #101 (DONE, jamais fermée), #103 (DONE, jamais fermée), #108, #125, #126 — sans milestone, non traitées cette session
- **Audit de contexte** (fin de session, commit `054c19d`) : 5 références cassées corrigées (suffixe `.template.md` manquant sur `qa`/`code-reviewer`/`security`/`marketing-release`/`cdp` — cassait notamment les prompts de spawn de `/secu` et `/marketing`), 6 règles migrées de MEMORY.md vers `CLAUDE.md`/`context/GITHUB.md`/`context/CDP_WORKFLOWS.md`. `dev-buzzclick` passé de `permanent` à `ponctuel` (plus d'évolution active sur le firmware).
- **Site marketing** (`gh-pages`) : section Fonctionnalités réorganisée le 2026-08-02 (commits `dd5d5a1`, `26f4440`) — voir archive `.remember/` pour le détail, hors scope du chantier stabilité.
- **Pattern de session validé, confirmé sur ce chantier (5 cycles consécutifs)** : investigation→GATE2(plan détaillé, arbitrages explicites)→dev/test/review/QA parallèle→QUALIF cumulative("avec réserves", browser/hardware jamais dispo en sandbox)→validation manuelle utilisateur→PROD groupé en fin de milestone.

