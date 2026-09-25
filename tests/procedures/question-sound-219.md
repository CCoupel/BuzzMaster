# Procédure de Test — Question sonore (#219, milestone v11.1)

**Version** : v11.1.0.x (QUALIF)
**Date** : 2026-09-23 (addendum « Média indisponible = lancement bloqué »)
**Testeur** : Utilisateur (validation auditive — aucun navigateur/enceinte fiable côté agents)
**Contrat** : `contracts/sound.md` §10 (socle) et §10.8 (addendum T0/T1, échappatoire, puce globale)
**Plan** : `_work/reports/plan-20260922-103848.md` (rév. 3, socle) et
`_work/reports/plan-20260923-101500.md` (rév. 8, addendum — **la seule valable** pour l'addendum)
**Maquette normative** : `docs/mockups/question-sound-219.html` (**révision 8**) — **les deux
machines à états du §03** (le son, le chronomètre de réponse différé), **le §06** (média
indisponible = lancement bloqué) et **le §07** (la puce d'alerte globale) sont la source de vérité
de cette procédure.

## Important — pourquoi cette procédure ne peut être exécutée que par l'utilisateur

Aucun test automatisé ne peut constater qu'un son a été **entendu**, ni qu'un chronomètre figé à
l'écran est réellement perçu comme « en attente » plutôt que comme une panne. `qa` et `deployer`
n'exécutent **jamais** cette procédure (règle projet) : ici il n'y a ni navigateur fiable ni
enceinte dans leur environnement d'exécution. C'est le **seul filet** avant PROD pour le rendu
sonore et pour les risques R10/R11 (chronomètre figé pris pour une panne) **et** R16/R17 (blocage
définitif d'un quiz sonore sans l'échappatoire du §06).

## ⚠️ Deux moments, jamais confondus (lire avant de commencer)

L'addendum du 2026-09-23 distingue strictement **T0** (avant le lancement — la question ne doit
**jamais démarrer** si son média est indisponible) et **T1** (pendant la lecture, question déjà
`STARTED` — le mécanisme technique historique « jamais de blocage » reste en place : le chronomètre
différé se libère **immédiatement**). Les Scénarios 11a et 11b couvrent chacun **exactement un seul**
de ces deux moments.

**Mise à jour du 2026-09-25 (décision utilisateur)** : seul **T0** (Scénario 11a) reste **requis**
pour la validation de ce cycle avant PROD. **T1** (Scénario 11b) est **reclassé non pertinent pour
la validation** — voir sa section dédiée ci-dessous pour le raisonnement complet consigné tel quel.
Le mécanisme technique sous-jacent (CA12, les 3 tests `TestQuestionSoundAdapter_CA12_*`) reste
inchangé dans le code ; seul son statut de vérification manuelle bloquante change. Ne jamais sauter
le Scénario 11a — c'est désormais le **seul** des deux qui conditionne la validation (piège R18
documenté au plan rév. 8 §8, qui avertissait déjà contre l'idée que 11b puisse à lui seul suffire).

## Prérequis

- [ ] Environnement : QUALIF (poste avec une sortie audio réelle — obligatoire, contrairement aux
      procédures purement logicielles de ce projet)
- [ ] Binaire buildé depuis la branche `milestone/v11.1` (ou merge ultérieur), lot socle (rév. 3) et
      addendum (rév. 8 — gate T0, cache, échappatoire `Ctrl`+clic, puce globale) tous deux livrés
- [ ] Accès **admin** (régie, `/admin`) — l'échappatoire `Ctrl`+clic n'existe que là (jamais sur
      `/anim`)
- [ ] Jeu de données : au moins
  - [ ] 1 question **SPEEDY** avec un son valide (WAV canonique, ~10-15 s), **chronomètre
        simultané** (réglage par défaut)
  - [ ] 1 question **QCM** avec un son valide (~20-24 s), **chronomètre différé**
  - [ ] 1 question **ARDOISE** avec un son de **exactement 30 s** (durée maximale, CA7)
  - [ ] 1 question **SPEEDY ou QCM sans aucun son** (non-régression, CA9)
  - [ ] 1 question intégrée dans une **manche RAFALE** (au moins 2 questions), l'une d'elles
        portant un son
  - [ ] 1 question **MEMORY en mode SOLO, à son, sans équipe participante sélectionnée**
        (participants non conformes) — dédiée au Scénario 15/CA27
  - [ ] **Plusieurs** questions à son dans le même quiz (≥ 3, idéalement 5+) — dédié au Scénario 16
        (la puce globale compte/nomme le nombre de questions concernées)
  - [ ] Fichiers de test prêts hors serveur : un `.mp3` quelconque, un `.wav` en 48 kHz mono, un
        `.wav` de plus de 30 s (ex. 42 s), et un `.wav` **valide de remplacement** (pour le
        Scénario 14, cas A) — pour les Scénarios 2 et 14
- [ ] Accès : `/questions` (édition), `/anim` (conduite tablette), `/admin` (miroir), `/tv`
      (affichage public), configuration serveur (`config.json`, section son) pour le Scénario 11a,
      et **capacité de redémarrer le serveur** (Scénarios 11a, 14, 16 — §10.8.8 : réactiver le son
      ou brancher l'enceinte n'a d'effet qu'après redémarrage)
- [ ] Accès **au système de fichiers du serveur** (pour altérer/tronquer/restaurer un fichier
      `.wav` hors de l'éditeur — Scénarios 11a, 11b, 14)

## Scénarios

### Scénario 1 — Attacher, écouter, supprimer un son (CA1, CA8)

**Objectif** : vérifier le cycle complet d'édition du champ son, identique dans ses gestes à celui
de l'image (maquette §01).

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
contextuel n'apparaît que quand il est pertinent (maquette §01).

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
naturelle du son, chronomètre figé puis libéré (maquette §02-§03).

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

### Scénario 11a — T0 (avant le lancement) : média indisponible = la question ne démarre pas (CA17, CA20, CA21, CA23, CA24, CA26, ⚠️ nouveau point de revue critique)

**Objectif** : prouver que l'addendum du 2026-09-23 **bloque** — plutôt que dégrade — les trois
mêmes causes d'indisponibilité **quand elles sont connues avant le clic LANCER** (maquette §06,
« Deux moments, deux règles »). ⚠️ Ne pas confondre avec le Scénario 11b (T1, question déjà lancée)
— les deux sont **volontairement différents**, voir l'avertissement en tête de ce document.

> ⚠️ Aussi important que l'ancien Scénario 11 (désormais 11b) : le risque R16/R17 (plan rév. 8 §8)
> est qu'un quiz sonore entier devienne injouable sans le Scénario 15 (échappatoire) — vérifier les
> deux ensemble avant de conclure.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | **Son désactivé** : couper `sound.enabled` dans la configuration serveur, redémarrer. Sélectionner (sans lancer) une question à son sur `/admin` et `/anim` | Bouton LANCER **grisé**, sous-libellé « son introuvable » ou motif équivalent nommant la cause — **exactement le même mécanisme visuel** que « buzzers en attente » (maquette §06, « rien de nouveau à apprendre ») | | |
| 2 | Vérifier le texte exact du motif | « Les sons sont désactivés. Les réactiver demande un redémarrage du serveur. » — le remède est **nommé explicitement** | | |
| 3 | **Sortie audio indisponible** : débrancher l'enceinte (ou démarrer le serveur sans périphérique audio) puis redémarrer. Sélectionner la même question | Bouton LANCER grisé, motif : « Aucune sortie audio n'a pu être ouverte au démarrage. Brancher l'enceinte puis redémarrer. » | | |
| 4 | **Fichier illisible ou corrompu (CA23, pas seulement absent)** : sur le disque serveur, **tronquer ou corrompre** le contenu du fichier `.wav` d'une question à son (garder le même nom de fichier), sans passer par l'éditeur. Sélectionner cette question | Bouton LANCER grisé, motif : « Le fichier son de cette question est introuvable ou illisible. » — **détecté avant le lancement**, pas découvert à la lecture | | |
| 5 | Pour chacun des 3 cas ci-dessus, **tenter de cliquer LANCER quand même** | Rien ne se produit — le bouton est réellement inactif, la question reste en `PREPARE` | | |
| 6 | Pour chacun des 3 cas, observer `/anim` | Le **même motif** est affiché, **sans aucun moyen de le contourner** depuis la tablette (CA26 — vérifié en détail au Scénario 15) | | |
| 7 | Vérifier qu'une question **sans aucun son** reste totalement insensible à ces trois manipulations (CA21/R22) | Aucun motif, bouton LANCER dans son état habituel, comportement strictement identique à avant l'addendum | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 11b — T1 (pendant la lecture) : les trois dégradations d'une question déjà lancée ne figent jamais la partie (CA12)

> ## ⚠️ RECLASSÉ NON PERTINENT POUR LA VALIDATION (décision utilisateur, 2026-09-25)
>
> **Raisonnement de l'utilisateur, consigné tel quel** : la règle « jamais de blocage » (R10/CA12)
> visait à l'origine les bruitages d'ambiance (les 7 cues indépendants du contenu d'une question),
> qui ne doivent effectivement jamais bloquer le jeu. Mais un son de QUESTION est différent — si le
> son ne peut pas être joué, il est normal que la question ne soit pas lancée (c'est justement
> l'objet de la gate T0). La validité étant désormais vérifiée au PREPARE (avec le Ctrl+clic comme
> échappatoire explicite si l'animateur force quand même), une fois la question STARTED, soit le
> son a été validé et devrait jouer normalement, soit l'animateur a délibérément choisi de lancer
> sans son. Un problème survenant PENDANT la lecture d'une question déjà lancée (ex. enceinte
> débranchée en plein milieu) devient un cas rare que l'animateur gère manuellement (Stop/relance),
> pas quelque chose que le logiciel doit nécessairement couvrir automatiquement pour la validation
> de ce lot.
>
> **Ce qui ne change pas** : le mécanisme technique existant (chronomètre différé qui se libère
> immédiatement en cas de dégradation pendant la lecture, CA12, les 3 tests
> `TestQuestionSoundAdapter_CA12_*`) reste en place tel quel dans le code — ce n'est **pas** un
> retrait de fonctionnalité, seulement un déclassement de sa validation manuelle comme non-bloquante
> pour ce cycle.
>
> Ce scénario reste documenté ci-dessous pour référence (le mécanisme existe et peut être vérifié à
> l'occasion), mais son échec **ne bloque plus** la validation avant PROD — voir « Critères de
> Validation » en fin de document.

**Objectif** : prouver la règle de non-blocage historique (contract §10.7/§10.8) : pour une question
**déjà `STARTED`**, dans les trois mêmes cas d'indisponibilité, le mode différé dégrade
**immédiatement** vers le mode simultané — la partie ne se bloque jamais en direct. **Distinct du
Scénario 11a** : ici l'indisponibilité survient (ou n'a pas pu être détectée) **après** que la
question a déjà démarré.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | **Enceinte indisponible** : LANCER d'abord une question à son en mode différé (média disponible au lancement), **puis** débrancher/couper la sortie audio du serveur pendant la lecture (ou reproduire une panne équivalente en cours de partie) | Le chronomètre démarre **immédiatement** dès que la dégradation survient — la question reste jouable, aucun blocage, aucune erreur visible | | |
| 2 | **Fichier illisible en cours de lecture** : sur une question en mode différé déjà `STARTED`, altérer/supprimer manuellement le fichier `.wav` sur le disque serveur pendant que le son joue | Le chronomètre démarre **immédiatement** — même comportement | | |
| 3 | **Son désactivé en cours de partie** : couper `sound.enabled` pendant qu'une question à son différé est `STARTED` | Le chronomètre démarre **immédiatement** — même comportement | | |
| 4 | Pour chacun des 3 cas ci-dessus, vérifier `/anim`/`/admin`/`/tv` | **Aucune** mention « le chronomètre démarrera à la fin du son » ne reste affichée alors que le chronomètre tourne déjà (pas de mention mensongère) | | |
| 5 | (Optionnel, robustesse) Simuler une lecture qui ne se termine jamais (ex. couper le processus de lecture sans le signaler) et attendre `MaxQuestionSoundDuration + 2 s` (~32 s) | Le chien de garde libère le chronomètre **au plus tard** à cette échéance | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] NON APPLICABLE — non bloquant, reclassé non pertinent pour la validation (décision utilisateur, 2026-09-25)

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
| 4 | `go test ./internal/game/... ./internal/server/... ./internal/protocol/... -v` | 100 % vert, y compris `sound_cues_chain_test.go`/`TestSoundChain_MotionCardTimerExpiry_PlaysNoSound` (CA10) et les tests de gate/cache/réversibilité de l'addendum (CA19, CA24, CA25) | | |
| 5 | `go test ./cmd/server/... -run 'TestSoundSites|TestQuestionSound' -v` | 100 % vert — frontière des chemins média/cues respectée (§10.5), dispatch `START` sans `FORCE` refusé (CA18), `FORCE_READY` sur participants non conformes toujours refusé (CA27) | | |
| 6 | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` puis `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...` | Compile sans erreur sur les deux cibles CI (CA11) | | |
| 7 | `cd web && npx vitest run` | 100 % vert, y compris `prepareWaitReason.test.js` (les trois motifs) et la non-régression `GamePage`/puce globale (CA28/CA29) | | |

> ℹ️ Les items 4-5-7 seront complétés par les tests dédiés de l'addendum (Batch 1/2, tâches 10-13
> du plan rév. 8) au fur et à mesure de leur livraison — cette ligne du tableau n'a pas besoin
> d'être réécrite à chaque ajout, `qa` exécute simplement la commande sur l'état livré.

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 14 — Réversibilité automatique et course affichage/clic (CA19, CA20)

**Objectif** : vérifier que le blocage T0 (Scénario 11a) n'est jamais définitif au-delà de son
motif réel (maquette §06, « Dès que c'est réparé, ça repart tout seul »), et que la fenêtre entre
l'affichage du bouton et le clic ne peut pas faire glisser une question indisponible en `STARTED`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | **Cas A — fichier réparé, sans redémarrage.** Reproduire le blocage « fichier illisible » (Scénario 11a, étape 4) sur une question sélectionnée. Remplacer le fichier corrompu par un `.wav` valide (même nom de fichier), **sans redémarrer le serveur** | En quelques secondes (revalidation par le cache, empreinte `mtime`+taille modifiée), la question repasse **d'elle-même** en `READY` — bouton LANCER redevient actif, motif disparaît, **aucun geste supplémentaire** requis (CA19) | | |
| 2 | **Cas B — son réactivé, avec redémarrage.** Reproduire le blocage « son désactivé » (Scénario 11a, étape 1). Réactiver `sound.enabled`, **redémarrer le serveur** (§10.8.8 : obligatoire pour ce motif) | Après redémarrage et reconnexion, la question à son repasse en `READY` **automatiquement** — pas besoin de rouvrir/re-sélectionner chaque question une par une | | |
| 3 | **Course affichage/clic (CA20).** Sélectionner une question à son **disponible** (bouton LANCER actif, aucun motif). **Juste avant de cliquer**, rendre le média indisponible sur le serveur (ex. supprimer le fichier), puis cliquer LANCER immédiatement | La question **ne démarre pas** malgré le clic — elle reste en `PREPARE`, le motif apparaît maintenant. **Aucun plantage, aucun blocage du jeu** — le clic « raté » est silencieusement absorbé | | |
| 4 | Depuis l'état de l'étape 3, restaurer le fichier et cliquer LANCER à nouveau | La question démarre normalement cette fois | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 15 — L'échappatoire régie : `Ctrl`+clic sur la question (CA22, CA23, CA25, CA26, CA27)

**Objectif** : vérifier le contournement existant (maquette §06, « la sortie de secours ») et sa
**limite volontaire** — il ne lève que la gate son, jamais la gate participants.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Sur `/admin`, sélectionner (**clic simple**) une question à son dont le média est indisponible (réutiliser un cas du Scénario 11a) | La question reste **bloquée** en `PREPARE`, exactement comme au Scénario 11a | | |
| 2 | Sur la **même** question, `Ctrl`+**clic** (sur la ligne de la question dans la liste, **pas** sur le bouton LANCER — maquette §06 : « rien de neuf à l'écran, le bouton LANCER ne change pas ») | La question passe en `READY` **malgré** l'indisponibilité — le bouton LANCER redevient actif | | |
| 3 | Cliquer LANCER | La question **démarre**, mais joue **sans son** — aucun geste de conduite son n'apparaît (comme au Scénario 10, CA9), et si le mode différé était activé, **le chronomètre ne se met jamais en attente** (démarre immédiatement, le mode différé est sans objet) | | |
| 4 | Arrêter cette question, sélectionner une **autre** question à son également indisponible, **sans refaire `Ctrl`+clic** | Cette nouvelle question reste **bloquée** normalement — le contournement **ne persiste pas** d'une question à l'autre (CA25) | | |
| 5 | Sélectionner la question **MEMORY SOLO à son, sans équipe participante** (jeu de données dédié). `Ctrl`+clic dessus | La question reste **bloquée** en `PREPARE` — **la limite du contournement** (CA27) : `Ctrl`+clic ne lève que la branche son, jamais la conformité des participants (arbitrage #172 B5, rappelé par la maquette §06 : « elle serait injouable ») | | |
| 6 | Reproduire l'étape 1 en se connectant sur `/anim` (tablette animateur) au lieu de `/admin` | Le motif est affiché comme sur `/admin`, mais **aucun** geste équivalent au `Ctrl`+clic n'existe sur cette surface — la question reste bloquée sans recours depuis la tablette (CA26) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 16 — La puce d'alerte globale (CA28, CA29)

**Objectif** : vérifier le signal complémentaire à l'échelle du quiz entier (maquette §07) — visible
avant même d'ouvrir une question précise — et sa non-interférence avec la pastille de menu
existante.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-------------------|------------------|------|
| 1 | Quiz contenant ≥ 1 question sonore, audio **actif** (son activé, enceinte branchée) | **Aucune** puce d'alerte sur `/questions` ni sur `/admin` | | |
| 2 | Couper `sound.enabled` (ou débrancher l'enceinte), **redémarrer le serveur** | Une puce d'alerte apparaît **en haut de `/questions`**, nommant le **nombre** de questions sonores concernées et le remède (« Vérifier la configuration ou brancher l'enceinte, puis redémarrer le serveur ») | | |
| 3 | Ouvrir `/admin` (sans sélectionner de question précise) | **La même puce** est visible **avant même** de choisir une question — pas seulement en la sélectionnant (maquette §07, « on le sait avant de choisir ») | | |
| 4 | Ouvrir `/anim` (tablette animateur) | **Aucune** puce — cette surface ne connaît pas la liste des questions et n'a de toute façon aucune action possible dessus | | |
| 5 | Réactiver l'audio (`sound.enabled` + enceinte), redémarrer | La puce **disparaît d'elle-même**, sur `/questions` **et** `/admin`, sans action d'acquittement | | |
| 6 | Audio de nouveau désactivé (retour à l'étape 2). Supprimer/désattacher le son de **chaque** question sonore du quiz une par une, jusqu'à la dernière | La puce **disparaît** dès que la **dernière** question sonore perd son son — même sans jamais avoir réactivé l'audio | | |
| 7 | Pendant que la puce est affichée (audio désactivé, quiz sonore), observer la pastille de menu « Ambiance » (haut-parleur) | La pastille garde **ses deux formes habituelles** (actif/inactif) — **jamais** une 3ᵉ variante « alerte » visuelle sur cette pastille précise (CA29) ; le message d'alerte vit **uniquement** dans la puce du §07, pas dans l'icône de menu | | |

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
- [ ] **Scénario 11a (T0, blocage avant lancement) : PASS — bloquant, aucune exception**
- [ ] Scénario 11b (T1, dégradation jamais bloquante) — **non bloquant, reclassé non pertinent pour
      la validation (décision utilisateur, 2026-09-25)** ; documenté pour référence, pas exigé pour
      la clôture de ce cycle
- [ ] Scénario 12 (non-régression MEMOTION/ENTRACTE) : PASS
- [ ] Scénario 13 (suite automatisée) : PASS
- [ ] Scénario 14 (réversibilité automatique + course affichage/clic) : PASS
- [ ] Scénario 15 (échappatoire `Ctrl`+clic et sa limite) : PASS
- [ ] Scénario 16 (puce d'alerte globale, sans détourner la pastille de menu) : PASS
- [ ] Aucune régression constatée sur une partie normale (SPEEDY/QCM/ARDOISE/MEMORY/MEMOTION/RAFALE
      mélangés, avec et sans son)

## Notes QA

- Tous les scénarios sauf le 13 nécessitent un navigateur **et** une sortie audio réelle — ils
  restent du ressort de l'**utilisateur**, jamais de `qa`/`deployer` (règle projet).
- Le Scénario 13 est une suite de commandes `go test`/`go build`/`vitest` : il peut être exécuté par
  `qa` sans navigateur, en complément de cette procédure.
- **Mise à jour du 2026-09-25 (décision utilisateur)** : le **Scénario 11a (T0) est désormais le
  seul bloquant** des deux — un échec (une question à média indisponible qui démarre quand même)
  reste un **bloquant absolu**, exactement l'inverse du comportement voulu par l'addendum. Le
  **Scénario 11b (T1) est reclassé non pertinent pour la validation** (voir sa section dédiée pour
  le raisonnement complet) : le mécanisme technique (CA12) reste en place dans le code et documenté
  ici pour référence, mais son échec ne bloque plus ce cycle. Ne jamais sauter le Scénario 11a pour
  autant (piège R18, plan rév. 8 §8).
- **Scénario 15, étape 5 (CA27)** est la limite de sécurité la plus facile à casser par erreur lors
  d'une évolution future : si `Ctrl`+clic finit par débloquer une question MEMORY/MEMOTION sans
  participants conformes, c'est une réouverture du bug historique #172 — à signaler immédiatement,
  jamais comme un simple écart mineur.
- **Scénario 16, étape 7 (CA29)** protège une décision produit explicite (#234, « deux formes
  seulement ») — si la pastille de menu affiche un jour un 3ᵉ état « alerte », c'est un écart de
  conception à signaler, pas une amélioration.
