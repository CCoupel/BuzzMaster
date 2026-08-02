# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## Démarrage de session

Utiliser `/start-session` pour démarrer chaque session : crée la TEAM de travail.
Source de vérité MEMORY : `.claude/memory/MEMORY.md` uniquement (versionné Git).
Le hook SessionStart a été supprimé — plus de démarrage automatique.

## Corrections comportementales

- **GitHub Issues/PR** : ne jamais fermer/rouvrir/commenter une issue ou PR sans validation explicite de l'utilisateur — voir [feedback_github_actions.md](feedback_github_actions.md)
- **QA → DOC → QUALIF** : pas besoin de validation utilisateur entre ces phases — lancer directement — voir [feedback_qa_workflow.md](feedback_qa_workflow.md)
- **Binaire Windows obligatoire** : toujours builder le .exe Windows AVANT de demander validation QUALIF (l'utilisateur teste depuis Windows) — voir [feedback_windows_binary.md](feedback_windows_binary.md)
- **OTA firmware bootstrap** : si OTA échoue à ~20-30% sur un buzzer, flash USB avec firmware merged (0x0) — voir [project_v370_firmware_ota.md (dans /home/cyril/.claude/projects/...)]()


- **TeamDelete** : proposer à l'utilisateur après PROD validé, **jamais supprimer automatiquement**
- **Corrections intra-feature** : SendMessage vers l'agent existant, jamais créer un nouvel agent
- **Commande /start-session** : créer la TEAM **directement sans demander** confirmation ni sujet. Le nom de la TEAM est **toujours `TEAM-Buzz`**, quelle que soit la session.
- **Architecture team** : Claude principal (`main`) est l'orchestrateur — pas d'agent CDP séparé. Agents spawned à `/start-session` (IDLE), dispatched via `SendMessage` uniquement pendant la session. Rapports inter-agents vers `main`. Chemins rapports : `_work/reports/`.
- **Dispatcher test-writer en parallèle de chaque lot dev** : jamais laisser un dev écrire ses propres tests faute de handoff séparé — voir [feedback_test_writer_parallel.md](feedback_test_writer_parallel.md)
- **`gh issue create` : ne jamais passer `--body` deux fois** (écrase silencieusement en vide) — voir [feedback_gh_issue_body_flag.md](feedback_gh_issue_body_flag.md)
- **Répertoire de travail partagé entre teammates** : un checkout/stash par un agent peut effacer silencieusement les modifications non commitées de `workflow-state.json` (main) — toujours revérifier après un incident git signalé — voir [feedback_shared_worktree_git_collateral.md](feedback_shared_worktree_git_collateral.md)
- **Push PROD asynchrone** : un ordre de deploy peut s'exécuter avant qu'un "STOP" utilisateur n'arrive — toujours vérifier l'état réel (git log/tags/CI) après coup, jamais supposer que l'annulation est arrivée à temps — voir [feedback_prod_push_race.md](feedback_prod_push_race.md)
- **Versions divergentes entre branches parallèles** : normal pendant le dev, ne pas synchroniser — réconcilier une seule fois au merge PROD réel — voir [feedback_version_reconciliation.md](feedback_version_reconciliation.md)
- **Maquette site marketing avant implémentation** : toujours passer par une maquette HTML publiée en Artifact, itérée jusqu'à validation explicite, avant tout commit sur `gh-pages` — voir [feedback_marketing_gate_iterations.md](feedback_marketing_gate_iterations.md)
- **Vérifier visuellement une image avant réutilisation** : ne jamais assigner une image sur la base de son seul nom de fichier/label hérité — voir [feedback_marketing_image_verification.md](feedback_marketing_image_verification.md)

## État du projet (2026-08-02)

- **PROD** : v5.9.0 déployée (tag + release GitHub + CI verte), milestone v5.9.x fermé (6/6 : #109,#112,#113,#115,#117 déjà en v5.8.x ; #120,#118,#123,#122,#116,#124 en v5.9.0)
- **Backlog ouvert** : milestone v5.10.x (#111 OOM ConfigPage.test.jsx récurrent x8, #114, #119, #121 flake E2E race, #125 captures d'écran site marketing — traité cette session, #126 site jamais bilingue malgré le template `/marketing` — toujours ouvert, hors scope de cette session), + #122 issue R2 potentiellement encore à affiner selon retours terrain
- **Site marketing** (`gh-pages`) : section Fonctionnalités entièrement réorganisée le 2026-08-02 (commits `dd5d5a1` puis fix `26f4440`) — 13 captures réelles ajoutées dans `images/Features/`, 3 classements distincts (Catégorie/Équipes/Joueur) au lieu du "Palmarès" générique, carrousel CSS 3 images sur "Classements en Direct", OTA fusionnée (firmware + auto-update serveur), Sauvegarde déplacée vers Fiabilité & maintenance avec texte corrigé, carte Interface d'Administration ajoutée, Multi-équipes déplacée vers Gestion & suivi ; "Temps Réel" et "BuzzApp Toujours Connecté" retirés à la demande utilisateur. Issue #125 (captures d'écran) traitée par cette session — à fermer si l'utilisateur confirme.
- **Pattern de session validé** : plusieurs cycles investigation→GATE2(maquette)→dev/test/review/QA parallèle→QUALIF("avec réserves", browser/hardware jamais dispo en sandbox)→validation manuelle utilisateur→PROD groupé en fin de milestone — a bien fonctionné sur 5 issues d'affilée en session du 2026-08-01

