# Mémoire Claude - BuzzControl

> Règles techniques et workflows : `.claude/commands/context/COMMON.template.md` (chargé automatiquement par les commandes)
> Architecture projet : `CLAUDE.md` (chargé automatiquement)

## Démarrage de session

Utiliser `/start-session` pour démarrer chaque session : crée la TEAM de travail.
Source de vérité MEMORY : `.claude/memory/MEMORY.md` uniquement (versionné Git).
Le hook SessionStart a été supprimé — plus de démarrage automatique.

## État courant (2026-10-16) — v11.2.3 en PROD

- **PROD = v11.2.3** « Tests et sécurité » : merge `main` `46b7fdc8` (`--no-ff`), tag `v11.2.3`, Release https://github.com/CCoupel/BuzzMaster/releases/tag/v11.2.3 (4 assets), `release.yml` run 38784948251 vert. `main` = `6c325442` (notes + posts `docs/releases/v11.2.3/`). Milestone #41 clôturé 5/5, branche `milestone/v11.2.3` supprimée. Branche courante : `main`. Seules branches restantes : `main`, `gh-pages`.
- **Livré** (3 lots, chacun validé au GATE LOT, QUALIF 11.2.3.1 → .3) : LOT-1 #273 (vitest étape [2/7] de `scripts/ci.sh`, bloquant en CI ; 144 s en CI) + #274 (dials de test attendent l'inscription via `waitRegistered`, 40 `Sleep` retirés, `-race -count=20` sous hook 20 ms) ; LOT-2 #278 (`POST /questions` refusé → aucun dossier/fichier, son validé avant les médias, purge de secours, id libéré) + #275 (`NETWORK` dans `AdminOnlyGameFields` : TV/anim/VJoueur sans `NETWORK`, miroir `NETWORK_ONLY_LOCALHOST` diffusé à tous) ; LOT-3 #277 (react-router-dom 7.18.4, imports `react-router-dom` conservés, 0 fichier source, 2 tests adaptés à `startTransition`, procédure manuelle `tests/procedures/router-277.md` validée 8/8 par l'utilisateur).
- **Site gh-pages** : volontairement laissé en v11.2.2 (décision utilisateur, GATE 4d : pas de nouvelle fonctionnalité, pas de badge). Posts réseaux à publier par l'utilisateur.
- **Backlog créé** : #280 durée vitest en CI (`pool: vmThreads` à qualifier), #281 `ai_job.go` laisse un dossier vide après échec d'écriture (même garantie que #278 attendue). Backlog restant sans milestone : #259 (sécurité endpoints), #209 (WLED). Milestones ouverts sans échéance : v11.3 (#36), v12.0/v12.1, v13.0, v14.0, v15.0, vX (DNS), vx (scoring MEMORY).
- **Branches supprimées 2026-10-15** (décision utilisateur) : `chore/template-v3.8.0`, `spike/226-audio-bluetooth` (local + remote). Commits non mergés tracés dans le rapport de session ; le commit local `1335ba13` « Record spike #226 verdict and accepted Pi risk » n'était sur aucun remote, récupérable par SHA via reflog tant qu'il n'est pas purgé. 18 stashes historiques toujours non relus.
- **Template** : v3.21.1 (`2f1f3374`, commit `f378e523`) — modèle/effort résolus au spawn (`TEAMMATES_PROTOCOL.md` §7), `settings.json` teamleader `fable`/`medium`, code-reviewer et infra en `medium`, **doc-updater suit le template `haiku`/`low`** (écart `sonnet` levé après revalidation v11.2.2, note du compagnon réécrite `cf7adad5`) — aucun DONE oublié sous Haiku en v11.2.3. Audit contexte commité `1dec1870` (project-config version 11.2.2 → à passer à 11.2.3 au prochain audit ; MEMORY archives → `ARCHIVE.md`).
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

## État précédent (2026-10-13) — v11.2.1 en PROD (archivé)

- **PROD = v11.2.1** : merge `main` `01e36339`, tag `v11.2.1`, Release https://github.com/CCoupel/BuzzMaster/releases/tag/v11.2.1 (exe Windows, linux-arm64, firmware merged, SHA256SUMS). Dernière QUALIF : 11.2.1.1 (`f97600c5`, `build/qualif_v11.2.1/`). Installation PROD manuelle (DEPLOY PROD = no-op). Milestone v11.2.1 (#39) fermé 8/8, branche `milestone/v11.2.1` supprimée (local + remote). Branche courante : `main`.
- **Livré** (toutes validées manuellement par l'utilisateur avant fermeture) : #261 arrêt idempotent (plus de panic `/shutdown`) ; #247 PAUSE seulement sur appui accepté, une seule fois, ARDOISE gardé par le moteur, écho ciblé sur buzz SPEEDY refusé d'un VJoueur ; #189 AnimAnswerZone en MEMOTION REVEAL ; #242 a11y navbar ; #188/#264 tests stabilisés ; #248 accusé + renvoi tablette (QCM, SPEEDY via `TIME`, retournement MEMORY/MEMOTION via `FLIP_ID` + `FLIP_MEMORY_CARD_ACK`, hook `useAckedSend`) ; #265 attribution multi-équipes en feu d'artifice par lots de 6, tablette calée sur la TV (`SCORE_CHANGE.GAINS`, module `pointsAwardLots.js`). Contrats : `contracts/game-state.md` (GAINS), `contracts/websocket-actions.md` (FLIP_ID/ACK). Maquettes validées : `docs/mockup/v11.2.1/ui/`.
- **Marketing** : site gh-pages `0164f3e0` (2 cartes « Nouveau v11.2.1 ») ; notes + posts `docs/releases/v11.2.1/` (commit `f071ef56` sur main). Posts réseaux à publier par l'utilisateur. Le worktree `MARKETING/` était cassé (`.git` vers un ancien chemin), recréé depuis `origin/gh-pages`. Le site n'a jamais reçu de communication v11.2.0 (rattrapage proposé, non décidé).
- **Commits locaux non poussés sur main** : `f071ef56`, `98a3cd8f` + `4406d42b` (promotion en `regression` des tests v11.2.0 restés en `feature`, oubli de la release précédente — plus aucune ligne `feature` dans `tests/INDEX.md`).
- **Branches distantes non mergées, conservées (à trancher par l'utilisateur)** : `origin/chore/template-v3.8.0` (2 commits), `origin/spike/226-audio-bluetooth` (3 commits).
- **Prochain : milestone v11.2.2 (#40, aucune branche)** : #267 alerte plein écran admin quand le serveur n'a pas de réseau (localhost seulement — l'utilisateur a demandé de ne PAS démarrer son dev, cadrage à faire), #266 fenêtres fixes restantes des tests `cmd/server`, #268 react-router (3 « high » `npm audit` non atteignables) + ajouter `npm audit` à `commands.audit`, #269 data race de test (timer résiduel `h246Setup` vu sur la CI de release), #221 composant catégorie/difficulté partagé, #190 barème `POINTS_RULE` par carte MEMOTION. #259 (sécurité endpoints) reste hors milestone.
- **Leçons v11.2.1 (permanentes)** :
  1. Au démarrage d'un milestone, faire cadrer par le planner les lots, dépendances et parallélisme AVANT tout dev, et poser un label `LOT-<entier>` par issue (demande utilisateur).
  2. Clone partagé : un agent ne fait jamais `git stash`/`checkout`/`reset` dans le dépôt principal (dev-backend et test-writer l'ont fait, rien perdu) ; comparaison = worktree jetable par chemin absolu. Un test Go qui référence un champ inexistant casse la compilation du paquet pour tous : écrire les tests RED via JSON/messages bruts, ou séparer les paquets entre agents.
  3. La relecture factuelle du CHANGELOG par code-reviewer a encore trouvé 7 erreurs (dont une introduite par la relecture elle-même) : la garder systématique, avec re-relecture.
  4. `npm audit` n'était pas dans les contrôles pré-PROD : ajouté via #268.
  5. Ne pas mettre une petite action (créer une issue, un milestone) en file derrière une tâche longue d'un agent : la lancer en parallèle (agent ponctuel).
  6. Une QA « validée » sans navigateur laisse le visuel à l'utilisateur : toujours lui fournir les procédures `tests/procedures/` avant de proposer la fermeture.

## État précédent (2026-10-12) — v11.2.0 en PROD (archivé)

- **PROD = v11.2.0** : merge `main` `9fdb7f62`, tag `v11.2.0`, Release https://github.com/CCoupel/BuzzMaster/releases/tag/v11.2.0, CI run 38250528708. Dernier candidat QUALIF : 11.2.0.9 (`build/qualif_v11.2.0/`). Milestone v11.2 clôturé (18 issues), branche `milestone/v11.2` supprimée. Branche courante : `main`.
- **Livré en v11.2** : #243, #244, #245 (manches, entractes), #246, #249 à #258, #233/#257 (tests Go stabilisés), #260 (govulncheck : Go 1.26.9, x/net v0.60.0, gorilla/websocket v1.5.3), #262/#263 (tests fragiles sur le runner CI).
- **À venir — milestone v11.2.1** (aucune branche créée) : #265 animation d'attribution de points à plusieurs équipes (cadrage et maquettes à faire) ; #264 et #188 tests instables ; #261 panic d'arrêt `close of closed channel` ; #247, #248, #189 bugs ; #242 a11y navbar. #259 (endpoints sans authentification) : milestone dédié ultérieur, non planifié.
- **Leçons de release (permanentes)** :
  1. Release PROD : `ci-branch.yml` doit être vert sur le commit exact avant tout merge dans `main` et tout tag ; `govulncheck` propre avant le tag (`docs/RELEASE_PROCEDURE.md` §9bis, `.claude/agents/environments/publish.prod.md`, `.claude/agents/qa.md`). Un tag ne sert jamais de test.
  2. Quatre tentatives pour 11.2.0 : govulncheck (Go 1.25.14), `TestLingerListener_SetsLinger`, `TestDNSServer_Start_UsesConfiguredPort`.
  3. Un revert de merge impose d'annuler le revert avant de re-merger.
  4. Go 1.26.9 minimum. `release.yml` ne publie que Windows, Linux arm64 et le firmware.
  5. Tests : pas de `time.Sleep` d'attente, pas de singleton `config` en parallèle, attentes par événement avec échéance large (`docs/DEV_COMMANDS.md`).
  6. L'utilisateur transmet lui-même aux templates la spécification « script CI unique, workflow de pré-validation, barrière du deployer » : à suivre côté dépôt des templates.
- **Environnements** : PROD = v11.2.0 ; QUALIF = 11.2.0.9.

## Sync template v3.17.0 (2026-10-12)

- **Template passé de v3.14.2 à v3.17.0** (commits locaux `65747c48` + `d32d9ddf`, aucun push). Nouveaux contextes `CDP_PHASES_{PLAN_DEV,RELEASE,REVIEW_QA}`, bloc `TEAMLEADER_PROTOCOL` à jour (section « Labels d'issues » : le teamleader fait poser les labels par `deployer`). `/context-audit` : seules des références mortes de ce fichier ont été retirées.
- **Mod `milestone-panel`** (`.claude/skills/milestone-panel/`, gitignoré, copié depuis `TEMPLATE_claude/mods/`) : `session.start` sans garde, donc chaque session qui le charge ouvre le panneau et lance un timer `gh` toutes les 300 s. **Problème ouvert** : l'utilisateur veut qu'il ne se charge que pour le teamleader ; l'API de `session.start` ne distingue pas un teammate (seulement `cwd`, `surface`, `isInteractive`). Le hook `tool.call` doit rester ouvert aux teammates (les `gh issue edit` passent par `deployer`). La correction doit être reportée dans le dépôt du template, `.claude/skills/` étant régénéré à chaque sync.
- **Mise à jour 2026-10-12** : template synchronisé en **v3.17.2** (`template_version` dans `project-config.json`, commit `cf03b5ce`) ; le mod `milestone-panel` détecte désormais les sessions teammate (variables d'env + flags de lancement) — le problème ci-dessus est traité côté template.

## Sync template v3.14.1 (2026-10-10)

- **Template passé de v3.9.1 à v3.14.1** (commits locaux `42094004` + audit), aucun push. Aucune vraie customisation de commande perdue ; `init-project.md` pris au tag. Bloc `TEAMLEADER_PROTOCOL` de CLAUDE.md à jour : ordres > 3 lignes via `_work/tasks/*.md`, questions utilisateur toujours via `AskUserQuestion`.
- **Adresse du teamleader = `team-lead`**, jamais `main` (rejeté). Corrigé durablement par le compagnon `.claude/agents/context/TEAMMATES_PROTOCOL.md` (le template, lui, dit encore `main`). Le bug signalé à la session v11.1.0 est donc traité côté projet.
- **Squelettes créés** : `docs/mockup/{INDEX,DECISIONS}.md` (les 19 maquettes historiques restent dans `docs/mockups/`, non indexées — choix utilisateur), `tests/INDEX.md`, `tests/METRICS.md`; `project-config.json` : `docs.mockup_dir` + défauts `testing.*`. `marketing-release` ajouté au tableau Agents de CLAUDE.md.
- **Gitignore** : ajout `_work/`, `.claude/worktrees/`, `build/`. Fichier `data/WEAPSwrd_Epee…wav` toujours non suivi (hors sujet).
- Aucun chantier ouvert ; prochain : choisir dans le backlog (v11.2 bruitages #231/#232/#235, ou bugs #189/#242).

## Archives

États antérieurs au 2026-10-10 (v11.0.0 → v11.1.0, chantiers d’août-septembre 2026) : `.claude/memory/ARCHIVE.md`.
