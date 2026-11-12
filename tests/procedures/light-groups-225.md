# Procédure de Test — Groupes d'ambiance : trois zones Ambiance / Animateur / Public (#225, milestone v11.7.0, LOT-1)

**Version** : v11.7.0.x (QUALIF)
**Date** : 2026-11-11 (révision 4 : **trois zones** — Ambiance, Animateur, Public —, 12 groupes au plus par zone, groupes **sans nom**, `id` stable, essai en séquence, zone Public non pilotée sans effet)
**Testeur** : QA (scénarios AUTO) / Utilisateur (scénarios HUMAIN)
**Contrat** : `contracts/light-effects.md` §2 (dont §2.5), `contracts/hue-bridge.md` §11, `contracts/http-endpoints.md`
**Maquette** : `docs/mockup/v11.7.0/ui/ambiance_page__light_groups_zones.html`
**Plan** : `.claude/workflow/plan-11.7.0.md` (LOT-1, tâches Z1 à Z4)

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows, pont Hue réel pour le volet « ampoules réelles »)
- [ ] Données : pont Hue associé, au moins 8 ampoules dont **2 de rôle équipe** ; équipes « Rouges » et « Bleus » créées ;
      un quiz avec une question chronométrée
- [ ] Accès : page Ambiance › Lumière (étape 3, pont associé)
- [ ] Config QUALIF copiée depuis `build/qualif_v11.6.0/config.json` (appairage Hue conservé) — fait par `deployer`

## Scenarios

### Scenario 1 — Tests automatisés du lot (non-régression critique incluse)

**Validation** : AUTO — **Issues** : #225

**Objectif** : les tests écrits depuis le contrat passent, dont la non-régression « sans groupe, mêmes écritures qu'en v11.6.0 » et « zone Public jamais écrite sans effet ».

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go test -race ./internal/lighting/hue ./internal/config ./internal/server ./internal/ambiance -run 'TW225|GroupTest225|Reader' -count=1` | Tous verts (pilote Hue par zone, validation de `lighting.groups` — 12 par zone, rôle `public` —, API `host<k>` / `public<k>`, essai de groupe) | | |
| 2 | `cd server-go && go test ./cmd/server -run 'TW225' -count=1` | Verts : extinction de la zone Animateur **et** de la zone Public à l'arrêt et à l'interrupteur général ; état des lampes publiques réécrit après un essai (ou éteintes si la lecture est impossible) | | |
| 3 | `cd server-go/web && npx vitest run src/components/LightGroupsManager src/pages/AmbiancePage.groups225.test.jsx` | Tous verts | | |
| 4 | `cd server-go && go test ./internal/lighting/... ./internal/ambiance/... ./cmd/server -run 'Ambiance|Lighting|Hue' -count=1` | Aucune régression sur l'éclairage existant (scènes, pulsation, SCORE, Flash, ON/OFF, extinction) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Sous-onglets de Lumière et colonne « Groupe »

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : l'onglet Lumière est découpé en trois sous-onglets ; la colonne « Groupe » nomme la zone et le numéro (maquette §01).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir Ambiance › Lumière sans pont associé | Étapes 1 et 2 (recherche, association) inchangées, **aucun** sous-onglet | | |
| 2 | Pont associé : ouvrir Lumière | Trois sous-onglets « Pont et ampoules » (actif), « Groupes » (avec le nombre de groupes), « Effets » | | |
| 3 | Sous-onglet « Pont et ampoules » | Contenu d'avant inchangé, plus une colonne « Groupe » : « hors groupe » au départ ; « — » pour une ampoule d'équipe ; « Ambiance · g1 », « 🎤 Animateur · g1 », « 👥 Public · g1 » selon la zone et le rang dans la zone | | |
| 4 | Cliquer sur la pastille d'une ampoule | Bascule sur le sous-onglet « Groupes » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Créer, ordonner et supprimer des groupes (sans nom, trois zones)

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : la gestion des groupes est utilisable à la souris et au clavier, dans chacune des trois zones (maquette §02).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sous-onglet Groupes, aucun groupe : lire la page | Trois zones « Zone Ambiance », « Zone Animateur », « Zone Public », chacune avec son bouton de création (« + Nouveau groupe », « 🎤 + Nouveau groupe animateur », « 👥 + Nouveau groupe public ») ; **aucun** bouton « Créer un groupe par ampoule » | | |
| 2 | « + Nouveau groupe » quatre fois (zone Ambiance) | Groupes « Groupe 1 »…« Groupe 4 », **sans champ de nom** ; « Enregistrer » actif | | |
| 3 | Glisser une ampoule d'un groupe vers un autre, puis vers un groupe d'une **autre zone** | L'ampoule change de groupe (et de zone), elle n'est jamais dans deux groupes | | |
| 4 | Glisser un groupe par sa poignée ⠿ dans sa zone | Son rang change, **les numéros de cette zone se recalculent** ; « Ordre de parcours » de la zone se met à jour (g1 → g2 → … → retour à g1) ; les autres zones ne bougent pas | | |
| 5 | Sans souris : « Déplacer vers… » (liste « 💡 Ambiance · Groupe 1 », « 🎤 Animateur · Groupe 1 »…) et ↑ / ↓ | Même résultat qu'au glisser-déposer | | |
| 6 | Chercher un champ de nom ou « Équipes à proximité » | **Aucun** : un groupe est son numéro dans sa zone | | |
| 7 | Réordonner deux groupes déjà enregistrés d'une zone | Un avertissement « g1, g2… désignent maintenant d'autres groupes dans les effets » apparaît ; « Annuler les modifications » le fait disparaître | | |
| 8 | « Annuler les modifications » | Retour à l'état enregistré | | |
| 9 | Enregistrer, recharger la page (F5) | Les groupes et leur ordre sont conservés dans leur zone | | |
| 10 | Supprimer un groupe, enregistrer | Ses ampoules reviennent dans « hors groupe », les numéros de la zone se tassent | | |
| 11 | Créer 12 groupes dans la zone Ambiance | Le bouton « + Nouveau groupe » se grise (« 12 groupes au plus dans la zone Ambiance ») ; les boutons des deux autres zones restent actifs | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Rôle d'une ampoule et groupes (retrait automatique)

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : une ampoule passée en rôle équipe quitte son groupe, quelle que soit sa zone (contrat §2.1).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ranger l'ampoule A dans un groupe (zone Public, par exemple) et enregistrer | Pastille « 👥 Public · g1 » dans « Pont et ampoules » | | |
| 2 | Passer A au rôle « Équipe Rouges » : lire l'avertissement avant d'enregistrer | Message indiquant qu'elle sera retirée de son groupe | | |
| 3 | Enregistrer | A n'est plus dans aucun groupe ; le groupe (éventuellement vide) est conservé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Zone Animateur : plusieurs groupes, rang propre, blanc fixe

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : jusqu'à 12 groupes animateur, rang propre à la zone, blanc fixe sans effet (maquette `zones`, contrat §2.5).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « 🎤 + Nouveau groupe animateur » deux fois | Deux groupes « Groupe 1 » et « Groupe 2 » dans la zone Animateur, sans nom | | |
| 2 | Y glisser une ampoule générale chacun, enregistrer | Pastilles « 🎤 Animateur · g1 » et « 🎤 Animateur · g2 » ; la numérotation de la zone Ambiance n'est pas décalée | | |
| 3 | Lire la mention de la zone | « sans effet : blanc fixe » ; « n'est entraînée ni par l'éclairage de la salle, ni par ON / OFF, ni par le Flash » | | |
| 4 | « Choisir les effets de la zone » (groupes enregistrés, aucune modification en cours) | Ouvre Effets sur le bloc « Zone Animateur » | | |
| 5 | Supprimer un groupe animateur, enregistrer | Ses ampoules reviennent dans « hors groupe » ; les rangs suivants se renumérotent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Zone Public : rien n'est envoyé tant qu'aucun effet ne lui est attribué

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : la zone Public ne change jamais l'éclairage des lampes du public sans effet choisi (contrat §2.5). Prérequis : une lampe du public réglée à la main (couleur et intensité reconnaissables), rangée dans un groupe de la zone Public.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « 👥 + Nouveau groupe public », y ranger la lampe, enregistrer | Pastille « 👥 Public · g1 » ; mention « sans effet : non pilotée » | | |
| 2 | Redémarrer le serveur, observer la lampe | **Aucun changement** : la lampe garde son réglage manuel (rien n'est envoyé, même au démarrage) | | |
| 3 | Dérouler une partie : attente, chrono, buzz, score, entracte | La lampe du public ne bouge jamais | | |
| 4 | Popover : ON, OFF, Flash | Aucune réaction de la lampe du public | | |
| 5 | Interrupteur général « Éclairage d'ambiance » : désactiver | La lampe du public **s'éteint** (seul cas, avec l'arrêt du serveur) | | |
| 6 | Réactiver, régler la lampe à la main, couper le serveur | À l'arrêt, la lampe du public s'éteint | | |
| 7 | Retirer tous les groupes publics, enregistrer | La zone Public n'existe plus : plus de bloc dans Effets, aucune lampe du public touchée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Essai d'un groupe en séquence sur les ampoules réelles (trois zones)

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : « Tester le groupe » joue une séquence visible de 6 secondes sur les seules ampoules du groupe, puis les rend à leur état vivant (contrat §2.4). À faire **sur chaque zone** : un groupe de la zone Ambiance, un de la zone Animateur, un de la zone Public.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « Tester le groupe » sur un groupe de la zone Ambiance enregistré | Réponse immédiate ; le bouton devient « ■ Arrêter l'essai » avec l'infobulle « Étape n/6 » | | |
| 2 | Observer les ampoules du groupe, une seconde par pas | Dans l'ordre : **éteint**, **blanc 50 %**, **blanc 100 %**, **rouge**, **vert**, **bleu**, chacun environ 1 s ; seules les ampoules du groupe changent | | |
| 3 | Après le 6e pas | Retour à l'état vivant du groupe (scène du moment) ; bouton « Tester le groupe » de nouveau | | |
| 4 | Idem sur un groupe de la zone Animateur | La séquence se voit sur ses ampoules ; retour au **blanc fixe** (ou à son effet) ; les autres groupes ne sont pas touchés | | |
| 5 | Idem sur un groupe de la zone Public (lampe réglée à la main avant l'essai) | La séquence se voit ; **après l'essai la lampe retrouve exactement son réglage d'avant** (couleur et intensité), pas un blanc fixe ni l'extinction | | |
| 6 | Zone Public : éteindre le pont ou rendre la lecture impossible juste avant l'essai | À la fin de l'essai la lampe est **éteinte** (état non lisible ⇒ éteinte) | | |
| 7 | Lancer l'essai du groupe 1, puis aussitôt celui d'un groupe d'une autre zone | Le second **remplace** le premier, qui revient aussitôt à son état vivant ; un seul bouton « Arrêter l'essai » | | |
| 8 | « ■ Arrêter l'essai » en cours de séquence | Le groupe revient aussitôt à son état vivant | | |
| 9 | Pendant l'essai d'un groupe, mettre le popover sur OFF / ON / Flash | Le groupe testé continue sa séquence ; les autres groupes suivent leur règle | | |
| 10 | Modifier un groupe sans enregistrer : survoler « Tester le groupe » | Bouton grisé, infobulle « Enregistrez d'abord » ; groupe vide : « Groupe vide » | | |
| 11 | Désactiver l'éclairage d'ambiance | Bouton grisé (« Éclairage d'ambiance désactivé ») | | |
| 12 | Éteindre le pont (ou débrancher), essayer | Message « Pont injoignable », l'interface reste utilisable (plus de message « essai déjà en cours ») | | |
| 13 | Arrêter le serveur (ou désactiver l'éclairage) pendant un essai | L'essai s'arrête, toutes les ampoules s'éteignent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Non-régression : sans groupe, l'éclairage est celui de v11.6.0

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : aucun changement visible, hors le nouveau rendu de la pulsation, ni de débit tant qu'aucun groupe n'existe (critère d'acceptation).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Supprimer tous les groupes, enregistrer | Aucun groupe | | |
| 2 | Dérouler une partie : attente, question chronométrée (pulsation de fin de chrono), buzz, révélation, score, entracte | Mêmes scènes et même clignotement or de l'équipe créditée qu'en v11.6.0 ; la pulsation de fin de chrono suit le **nouveau rendu** de la v11.7.0 (phase haute au thème, phase basse blanche à 50 % puis orange puis rouge, 500 ms chacune) : ce n'est plus identique à v11.6.0 (décision) | | |
| 3 | Popover Ambiance : ON, OFF, AUTO, Flash | Mêmes effets qu'en v11.6.0 sur toute la salle | | |
| 4 | Couper le serveur (`/shutdown`) | Toutes les ampoules s'éteignent | | |
| 5 | Démarrer avec une configuration issue de la QUALIF 11.7.0.x antérieure (groupes avec `name` / `near_teams`, un groupe animateur unique, `host_effects` à plat) | Le serveur démarre sans erreur ; les groupes sont conservés dans leur zone, les anciens champs ignorés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 9 — Volet « ampoules réelles » : zone Animateur insensible à la salle, extinction

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : la zone Animateur garde son blanc fixe quoi que fasse la salle ; l'interrupteur général et l'arrêt éteignent les zones Animateur et Public (contrat §2.5).
Prérequis : deux groupes animateur (1 à 3 ampoules chacun), un groupe public et au moins un groupe de la zone Ambiance enregistrés.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éclairage actif, partie au repos | Ampoules de la zone Animateur en **blanc fixe** ; la salle suit sa scène ; la zone Public ne change pas | | |
| 2 | Lancer une question, laisser tourner le chrono jusqu'à la pulsation | La salle pulse ; la zone Animateur reste blanc fixe | | |
| 3 | Attribuer des points | Clignotement or de l'équipe seulement sur son ampoule ; salle en scène SCORE ; la zone Animateur reste blanche | | |
| 4 | Popover : OFF | La salle s'éteint ; **la zone Animateur reste allumée** ; la zone Public ne bouge pas | | |
| 5 | Popover : ON, puis Flash | Salle blanche / clignotante ; la zone Animateur ne clignote pas | | |
| 6 | Interrupteur général « Éclairage d'ambiance » : désactiver | Les zones **Animateur et Public** s'éteignent (seul cas avec l'arrêt du serveur) | | |
| 7 | Réactiver, puis couper le serveur | La zone Animateur se rallume en blanc fixe ; à l'arrêt tout s'éteint, zones Animateur et Public comprises | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 10 — Volet « ampoules réelles » : relevés du contrat §8.2 (seuil `SubGroupMinLights`) et groupes Hue par zone

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : confirmer ou amender le seuil de 3 ampoules pour créer un groupe Hue de groupe (contrat §2.3) et vérifier les groupes Hue de chaque zone. Mesure à consigner dans le rapport de QUALIF.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer 2 groupes de 2 ampoules (zone Ambiance), déclencher un changement de scène (ex. buzz) | Noter la durée visible entre la 1re et la dernière ampoule (écriture ampoule par ampoule sous 3) | | |
| 2 | Configurer 2 groupes de 3 ou 4 ampoules, même changement | Noter la durée (écriture par groupe Hue) ; vérifier dans l'application Hue la présence de `buzzmaster-general-1`, `-2` | | |
| 3 | Comparer : un groupe de 2 ampoules est-il visiblement décalé ? | Consigner le chiffre ; si le décalage gêne, proposer d'abaisser `SubGroupMinLights` | | |
| 4 | Groupes animateur et public de 3 ampoules et plus | Groupes Hue `buzzmaster-host`, `buzzmaster-host-1`…, `buzzmaster-public`, `buzzmaster-public-1`… créés (un groupe Hue de zone dès 2 ampoules, de groupe dès 3) ; `buzzmaster-general` ne contient AUCUNE ampoule d'animateur ni de public | | |
| 5 | Config issue de la QUALIF 11.7.0.x antérieure (anciens groupes Hue `buzzmaster-general-host`, `buzzmaster-general-host<k>`) | Les anciens groupes Hue disparaissent à la réconciliation ; le serveur démarre sans erreur | | |
| 6 | Supprimer les groupes, enregistrer | Les groupes Hue `buzzmaster-general-<n>`, `buzzmaster-host…`, `buzzmaster-public…` disparaissent à la réconciliation suivante | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Scénario 1 (AUTO) vert, y compris la non-régression des écritures sans groupe
- [ ] Les scénarios nominaux 2, 3, 5, 7 passent (essai en séquence : éteint, blanc 50 %, blanc 100 %, rouge, vert, bleu)
- [ ] Zone Public : jamais écrite sans effet, éteinte à l'interrupteur et à l'arrêt, état rétabli après un essai (scénarios 6 et 7)
- [ ] Zone Animateur jamais modifiée par la salle, ON / OFF ni le Flash (scénario 9)
- [ ] L'essai en séquence se voit sur chaque zone (scénario 7)
- [ ] Le seuil `SubGroupMinLights` est relevé et consigné (scénario 10)
- [ ] Aucune régression de l'éclairage sans groupe (scénario 8)

## Notes QA

[Espace pour observations — noter les durées du scénario 10.]
