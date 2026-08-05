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


- **Corrections intra-feature** : SendMessage vers l'agent existant, jamais créer un nouvel agent
- **Commande /start-session** : créer la TEAM **directement sans demander** confirmation ni sujet. Le nom de la TEAM est **toujours `TEAM-Buzz`**, quelle que soit la session.
- **Architecture team** : Claude principal (`main`) est l'orchestrateur — pas d'agent CDP séparé. Agents spawned à `/start-session` (IDLE), dispatched via `SendMessage` uniquement pendant la session. Rapports inter-agents vers `main`. Chemins rapports : `_work/reports/`.
- **Maquette site marketing avant implémentation** : toujours passer par une maquette HTML publiée en Artifact, itérée jusqu'à validation explicite, avant tout commit sur `gh-pages` — voir [feedback_marketing_gate_iterations.md](feedback_marketing_gate_iterations.md)
- **Vérifier visuellement une image avant réutilisation** : ne jamais assigner une image sur la base de son seul nom de fichier/label hérité — voir [feedback_marketing_image_verification.md](feedback_marketing_image_verification.md)
- **Signal de branche entre teammates** : écrire "X peut checkout la branche" dans un rapport adressé à `main` ne notifie PAS réellement X — récurrent 2 fois sur la même session (#130, #134) avant correction. L'agent qui crée une branche doit envoyer un `SendMessage` direct à chaque teammate qui doit checkout, en plus de son rapport à `main`.

## État du projet (2026-08-05)

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

