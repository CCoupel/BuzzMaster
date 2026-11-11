# Procédure de Test — Groupes d'ambiance et groupes animateur (#225, milestone v11.7.0, LOT-1)

**Version** : v11.7.0.x (QUALIF)
**Date** : 2026-11-11 (révisée après le refus du GATE LOT-1 : groupes animateur multiples, `id` stable, essai en séquence)
**Testeur** : QA (scénarios AUTO) / Utilisateur (scénarios HUMAIN)
**Contrat** : `contracts/light-effects.md` §2 (dont §2.5), `contracts/hue-bridge.md` §11, `contracts/http-endpoints.md`
**Maquette** : `docs/mockup/v11.7.0/ui/ambiance_page__light_groups_multi_host.html`
**Plan** : `.claude/workflow/plan-11.7.0.md` (LOT-1, tâches 2 à 6, 32, corrections C1 à C6)

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows, pont Hue réel pour le volet « ampoules réelles »)
- [ ] Données : pont Hue associé, au moins 6 ampoules dont **2 de rôle équipe** ; équipes « Rouges » et « Bleus » créées ;
      un quiz avec une question chronométrée
- [ ] Accès : page Ambiance › Lumière (étape 3, pont associé)
- [ ] Config QUALIF copiée depuis `build/qualif_v11.6.0/config.json` (appairage Hue conservé) — fait par `deployer`

## Scenarios

### Scenario 1 — Tests automatisés du lot (non-régression critique incluse)

**Validation** : AUTO — **Issues** : #225

**Objectif** : les tests écrits depuis le contrat passent, dont la non-régression « sans groupe, mêmes écritures qu'en v11.6.0 ».

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go test ./internal/lighting/hue ./internal/config ./internal/server -run TW225 -count=1 -race` | Tous verts (pilote Hue, validation de `lighting.groups`, API) | | |
| 2 | `cd server-go/web && npx vitest run src/components/LightGroupsManager.test.jsx src/pages/AmbiancePage.groups225.test.jsx` | Tous verts | | |
| 3 | `cd server-go && go test ./internal/lighting/... ./internal/ambiance/... ./cmd/server -run 'Ambiance|Lighting|Hue' -count=1` | Aucune régression sur l'éclairage existant (scènes, pulsation, SCORE, Flash, ON/OFF, extinction) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Sous-onglets de Lumière et colonne « Groupe »

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : l'onglet Lumière est découpé en trois sous-onglets sans changer le contenu existant (maquette §01).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir Ambiance › Lumière sans pont associé | Étapes 1 et 2 (recherche, association) inchangées, **aucun** sous-onglet | | |
| 2 | Pont associé : ouvrir Lumière | Trois sous-onglets « Pont et ampoules » (actif), « Groupes » (avec le nombre de groupes), « Effets » (« bientôt ») | | |
| 3 | Sous-onglet « Pont et ampoules » | Contenu d'avant inchangé, plus une colonne « Groupe » (« hors groupe » au départ ; « — » pour une ampoule d'équipe ; « n · nom » pour la salle ; « 🎤 A1 · nom » pour un groupe animateur) | | |
| 4 | Cliquer sur « hors groupe » d'une ampoule | Bascule sur le sous-onglet « Groupes » | | |
| 5 | Sous-onglet « Effets » | Message « Bientôt », aucun réglage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Créer, nommer, ordonner et supprimer des groupes

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : la gestion des groupes est utilisable à la souris et au clavier (maquette §02).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sous-onglet Groupes, aucun groupe : lire le texte d'aide | Boutons « + Nouveau groupe » et « 🎤 + Nouveau groupe animateur » ; **aucun** bouton « Créer un groupe par ampoule » (supprimé) | | |
| 2 | « + Nouveau groupe » quatre fois, nommer chaque groupe | Groupes numérotés 1…4 ; « Enregistrer » actif | | |
| 3 | Glisser une ampoule d'un groupe vers un autre | L'ampoule change de groupe, elle n'est jamais dans deux groupes | | |
| 4 | Glisser un groupe par sa poignée ⠿ | Son rang change, **les numéros se recalculent** ; « Ordre de parcours » se met à jour | | |
| 5 | Sans souris : « Déplacer vers… » (liste) et ↑ / ↓ | Même résultat qu'au glisser-déposer | | |
| 6 | Saisir deux fois le même nom (casse différente), puis un nom vide | Message d'erreur sous le champ, « Enregistrer » grisé | | |
| 7 | Ajouter l'équipe « Rouges » à un groupe | Elle disparaît de la liste des autres groupes | | |
| 8 | « Annuler les modifications » | Retour à l'état enregistré | | |
| 9 | Enregistrer, recharger la page (F5) | Les groupes, leur ordre et leurs équipes sont conservés (leur identifiant ne change pas : renommer ou réordonner un groupe ne casse rien) | | |
| 9b | Survoler le « ? » d'« Équipes à proximité » | Infobulle : la vague part du groupe qui cite l'équipe, sinon du groupe 1 | | |
| 10 | Supprimer un groupe, enregistrer | Ses ampoules reviennent dans « hors groupe », les numéros se tassent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Rôle d'une ampoule et groupes (retrait automatique)

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : une ampoule passée en rôle équipe quitte son groupe (contrat §2.1).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ranger l'ampoule A dans un groupe et enregistrer | Pastille « n · nom » dans « Pont et ampoules » | | |
| 2 | Passer A au rôle « Équipe Rouges » : lire l'avertissement avant d'enregistrer | Message indiquant qu'elle sera retirée de son groupe | | |
| 3 | Enregistrer | A n'est plus dans aucun groupe ; le groupe (éventuellement vide) est conservé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Groupes animateur : plusieurs, rang propre, règles

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : jusqu'à 4 groupes animateur, hors numérotation de salle (maquette `multi_host`, contrat §2.5).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « 🎤 + Nouveau groupe animateur » deux fois, nommer « Pupitre » et « Régie » | Deux blocs numérotés 🎤1 et 🎤2, sans numéro de salle ni « équipes à proximité » | | |
| 2 | Y glisser une ampoule générale chacun, enregistrer | Pastilles « 🎤 A1 · Pupitre » et « 🎤 A2 · Régie » dans « Pont et ampoules » ; la numérotation des groupes de salle n'est pas décalée | | |
| 3 | Réordonner les deux groupes animateur (↑ / ↓) | Les rangs 🎤1 / 🎤2 s'inversent, leurs réglages d'effets suivent leur groupe (identifiant stable) | | |
| 4 | Créer un 3e et un 4e groupe animateur | Le bouton de création se grise au 4e, message « 4 groupes animateur au plus » | | |
| 5 | Créer 12 groupes de salle en plus | Le plafond de 12 ne compte que les groupes de salle : 12 salles + 4 animateurs sont possibles | | |
| 6 | Supprimer un groupe animateur, enregistrer | Ses ampoules reviennent dans « hors groupe » ; les rangs suivants se renumérotent | | |
| 7 | Lire la mention des blocs | « Blanc fixe par défaut » ; « Choisir ses effets » actif pour un groupe enregistré et sans modification en cours | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Essai d'un groupe en séquence sur les ampoules réelles

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : « Tester le groupe » joue une séquence visible de 6 secondes sur les seules ampoules du groupe, puis les rend à leur état vivant (contrat §2.4). À faire **sur chaque type de groupe** : un groupe de salle, puis chaque groupe animateur.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « Tester le groupe » sur un groupe de salle enregistré | Réponse immédiate ; le bouton devient « ■ Arrêter l'essai » avec l'infobulle « Étape n/6 » | | |
| 2 | Observer les ampoules du groupe, une seconde par pas | Dans l'ordre : **éteint**, **blanc 50 %**, **blanc 100 %**, **rouge**, **vert**, **bleu**, chacun environ 1 s ; seules les ampoules du groupe changent | | |
| 3 | Après le 6e pas | Retour à l'état vivant du groupe (scène du moment) ; bouton « Tester le groupe » de nouveau | | |
| 4 | Idem sur un groupe animateur (chacun des groupes animateur créés) | La séquence se voit sur ses ampoules ; retour au **blanc fixe** (ou à son effet) ; les autres groupes ne sont pas touchés | | |
| 5 | Lancer l'essai du groupe 1, puis aussitôt celui du groupe 2 | L'essai du groupe 2 **remplace** celui du groupe 1, qui revient aussitôt à son état vivant ; un seul bouton « Arrêter l'essai » | | |
| 6 | « ■ Arrêter l'essai » en cours de séquence | Le groupe revient aussitôt à son état vivant | | |
| 7 | Pendant l'essai d'un groupe, mettre le popover sur OFF / ON / Flash | Le groupe testé continue sa séquence ; les autres groupes suivent le mode choisi | | |
| 8 | Modifier un groupe sans enregistrer : survoler « Tester le groupe » | Bouton grisé, infobulle « Enregistrez d'abord » ; groupe vide : « Groupe vide » | | |
| 9 | Désactiver l'éclairage d'ambiance | Bouton grisé (« Éclairage d'ambiance désactivé ») | | |
| 10 | Éteindre le pont (ou débrancher), essayer | Message « Pont injoignable », l'interface reste utilisable (plus de message « essai déjà en cours ») | | |
| 11 | Arrêter le serveur (ou désactiver l'éclairage) pendant un essai | L'essai s'arrête, toutes les ampoules s'éteignent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Non-régression : sans groupe, l'éclairage est celui de v11.6.0

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : aucun changement visible ni de débit tant qu'aucun groupe n'existe (critère d'acceptation).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Supprimer tous les groupes, enregistrer | Aucun groupe | | |
| 2 | Dérouler une partie : attente, question chronométrée (pulsation de fin de chrono), buzz, révélation, score, entracte | Mêmes scènes, même pulsation, même clignotement or de l'équipe créditée qu'en v11.6.0 | | |
| 3 | Popover Ambiance : ON, OFF, AUTO, Flash | Mêmes effets qu'en v11.6.0 sur toute la salle | | |
| 4 | Couper le serveur (`/shutdown`) | Toutes les ampoules s'éteignent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Volet « ampoules réelles » : groupe animateur insensible à la salle

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : **chaque** groupe animateur garde son blanc fixe quoi que fasse la salle (contrat §2.5).
Prérequis : deux groupes animateur (1 à 3 ampoules chacun) et au moins un groupe de salle enregistrés.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éclairage actif, partie au repos | Ampoules de **chaque** animateur en **blanc fixe** ; la salle suit sa scène | | |
| 2 | Lancer une question, laisser tourner le chrono jusqu'à la pulsation | La salle pulse ; l'animateur reste blanc fixe | | |
| 3 | Attribuer des points | Clignotement or de l'équipe seulement sur son ampoule ; salle en scène SCORE ; l'animateur reste blanc | | |
| 4 | Popover : OFF | La salle s'éteint ; **les animateurs restent allumés** | | |
| 5 | Popover : ON, puis Flash | Salle blanche / clignotante ; l'animateur ne clignote pas | | |
| 6 | Interrupteur général « Éclairage d'ambiance » : désactiver | **Tous** les animateurs s'éteignent (seul cas avec l'arrêt du serveur) | | |
| 7 | Réactiver, puis couper le serveur | Les animateurs se rallument ; à l'arrêt tout s'éteint, chaque groupe animateur compris | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 9 — Volet « ampoules réelles » : relevés du contrat §8.2 (seuil `SubGroupMinLights`)

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : confirmer ou amender le seuil de 3 ampoules pour créer un groupe Hue `buzzmaster-general-<n>`
(contrat §2.3). Mesure à consigner dans le rapport de QUALIF ; le banc `hue/bench_real_test.go` peut être étendu aux groupes.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer 2 groupes de 2 ampoules, déclencher un changement de scène (ex. buzz) | Noter la durée visible entre la 1re et la dernière ampoule (écriture ampoule par ampoule sous 3) | | |
| 2 | Configurer 2 groupes de 3 ou 4 ampoules, même changement | Noter la durée (écriture par groupe Hue) ; vérifier dans l'application Hue la présence de `buzzmaster-general-1`, `-2` | | |
| 3 | Comparer : un groupe de 2 ampoules est-il visiblement décalé ? | Consigner le chiffre ; si le décalage gêne, proposer d'abaisser `SubGroupMinLights` | | |
| 4 | Groupes animateur de 3 ampoules et plus (A1, A2) | Groupes Hue `buzzmaster-general-host1`, `buzzmaster-general-host2` créés ; `buzzmaster-general` ne contient AUCUNE ampoule d'animateur | | |
| 4b | Config issue de la QUALIF 11.7.0.2 (ancien groupe `buzzmaster-general-host`) | L'ancien groupe Hue disparaît à la réconciliation ; le serveur démarre sans erreur avec l'ancienne configuration | | |
| 5 | Supprimer les groupes, enregistrer | Les groupes Hue `buzzmaster-general-<n>` disparaissent à la réconciliation suivante | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Scénario 1 (AUTO) vert, y compris la non-régression des écritures sans groupe
- [ ] Les scénarios nominaux 2, 3, 5, 6 passent (essai en séquence : éteint, blanc 50 %, blanc 100 %, rouge, vert, bleu)
- [ ] Chaque groupe animateur n'est jamais modifié par la salle, ON / OFF ni le Flash (scénario 8)
- [ ] L'essai en séquence se voit sur chaque type de groupe (scénario 6)
- [ ] Le seuil `SubGroupMinLights` est relevé et consigné (scénario 9)
- [ ] Aucune régression de l'éclairage sans groupe (scénario 7)

## Notes QA

[Espace pour observations — noter les durées du scénario 9.]
