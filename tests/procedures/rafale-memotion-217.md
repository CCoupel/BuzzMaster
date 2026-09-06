# Procédure de Test — RAFALE en carte MEMOTION (#217)

**Version** : 9.0.0.x (milestone v9.0.0, Batch 4 — dernier lot du milestone)
**Date** : 2026-09-05 (mise à jour : second passage, implémentation complète confirmée)
**Issue** : #217
**Maquette** : couverte par `memotion-card-type-selector-184.html` (pas de nouvelle maquette dédiée)
**Testeur** : QA / Utilisateur (validation manuelle obligatoire — jamais exécuté par `qa`/`deployer`)

---

## ✅ Mise à jour second passage

Les deux zones signalées non couvertes au premier passage (scoping `MOTION_CARD_ID` sur
`RAFALE_VALIDATE`/`INVALIDATE`, rendu `PlayerDisplay.jsx`/`AnimConductPanel.jsx`) sont désormais
implémentées et testées automatiquement (`RafaleValidateCard`/`InvalidateCard` côté moteur,
`getTypeState(...).rafale` côté frontend — voir le rapport du second passage). Les scénarios
ci-dessous n'ont pas nécessité de changement de fond, seul le nom exact du champ « durée propre »
(`RAFALE_ROUND_TIME`) a été précisé au Scénario 1.

---

## ⚠️ Mise à jour Batch 2 — retour QUALIF v9.0.0.4 (C1/C3/C4)

Trois régressions ont été détectées en QUALIF v9.0.0.4 sur cette même fonctionnalité, toutes
corrigées dans ce lot de correctifs — **Scénario 5 ci-dessous, à exécuter en priorité** avant de
reconsidérer les Scénarios 1-4 comme acquis :

- **Chrono figé** (C1) : le décompte du temps par question, dans la carte, restait visuellement figé
  côté TV alors que le moteur décomptait bien en interne (`RAFALE_TICK` ne portait pas l'identifiant
  de la carte, un client en contexte carte ne pouvait donc pas savoir qu'un tick lui était destiné).
  Le Scénario 2 (étape 2, « temps décompté ») ne l'avait **pas** détecté — il vérifiait la présence
  du décompte, pas son mouvement visible dans le temps.
- **Montant faux à l'animateur** (C3) : le bouton d'attribution de points affichait le montant
  étoilé PLEIN pour une carte RAFALE, jamais le prorata réellement crédité par le serveur (barème
  `STARS_PRORATA`, comme MEMORY). Le Scénario 2 (étape 5, « barème étoilé appliqué ») ne l'avait pas
  non plus détecté — il ne comparait pas le montant AFFICHÉ au montant réellement attribué.
- **Débordement du sélecteur de type de carte** (C4) : RAFALE devenu nestable a fait passer ce
  sélecteur de 3 à 4 types sans `flex-wrap` — dernier bouton tronqué en largeur réduite.

---

## Contexte

RAFALE devient un sous-type de carte MEMOTION nestable, comme QCM (#185) et MEMORY (#187) avant lui
— une mini-manche RAFALE jouable au sein d'une manche MEMOTION, en **mode SOLO forcé** (la rotation
d'équipes du mode manche classique n'a pas de sens pour une carte). **Aucune génération IA** pour ce
type, décision non rouverte.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, serveur démarré
- [ ] Accès admin (`/admin/quiz`, onglet Questions) et animateur (`/anim`)
- [ ] Un réservoir RAFALE peuplé (au moins une catégorie/difficulté avec plusieurs questions),
      accessible depuis l'onglet **Rafale** de `/admin/quiz`

---

## Scénario 1 — Créer une carte MEMOTION+ de type RAFALE

**Objectif** : Vérifier que RAFALE est sélectionnable comme type de carte, avec ses propres bornes.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer/éditer une question MEMOTION, ajouter une carte | Éditeur de carte affiché | | |
| 2 | Dans le sélecteur de type de carte, choisir « Rafale » | Type disponible dans la liste (aux côtés de Speedy/QCM/Memory) | | |
| 3 | Configurer catégories/difficultés (comme pour une manche classique) | Sélecteur multi-chips identique à celui de #216 | | |
| 4 | Configurer une **durée propre** à la carte (champ « Durée de la manche », `RAFALE_ROUND_TIME` — distinct du temps par question) ET un **nombre de questions maximum** | Les deux champs sont présents et modifiables | | |
| 5 | Enregistrer la carte | Enregistrement réussi | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Jouer la carte en manche MEMOTION

**Objectif** : Vérifier le mode SOLO forcé et l'affichage TV/animateur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la manche MEMOTION, sélectionner la carte RAFALE | Aucune sélection d'équipe proposée pour cette carte — mode SOLO forcé, contrairement à une manche RAFALE classique qui peut être multi | | |
| 2 | Retourner/lancer la carte | Le mini-jeu RAFALE démarre à l'intérieur de la carte (question du réservoir affichée, temps décompté) | | |
| 3 | Observer TV et interface animateur | Rendu cohérent avec le reste de MEMOTION (encart carte), la question/le temps de la carte s'affichent correctement | | |
| 4 | Valider/invalider quelques réponses | Le compteur avance ; jouer une AUTRE carte du même MEMOTION (si présente) en parallèle d'un autre passage ne doit jamais mélanger les compteurs des deux cartes | | |
| 5 | Laisser la carte atteindre sa borne (durée écoulée OU nombre de questions atteint, selon ce qui arrive en premier) | La carte se termine proprement, score attribué au barème étoilé (STARS_PRORATA, comme MEMORY) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Réservoir partagé avec les manches RAFALE classiques

**Objectif** : Vérifier qu'une question utilisée en carte ne réapparaît pas dans une manche classique
de la même partie, et vice-versa.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Noter les questions disponibles pour un couple catégorie/difficulté donné (via l'onglet Rafale ou `GET /api/rafale/pool`) | Compte de référence noté | | |
| 2 | Jouer la carte MEMOTION RAFALE (Scénario 2) sur ce même couple, en notant les questions qui apparaissent | Questions tirées, marquées « déjà utilisées » | | |
| 3 | Lancer ensuite une manche RAFALE **classique** (question du déroulé, pas une carte) sur le même couple | Les questions déjà tirées par la carte au Scénario 2 ne réapparaissent **jamais** dans cette manche classique | | |
| 4 | Inversement : jouer une manche classique en premier, puis une carte sur le même couple | Même garantie dans l'autre sens | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Absence de RAFALE dans la génération IA MEMOTION+

**Objectif** : Vérifier que la décision de ne pas rouvrir la génération IA pour RAFALE est respectée.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la modale de génération IA pour une question MEMOTION+ | Modale affichée avec la distribution de types générables | | |
| 2 | Observer la liste des types proposés/générables | « Rafale » n'apparaît **pas** dans la distribution — seuls Speedy/QCM (et MEMOTION+ lui-même) restent proposés | | |
| 3 | Lancer une génération | Aucune carte de type RAFALE n'est jamais produite par le générateur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Régressions QUALIF v9.0.0.4 : chrono figé, montant faux, sélecteur de carte (C1/C3/C4)

**Objectif** : Vérifier explicitement les 3 symptômes rapportés en QUALIF v9.0.0.4 sur cette même
carte — pas seulement « ça ne plante pas », mais le comportement VISIBLE attendu.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une carte RAFALE (durée par question ≥ 3s pour bien observer), regarder le chrono affiché sur `/tv` **pendant plusieurs secondes sans détourner le regard** | Le chiffre affiché **décompte visiblement** seconde par seconde (ex. 3 → 2 → 1) — jamais figé sur une valeur, même après plusieurs questions successives | | |
| 2 | Sur `/anim`, observer le même chrono de question en contexte carte (zone dédiée à la carte, pas le chrono de manche générique) | Décompte identique à celui de la TV, jamais figé, jamais un doublon de chrono à un autre endroit de l'écran animateur | | |
| 3 | Jouer une carte RAFALE en laissant échouer/manquer volontairement une partie des questions (ex. 2 bonnes réponses sur 3 posées avant d'arrêter la carte) | La carte se termine, le bouton d'attribution de points à l'animateur affiche un montant | | |
| 4 | Comparer le montant affiché à l'étape 3 avec le calcul réel : `valeur étoilée de la carte × bonnes réponses ÷ questions posées` (arrondi à l'entier inférieur) | Le montant AFFICHÉ correspond exactement à ce calcul — **jamais** la valeur étoilée pleine de la carte quand le round est incomplet | | |
| 5 | Répéter l'étape 3-4 en répondant correctement à TOUTES les questions posées avant d'arrêter la carte | Le montant affiché est cette fois la valeur pleine (round complet, prorata = 100%) | | |
| 6 | Dans l'éditeur, ajouter une 2e carte MEMOTION+ et ouvrir le sélecteur de type de carte en largeur de fenêtre réduite (tablette portrait ou fenêtre navigateur rétrécie) | Les 4 types (Speedy/QCM/Memory/Rafale) restent tous visibles et cliquables, quitte à passer sur 2 lignes — aucun bouton tronqué ni poussé hors du cadre visible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] RAFALE sélectionnable comme type de carte, bornée par durée ET nombre de questions
- [ ] Mode SOLO forcé, jamais de sélection d'équipe pour cette carte
- [ ] Barème STARS_PRORATA appliqué correctement, **et affiché correctement à l'animateur**
      (Scénario 5 — le calcul serveur était déjà juste, seul l'affichage était faux en QUALIF v9.0.0.4)
- [ ] Chrono de question en carte visiblement décompté, sur TV **et** animateur, jamais figé
- [ ] Sélecteur de type de carte (4 types) ne déborde/ne tronque jamais, y compris en largeur réduite
- [ ] État de la carte jamais mélangé avec une autre carte ni avec une manche RAFALE classique en
      cours
- [ ] Réservoir partagé confirmé dans les deux sens (carte → classique, classique → carte)
- [ ] Aucune génération IA pour RAFALE, y compris imbriqué

---

## Notes QA

[Espace pour observations, captures d'écran, anomalies constatées]
