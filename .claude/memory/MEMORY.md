# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.template.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## État courant (2026-11-22) — v12.2.0 en PROD

- **PROD = v12.2.0** « Roue : BONUS sur case à points » livrée le 2026-11-22 : tag `v12.2.0`, main `3d353e71` (merge `--no-ff`), release https://github.com/CCoupel/BuzzMaster/releases/tag/v12.2.0 (CI 45043348860). Commit de version `d7142f35` ajouté avant merge (exigé par `release.yml` : 5 fichiers de version à X.Y.Z). Milestone #47 clos (2/2).
- **Marketing** : site publié (gh-pages `6d31290e`, carte Roue « voyelles, BONUS et roue sur mesure » mise à jour, badge Nouveau v12.2.0, pas de nouvelle carte), notes + posts dans main (`036c71f2`). Posts à publier par l'utilisateur.
- **Contenu v12.2.0** :
  - #316 : BONUS = marque 🎁 posée sur une case à points (le nombre de cases ne change pas) ; arrêt dessus = bandeau « BONUS ! case 800 » puis consonne (ou voyelle) au montant, **sans relance** ; `ROUE_PASS` refusé (`WRONG_SUBPHASE`).
    Case marquée : dorée + ruban, **🎁 seul** (montant sur le bandeau), aria-label « 800, BONUS ».
    BANQUEROUTE / PERDU gardent leurs positions v12.0 ; seuls les BONUS s'adaptent (hasard, répartition homogène,
    jamais voisins d'une case spéciale) ; max = min(4, place libre, « autant de cases à points normales que de spéciales ») ;
    trop de BONUS = refus `ROUE_BONUS_COUNT_INVALID` à l'enregistrement, réduction au départ.
  - #317 : mode **éphémère (défaut)** / permanent, global `wheel.bonus_mode` + par question `ROUE_BONUS_MODE` (🔗/✏️,
    interrupteur sous le compteur BONUS) ; éphémère : la marque disparaît à la lettre acceptée, fondu 600 ms (case et
    bandeau) ; permanent : marque conservée, bandeau retiré sans fondu.
  - Contrats : `roue.md` rév. 9.1 §17, `wheel-component.md` §9. Un seul lot, 1 cycle QUALIF (12.2.0.7).
- **Travail en cours** : aucun. Aucun milestone ouvert en développement ; prochains milestones ouverts : v13.0 PICTIONARY,
  vX.0.x DNS, vx.x.x Scoring MEMORY (non cadrés).
- **Décisions et leçons de la session** :
  - Le raccord Go → hook (leçon #312) a de nouveau attrapé un vrai bug : `useWebSocket` ne recopiait pas
    `ROUE_BONUS_MODE` — garder ce test de raccord pour tout nouveau champ GameState.
  - Correctifs de review pendant une QA : worktree jetable `tmp/*` puis cherry-pick séquentiel après le verdict (OK).
  - Relecture factuelle des docs : 1re passe KO (2 erreurs ADMIN_GUIDE « 64 cases »), 2e OK — la règle reste utile.
  - Maquette marketing au GATE 4d : à ouvrir dans Chrome via le launcher (`MARKETING/index.html`), pas seulement l'Artifact.
  - `.claude/workflow-state.json` : commité systématiquement en fin de session.
  - Les sous-agents signent leurs commits avec leur propre modèle (Sonnet / Haiku) : ne pas imposer « Opus » dans les ordres.
- **Backlog / points de vigilance** :
  - retouches doc optionnelles : `ADMIN_GUIDE.md` « proposé » → « actif » (section BONUS) ; accents de la ligne ROUE du README ;
    `contracts/roue.md` « case BONUS » (§15) ;
  - review M3 non traitée : mutation de ref pendant le rendu dans `Wheel.jsx` (`markedId`/`everMarked`), sans effet visible ;
  - refus « trop de BONUS » de `POST /game-config.json` en texte brut (pas JSON) ;
  - tests Go lents sous `-race` : `cmd/server` ~158 s, `internal/server` ~126 s, `internal/game` ~125 s (découpage à prévoir) ;
  - `gofmt -l` : 31 fichiers non conformes préexistants ;
  - pas de script lint npm ;
  - `StartImmediate` sans la garde `participantsConform` (proposé depuis v12.0, non traité) ;
  - stash : 18 entrées anciennes toujours non nettoyées.

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

États antérieurs (v11.0.0 → v12.1.0, synchronisations du template v3.14 → v4.2) : `.claude/memory/ARCHIVE.md`.
