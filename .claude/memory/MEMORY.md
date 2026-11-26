# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.template.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## État courant (2026-11-26) — v13.1.0 (Picto Rafale) : milestone livré, PROD à publier

- **Milestone v13.1.0 — PICTO RAFALE** (#48) : GATE 4 validé le 2026-11-26, code freeze. Documentation finalisée
  (CHANGELOG daté 2026-11-26, README à jour). **PROD : à publier par deployer** (SHA et tag à compléter après publication).
- **Contenu livré** :
  - **Présélection des gagnants au crédit (#321)** : à la révélation, la page Jeu coche d'office les unités gagnantes
    (Picto, Mémoire, Rafale Speedy, QCM, Speedy, Ardoise, Memotion). Crédit toujours manuel. Chip « +N pts » sur l'équipe cochée.
  - **x/net v0.62.0, x/sys v0.49.0 (#322)** : GO-2026-6866, GO-2026-6860.
  - **Pool Picto (#324)** : réservoir de mots sur le moteur de pool généralisé (deux instances, Pool Speedy et Pool Picto),
    génération IA 10/20/50, collage d'une liste, sauvegarde « Pools » à trois états, verrou 409 pendant une manche Picto.
    Renommage d'affichage : « Rafale Speedy » / « Pool Speedy » (identifiants inchangés).
  - **Rafale Picto (#318)** : type à part entière (🖍️), famille Picto. Le plus de mots possible en un temps donné, même
    dessinateur ; passes annoncées à l'oral et déclenchées par admin ou anim (coût 0 à 100, 1 passe ou illimitées) ; fin de
    pool en reboucle ou fin de manche ; crédit suggéré `max(0, trouvés × points − passes × coût)` ; TV avec mot trouvé 1 s.
  - **Décompte Picto sur la tablette du dessinateur (#326).**
- **Documentation** : `contracts/picto.md` §17, `picto-pool.md` rév. 3, `award-preselect.md` rév. 3 ; docs techniques
  (DATA_MODELS, WEBSOCKET_PROTOCOL, GAME_STATE_MACHINE, REACT_INTERFACE, ADMIN_GUIDE) à jour.
- **Décisions de ce milestone** : Rafale Picto est un type (pas un mode de PICTO) ; le pool Picto est une instance du moteur
  Speedy généralisé ; présélection du crédit à la révélation, sans nouvel envoi serveur.
- **Travail en cours** : aucun.

## Backlog et points pour la prochaine session

- **#259** : endpoints sans authentification (`/api/picto/*`, `/questions`, `/files`, `/ws/*`) ; la liste des mots Picto est
  lisible hors manche par qui peut joindre le serveur (refus 409 seulement pendant `STARTED` / `PAUSED`).
- **#325** : flake `TestCategories285_HistoryBackups_RotateToFive_…` (RemoveAll sur un TempDir, sauvegarde asynchrone).
- **#327** : à reprendre selon le suivi GitHub.
- `PoolPage` ne se recharge pas à la fin de la manche Picto (il faut rouvrir l'onglet).
- Récapitulatif TV et tablette tronqué au-delà d'une dizaine de mots trouvés (voulu : TV statique).
- Taille `22vh` du décompte du dessinateur (tablette) : non vérifiée sur appareil.
- Scénarios IA 4 et 5 (génération Pool Picto et Pool Speedy, job croisé) jamais testés avec une clé API réelle ; à rejouer
  à la QUALIF finale.
- Tests Go lents sous `-race` (`cmd/server`, `internal/server`, `internal/game`) : découpage à prévoir.
- `gofmt -l` : fichiers non conformes préexistants ; pas de script lint npm ; stash ancien non nettoyé.
- `StartImmediate` sans la garde `participantsConform` (proposé depuis v12.0, non traité).
- Libellé « Reveal » dans le texte d'issue TIMEOUT de `PictoPanel.jsx` (le bouton admin s'appelle « REPONSE ») : à aligner.
- Point UX non bloquant (v11.4) : écran Groupes — lignes provisoires `cfg-` remplacées à l'arrivée de l'inventaire.
- Panneau milestone-panel : sélection PLANNING à remonter au template (`teamleader.template.md`).
- **Hors milestone** : #209 #281 #296 #297 #298 #300 #301 #302 #303 #304 #306 #307. #306 : chargeur du dossier d'effets
  (mêmes refus que l'import) ; #307 : sauvegarde des effets lumière (case cochée par défaut, restauration contrôlée).
- **À remonter au template** : Go 1.26.11 dans `env-template.yaml` ; défauts des modèles de procédure (messages d'échec
  adressés à `"main"` au lieu de `"team-lead"`, suivi CI à viser `release.yml`, contrôle CHANGELOG cherchant `X.Y.Z.a`) ;
  `deploy.md` compagnon : exception QUALIF `access.verified` (infra 2026-11-12) ; liste « Nommage des Agents » de
  `teamleader.template.md` (`dev-plugin` absent, `marketing` omis).
- Site marketing : règle des badges « Nouveau » = majeure en cours + 2 dernières mineures, retrait à chaque republication
  (`.claude/agents/marketing-release.md`). Carte : 1 à 2 phrases, visuel + gameplay, mots clés en `<strong>`.

## Règles permanentes (adoptées par l'utilisateur)
1. dev-* et test-writer ne lancent que les tests touchés ; `qa` et la CI seuls lancent les suites complètes.
2. Jalon `EN COURS` au moins toutes les 10 min pendant toute attente.
3. Maquettes ouvertes dans Chrome (extension claude-in-chrome) par l'URL du serveur web du launcher — `commands/context/MOCKUPS.md` 14.9.
4. Jamais d'amend ni de réécriture dans le dépôt principal. (porté par `CLAUDE.md` « Branche courante et worktrees »)
5. Environnement des agents : conteneur Claude-Launcher (Linux, pas de `/mnt/c`) ; Go 1.26.11 et Node Linux v22.23.3 portés par `project-config.json` → `commands.env` (à évaluer avant toute commande `commands.*`) ; worktrees jetables sous `.claude/worktrees/<nom>` (`agents/context/COMMON.md` « Confinement ») ; `actionlint` absent. Mémoire auto `project_go_path_agents`.

## Migration RETEX v13.1.0

Éléments de MEMORY examinés pour migration (`commands/context-audit.md` §2.4) : aucun nouvel élément de règle permanente.
Les règles du milestone (contrats Picto, libellés, secrets `RAFALE_PICTO_WORDS`, présélection, pool) sont déjà dans les docs
projet (`docs/*.md`, `contracts/`, CHANGELOG). Les points restants sont du backlog, gardés ici pour la prochaine session.

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

États antérieurs (v11.0.0 → v13.0.0, synchronisations du template) : `.claude/memory/ARCHIVE.md`.
