# Procédure de Test — Mode de jeu RAFALE (v8.0.0, milestone #16)

**Version** : v8.0.0 (branche `milestone/v8.0.0`)
**Date** : 2026-08-28
**Testeur** : Utilisateur (validation manuelle — ni `qa` ni `deployer` n'exécutent cette procédure,
aucun navigateur fiable dans les sessions agents)
**Issues** : #107 (moteur solo) · #197 (réservoir + éditeur) · #198 (interface TV/anim) · #199 (modes multi)
**Contrat de référence** : `contracts/rafale.md` · **Maquette** : `docs/mockups/rafale-v8.html`

> **Écrit en Batch 1 (test-writer), avant l'implémentation complète du moteur RAFALE.** Cette
> procédure couvre l'ENSEMBLE du milestone (#107/#197/#198/#199, Batches 1→3) — elle ne peut être
> exécutée intégralement qu'une fois les trois batches livrés et un binaire QUALIF buildé. Au moment
> de l'écriture, seul le socle #197 (réservoir + éditeur) et l'essentiel de l'affichage TV/anim
> (#198) sont livrés — le moteur solo (#107) et les 4 modes multi-équipes (#199) restent à livrer.
> Les scénarios 1-4 (une manche par mode) et le scénario 9 (appui buzzer ignoré) ne sont donc
> exécutables qu'après le Batch 2/3. Ne pas exécuter cette procédure avant que le CDP confirme le
> milestone complet livré en QUALIF.

---

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows buildé, cf. `docs/QUALIF_PROCEDURE.md`)
- [ ] Réservoir RAFALE (`/admin/rafale`) peuplé avec **au moins 15 questions** couvrant :
  - au moins 2 catégories (ex. HISTORY, SCIENCE), 3 niveaux de difficulté (1/2/3)
  - au moins 6 questions dans le couple catégorie/difficulté utilisé pour les scénarios 1-4 et 5
    (assez pour une manche complète sans épuisement accidentel, sauf scénario 5 qui le provoque
    volontairement)
- [ ] Une question de type `RAFALE` configurée dans un quiz (`TIME` ~60-120s pour raccourcir le test,
  `RAFALE_QUESTION_TIME` ~3s, `POINTS` = barème de manche, `RAFALE_MAX_QUESTIONS` par défaut)
- [ ] Au moins 2 équipes créées, avec au moins 1 buzzer physique ou VJoueur assigné à chacune
- [ ] Postes ouverts : `/admin` (régie), `/anim` (tablette animateur), `/tv` (écran salle), `/player`
  (tablette joueur, au moins une équipe)
- [ ] Accès à la console réseau du navigateur (DevTools) sur `/tv` et `/player`, pour le scénario 11

---

## Scénario 1 — Manche complète, mode SOLO

**Objectif** : Vérifier le parcours nominal solo — pioche, question/réponse au rythme du chrono
question, décompte du chrono de manche, fin de manche par expiration du timer.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer la manche : mode `SOLO`, 1 catégorie, 1 difficulté, DÉMARRER | Première question posée à l'oral côté anim (texte + réponse attendue visibles UNIQUEMENT sur `/anim`), double timer visible sur `/tv` et `/anim` (manche + question) | | |
| 2 | Répondre correctement (dire la réponse), animateur clique RÉPONSE VALIDE | Question suivante enchaîne automatiquement, chrono question repart, compteur de l'équipe +1 (visible côté anim/admin, PAS de point réel attribué) | | |
| 3 | Répondre incorrectement, cliquer RÉPONSE INVALIDE | Question suivante enchaîne, compteur inchangé (SOLO n'a pas de pénalité, cf. contrat §6.1) | | |
| 4 | Laisser le chrono QUESTION descendre à 0 sans répondre | Comportement identique à une réponse invalide (question suivante enchaîne automatiquement, contrat §6.1) | | |
| 5 | Laisser le chrono de MANCHE descendre à 0 | Manche se termine (`ROUND_END`), écran de fin de manche affiche le compteur final, valeur suggérée pré-remplie = compteur × barème (`POINTS`) | | |
| 6 | Cliquer sur l'équipe pour attribuer les points | Points RÉELLEMENT crédités à l'équipe (action `TEAM_POINTS` existante) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Manche complète, mode CHACUN_SON_TOUR

**Objectif** : Vérifier la rotation d'équipe à CHAQUE réponse (bonne ou mauvaise).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer : mode `CHACUN_SON_TOUR`, ≥2 équipes participantes (ordre de passage), DÉMARRER | Équipe active affichée fort sur `/tv` (bandeau) et en plein écran sur `/player` de l'équipe active ; LED de l'équipe active allumées (SOLID, intensité 255) | | |
| 2 | Répondre correctement → RÉPONSE VALIDE | Compteur de l'équipe active +1, la main passe à l'équipe SUIVANTE (indicateur TV/VPlayer/LED se met à jour) | | |
| 3 | Répondre incorrectement → RÉPONSE INVALIDE | La main passe à l'équipe suivante (même règle, bonne OU mauvaise réponse fait tourner la main) | | |
| 4 | Faire un tour complet (repasser sur la 1ère équipe) | La rotation boucle correctement, aucune équipe sautée ni doublée | | |
| 5 | Fin de manche (timer à 0) | Classement des compteurs affiché, attribution par clic équipe fonctionne pour chaque équipe séparément | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Manche complète, mode TANT_QUE_JE_GAGNE

**Objectif** : Vérifier qu'une équipe qui répond juste GARDE la main (pas de rotation), et qu'une
mauvaise réponse fait tourner la main.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer : mode `TANT_QUE_JE_GAGNE`, ≥2 équipes, DÉMARRER | Équipe active affichée (mêmes 3 canaux qu'au scénario 2) | | |
| 2 | Répondre correctement 3 fois de suite (même équipe) → RÉPONSE VALIDE ×3 | Compteur de l'équipe monte à 3, la main NE CHANGE PAS d'équipe entre chaque bonne réponse | | |
| 3 | Répondre incorrectement → RÉPONSE INVALIDE | La main passe à l'équipe suivante (compteur de la première équipe conservé, pas remis à 0 — contrat §6.1) | | |
| 4 | Laisser le chrono question expirer sans répondre | Traité comme une réponse invalide : rotation vers l'équipe suivante | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Manche complète, mode MAILLON_FAIBLE

**Objectif** : Vérifier la remise à 0 du compteur sur mauvaise réponse ET la mémorisation du
meilleur compteur atteint (utilisé comme valeur suggérée en fin de manche).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer : mode `MAILLON_FAIBLE`, ≥2 équipes, DÉMARRER | Équipe active affichée | | |
| 2 | Équipe A répond juste 4 fois de suite → RÉPONSE VALIDE ×4 | Compteur Équipe A = 4 (rotation à chaque bonne réponse comme `CHACUN_SON_TOUR`) | | |
| 3 | Au retour sur Équipe A, répondre FAUX → RÉPONSE INVALIDE | Compteur Équipe A retombe à **0** immédiatement, la main passe à l'équipe suivante | | |
| 4 | Refaire remonter le compteur d'Équipe A à 2, puis fin de manche | Le compteur affiché en fin de manche / la valeur suggérée reflète le **meilleur** compteur atteint dans la manche (4, mémorisé à l'étape 2), PAS le compteur courant (2) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Épuisement du pool en cours de manche

**Objectif** : Vérifier qu'une manche configurée sur un pool volontairement TROP PETIT se termine
proprement, sans jamais reproposer une question déjà vue.

**Préparation spécifique** : configurer un filtre catégorie/difficulté ne matchant que 3-4 questions
disponibles dans le réservoir, avec un timer de manche assez long (~2mn) pour les épuiser avant la
fin naturelle du chrono.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avant de démarrer, observer l'alerte de pool | Nombre de questions disponibles affiché ; si `disponibles < besoin estimé` (⌈TIME/RAFALE_QUESTION_TIME⌉), avertissement visible ; démarrage reste possible | | |
| 2 | Démarrer et valider/invalider jusqu'à épuiser TOUT le pool filtré | Dès que la dernière question du filtre est posée et jugée, la manche se termine IMMÉDIATEMENT (`RAFALE_EXHAUSTED`), même si le chrono de manche n'est pas à 0 | | |
| 3 | Vérifier le message affiché à l'animateur | Message explicite indiquant l'épuisement du pool (pas un silence, pas une erreur générique) | | |
| 4 | Vérifier qu'aucune question déjà vue pendant CETTE manche n'a été reposée | Relire la liste des questions posées (mentalement ou noté) — aucun doublon | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Redémarrage serveur en pleine manche (flags préservés)

**Objectif** : Vérifier que le marquage « question déjà utilisée » survit à un redémarrage serveur
(fichier `data/config/rafale_used.json`), et ne reproduit jamais une question déjà vue après reboot.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer une manche, valider/invalider 4-5 questions (noter leurs énoncés) | Compteur avance normalement | | |
| 2 | Arrêter le serveur (`curl -s http://localhost/shutdown`) PENDANT que la manche est en cours, puis le relancer | Le serveur redémarre normalement (cf. `docs/DEV_PROCEDURE.md`, méthode obligatoire de relance) | | |
| 3 | Aller sur `/admin/rafale` (éditeur réservoir), observer la colonne État | Les 4-5 questions posées avant le redémarrage sont marquées « utilisee » — le flag a survécu | | |
| 4 | Démarrer une NOUVELLE manche avec le même filtre catégorie/difficulté | Aucune des questions notées à l'étape 1 n'est reproposée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Sauvegarde sélective → réinitialisation → restauration

**Objectif** : Vérifier que le réservoir RAFALE et ses flags sont bien inclus dans le cycle complet
sauvegarde/réinitialisation/restauration sélective (`/admin/backup`) — risque de perte silencieuse
identifié au contrat §10.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/backup`, cocher UNIQUEMENT « Questions RAFALE », lancer la sauvegarde sélective | Un fichier d'archive est téléchargé | | |
| 2 | Ouvrir l'archive (ou vérifier son contenu) | Contient `files/rafale/` (réservoir) ET `config/rafale_used.json` (flags) | | |
| 3 | Sur `/admin/backup`, cocher UNIQUEMENT « Questions RAFALE » dans la réinitialisation sélective, confirmer | Le réservoir RAFALE est VIDÉ (`/admin/rafale` affiche « Aucune question »), les autres données (quiz, équipes, etc.) restent INTACTES | | |
| 4 | Restaurer l'archive de l'étape 1 | Le réservoir RAFALE ET les flags « déjà utilisée » sont restaurés à l'identique de l'étape 1 | | |
| 5 | Non-régression : sauvegarde sélective SANS cocher « Questions RAFALE » | L'archive ne contient PAS `files/rafale/` ni `config/rafale_used.json` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Indicateur d'équipe active sur les 3 canaux (mode multi)

**Objectif** : Vérifier la cohérence de l'indicateur d'équipe active sur les TROIS canaux
simultanément (contrat §8.2/§8.3), y compris à chaque rotation.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer une manche `CHACUN_SON_TOUR` avec 3 équipes participantes | `/tv` : bandeau fort avec nom + couleur de l'équipe active ; `/player` de l'équipe active : indicateur plein écran « À VOUS DE RÉPONDRE » ; `/player` des autres équipes : indication neutre, sans appel à l'action ; LED de l'équipe active : SOLID pleine intensité (255) | | |
| 2 | Observer les LED des buzzers de l'équipe SUIVANTE dans l'ordre de passage | LED en SOLID à intensité réduite (128) | | |
| 3 | Observer les LED des équipes participantes NI actives NI suivantes | LED en mode `DIM` (atténuées, pas éteintes) | | |
| 4 | Observer les LED des équipes NON participantes (si applicable) | LED complètement éteintes | | |
| 5 | Répondre (valide ou invalide) pour déclencher une rotation | Les 3 canaux (TV, VPlayer, LED) se mettent à jour EN MÊME TEMPS vers la nouvelle équipe active — pas de décalage entre canaux | | |
| 6 | Configurer une manche en mode `SOLO` | Aucun indicateur d'équipe active sur aucun canal (TV neutre, VPlayer neutre, LED entièrement éteintes) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Appui buzzer ignoré pendant RAFALE

**Objectif** : Vérifier que RAFALE ne traite AUCUN appui buzzer, même sur un buzzer dont la LED est
allumée (équipe active) — contrat §8.1/§8.3, distinction stricte pilotage LED (sortie) vs traitement
appui (entrée).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Pendant une manche RAFALE en cours (n'importe quel mode), appuyer sur le buzzer PHYSIQUE d'un joueur de l'équipe ACTIVE (LED allumée forte) | Aucun effet visible : pas de son, pas de changement d'état de jeu, pas de score, LED inchangée par cet appui | | |
| 2 | Appuyer sur le buzzer d'un joueur d'une équipe NON active | Aucun effet non plus | | |
| 3 | Vérifier côté `/admin` qu'aucun log/événement de buzz n'a modifié l'état de la manche | Compteurs et sous-phase inchangés par les appuis des étapes 1-2 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Alertes de pool avant démarrage (3 états)

**Objectif** : Vérifier les 3 états de l'alerte pré-manche (contrat §7.2).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Choisir un filtre catégorie/difficulté dont le pool est VIDE (0 disponible) | Démarrage **bloqué**, message explicite | | |
| 2 | Choisir un filtre avec un pool NON vide mais INFÉRIEUR au besoin estimé (⌈TIME/RAFALE_QUESTION_TIME⌉) | **Avertissement** affiché, démarrage reste **autorisé** | | |
| 3 | Choisir un filtre avec un pool LARGEMENT suffisant | Information neutre (pas d'alerte), démarrage normal | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 11 — Fuite de la réponse : vérification réseau (critère bloquant)

**Objectif** : Vérifier, au niveau réseau (pas seulement visuel), que la réponse attendue n'atteint
JAMAIS `/tv` ni `/player` — complète les tests automatisés protocole
(`internal/protocol/rafale_leak_test.go`) par une vérification bout-en-bout réelle.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir les DevTools réseau (onglet WS) sur `/tv`, démarrer une manche RAFALE | Observer les messages WebSocket entrants sur `/tv` pendant plusieurs questions | | |
| 2 | Rechercher dans tous les messages reçus le texte de la réponse attendue de la question en cours (connue via `/anim`) | **Absent** de tous les messages reçus par `/tv`, y compris `RAFALE_CURRENT_QUESTION` (qui ne doit contenir que ID/QUESTION/CATEGORY/DIFFICULTY) | | |
| 3 | Répéter l'étape 1-2 sur `/player` (une tablette VJoueur) | Même résultat : réponse absente de tous les messages reçus | | |
| 4 | Sur `/anim`, vérifier que la réponse EST bien reçue (action `RAFALE_ANSWER`) | La réponse est visible côté anim — c'est le seul canal légitime avec `/admin` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Les 4 modes (SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE, MAILLON_FAIBLE) appliquent leurs règles
  de rotation/compteur exactement comme décrit au contrat §6.1
- [ ] Aucun point réel n'est attribué pendant une manche — uniquement en fin de manche, par clic équipe
- [ ] Le marquage « déjà utilisée » survit à un redémarrage et se réinitialise au NEW_GAME
- [ ] L'épuisement du pool termine la manche proprement, sans jamais reproposer une question déjà vue
- [ ] Le réservoir et ses flags sont couverts par le cycle sauvegarde/réinitialisation/restauration sélective
- [ ] L'indicateur d'équipe active est cohérent sur les 3 canaux (TV/VPlayer/LED) à tout instant, y
  compris pendant une rotation
- [ ] Aucun appui buzzer n'a d'effet sur l'état de jeu pendant RAFALE, quel que soit l'état de sa LED
- [ ] **La réponse attendue n'apparaît JAMAIS, au niveau réseau, dans les messages reçus par `/tv`
  ou `/player`** (critère bloquant — un échec ici bloque la release, quel que soit l'état des autres
  scénarios)
- [ ] `/tv` reste strictement statique pendant tout le déroulé (aucun défilement observé, quel que
  soit le nombre d'équipes ou la longueur de l'énoncé)
- [ ] Aucune régression observée sur les autres modes de jeu (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE) —
  timer global, LED MEMORY multi-équipes

## Notes QA

[Espace pour observations]
