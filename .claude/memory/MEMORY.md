# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.template.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## État courant (2026-11-24) — v13.0.0 (Picto) : milestone livré, PROD à publier

- **Milestone v13.0.0 — PICTO** (#30) : GATE 4 et GATE 4d validés par l'utilisateur le 2026-11-24. QUALIF 13.0.0.30, CI verte
  (run 45380755436). **PROD : à publier par deployer** (SHA et tag à compléter après publication).
- **Contenu livré** :
  - Type de question **Picto** (#191) : mots saisis dans la question (1 à 20), éditeur, durée 30 à 300 s (90 par défaut).
  - Manche jouable (#192, #193, #194) : équipe puis dessinateur (choix ou tirage parmi les VJoueurs connectés), mot tiré au
    lancement et envoyé seulement au dessinateur, admin et animateur ; dessin en direct sur la TV ; issues trouvé (y compris
    sur le gong), temps écoulé, arrêt, sans mot ; révélation du mot seul sur la TV et les tablettes ; crédit manuel à l'équipe
    (ligne commune, pré-remplie aux points) ; relance de la question ; LED de l'équipe qui dessine.
  - Page Jeu : temps et points repris de la question, pour tous les types (#192).
  - Jeton de session des tablettes (#319) : un 2e joueur ne s'inscrit pas sur un navigateur qui garde une place ; libération
    par l'animateur ; `vplayer_sessions.json` (empreintes seules, écriture synchrone) ; WARN commun démarrage et restauration.
  - Sécurité : `gorilla/websocket` mis à jour (#320, GO-2026-6278).
  - Perf : relais `PICTO_STROKES` sérialisé une seule fois (#193).
- **Décisions de ce milestone** : une manche = un mot, sans passe (passes reportées v13.1.0) ; mot saisi dans la question, pas de
  réservoir ni d'IA (reportés v13.1.0, #48 / #318) ; mode RAFALE de PICTO (maximum de mots) reporté v13.1.0.
- **Travail en cours** : aucun. Milestone à clore après PROD.
- **Points à trancher, non migrés au RETEX** (règle permanente ou cible incertaine) :
  1. capture marketing : navigateur sans tête (Playwright + chrome-headless-shell) à inscrire dans `.claude-launcher/env.yaml`
     (fichier infra, modifié non commité — à traiter par `infra`).
- **Backlog / points de vigilance** :
  - **#259** : endpoints sans authentification (`/questions`, `/files`, `/ws/*`) ; les mots Picto sont lisibles avant correction.
  - **#318 / #48** (v13.1.0) : réservoir Picto, génération IA, catégorie « Pools » de la sauvegarde, mode RAFALE de PICTO.
  - Tests Go lents sous `-race` (`cmd/server`, `internal/server`, `internal/game`) : découpage à prévoir.
  - `gofmt -l` : fichiers non conformes préexistants ; pas de script lint npm ; stash ancien non nettoyé.
  - `StartImmediate` sans la garde `participantsConform` (proposé depuis v12.0, non traité).
  - Libellé « Reveal » dans le texte d'issue TIMEOUT de `PictoPanel.jsx:78` (le bouton admin s'appelle « REPONSE ») : à aligner.
  - Point UX non bloquant (v11.4) : écran Groupes — lignes provisoires `cfg-` remplacées à l'arrivée de l'inventaire (clic perdu possible).
  - Panneau milestone-panel : sélection PLANNING à remonter au template (`teamleader.template.md`).
- **Hors milestone** : #209 #281 #296 #297 #298 #300 #301 #302 #303 #304 #306 #307. #306 : chargeur du dossier d'effets (mêmes refus
  que l'import) ; #307 : sauvegarde des effets lumière (case cochée par défaut, restauration contrôlée).
- **À remonter au template** : Go 1.26.11 dans `env-template.yaml` ; défauts des modèles de procédure (messages d'échec adressés
  à `"main"` au lieu de `"team-lead"`, suivi CI à viser `release.yml`, contrôle CHANGELOG cherchant `X.Y.Z.a`) ; `deploy.md`
  compagnon : exception QUALIF `access.verified` (infra 2026-11-12) ; liste « Nommage des Agents » de `teamleader.template.md`
  (`dev-plugin` absent, `marketing` omis).
- Site marketing : règle des badges « Nouveau » = majeure en cours + 2 dernières mineures, retrait à chaque republication
  (`.claude/agents/marketing-release.md`). Carte : 1 à 2 phrases, visuel + gameplay, mots clés en `<strong>`.

## Règles permanentes (adoptées par l'utilisateur)
1. dev-* et test-writer ne lancent que les tests touchés ; `qa` et la CI seuls lancent les suites complètes.
2. Jalon `EN COURS` au moins toutes les 10 min pendant toute attente.
3. Maquettes ouvertes dans Chrome (extension claude-in-chrome) par l'URL du serveur web du launcher — `commands/context/MOCKUPS.md` 14.9.
4. Jamais d'amend ni de réécriture dans le dépôt principal. (porté par `CLAUDE.md` « Branche courante et worktrees »)
5. Environnement des agents : conteneur Claude-Launcher (Linux, pas de `/mnt/c`) ; Go 1.26.11 et Node Linux v22.23.3 portés par `project-config.json` → `commands.env` (à évaluer avant toute commande `commands.*`) ; worktrees jetables sous `.claude/worktrees/<nom>` (`agents/context/COMMON.md` « Confinement ») ; `actionlint` absent. Mémoire auto `project_go_path_agents`.

## Leçons en fichiers (`.claude/memory/feedback_*.md`)

Non chargées automatiquement : à lire quand le sujet se présente.
- `feedback_agent_context_crash.md` — agent qui plante quand son contexte devient trop gros (runtime bun).
- `feedback_cdp_delegation.md` — le CDP délègue toujours (repris dans `CLAUDE.md`, « Rôle Teamleader »).
- `feedback_gh_issue_body_flag.md` — `gh issue create` : un 2e `--body` écrase le premier en silence.
- `feedback_github_actions.md` — PÉRIMÉ (labels et fermeture automatiques).
- `feedback_marketing_gate_iterations.md` — maquette marketing itérée en Artifact avant tout commit gh-pages.
- `feedback_marketing_image_verification.md` — ouvrir une image avant de la réutiliser.
- `feedback_prod_push_race.md` — un « stop » peut arriver après le push PROD : vérifier l'état distant.
- `feedback_qa_workflow.md` — QA validée : enchaîner DOC + QUALIF sans demander.
- `feedback_shared_worktree_git_collateral.md` — répertoire de travail partagé (voir `CLAUDE.md`, « Branche courante et worktrees »).
- `feedback_test_writer_parallel.md` — test-writer dispatché avec chaque lot dev.
- `feedback_version_reconciliation.md` — PÉRIMÉ (versionnage milestone X.Y.Z / X.Y.Z.a).
- `feedback_windows_binary.md` — PÉRIMÉ (BUILD + PUBLISH QUALIF).

## Démarrage de session

Utiliser `/start-session` pour démarrer chaque session : crée la TEAM de travail.
Source de vérité MEMORY : `.claude/memory/MEMORY.md` uniquement (versionné Git).
Le hook SessionStart a été supprimé — plus de démarrage automatique.

## Archives

États antérieurs (v11.0.0 → v12.1.0, synchronisations du template v3.14 → v4.2) : `.claude/memory/ARCHIVE.md`.
