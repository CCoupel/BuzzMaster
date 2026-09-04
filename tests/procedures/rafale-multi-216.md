# Procédure de Test — RAFALE multi-catégorie / multi-difficulté (#216)

**Version** : 9.0.0.x (milestone v9.0.0, Batch 1 — **backend uniquement**)
**Date** : 2026-09-04
**Issue** : #216
**Maquette** : `docs/mockups/rafale-multi-216.html`
**Testeur** : QA / Utilisateur (validation manuelle obligatoire — jamais exécuté par `qa`/`deployer`)

---

## ⚠️ Prérequis — périmètre livré à ce jour

**Seul le Lot 1B (moteur backend) est livré à la date de rédaction.** Le Lot 2 frontend (chips
multiples sur la card question `QuestionsPage.jsx`, sélecteur multi-catégories/difficultés dans
l'éditeur, affichage du barème résolu sur TV/animateur, `RafalePoolAlert.jsx` en union) **n'est pas
encore développé** — le formulaire d'édition RAFALE dans `/admin/quiz` n'accepte encore qu'**une
seule** catégorie et **une seule** difficulté par l'interface.

**Conséquence pour cette procédure** :
- Les scénarios 1 à 3 (configuration multi, affichage chips, barème sur TV/anim) **ne sont pas
  testables via l'interface actuelle** — une méthode API alternative (`curl`) est fournie pour
  valider le moteur backend dès maintenant, en attendant le Lot 2.
- Les scénarios 4 à 7 (épuisement, couple vide, rétro-compatibilité, endpoint pluriel) sont
  **pleinement testables dès aujourd'hui**, y compris via l'interface pour ceux qui ne nécessitent
  pas de configuration multi (rétro-compat mono, scénario 6).
- **Refaire passer les scénarios 1 à 3 en méthode UI une fois le Lot 2 livré** — cette procédure sera
  alors mise à jour (ou une nouvelle version publiée) pour retirer la mention Lot 2.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL, serveur démarré
- [ ] Accès admin (`/admin/quiz`, onglet Rafale) pour préparer le réservoir
- [ ] `curl` disponible (scénarios 1-3, méthode API) — ou tout client HTTP équivalent (Postman...)
- [ ] Au moins 2 catégories de réservoir RAFALE peuplées avec plusieurs questions chacune (ex.
      HISTORY et SCIENCE, difficultés 1 et 2) — préparables depuis l'onglet **Rafale** de
      `/admin/quiz`

---

## Scénario 1 — Configurer une manche avec plusieurs catégories ET difficultés (méthode API)

**Objectif** : Vérifier que le moteur accepte un filtre multi et joue effectivement sur l'union.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer/éditer une question RAFALE via `POST /questions` (multipart), en envoyant `RAFALE_CATEGORIES` = `["HISTORY","SCIENCE"]` (JSON) et `RAFALE_DIFFICULTIES` = `[1,2]` (JSON), en plus des champs standards (TYPE=RAFALE, QUESTION, etc.) | Réponse 200/201, question enregistrée | | |
| 2 | Recharger `/admin/quiz`, ouvrir la question créée en édition | La question est reconnue comme RAFALE (le formulaire peut n'afficher qu'une valeur mono par limitation d'interface actuelle — **attendu**, voir prérequis) | | |
| 3 | Lancer la manche depuis `/admin` (GamePage) | La manche démarre normalement, sans erreur | | |
| 4 | Jouer plusieurs questions à la suite (valider/invalider) en notant la catégorie/difficulté de chaque question affichée (visible dans le payload WebSocket `RAFALE_CURRENT_QUESTION` ou via les outils développeur réseau) | Les questions proviennent des **deux** catégories (HISTORY et SCIENCE) et des **deux** difficultés (1 et 2) — jamais d'une catégorie/difficulté hors de cet ensemble | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (Lot 2 UI non disponible, testé en API uniquement)

---

## Scénario 2 — Affichage en chips multiples sur la card question

**Objectif** : Vérifier que la card question affiche N chips au lieu d'une valeur scalaire.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/admin/quiz`, repérer la card de la question RAFALE multi créée au Scénario 1 | La card affiche **plusieurs chips** de catégorie (HISTORY, SCIENCE) et **plusieurs chips** de difficulté (1, 2), pas une seule valeur | | |

**Verdict** : [ ] N/A — **Lot 2 frontend non livré à ce jour.** Marquer EN ATTENTE, à rejouer après
livraison de `QuestionCard.jsx` (chips multiples).

---

## Scénario 3 — Barème par difficulté, affiché sur TV et interface animateur

**Objectif** : Vérifier que le barème configuré est bien celui affiché en jeu.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Éditer la question RAFALE multi, envoyer `RAFALE_POINTS_BY_DIFFICULTY` = `{"1":5,"2":15}` (JSON) via `POST /questions` | Question enregistrée avec le barème | | |
| 2 | Lancer la manche, observer le payload WebSocket `RAFALE_CURRENT_QUESTION.POINTS` à chaque question | La valeur correspond au barème de la difficulté tirée (5 pour difficulté 1, 15 pour difficulté 2) | | |
| 3 | *(une fois Lot 2 livré)* Observer l'écran TV et l'interface animateur pendant la manche | La valeur de la question en cours (POINTS résolu) est visible à l'écran | | |
| 4 | Éditer une question RAFALE **sans** `RAFALE_POINTS_BY_DIFFICULTY` | `RAFALE_CURRENT_QUESTION.POINTS` retombe sur la valeur générique `POINTS` de la manche | | |

**Verdict** : [ ] PASS (moteur, étapes 1-2-4)  [ ] N/A (étape 3, affichage TV/anim — Lot 2 non livré)

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

- [ ] Scénarios 4, 5, 6, 7 : PASS (pleinement testables, backend seul)
- [ ] Scénarios 1, 3 (moteur) : PASS via méthode API
- [ ] Scénarios 2, 3 (affichage) : marqués N/A, à rejouer explicitement après livraison du Lot 2
      frontend — **ne pas fermer #216 comme totalement validé tant que ces 2 points restent N/A**
      si la clôture attendue couvre l'issue complète (à clarifier avec le CDP selon le découpage
      réel du lot)
- [ ] Aucun blocage observé sur couple vide/épuisé, à aucun moment
- [ ] Rétro-compatibilité confirmée sans reconfiguration manuelle

---

## Notes QA

[Espace pour observations, réponses JSON brutes des requêtes curl, anomalies constatées]
