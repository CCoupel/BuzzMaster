# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.template.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## État courant (2026-11-09) — v11.5.0 en PROD

- **PROD = v11.5.0** « Optimisation des tests » : version officielle `d5095ce8`, merge `main` `6c826cd3` (`--no-ff`), tag `v11.5.0` (sur `6c826cd3`), Release https://github.com/CCoupel/BuzzMaster/releases/tag/v11.5.0 (4 assets), `release.yml` run 42598655680 vert. Notes + posts : `6719d96b` (`docs/releases/v11.5.0/`). Milestone #44 clôturé 6/6. Branche milestone supprimée (local + remote). Branche courante : `main`.
- **Pas de QUALIF** (`qualif: false`, décision utilisateur) : GATE 4 validé sur l'exe candidat 11.5.0.28 (`build/candidate_v11.5.0/`). La QUALIF n'est plus exécutable depuis le conteneur : l'exe est lancé par l'utilisateur sous Windows (`deploy.qualif.md`, `access.verified = failed`).
- Site `gh-pages` laissé en v11.2.2 (GATE 4d : pas de nouveauté fonctionnelle, comme v11.2.3). Posts réseaux à publier par l'utilisateur.

### Livré (4 lots, auto-validés QA + CI, sans VALID-HUMAINE)
- LOT-1 #292 #293 #294 : environnement conteneur Claude-Launcher constaté et documenté (Linux natif, pas de WSL ni `/mnt/c`, Go 1.26.10 / Node 22.23.3 / `pio` dans `~/.local/bin`), `commands.test_targeted`, firmware compilé dans `ci-branch.yml` via le script commun `scripts/build-firmware.sh` (appelé par `scripts/ci.sh` étape [1/8], par la CI, `release.yml` et BUILD ; `SKIP_FIRMWARE=1` en CI). `scripts/ci.sh` = **8 étapes** (vitest [3/8], Go [6/8]).
- LOT-2 #280 : vitest `pool: vmThreads` + environnement `node` pour 33 fichiers sans DOM : vitest CI 155 s → 30–53 s.
- LOT-3 #291 : Phase A (durées injectables **par instance**, barrières positives) + Phase B (7 paquets `internal/{vplayer,colors,coalesce,rounds,startup,diag,demo}`) : `cmd/server` CI 221 s → 100,5 s ; écarts acceptés (cibles 90 s et `main.go` 6 000).
- LOT-4 #295 : `internal/led` + `internal/ambiance`, Option P (15 tests `t.Parallel` ajoutés, 27 au total) ; `main.go` 8 701 → 6 217 lignes ; `cmd/server` CI 86,6–90,9 s (cible 75 s non atteinte, écart accepté).
- Bilan : job CI complet 7 min 23–8 min 07 → 3 min 32–3 min 38 ; `go test -race ./...` local 266 s → 114–123 s ; 9 → 18 paquets testés ; 2 595 → 2 612 tests de premier niveau, aucun disparu.

### Décisions techniques
- Durées de test injectées **par instance** (`Engine.SetTickInterval`, `SetConnGreenMinDuration`, `watchdogDelay`, `flashPhases`), jamais par variable de paquet (`t.Parallel` les partagerait).
- Barrière positive (`settle_barrier_291_test.go`) au lieu des fenêtres « aucun message » ; marge 250 ms après une action client (HELLO / PLAYER_CONNECT).
- Cadences de test > 2 × le pacing du writer lumière (`MinInterval` 100 ms) ; ne pas comparer deux `Last()` successifs (cause des 2 CI rouges du LOT-4, corrigées `465a8590` et `0d2ca78b`).
- Cadences de production verrouillées par `TestFlashCadenceConstants_291`.
- `version.txt` : chaque BUILD commite la valeur candidate `X.Y.Z.a` ; la PROD pose `X.Y.Z` dans un commit séparé.
- Propriété de l'état : LED → l'`App` (partagé par pointeur) ; ambiance → le `Controller` (détails dans `docs/GO_SERVER.md`).

### Incidents / leçons (v11.5.0)
1. Le partage `/share/MFS` a `core.fileMode=false` et perd le bit exécutable : un nouveau script passe par `git update-index --chmod=+x` (vérifier `git ls-tree`) et est appelé via `bash <script>`.
2. Ne jamais lier `node_modules` par symlink dans un worktree jetable : `git worktree remove --force` suit le lien et vide `server-go/web/node_modules` du dépôt principal (restauré par `npm ci`).
3. Pendant une QA dans le dépôt principal, les devs travaillent en worktree `tmp/*` puis cherry-pick après le verdict (appliqué 3 fois, sans incident).
4. Identité git : `.git/config` portait la coquille `user.mail` → les commits du milestone (mesure `8dc81b23..HEAD` : 43 commits, dont le tag `v11.5.0`) ont l'e-mail du conteneur (`ubuntu@…svc.cluster.local`). Corrigé en fin de session (`user.email` dans `PRIVATE/gitconfig`, inclus par `~/.gitconfig`) ; pas de réécriture d'historique.
5. Les reviewers et devs ne lancent pas de suite complète (règle rappelée à code-reviewer : son run vitest pendant la QA a faussé une mesure).
6. Label `ci` créé par deployer alors que `ci-cd` existait : doublon à fusionner un jour.
7. Documentation : la relecture factuelle a trouvé des erreurs à chaque passe (E1–E3 de la relecture finale : chiffres de baseline, décompte `t.Parallel`, description du job firmware) : re-relecture après chaque correction, comme prévu.

### Règles permanentes (adoptées par l'utilisateur)
1. dev-* et test-writer ne lancent que les tests touchés ; `qa` et la CI seuls lancent les suites complètes.
2. Jalon `EN COURS` au moins toutes les 10 min pendant toute attente.
3. Maquettes ouvertes avec `cmd.exe /c start chrome <chemin Windows>`.
4. Jamais d'amend ni de réécriture dans le dépôt principal.

### Backlog / suite
- Suites v11.5.0 : **#296** attentes fixes `internal/game` (118 s), **#297** attentes fixes `internal/server` (110 s), **#298** logger + config par `App` pour paralléliser `cmd/server` — prochain levier pour l'étape Go de la CI (134–137 s).
- **#36 v11.6.0 Ambiance** : #290, #231, #232, #235. Milestones v12.0/v12.1 (Roue), v13.0 (Pictionary), v15.0, vX (DNS #65), vx (#12).
- Toujours : #283 (observabilité audio, PLANNING, sans milestone), #259, #281, #209 en backlog.
- Template : nouvelle version disponible (`c3faa5c` vs `b80b912` local) — l'utilisateur a continué sans sync ; à faire via `/init-project` (option d).
- Toujours valables : panneau milestone-panel (sélection PLANNING), liste « Nommage des Agents » du bloc template.

## État précédent (2026-11-06) — v11.4.0 en PROD (archivé)

- **PROD = v11.4.0** « RAFALE Alphabétique » : merge `main` `9b76ef61`, tag `v11.4.0`, Release https://github.com/CCoupel/BuzzMaster/releases/tag/v11.4.0, milestone #43 clôturé 7/7. Lots : popover ambiance, RAFALE alphabétique, son de décompte, catégories éditables (#282–#289). Détail : historique git de ce fichier (commit `2661dd56`).
- Leçons conservées : doc relue seulement en fin de milestone (7 erreurs trouvées) ; `TEST_PROCEDURE.md` §2.3 évite les tests Go à symboles absents.

## Démarrage de session

Utiliser `/start-session` pour démarrer chaque session : crée la TEAM de travail.
Source de vérité MEMORY : `.claude/memory/MEMORY.md` uniquement (versionné Git).
Le hook SessionStart a été supprimé — plus de démarrage automatique.

## Sync template v3.28.0 → v4.2.0 (2026-11-03)

- **Template** : v4.2.0 (`b6626d60`), synchronisé en 4 commits poussés sur `origin/main` : `eb2465ff` (v4.1.0), `52be5d63` (context-audit post-sync), `fa139514` (v4.1.1), `bb744219` (v4.2.0). Teamleader `opus`/`medium` (`settings.json`). Registre des agents (16) et nouveau registre des contextes (14, `contexts.registry[]`) à v4.2.0.
- **v4 — cadrage par milestone** : `milestone.defaults` (patch léger, minor/major complet) ajouté à `project-config.json`. Spawn : `socle` = planner + deployer au `/start-session` ; `milestone` = dev-*, test-writer, code-reviewer, qa, doc-updater (team du milestone validée au GATE 2) ; `ponctuel` = security, infra, marketing. Table « Agents Disponibles » de CLAUDE.md réécrite en conséquence.
- **Agent marketing** : nom canonique `marketing` (fichier inchangé `marketing-release.template.md` + compagnon `marketing-release.md`).
- **Licence** (décision utilisateur 2026-11-03) : GPL-3.0-or-later, titulaire `ccoupel`, 2026. `LICENSE` créé, `license` dans `server-go/web/package.json` et `project-config.json`, section Licence du README remplacée (était « Projet privé - Tous droits réservés »). Dépôt GitHub public. Rappel : GPL-3.0 impose de pouvoir réinstaller un firmware modifié sur le buzzer.
- **PRIVATE/** (v4.2.0) : dossier gitignoré créé, section « Éléments privés » ajoutée en fin de CLAUDE.md. Étude : aucun secret réel tracké (clés IA vides ; Wi-Fi/AP par défaut des `config.json` conservés, choix utilisateur). `project-config.json` → `private.items` : `PRIVATE/ACCES.md` attendu (notes d'accès, à fournir).
- **Incident** : `.git/index.lock` orphelin (14:57, aucun process git) supprimé — deuxième occurrence après le 2026-10-27 ; à surveiller.
- **Écarts laissés** (context-audit) : liste « Nommage des Agents » du bloc TEAMLEADER_PROTOCOL contient `dev-plugin` et omet `marketing` → à corriger dans le dépôt du template (bloc écrasé à chaque sync) ; `.claude/agents/git-squash-merge.md` hors table Agents Disponibles (agent autonome, conservé).
- **Session** : sans team (pas de `/start-session`) ; tests/build non joués (aucun code applicatif touché). Aucune branche résiduelle (seules `main`, `gh-pages`). 18 stashes historiques toujours non relus.
- **Prochain** : choisir un milestone ouvert (v11.3, v12.0, v12.1, v13.0, v15.0...) ; backlog #259, #280, #281.

## État précédent (2026-10-16) — v11.2.3 en PROD (archivé)

- **PROD = v11.2.3** « Tests et sécurité » : merge `main` `46b7fdc8` (`--no-ff`), tag `v11.2.3`, Release https://github.com/CCoupel/BuzzMaster/releases/tag/v11.2.3 (4 assets), `release.yml` run 38784948251 vert. `main` = `6c325442` (notes + posts `docs/releases/v11.2.3/`). Milestone #41 clôturé 5/5, branche `milestone/v11.2.3` supprimée. Branche courante : `main`. Seules branches restantes : `main`, `gh-pages`.
- **Livré** (3 lots, chacun validé au GATE LOT, QUALIF 11.2.3.1 → .3) : LOT-1 #273 (vitest étape [2/7] de `scripts/ci.sh`, bloquant en CI ; 144 s en CI) + #274 (dials de test attendent l'inscription via `waitRegistered`, 40 `Sleep` retirés, `-race -count=20` sous hook 20 ms) ; LOT-2 #278 (`POST /questions` refusé → aucun dossier/fichier, son validé avant les médias, purge de secours, id libéré) + #275 (`NETWORK` dans `AdminOnlyGameFields` : TV/anim/VJoueur sans `NETWORK`, miroir `NETWORK_ONLY_LOCALHOST` diffusé à tous) ; LOT-3 #277 (react-router-dom 7.18.4, imports `react-router-dom` conservés, 0 fichier source, 2 tests adaptés à `startTransition`, procédure manuelle `tests/procedures/router-277.md` validée 8/8 par l'utilisateur).
- **Site gh-pages** : volontairement laissé en v11.2.2 (décision utilisateur, GATE 4d : pas de nouvelle fonctionnalité, pas de badge). Posts réseaux à publier par l'utilisateur.
- **Backlog créé** : #280 durée vitest en CI (`pool: vmThreads` à qualifier), #281 `ai_job.go` laisse un dossier vide après échec d'écriture (même garantie que #278 attendue). Backlog restant sans milestone : #259 (sécurité endpoints), #209 (WLED). Milestones ouverts sans échéance : v11.3 (#36), v12.0/v12.1, v13.0, v14.0, v15.0, vX (DNS), vx (scoring MEMORY).
- **Branches supprimées 2026-10-15** (décision utilisateur) : `chore/template-v3.8.0`, `spike/226-audio-bluetooth` (local + remote). Commits non mergés tracés dans le rapport de session ; le commit local `1335ba13` « Record spike #226 verdict and accepted Pi risk » n'était sur aucun remote, récupérable par SHA via reflog tant qu'il n'est pas purgé. 18 stashes historiques toujours non relus.
- **Template** : v4.2.0 (`b6626d60`, voir « Sync template v3.28.0 → v4.2.0 ») — modèle/effort résolus au spawn (`TEAMMATES_PROTOCOL.md` §7), `settings.json` teamleader `opus`/`medium`, code-reviewer et infra en `medium`, **doc-updater suit le template `haiku`/`low`** (écart `sonnet` levé après revalidation v11.2.2, note du compagnon réécrite `cf7adad5`) — aucun DONE oublié sous Haiku en v11.2.3. Audit contexte commité `1dec1870` (project-config version 11.2.2 → à passer à 11.2.3 au prochain audit ; MEMORY archives → `ARCHIVE.md`).
- **Environnement agents (permanent)** : `go` hors PATH (`export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin`, govulncheck dans `~/go/bin`) ; `npm` du PATH = npm Windows → worktrees jetables **sous `/mnt/c`** uniquement (un worktree sous `/tmp` casse le build vite) ; `actionlint` absent. Mémoire auto `project_go_path_agents`.
- **Leçons v11.2.3 (permanentes)** :
  1. Un agent qui exécute `go test`/`scripts/ci.sh` dans le dépôt principal ramasse le travail non commité des autres (hook temporaire de la QA, http.go en cours) : pendant une QA/un build, les devs travaillent dans un worktree jetable sous `/mnt/c` sur une branche `tmp/*`, puis cherry-pick après le verdict (fait pour le correctif #278, sans conflit).
  2. Label `DONE` dès QA validée, fermeture au GATE LOT + CI verte : j'avais retiré `EN QA` sans poser `DONE` (issue sans phase, signalé par l'utilisateur) — appliquer la règle telle qu'écrite.
  3. Le planner ne pose pas les labels de lot : deployer les pose à l'ouverture du cycle (`PRET` + `LOT-N`), avec la branche et la version `X.Y.Z.0` (seul `server-go/config.json` à l'ouverture ; les autres fichiers de version au bump PROD).
  4. Revue à chaque lot = valeur réelle : 2 majeurs trouvés (script de porte sous `/tmp`, refus `sound` après écriture des médias) et un test « exerçant la purge » prouvé RED en neutralisant la purge.
  5. test-writer doit compiler ses tests (RED compilable) : à la première livraison il ne trouvait pas `go` ; préciser le PATH dans chaque ordre.
  6. Durée vitest en CI : 176 s (LOT-1), 120 s, 144 s — variable, critère « ≤ 2 min » non tenu → #280.

## État précédent (2026-10-15) — v11.2.2 en PROD (archivé)

- **PROD = v11.2.2** : merge `main` `7d8f4fd8` (`--no-ff`), tag `v11.2.2`, Release https://github.com/CCoupel/BuzzMaster/releases/tag/v11.2.2 (4 assets), run `release.yml` 38731898818 vert — premier run avec les actions Node 24 (#276). `main` = `4ba9f401` (release notes cherry-pickées). Milestone v11.2.2 (#40) fermé 11/11, branche `milestone/v11.2.2` supprimée (local + remote). Branche courante : `main`. Installation PROD manuelle (DEPLOY PROD = no-op). Dernière QUALIF : 11.2.2.6 (`build/qualif_v11.2.2/`).
- **Livré en v11.2.2** (6 lots, tous validés manuellement par l'utilisateur en QUALIF) : LOT-1 #266 #268 #269 #271 #272 (tests/CI/deps), LOT-2 #270 (vite 8 / vitest 5, porte npm audit), LOT-3 #267 (alerte plein écran sans réseau), LOT-4 #221 (CategoryPicker / DifficultyPicker, 15 sites, garde #197 interne) + #276 (actions GitHub v7/v8, Go lu depuis `server-go/go.mod` + `GOTOOLCHAIN=local`, cache Go, runner `ubuntu-24.04`), LOT-5 #190 (POINTS_RULE par carte MEMOTION : Étoiles / Montant fixe / Par unité / Prorata / Étoiles pleines, validation 400 serveur, parité anim/admin, grilles anim/TV en ★ seules), LOT-6 #279 (cartes QCM MEMOTION sans indices : clés ignorées et retirées à l'enregistrement, moteur sans `QCM_HINT` en carte).
- **Décisions utilisateur 2026-10-15** (tracées `docs/mockup/DECISIONS.md` § admin_question_editor) : parité des montants limitée aux surfaces affichant déjà un montant ; principe « pas d'indice sans effet sur les points » → indices interdits sur les cartes (option « ignorer silencieusement », non BREAKING, contrat §7.1 `d554f017`).
- **Marketing** : site gh-pages `acf1ae35` (hero v11.2.2 + 2 cartes « Nouveau v11.2.2 »), validé a posteriori par l'utilisateur ; notes + posts `docs/releases/v11.2.2/` (sur main). Posts réseaux à publier par l'utilisateur. ⚠️ Écart de process : site publié sans aperçu ni GATE 4d (mémoire auto `feedback_marketing_gate_4d`) — ne plus fusionner PREPARE et PUBLISH.
- **Backlog créé** : #277 react-router 2 avis moderate (migration v6→v7, barrière PROD `high` non déclenchée), #278 `POST /questions` laisse un dossier vide après un 400 (préexistant). #259 (sécurité endpoints), #273, #274, #275 toujours en backlog sans milestone.
- **Gouvernance Go (#276)** : la CI n'installe plus automatiquement le dernier patch 1.26.x ; un correctif stdlib signalé par `govulncheck` impose un bump de la directive `go` dans `server-go/go.mod` (`docs/GO_SERVER.md` « Chaîne d'outils CI »). Prochain point d'attention : `ubuntu-latest` → Ubuntu 26 le 2026-10-19 n'affecte plus les workflows (runner épinglé).
- **Template** : v3.19.1 (`ab7561c0`), mod `milestone-panel` avec consommation du compte en tête du panneau. Audit contexte : `docs/TEST_PROCEDURE.md` §2.3 (tests RED en clone partagé) ajouté — commité ou non selon la décision de fin de session du 2026-10-15.
- **Branches distantes non mergées, conservées (à trancher)** : `origin/chore/template-v3.8.0`, `origin/spike/226-audio-bluetooth`. 18 stashes historiques (v2.x à v7.0.0), jamais relus.
- **Leçons v11.2.2 (permanentes)** :
  1. Index git partagé : `git commit -- <chemins>` obligatoire (incident test-writer / dev-frontend, rien perdu) — mémoire auto `feedback_commit_scoped_paths`.
  2. Relecture factuelle des docs par code-reviewer à chaque lot : 3 erreurs trouvées au LOT-4 (dont une affirmation produit fausse), 1 au LOT-6, 1 au CHANGELOG complet ; re-relecture après correction systématique.
  3. Tests Go de test-writer par la frontière HTTP uniquement (`docs/TEST_PROCEDURE.md` §2.3) : aucune casse de compilation sur 3 lots.
  4. Un test visuel reste à la charge de l'utilisateur ; lui transmettre binaire + procédure `tests/procedures/` à chaque GATE LOT.
  5. Modèle des agents : depuis le template v3.21.1, `model`/`effort` sont résolus au spawn (compagnon > template, `TEAMMATES_PROTOCOL.md` §7). Décision 2026-10-15 : doc-updater suit le template (`haiku`/`low`), l’override `model: sonnet` du compagnon a été retiré ; aucun DONE oublié en v11.2.2 sous Haiku.
## Sync template v3.17.0 (2026-10-12)

- **Template passé de v3.14.2 à v3.17.0** (commits locaux `65747c48` + `d32d9ddf`, aucun push). Nouveaux contextes `CDP_PHASES_{PLAN_DEV,RELEASE,REVIEW_QA}`, bloc `TEAMLEADER_PROTOCOL` à jour (section « Labels d'issues » : le teamleader fait poser les labels par `deployer`). `/context-audit` : seules des références mortes de ce fichier ont été retirées.
- **Mod `milestone-panel`** (`.claude/skills/milestone-panel/`, gitignoré, copié depuis `TEMPLATE_claude/mods/`) : `session.start` sans garde, donc chaque session qui le charge ouvre le panneau et lance un timer `gh` toutes les 300 s. **Problème ouvert** : l'utilisateur veut qu'il ne se charge que pour le teamleader ; l'API de `session.start` ne distingue pas un teammate (seulement `cwd`, `surface`, `isInteractive`). Le hook `tool.call` doit rester ouvert aux teammates (les `gh issue edit` passent par `deployer`). La correction doit être reportée dans le dépôt du template, `.claude/skills/` étant régénéré à chaque sync.
- **Mise à jour 2026-10-12** : template synchronisé en **v3.17.2** (`template_version` dans `project-config.json`, commit `cf03b5ce`) ; le mod `milestone-panel` détecte désormais les sessions teammate (variables d'env + flags de lancement) — le problème ci-dessus est traité côté template.

## Sync template v3.14.1 (2026-10-10)

- **Template passé de v3.9.1 à v3.14.1** (commits locaux `42094004` + audit), aucun push. Aucune vraie customisation de commande perdue ; `init-project.md` pris au tag. Bloc `TEAMLEADER_PROTOCOL` de CLAUDE.md à jour : ordres > 3 lignes via `_work/tasks/*.md`, questions utilisateur toujours via `AskUserQuestion`.
- **Adresse du teamleader = `team-lead`**, jamais `main` (rejeté). Corrigé côté template depuis v3.26.1 (`TEAMMATES_PROTOCOL.template.md` impose `to: "team-lead"`) ; l'ancien compagnon projet `.claude/agents/context/TEAMMATES_PROTOCOL.md` a été supprimé à cette sync.
- **Squelettes créés** : `docs/mockup/{INDEX,DECISIONS}.md` (les 19 maquettes historiques restent dans `docs/mockups/`, non indexées — choix utilisateur), `tests/INDEX.md`, `tests/METRICS.md`; `project-config.json` : `docs.mockup_dir` + défauts `testing.*`. `marketing-release` ajouté au tableau Agents de CLAUDE.md.
- **Gitignore** : ajout `_work/`, `.claude/worktrees/`, `build/`. Fichier `data/WEAPSwrd_Epee…wav` toujours non suivi (hors sujet).
- Aucun chantier ouvert ; prochain : choisir dans le backlog (v11.2 bruitages #231/#232/#235, ou bugs #189/#242).

## Archives

États antérieurs au 2026-10-10 (v11.0.0 → v11.1.0, chantiers d’août-septembre 2026) : `.claude/memory/ARCHIVE.md`.
