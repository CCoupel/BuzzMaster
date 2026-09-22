# Procédure de Test — Question sonore (#219, milestone v11.1)

**Version** : v11.1.0.x (QUALIF)
**Date** : 2026-09-22
**Testeur** : Utilisateur (validation auditive — aucun navigateur/enceinte fiable côté agents)
**Contrat** : `contracts/sound.md` §10
**Plan** : `_work/reports/plan-20260922-103848.md` (révision 3)
**Maquette normative** : `docs/mockups/question-sound-219.html` (révision 3) — **les deux machines à
états du §03 (le son, le chronomètre de réponse différé) sont la source de vérité de cette
procédure.**

## Important — pourquoi cette procédure ne peut être exécutée que par l'utilisateur

Aucun test automatisé ne peut constater qu'un son a été **entendu**, ni qu'un chronomètre figé à
l'écran est réellement perçu comme « en attente » plutôt que comme une panne. `qa` et `deployer`
n'exécutent **jamais** cette procédure (règle projet) : ici il n'y a ni navigateur fiable ni
enceinte dans leur environnement d'exécution. C'est le **seul filet** avant PROD pour le rendu
sonore et pour le risque R10/R11 (chronomètre figé pris pour une panne).

## Prérequis

- [ ] Environnement : QUALIF (poste avec une sortie audio réelle — obligatoire, contrairement aux
      procédures purement logicielles de ce projet)
- [ ] Binaire buildé depuis la branche `milestone/v11.1` (ou merge ultérieur), toutes les Phases 0
      à 3 livrées (socle audio, contrats/upload, chronomètre différé, frontend)
- [ ] Jeu de données : au moins
  - [ ] 1 question **SPEEDY** avec un son valide (WAV canonique, ~10-15 s), **chronomètre
        simultané** (réglage par défaut)
  - [ ] 1 question **QCM** avec un son valide (~20-24 s), **chronomètre différé**
  - [ ] 1 question **ARDOISE** avec un son de **exactement 30 s** (durée maximale, CA7)
  - [ ] 1 question **SPEEDY ou QCM sans aucun son** (non-régression, CA9)
  - [ ] 1 question intégrée dans une **manche RAFALE** (au moins 2 questions), l'une d'elles
        portant un son
  - [ ] Fichiers de test prêts hors serveur : un `.mp3` quelconque, un `.wav` en 48 kHz mono, un
        `.wav` de plus de 30 s (ex. 42 s) — pour le Scénario 2
- [ ] Accès : `/questions` (édition), `/anim` (conduite tablette), `/admin` (miroir), `/tv`
      (affichage public), configuration serveur (`config.json`, section son) pour le Scénario 12

## Scénarios

### Scénario 1 — Attacher, écouter, supprimer un son (CA1, CA8)

**Objectif** : vérifier le cycle complet d'édition du champ son, identique dans ses gestes à celui
de l'image (maquette rév. 3 §01).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Ouvrir `/questions`, éditer une question **SPEEDY** | Le bloc « Médias » affiche un champ « Son de la question » sous les images, avec sa mention de format (WAV 44,1 kHz stéréo 16 bits, 30 s max) | | |
| 2 | Choisir un fichier `.wav` conforme (~15 s) | Le fichier apparaît (nom, durée, poids) avec une barre d'écoute locale (`<audio controls>`), lecture possible immédiatement | | |
| 3 | Enregistrer la question, revenir sur `/questions`, ré-ouvrir la même question | Le son est **toujours présent** — même nom de fichier, même durée (CA8 : « survit à une ré-édition ») | | |
| 4 | Modifier un autre champ (ex. le texte) **sans toucher au son**, enregistrer | Le son est **toujours présent** après ce ré-enregistrement (piège R3 : recopie de préservation) | | |
| 5 | Cliquer « Supprimer » sur le son, enregistrer, ré-ouvrir la question | Le champ son est **vide** — suppression réelle côté serveur, y compris le fichier sur disque (CA8) | | |
| 6 | Éditer une question **MEMOTION** ou **ENTRACTE** | **Aucun** champ son n'apparaît dans l'éditeur (hors périmètre v11.1, §0.3 du plan — restriction d'éditeur uniquement) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Refus nommés et avertissement contextuel (CA2)

**Objectif** : vérifier que chaque fichier refusé nomme sa cause exacte, et que l'avertissement
contextuel n'apparaît que quand il est pertinent (maquette rév. 3 §01).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Éditer une question, choisir le fichier `.mp3` de test | Message nommant explicitement le format compressé refusé (« Seul le WAV non compressé est accepté ») | | |
| 2 | Choisir le `.wav` 48 kHz mono de test | Message nommant explicitement fréquence **et** canaux (« 44 100 Hz, stéréo, 16 bits » attendu) | | |
| 3 | Choisir le `.wav` de plus de 30 s (ex. 42 s) | Message nommant la durée réelle et la limite de 30 s | | |
| 4 | Choisir un son de 28 s sur une question dont le temps de réponse (`TIME`) est de 20 s, **en mode simultané** | Avertissement (non bloquant) : le son dure plus longtemps que le temps de réponse, il sera coupé | | |
| 5 | Sur la **même** question, basculer sur « démarrer le chronomètre à la fin du son » | L'avertissement de l'étape 4 **disparaît** (situation qui ne peut pas se produire en mode différé, contract §10.4) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Mode simultané : le son et le chronomètre démarrent ensemble (CA3)

**Objectif** : vérifier la machine à états A (le son) et le couplage simultané par défaut, y compris
les deux pièges de non-régression signalés par le plan (reprise après pause, avance RAFALE).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Depuis `/anim`, LANCER la question SPEEDY à son (mode simultané) | Le son part **et** le chronomètre démarre **au même instant**, à la transition `→ STARTED` | | |
| 2 | PAUSE pendant que le son joue | Le son **et** le chronomètre se figent tous les deux (maquette §02, écran PAUSED) | | |
| 3 | CONTINUER | Le son reprend à la position gardée, le chronomètre reprend — **aucun redémarrage depuis zéro** | | |
| 4 | Lancer la manche RAFALE contenant une question à son, avancer d'une question à l'autre de la rafale | Le son **ne redémarre pas** à chaque avance (seulement au vrai lancement de la question qui le porte) — non-régression R5 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Mode différé : fin naturelle du son (CA4, CA15)

**Objectif** : vérifier la machine à états B (le chronomètre différé) sur son chemin nominal — fin
naturelle du son, chronomètre figé puis libéré (maquette rév. 3 §02-§03).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | LANCER la question QCM à son en mode différé | Le son part, **le chronomètre reste figé au temps plein** (aucun décompte) | | |
| 2 | Observer `/anim` pendant que le son joue | Mention explicite « le chronomètre démarrera à la fin du son » (jamais un chiffre figé sans explication, CA15) | | |
| 3 | Observer `/admin` en parallèle | **Même mention** — cohérente avec `/anim` | | |
| 4 | Observer `/tv` en parallèle | **Même mention** présente sur l'affichage public, **sans ajouter de hauteur** au flux (TV statique, `overflow: hidden`) | | |
| 5 | Laisser le son se terminer naturellement (sans intervention) | Le chronomètre **démarre exactement à la fin du son**, la mention d'attente disparaît | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Mode différé : arrêt manuel du son (CA4, CA13)

**Objectif** : vérifier l'autre évènement qui libère le chronomètre différé — l'arrêt manuel par
l'animateur — et la règle « Rejouer ne regèle jamais ».

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | LANCER une question à son en mode différé | Chronomètre figé, mention d'attente affichée | | |
| 2 | Cliquer **Stop** (geste animateur) avant la fin naturelle du son | Le son s'arrête **et** le chronomètre démarre **immédiatement** (fin manuelle = fin naturelle, contract §10.7) | | |
| 3 | Cliquer **Rejouer** une fois le chronomètre déjà en marche | Le son rejoue depuis le début, **le chronomètre ne se regèle pas** — il continue de décompter sans interruption (CA13) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Mode différé : libération pendant une pause de jeu (CA14)

**Objectif** : vérifier la règle « si le son se termine pendant une pause, le chronomètre démarre à
la reprise, jamais pendant la pause » (maquette §03, tableau de vérité).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | LANCER une question à son en mode différé | Son en lecture, chronomètre figé | | |
| 2 | Cliquer **PAUSE** (pause du jeu) pendant que le son joue encore | Le son se met en pause (maquette §02, écran PAUSED), le chronomètre reste figé | | |
| 3 | Depuis `/anim`, arrêter le son manuellement (**Stop**) **pendant que le jeu est toujours en pause** | Le son passe à IDLE, mais le chronomètre **ne démarre pas** — il reste en attente tant que le jeu est en pause | | |
| 4 | Cliquer **CONTINUER** (reprise du jeu) | Le chronomètre démarre **à cet instant précis**, jamais avant (CA14) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Les trois gestes de conduite (CA5)

**Objectif** : vérifier la présence conditionnelle et la synchronisation serveur→interface des trois
gestes (Rejouer / Pause / Stop), sur `/anim` et son miroir `/admin`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Éditer/lancer une question **sans son** | La rangée des trois gestes n'apparaît **pas du tout** sur `/anim` (pas grisée — absente, maquette §02) | | |
| 2 | Lancer une question **avec** son | La rangée apparaît, avec l'état courant (`en lecture · 0:MM / 0:SS`) | | |
| 3 | Cliquer **Pause** | Bouton Pause devient Reprendre, l'état affiché passe à `en pause`, **cohérent entre `/anim` et `/admin`** ouverts simultanément | | |
| 4 | Cliquer **Stop** | Les trois gestes retombent à l'état « au repos » (Rejouer actif, Pause/Stop inactifs, maquette §02 dernier écran) | | |
| 5 | Fermer `/anim` sur un poste, rouvrir | L'état affiché correspond **exactement** à l'état réel diffusé par le serveur (jamais un état local React obsolète, R4) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Mixage avec un bruitage pendant la lecture (CA6)

**Objectif** : vérifier que les deux chemins audio (média et bruitages) coexistent sans
s'interrompre.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | LANCER une question à son (mode simultané, ~15-20 s) | Le son de la question part | | |
| 2 | Pendant que le son joue, faire buzzer une équipe avec une réponse correcte | Le bruitage « gagné » sort **immédiatement**, sans attendre la fin du média | | |
| 3 | Écouter le résultat | Les **deux sons se mélangent** (audibles simultanément) — le média **n'est pas interrompu**, il continue jusqu'à sa fin normale | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 9 — Durée réelle de 30 s jouée en entier (CA7)

**Objectif** : vérifier que le plafond de 10 s du moteur de cues ne s'applique **pas** au second
chemin média.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | LANCER la question ARDOISE dont le son dure exactement 30 s | Le son joue **intégralement**, chronométrer à la main ou via la barre `/anim` (0:00 → 0:30) | | |
| 2 | Vérifier qu'aucune coupure/troncature n'intervient vers 10 s | Aucune coupure — le plafond de 10 s (`otoPlayMaxWait`, chemin des cues) ne s'applique qu'aux bruitages, jamais au média | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 10 — Non-régression sans son (CA9)

**Objectif** : vérifier qu'une question sans son se comporte **au bit près** comme avant #219.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Lancer la question de test **sans son** | Aucun bouton de conduite son n'apparaît, aucun log lié au son, comportement identique à une version pré-#219 | | |
| 2 | Enchaîner une partie complète (plusieurs types de questions) sans jamais toucher au son | Aucune différence perceptible avec le comportement d'avant cette version | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 11 — Les trois dégradations du mode différé ne figent jamais la question (CA12, ⚠️ le plus important)

**Objectif** : prouver la règle de non-blocage (contract §10.7) : dans les trois cas listés, le mode
différé dégrade **immédiatement** vers le mode simultané — la partie ne se bloque jamais en direct.

> ⚠️ Ce scénario est le point de revue n°1 du lot (risque R10, plan §11) : un échec ici est
> **critique**, pas une simple anomalie visuelle.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | **Enceinte indisponible** : débrancher/couper la sortie audio du serveur (ou lancer sur un poste sans périphérique audio), LANCER une question à son en mode différé | Le chronomètre démarre **immédiatement** — la question reste jouable, aucun blocage, aucune erreur visible | | |
| 2 | **Fichier illisible** : sur une question en mode différé, altérer/supprimer manuellement le fichier `.wav` sur le disque serveur sans passer par l'éditeur, puis LANCER la question | Le chronomètre démarre **immédiatement** — même comportement | | |
| 3 | **Son désactivé** : couper `sound.enabled` dans la configuration serveur, redémarrer, LANCER une question à son en mode différé | Le chronomètre démarre **immédiatement** — même comportement | | |
| 4 | Pour chacun des 3 cas ci-dessus, vérifier `/anim`/`/admin`/`/tv` | **Aucune** mention « le chronomètre démarrera à la fin du son » ne reste affichée alors que le chronomètre tourne déjà (pas de mention mensongère) | | |
| 5 | (Optionnel, robustesse) Simuler une lecture qui ne se termine jamais (ex. couper le processus de lecture sans le signaler) et attendre `MaxQuestionSoundDuration + 2 s` (~32 s) | Le chien de garde libère le chronomètre **au plus tard** à cette échéance | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 12 — Non-régression MEMOTION/ENTRACTE (CA16)

**Objectif** : vérifier que le chronomètre global ne démarre toujours pas sur les types hors
périmètre, quel que soit l'état du différé introduit par #219.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Lancer une carte MEMOTION (chronomètre par carte, existant) | Comportement identique à avant #219 — le chronomètre par carte fonctionne normalement, **aucun** chronomètre global ne se déclenche | | |
| 2 | Lancer un ENTRACTE | Aucun chronomètre, comportement identique à avant #219 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 13 — Suite automatisée (exécution QA)

**Objectif** : faire exécuter par `qa` la partie automatisable du lot, en complément de cette
procédure auditive.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | `cd server-go && go build ./...` | Compile sans erreur | | |
| 2 | `go test ./internal/audio/... -v` | 100 % vert, y compris les tests-gardes #228/#229/#230 rejoués sans modification (CA10) | | |
| 3 | `go test ./internal/audio/... -race` | Vert — aucune race détectée (concurrence son/bruitage) | | |
| 4 | `go test ./internal/game/... ./internal/server/... ./internal/protocol/... -v` | 100 % vert, y compris `sound_cues_chain_test.go`/`TestSoundChain_MotionCardTimerExpiry_PlaysNoSound` (CA10) | | |
| 5 | `go test ./cmd/server/... -run 'TestSoundSites|TestQuestionSound' -v` | 100 % vert — frontière des chemins média/cues respectée (§10.5) | | |
| 6 | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` puis `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...` | Compile sans erreur sur les deux cibles CI (CA11) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Scénario 1 (cycle édition) : PASS
- [ ] Scénario 2 (refus nommés + avertissement contextuel) : PASS
- [ ] Scénario 3 (mode simultané, non-régression pause/RAFALE) : PASS
- [ ] Scénario 4 (mode différé, fin naturelle) : PASS
- [ ] Scénario 5 (mode différé, arrêt manuel + Rejouer ne regèle pas) : PASS
- [ ] Scénario 6 (mode différé, libération pendant pause) : PASS
- [ ] Scénario 7 (trois gestes de conduite) : PASS
- [ ] Scénario 8 (mixage bruitage) : PASS
- [ ] Scénario 9 (30 s intégrales) : PASS
- [ ] Scénario 10 (non-régression sans son) : PASS
- [ ] **Scénario 11 (dégradation x3, jamais de blocage) : PASS — bloquant, aucune exception**
- [ ] Scénario 12 (non-régression MEMOTION/ENTRACTE) : PASS
- [ ] Scénario 13 (suite automatisée) : PASS
- [ ] Aucune régression constatée sur une partie normale (SPEEDY/QCM/ARDOISE/MEMORY/MEMOTION/RAFALE
      mélangés, avec et sans son)

## Notes QA

- Les Scénarios 1 à 12 nécessitent un navigateur **et** une sortie audio réelle — ils restent du
  ressort de l'**utilisateur**, jamais de `qa`/`deployer` (règle projet).
- Le Scénario 13 est une suite de commandes `go test`/`go build` : il peut être exécuté par `qa`
  sans navigateur, en complément de cette procédure.
- Le Scénario 11 est le plus important de tous (risque R10, « le mode différé fige la question ») —
  ne jamais le sauter, même sous pression de planning.
- Si un des trois cas du Scénario 11 échoue (le chronomètre ne démarre pas), c'est un **bloquant
  absolu** pour la PROD : une question figée en direct, sans recours, est le pire défaut que ce lot
  puisse produire.
