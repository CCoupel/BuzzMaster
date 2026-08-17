# Procédure de Test — Liste ARDOISE sur `/anim` (#158)

**Version** : v6.2.x (branche `feature/anim-question-display`)
**Date** : 2026-08-16
**Testeur** : QA
**Issue** : #158 — liste ARDOISE dans la colonne équipes de `/anim`, allégée (crédit porté par #170)
**Référence** : Plan `_work/reports/plan-20260816-125123.md` §6, `_work/reports/plan-20260816-171200.md`
(détail tri/délai/diffusion)

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] `/anim` sur une tablette (ou navigateur 1024×768), `/admin` sur un second poste
- [ ] Un quiz avec une question ARDOISE
- [ ] Au moins 3 équipes avec un VJoueur (application mobile) actif ; au moins une équipe SANS
      VJoueur (pour vérifier le filtre de parité #93)
- [ ] Prévoir une copie volontairement longue (plusieurs phrases) pour le test de lisibilité

---

## Scénario 1 — Réception des copies EN DIRECT (valide B1)

**Objectif** : Vérifier que `/anim` voit les réponses arriver au fur et à mesure, pas seulement
au changement de phase.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question ARDOISE (STARTED), ouvrir `/anim` | Liste ARDOISE affichée, équipes sans réponse | | |
| 2 | Depuis un VJoueur, taper une réponse lettre par lettre | Le texte apparaît sur `/anim` **au fur et à mesure de la frappe** (pas seulement à la validation) — comparer avec `/admin` : même comportement | | |
| 3 | Faire taper plusieurs équipes simultanément (rafale) | `/anim` reste réactif, aucun texte de copie perdu ou tronqué | | |
| 4 | Valider les réponses | Les rangs se mettent à jour selon l'ordre d'arrivée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Ordre de frappe et délais

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Faire répondre 3 équipes à des instants différents | Classées dans l'ordre de PREMIÈRE frappe (pas de validation) | | |
| 2 | Observer le délai affiché à côté de chaque équipe classée | Cohérent avec l'écart réel observé (en secondes, 3 décimales) | | |
| 3 | Laisser une équipe sans répondre du tout | Apparaît en fin de liste, sans rang ni délai | | |
| 4 | Comparer l'ordre/les délais avec `/admin` sur la même manche | Identiques (source unique `ardoiseOrder.js`) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Arbitrage ligne à ligne en REVEALED

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Arrêter la question, presser RÉPONSE (REVEALED) | Chaque ligne avec réponse propose "+N pts" et "0 pt" (AnimCreditControl) | | |
| 2 | Créditer une équipe ("+N pts") | Ligne verrouillée (✓ +N pts), plus de bouton | | |
| 3 | Refuser une autre équipe ("0 pt") | Ligne verrouillée (✓ 0 pt), plus de bouton | | |
| 4 | Sur une équipe SANS réponse | Seul "0 pt" est proposé (pas de "+N pts") | | |
| 5 | Observer `/admin` en parallèle | Les verrous se reflètent sur `/admin`/toute autre tablette (comportement #170) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Filtre équipes à joueur virtuel (parité #93)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avoir une équipe SANS VJoueur actif (uniquement des bumpers physiques) parmi les équipes | Cette équipe **n'apparaît PAS** dans la liste ARDOISE sur `/anim` | | |
| 2 | Comparer avec `/admin` | Même filtre, même liste d'équipes affichées | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Lisibilité d'une copie longue en 1024×768

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Redimensionner `/anim` à 1024×768 | Page tient, `overflow: hidden` respecté | | |
| 2 | Faire taper une réponse longue (plusieurs phrases) depuis un VJoueur | Le texte s'affiche **intégralement**, retour à la ligne (word-break), **jamais tronqué** ni coupé par "…" | | |
| 3 | Vérifier que la ligne ne déborde pas de la colonne équipes | Pas de chevauchement avec la zone contexte ou la zone conduite | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Non-régression `/admin`

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Conduire la même manche ARDOISE depuis `/admin` (sans toucher à `/anim`) | Comportement strictement identique à avant #158 : liste, tri, délai, bouton de crédit `/admin` inchangé | | |
| 2 | `go build ./...`, `go test ./... -race` | Build OK, tous les tests PASS | | |
| 3 | `npm test` | Tous les tests PASS, y compris `AnimArdoiseList.test.jsx`, `ardoiseOrder.test.js` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Copies visibles en direct sur `/anim`, comme `/admin` (Scénario 1)
- [ ] Ordre de frappe et délais corrects, identiques à `/admin` (Scénario 2)
- [ ] Crédit/refus ligne à ligne fonctionnel et synchronisé (Scénario 3)
- [ ] Seules les équipes à joueur virtuel apparaissent (Scénario 4)
- [ ] Copie longue lisible, non tronquée, en 1024×768 (Scénario 5)
- [ ] Aucune régression `/admin` (Scénario 6)

---

## Notes QA

[Espace pour observations]
