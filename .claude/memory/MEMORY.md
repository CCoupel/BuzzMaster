# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.template.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## État courant (2026-11-22) — v12.1.0 en PROD

- **PROD = v12.1.0** « Roue de la Fortune — Extensions » livrée le 2026-11-22 : tag `v12.1.0`, main `fae364e2`, release https://github.com/CCoupel/BuzzMaster/releases/tag/v12.1.0. Milestone #22 clos (11 issues).
- **Marketing** : site publié (gh-pages `d33cfefa`), notes de version dans main (`4cf87307`). Les posts sont à publier par l'utilisateur.
- **Contenu v12.1.0** :
  - #147 achat de voyelles : par question, moment BEFORE / AFTER / BOTH, nombre ONE / MANY, prix, gain ;
  - #308 case BONUS : annonce puis nouveau tour ; géré hors système par l'animateur ;
  - #309 composition de la roue par question : 5 paramètres, global par défaut, surcharge champ par champ, 🔗 / ✏️ dans l'éditeur ;
  - correctifs QUALIF #310 à #315.
  - 5 cycles QUALIF : 12.1.0.1 à 12.1.0.5.
- **Travail en cours** : aucun. Prochain milestone : **v12.2.0 (#47)**, non cadré.
  - #316 : le BONUS se pose sur une case de points, sans case en plus ; l'équipe joue ensuite la valeur de la case et ne relance plus la roue.
  - #317 : BONUS éphémère. Réglage global plus réglage par question ; la marque disparaît une fois tirée ; le bandeau s'estompe à la sélection d'une consonne ou d'une voyelle. Valeur par défaut à décider.
- **Décisions et leçons de la session** :
  - #312 est passé en QA parce que les tests construisaient le GameState à la main. Garde : test sur le vrai hook, plus une fixture Go → hook couvrant tous les champs GameState (3 exceptions justifiées).
  - Relecture M1/M2 du LOT-2 : la composition effective est validée à l'enregistrement, BONUS hérité compris. Si seul le BONUS dépasse 64 cases → `ROUE_BONUS_COUNT_INVALID` (contrat roue.md rév. 8.2).
  - Les agents n'ont pas de navigateur. Le Chrome du cluster ne joint pas la QUALIF de l'utilisateur (localhost, 192.168.1.84 et port 9090 injoignables). Les contrôles visuels passent par une page d'aperçu statique autonome : vrai composant, vrai CSS, JS inline (un build module + assets affiche une page blanche via le launcher), servie par launcher.local/files.
  - Badge « Nouveau » : les mineures comptées sont celles de la majeure en cours (à v12.1.0 : v12.0.0 et v12.1.0 seulement).
- **Backlog / points de vigilance** :
  - les paquets Go sont lents sous `-race` (cmd/server ~161 s ; internal/server et internal/game ~125 s) ;
  - pas de `eslint.config.*` ;
  - découpage optionnel du fichier de tests Go des voyelles ;
  - proposition de backlog : `StartImmediate` sans la garde `participantsConform` ;
  - proposer `govulncheck` au template.

## Règles permanentes (adoptées par l'utilisateur)
1. dev-* et test-writer ne lancent que les tests touchés ; `qa` et la CI seuls lancent les suites complètes.
2. Jalon `EN COURS` au moins toutes les 10 min pendant toute attente.
3. Maquettes ouvertes dans Chrome (extension claude-in-chrome) par l'URL du serveur web du launcher — `commands/context/MOCKUPS.md` 14.9.
4. Jamais d'amend ni de réécriture dans le dépôt principal. (porté par `CLAUDE.md` « Branche courante et worktrees »)
5. Environnement des agents : conteneur Claude-Launcher (Linux, pas de `/mnt/c`) ; Go 1.26.11 et Node Linux v22.23.3 portés par `project-config.json` → `commands.env` (à évaluer avant toute commande `commands.*`) ; worktrees jetables sous `.claude/worktrees/<nom>` (`agents/context/COMMON.md` « Confinement ») ; `actionlint` absent. Mémoire auto `project_go_path_agents`.

## Backlog / suite

- **Issues créées** : **#306** (chargeur du dossier d'effets : mêmes refus que l'import — alias, ids réservés) ; **#307** (sauvegarde : case « Effets lumière » cochée par défaut, effets joints aux questions, restauration contrôlée ; niveau B choix global `lighting.effects/speeds` à trancher, jamais la clé du pont).
- Point UX non bloquant : écran Groupes — lignes provisoires `cfg-` remplacées à l'arrivée de l'inventaire (clic perdu possible en théorie).
- À remonter au template : Go 1.26.11 dans `env-template.yaml` (puis retirer l'étape projet de `.claude-launcher/env.yaml`) ; défauts des modèles de procédure (messages d'échec adressés à `"main"` au lieu de `"team-lead"`, suivi CI à viser `release.yml`, contrôle CHANGELOG cherchant `X.Y.Z.a`) ; `deploy.md` compagnon devrait écrire l'exception QUALIF `access.verified` failed limitée à `local-shell` / `port 9090` (rapport infra 2026-11-12).
- Milestones ouverts : v12.2.0 (#47), v13.0 PICTIONARY, vX.0.x DNS, vx.x.x Scoring MEMORY ; hors milestone : #209 #259 #281 #296 #297 #298 #300 #301 #302 #303 #304 #306 #307.
- Site marketing : règle des badges « Nouveau » = majeure en cours + 2 dernières mineures, retrait à chaque republication (`.claude/agents/marketing-release.md`).
- Stash : 18 entrées anciennes toujours non nettoyées.
- À remonter au template (toujours vrai en v5.12.0) : la liste « Nommage des Agents » de `teamleader.template.md` contient `dev-plugin` (absent du projet) et omet `marketing` ; panneau milestone-panel (sélection PLANNING).

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

États antérieurs (v11.0.0 → v12.0.0, synchronisations du template v3.14 → v4.2) : `.claude/memory/ARCHIVE.md`.
