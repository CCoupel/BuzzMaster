# Procédure de Test — Groupes d'ambiance et groupe animateur (#225, milestone v11.7.0, LOT-1)

**Version** : v11.7.0.x (QUALIF)
**Date** : 2026-11-11
**Testeur** : QA (scénarios AUTO) / Utilisateur (scénarios HUMAIN)
**Contrat** : `contracts/light-effects.md` §2 (dont §2.5), `contracts/hue-bridge.md` §11, `contracts/http-endpoints.md`
**Maquette** : `docs/mockup/v11.7.0/ui/ambiance_page__light_groups.html`
**Plan** : `.claude/workflow/plan-11.7.0.md` (LOT-1, tâches 2 à 6, 32)

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
| 3 | Sous-onglet « Pont et ampoules » | Contenu d'avant inchangé, plus une colonne « Groupe » (« hors groupe » au départ ; « — » pour une ampoule d'équipe) | | |
| 4 | Cliquer sur « hors groupe » d'une ampoule | Bascule sur le sous-onglet « Groupes » | | |
| 5 | Sous-onglet « Effets » | Message « Bientôt », aucun réglage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Créer, nommer, ordonner et supprimer des groupes

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : la gestion des groupes est utilisable à la souris et au clavier (maquette §02).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sous-onglet Groupes, aucun groupe : lire le texte d'aide | « l'éclairage se comporte comme avant » ; boutons « Créer un groupe par ampoule », « + Nouveau groupe », « 🎤 + Créer le groupe animateur » | | |
| 2 | « Créer un groupe par ampoule » | Un groupe par ampoule générale, nommé d'après elle, numéroté 1…N ; « Enregistrer » actif | | |
| 3 | Glisser une ampoule d'un groupe vers un autre | L'ampoule change de groupe, elle n'est jamais dans deux groupes | | |
| 4 | Glisser un groupe par sa poignée ⠿ | Son rang change, **les numéros se recalculent** ; « Ordre de parcours » se met à jour | | |
| 5 | Sans souris : « Déplacer vers… » (liste) et ↑ / ↓ | Même résultat qu'au glisser-déposer | | |
| 6 | Saisir deux fois le même nom (casse différente), puis un nom vide | Message d'erreur sous le champ, « Enregistrer » grisé | | |
| 7 | Ajouter l'équipe « Rouges » à un groupe | Elle disparaît de la liste des autres groupes | | |
| 8 | « Annuler les modifications » | Retour à l'état enregistré | | |
| 9 | Enregistrer, recharger la page (F5) | Les groupes, leur ordre et leurs équipes sont conservés | | |
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

### Scenario 5 — Groupe animateur : création et règles

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : un seul groupe animateur, hors numérotation (maquette §03, contrat §2.5).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « 🎤 + Créer le groupe animateur » | Bloc « Groupe animateur » sans numéro, sans « équipes à proximité » ; le bouton de création disparaît | | |
| 2 | Y glisser une ampoule générale, enregistrer | Pastille « 🎤 nom (animateur) » dans « Pont et ampoules » ; la numérotation des groupes de salle n'est pas décalée | | |
| 3 | Lire la mention du bloc | « Hors des effets de salle · blanc fixe par défaut » ; « Choisir ses effets (bientôt) » inactif | | |
| 4 | Créer 12 groupes au total (animateur compris) | « + Nouveau groupe » grisé, message « 12 groupes au plus » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Essai d'un groupe sur les ampoules réelles

**Validation** : HUMAIN — **Issues** : #225

**Objectif** : « Tester le groupe » fait clignoter en blanc exactement les ampoules du groupe, puis restaure l'état vivant (contrat §2.4).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « Tester le groupe » sur un groupe de salle enregistré | Seules ses ampoules clignotent en blanc, puis retrouvent leur état | | |
| 2 | Idem sur le groupe animateur | Ses ampoules (seules) clignotent puis retrouvent leur état | | |
| 3 | Modifier un groupe sans enregistrer : survoler « Tester le groupe » | Bouton grisé, infobulle « Enregistrez d'abord » ; groupe vide : grisé aussi | | |
| 4 | Désactiver l'éclairage d'ambiance, essayer | Bouton grisé (ou message « L'éclairage d'ambiance est désactivé ») | | |
| 5 | Éteindre le pont (ou débrancher), essayer | Message « Pont injoignable ou refusé », l'interface reste utilisable | | |

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

**Objectif** : l'animateur garde son blanc fixe quoi que fasse la salle (contrat §2.5).
Prérequis : un groupe animateur (1 à 3 ampoules) et au moins un groupe de salle enregistrés.

> **Point connu** : le blanc fixe automatique du groupe animateur est livré par la tâche 32 (`Scene()`). Si ce scénario échoue
> seulement sur « blanc fixe », vérifier d'abord que la tâche 32 est dans le binaire QUALIF.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éclairage actif, partie au repos | Ampoules de l'animateur en **blanc fixe** ; la salle suit sa scène | | |
| 2 | Lancer une question, laisser tourner le chrono jusqu'à la pulsation | La salle pulse ; l'animateur reste blanc fixe | | |
| 3 | Attribuer des points | Clignotement or de l'équipe seulement sur son ampoule ; salle en scène SCORE ; l'animateur reste blanc | | |
| 4 | Popover : OFF | La salle s'éteint ; **l'animateur reste allumé** | | |
| 5 | Popover : ON, puis Flash | Salle blanche / clignotante ; l'animateur ne clignote pas | | |
| 6 | Interrupteur général « Éclairage d'ambiance » : désactiver | L'animateur **s'éteint** (seul cas avec l'arrêt du serveur) | | |
| 7 | Réactiver, puis couper le serveur | L'animateur se rallume ; à l'arrêt tout s'éteint | | |

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
| 4 | Groupe animateur de 3 ampoules et plus | Groupe Hue `buzzmaster-general-host` créé ; `buzzmaster-general` ne contient PAS les ampoules de l'animateur | | |
| 5 | Supprimer les groupes, enregistrer | Les groupes Hue `buzzmaster-general-<n>` disparaissent à la réconciliation suivante | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Scénario 1 (AUTO) vert, y compris la non-régression des écritures sans groupe
- [ ] Les scénarios nominaux 2, 3, 5, 6 passent
- [ ] Un groupe animateur n'est jamais modifié par la salle, ON / OFF ni le Flash (scénario 8)
- [ ] Le seuil `SubGroupMinLights` est relevé et consigné (scénario 9)
- [ ] Aucune régression de l'éclairage sans groupe (scénario 7)

## Notes QA

[Espace pour observations — noter les durées du scénario 9.]
