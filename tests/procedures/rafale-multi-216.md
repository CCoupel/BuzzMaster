# Procédure de Test — RAFALE multi-catégorie / multi-difficulté (#216)

**Version** : 9.0.0.x (milestone v9.0.0, Batch 1 + Batch 2)
**Date** : 2026-09-04 (mise à jour : Lot 2 frontend livré)
**Issue** : #216
**Maquette** : `docs/mockups/rafale-multi-216.html`
**Testeur** : QA / Utilisateur (validation manuelle obligatoire — jamais exécuté par `qa`/`deployer`)

---

## ⚠️ Mise à jour Batch 2 — Lot 2 frontend livré, N/A levés

Le Lot 2 frontend (sélecteur multi-catégories/difficultés + barème dans l'éditeur, chips multiples
sur la card question, affichage du barème sur TV/animateur, `RafalePoolAlert.jsx` en union) est
désormais livré et couvert par des tests automatisés (`_work/reports/test-writer-216-lot2-*.md`).
**Les scénarios 1 à 3 sont donc redevenus testables via l'interface normale** (méthode API conservée
en note pour qui préfère vérifier le moteur isolément) — les N/A précédents sont levés.

✅ **Régression détectée pendant la rédaction des tests automatisés, CORRIGÉE** (dev-frontend, commits
`8d76cc25`/`cca2b91b`, confirmée par re-test indépendant) : une question RAFALE mono, créée avant
#216, dont la difficulté n'avait jamais été explicitement configurée pouvait rester bloquée au
démarrage — régression du bug historique SHA 75b0472c (QUALIF 8.0.0.5). Fix : repli `[1]` restauré
localement dans `GamePage.jsx` (par-dessus `effectiveRafaleDifficulties`, jamais dans l'éditeur
`QuestionsPage.jsx` qui doit montrer l'état réel). La sous-étape correspondante du Scénario 6
ci-dessous reste comme test de non-régression, mais n'est plus un échec attendu.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, serveur démarré
- [ ] Accès admin (`/admin/quiz`, onglet Rafale) pour préparer le réservoir
- [ ] `curl` disponible (scénarios 1-3, méthode API) — ou tout client HTTP équivalent (Postman...)
- [ ] Au moins 2 catégories de réservoir RAFALE peuplées avec plusieurs questions chacune (ex.
      HISTORY et SCIENCE, difficultés 1 et 2) — préparables depuis l'onglet **Rafale** de
      `/admin/quiz`

---

## Scénario 1 — Configurer une manche avec plusieurs catégories ET difficultés

**Objectif** : Vérifier que l'éditeur accepte un filtre multi et que le moteur joue effectivement sur
l'union.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/quiz`, créer une question de type RAFALE | Section RAFALE affichée dans l'éditeur, avec un sélecteur de catégories en chips (plus le CategorySelector générique — retiré pour ce type) | | |
| 2 | Cliquer plusieurs chips de catégorie (ex. Histoire, Sciences) | Chaque chip cliqué devient actif, sélection non exclusive (plusieurs actifs en même temps) | | |
| 3 | Cliquer plusieurs chips de difficulté (ex. ★ et ★★★) | Même comportement non exclusif | | |
| 4 | Enregistrer la question | Enregistrement réussi | | |
| 5 | Lancer la manche depuis `/admin` (GamePage) | La manche démarre normalement, sans erreur | | |
| 6 | Jouer plusieurs questions à la suite (valider/invalider) en notant la catégorie/difficulté de chaque question affichée | Les questions proviennent des **deux** catégories et des **deux** difficultés sélectionnées — jamais d'une catégorie/difficulté hors de cet ensemble | | |

**Verdict** : [ ] PASS  [ ] FAIL

*(Méthode API alternative, pour isoler le moteur backend sans passer par l'éditeur : `POST
/questions` multipart avec `RAFALE_CATEGORIES`/`RAFALE_DIFFICULTIES` en JSON — voir
`_work/reports/test-writer-procedures-215-216-*.md` pour le détail des champs.)*

---

## Scénario 2 — Affichage en chips multiples sur la card question

**Objectif** : Vérifier que la card question affiche N chips au lieu d'une valeur scalaire.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/quiz`, repérer la card de la question RAFALE multi créée au Scénario 1 | La card affiche **plusieurs badges** de catégorie (un par catégorie sélectionnée) et **plusieurs chips étoilés** de difficulté (un par difficulté sélectionnée), pas une seule valeur | | |
| 2 | Repérer une question RAFALE mono, créée avant #216 (si disponible) | La card affiche un seul badge catégorie et un seul chip difficulté — comportement identique à avant #216, sans reconfiguration | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Barème par difficulté, affiché sur TV et interface animateur

**Objectif** : Vérifier que le barème configuré est bien celui affiché en jeu.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dans l'éditeur, après avoir sélectionné au moins une difficulté, repérer la zone « Points par bonne réponse, selon la difficulté » | Un champ de saisie par difficulté sélectionnée, vide par défaut (placeholder = barème général de la manche) | | |
| 2 | Saisir des valeurs différentes par difficulté (ex. 5 pour ★, 15 pour ★★★) puis enregistrer | Enregistrement réussi | | |
| 3 | Lancer la manche, observer l'écran TV et l'interface animateur à chaque question | La valeur en points affichée correspond au barème de la difficulté de la question EN COURS (varie d'une question à l'autre) | | |
| 4 | Éditer une question RAFALE **sans** saisir de barème par difficulté | Aucun champ de barème saisi ; en jeu, la valeur affichée retombe sur le `POINTS` générique de la manche | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Épuisement d'un couple en cours de manche (rééquilibrage sans blocage)

**Objectif** : Vérifier qu'un couple épuisé sort de la rotation sans interrompre la manche.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Préparer un réservoir avec 4 couples (ex. HISTORY×1, HISTORY×2, SCIENCE×1, SCIENCE×2) — un seul couple avec **1 seule** question (ex. SCIENCE×2), les 3 autres avec plusieurs | Réservoir préparé | | |
| 2 | Lancer une manche RAFALE multi sur ces 4 couples | Démarrage normal | | |
| 3 | Jouer suffisamment de questions pour épuiser le couple pauvre (SCIENCE×2) | Une fois SCIENCE×2 épuisé, la manche **continue** — aucune interruption, aucun message d'erreur bloquant | | |
| 4 | Continuer à jouer | Les questions suivantes proviennent uniquement des 3 couples restants (HISTORY×1, HISTORY×2, SCIENCE×1) — SCIENCE×2 n'est plus jamais proposé | | |
| 5 | Épuiser progressivement TOUS les couples | La manche se termine proprement (`RAFALE_EXHAUSTED`/fin de manche) uniquement quand **plus aucun** couple n'a de question disponible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Couple vide dès le lancement (même comportement qu'un épuisement)

**Objectif** : Vérifier le comportement unifié lancement/cours de manche.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer une manche sur 4 couples dont un est **vide dès le départ** (aucune question au réservoir pour ce couple, ex. SPORTS×3) | Configuration acceptée | | |
| 2 | Lancer la manche | Démarrage normal, **aucun blocage** malgré le couple vide | | |
| 3 | Jouer plusieurs questions | Aucune question ne provient jamais du couple vide (SPORTS×3) — identique au comportement du Scénario 4 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Rétro-compatibilité : question RAFALE mono existante (créée avant #216)

**Objectif** : Vérifier qu'une question créée avant #216 (un seul CATEGORY, un seul
RAFALE_DIFFICULTY, sans les nouveaux champs liste) continue de fonctionner sans reconfiguration.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Identifier (ou créer via l'interface standard, sans passer par l'API multi) une question RAFALE mono-catégorie/mono-difficulté existante | Question présente, format ancien (pas de `RAFALE_CATEGORIES`/`RAFALE_DIFFICULTIES`) | | |
| 2 | Lancer une manche sur cette question, sans aucune modification | Démarrage normal — aucune erreur, aucun message de configuration invalide | | |
| 3 | Jouer plusieurs questions | Toutes proviennent de la catégorie/difficulté unique d'origine (comportement identique à avant #216) | | |
| 4 | Non-régression (fix `8d76cc25`/`cca2b91b`, voir note en tête de fichier) — tester spécifiquement une question mono dont la difficulté n'a **jamais** été explicitement enregistrée (`RAFALE_DIFFICULTY` réellement absent, pas juste « star 1 » cochée puis sauvegardée) : catégorie définie, difficulté jamais touchée | Le bouton START reste cliquable (pas de blocage fail-closed) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Endpoint `GET /api/rafale/pool` avec paramètres pluriels

**Objectif** : Vérifier le comptage pré-manche avec le nouveau format de requête.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Exécuter `curl "http://localhost/api/rafale/pool?categories=HISTORY,SCIENCE&difficulties=1,2"` | Réponse JSON `{"AVAILABLE": N, "USED": N, "TOTAL": N}` — N = somme sur l'union des 4 couples, décoys (autres catégories) exclus | | |
| 2 | Comparer avec le décompte manuel (nombre de questions au réservoir pour chacun des 4 couples, non utilisées) | Les chiffres correspondent | | |
| 3 | Exécuter la même requête sans `difficulties` (ou vide) | Réponse 400 (paramètre requis, comme l'ancien format singulier) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénarios 1 à 7 : PASS via l'interface normale (Lot 2 livré, méthode API conservée en repli)
- [ ] Aucun blocage observé sur couple vide/épuisé, à aucun moment
- [ ] Rétro-compatibilité confirmée sans reconfiguration manuelle, y compris le cas régression
      SHA 75b0472c (Scénario 6, étape 4 : difficulté mono jamais explicitement configurée)

---

## Notes QA

[Espace pour observations, réponses JSON brutes des requêtes curl, anomalies constatées]
