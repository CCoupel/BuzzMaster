# Procédure de Test — Carte MEMOTION de type QCM, manche mixte SPEEDY + QCM (#185)

**Version** : v7.0.0 (branche `milestone/v7.0.0`)
**Date** : 2026-08-21
**Testeur** : QA
**Issue** : #185 — première carte MEMOTION réellement jouable au-delà de SPEEDY. QCM en carte est
**affichage + désignation, sans entrée joueur** (contrat §7.1) : les quatre réponses sont montrées,
les indices invalident progressivement des réponses à l'écran, et l'animateur désigne l'équipe
gagnante par le geste `MEMOTION_DONE` habituel — exactement comme une carte SPEEDY.
**Référence** : `contracts/question-types.md` §7/§7.1, plan `_work/reports/plan-memotion-v700-20260821.md`

---

## ⚠️ Prérequis obligatoire — non-régression #160 et #184

Avant de valider cette procédure, s'assurer que `tests/procedures/anim-memotion-160.md` et
`tests/procedures/memotion-card-type-184.md` ont déjà été rejoués et validés sur ce même build (ou
les rejouer si ce n'est pas le cas). Cette procédure-ci ne les remplace pas, elle **ajoute** la
recette spécifique à #185 : une carte QCM réellement en jeu au sein d'une manche MEMOTION.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Un quiz contenant une question MEMOTION **mélangeant** au moins 2 cartes SPEEDY et 2 cartes
      QCM (créées via l'éditeur `/admin` → Questions, cf. `memotion-card-type-184.md` Scénario 4
      pour créer une carte QCM), avec sur au moins une carte QCM les indices activés
      (`QCM_HINTS_ENABLED=true`) et un chrono de carte assez long pour observer les seuils (ex: 20s)
- [ ] Trois postes/onglets ouverts : `/anim` (tablette), `/tv` (affichage), `/admin` (régie)
- [ ] Au moins 2 équipes actives avec bumpers assignés

---

## Scénario 1 — Carte QCM conduite de bout en bout depuis `/anim`, affichée sur `/tv`

**Objectif** : Vérifier le parcours complet SELECTED → QUESTION → REVEAL → DONE d'une carte QCM,
identique dans sa conduite à une carte SPEEDY (AC1 de #185).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger la question MEMOTION mixte, LANCER | Grille visible sur `/anim` ET `/tv`, cartes SPEEDY et QCM indiscernables en `GRID` (thème + étoiles seulement, pas de différence visuelle avant sélection) | | |
| 2 | Sélectionner une carte QCM depuis `/anim` | Sous-phase `SELECTED` : carte au premier plan (thème + points), boutons DÉMARRER/ANNULER — **identiques** à une carte SPEEDY | | |
| 3 | Taper DÉMARRER | Sous-phase `QUESTION` : sur `/anim` **et** `/tv`, les **4 réponses de la carte** s'affichent (pas de contenu SPEEDY) ; chrono de la carte démarre ; boutons de conduite STOP CHRONO/RÉVÉLER/SANS VAINQUEUR **identiques** à une carte SPEEDY (aucun bouton QCM propre — contrat §7.1) | | |
| 4 | Observer la grille de réponses | **Aucun élément cliquable** parmi les 4 réponses, sur `/anim` comme sur `/tv` — affichage seul | | |
| 5 | Taper RÉVÉLER | Sous-phase `REVEAL` : la bonne réponse est marquée en couleur/mise en évidence sur `/anim` ET `/tv`, la grille des 4 réponses reste affichée (pas de saut visuel) | | |
| 6 | Désigner l'équipe gagnante (🏆 <Équipe>) | Carte `DONE`, points attribués selon le barème de l'hôte (étoiles/difficulté — **pas de barème propre à QCM**, contrat §6.1), retour à `GRID` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Indices QCM sur le timer de carte : seuils, apparition progressive

**Objectif** : Vérifier que le mécanisme d'indices d'une carte QCM se comporte comme celui d'une
question QCM classique (AC2 de #185) — sur les données observables directement : quelles réponses
s'invalident, et quand.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer une carte QCM avec indices activés (chrono ≥ 20s) | Aucune réponse invalidée au démarrage | | |
| 2 | Laisser le chrono descendre jusqu'au premier seuil (25% du temps écoulé par défaut) | Une réponse fausse s'invalide (grisée/barrée) sur `/anim` **et** `/tv`, jamais la bonne réponse | | |
| 3 | Laisser le chrono descendre jusqu'au second seuil (12,5% du temps restant par défaut) | Une **deuxième** réponse fausse s'invalide, la première le reste | | |
| 4 | Comparer au comportement d'une **question QCM classique** (hors carte) avec les mêmes seuils | Même logique d'apparition progressive (une réponse à la fois, jamais la bonne, jamais deux fois la même) | | |
| 5 | ⚠️ **Point d'attention connu** — observer la **barre de chrono** elle-même (pas la grille de réponses) | À la différence d'une question QCM classique, la barre de chrono d'une carte QCM ne porte **pas** les repères visuels (petits traits) indiquant à l'avance où les seuils se déclencheront — **comportement actuellement différent, signalé au CDP, pas un bug à corriger silencieusement par QA**. Noter si observé, ne pas bloquer la validation sur ce seul point. | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Non-régression : cartes SPEEDY de la même manche

**Objectif** : Vérifier qu'une manche mixte n'introduit aucune régression sur les cartes SPEEDY
qu'elle contient (AC1 de #185, "sans régression sur les cartes SPEEDY de la même manche").

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dans la même manche mixte, jouer une carte SPEEDY jusqu'à `DONE` | Déroulé strictement identique à avant #185 : thème → question → réponse texte/image → désignation | | |
| 2 | Alterner cartes SPEEDY et QCM plusieurs fois de suite | Aucune confusion de contenu entre cartes (jamais la grille QCM sur une carte SPEEDY, jamais le texte/image SPEEDY sur une carte QCM) | | |
| 3 | Terminer la manche (toutes les cartes `DONE`, mix SPEEDY/QCM) | Arrêt automatique de la question, comme pour une manche 100% SPEEDY | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Aucune action entrante nouvelle (Risque R7)

**Objectif** : Vérifier qu'une carte QCM active n'ouvre AUCUNE voie d'entrée joueur — ni buzzer
physique, ni VJoueur (AC3 de #185, contrat §7.1). C'est un garde-fou serveur déjà verrouillé par
test automatisé (`TestMEMOTION_QCMCard_NoNewInboundAction`,
`TestHandleVPlayerQCMAnswer_MEMOTIONQuestion_QCMCard_NoEffect`) — ce scénario en est la
**confirmation visuelle/manuelle**, utile pour QA qui n'a pas accès aux tests automatisés.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Carte QCM active en sous-phase `QUESTION`, presser un buzzer physique assigné à une équipe | **Aucun effet observable** : pas de son de buzz, pas de mise en évidence de bumper sur `/admin`/`/tv`, la sous-phase reste `QUESTION` | | |
| 2 | Même carte, tenter de répondre depuis un poste VJoueur (si le quiz a des VJoueurs configurés) | **Aucun effet observable** : aucune réponse enregistrée, aucun changement d'état | | |
| 3 | Comparer à une carte SPEEDY active dans la même manche | Comportement identique — MEMOTION ignore déjà tout buzz/VJoueur, avec ou sans carte QCM | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Carte QCM conduite de bout en bout depuis `/anim`, affichée correctement sur `/tv`
- [ ] Indices QCM déclenchés sur le timer de carte : seuils respectés, apparition progressive, jamais la bonne réponse invalidée
- [ ] Aucune régression sur les cartes SPEEDY d'une même manche mixte
- [ ] Aucune action entrante nouvelle (buzz et VJoueur restent sans effet sur une carte QCM)
- [ ] Point d'attention noté (barre de chrono sans repères visuels sur carte QCM) — signalé, pas bloquant

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go build ./... && go test ./... -race` | Build OK, tous les tests PASS, y compris les suites `*_185_test.go` (C-B1/C-B2) | | |
| 2 | `cd server-go/web && npx vitest run` | Tous les tests PASS, y compris `AnimConductPanel.test.jsx` (describe "L3, carte QCM #185/C-F1") et `PlayerDisplay.memotion.test.jsx` (describe "carte MEMOTION de type QCM #185/C-F2") | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
