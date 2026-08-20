# Procédure de Test — Cartes Memory + Contraste type de question TV (#108 + #114)

**Version** : milestone v6.5.2 (Phase 1)
**Date** : 2026-08-20
**Testeur** : QA
**Issues** : #108 — cartes Memory trop petites sur `/tv` ; #114 — lisibilité du texte du type de
question en phase READY sur `/tv`
**Référence** : Maquette validée `docs/mockups/tv-memory-contrast-108-114.html`, handoff
`_work/handoff/task-dev-frontend-impl-20260820-1127.md`

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Un quiz contenant au moins :
  - Une question MEMORY à petite grille (ex. 3 paires / 6 cartes) et une à grande grille (ex.
    10 paires / 20 cartes)
  - Une question de chaque type : SPEEDY, ARDOISE, QCM, MEMOTION
  - Idéalement un mot long (ex. "ELEPHANT", "ANTICONSTITUTIONNELLEMENT") disponible sur une
    carte Memory pour tester la troncature
- [ ] `/tv` ouvert sur un écran ou une fenêtre à résolution TV standard (1920×1080 ou équivalent)
- [ ] `/admin` ouvert sur un second poste pour piloter la partie
- [ ] Page de configuration des fonds d'écran (`/admin` → réglages fonds d'écran) accessible —
      vérifier qu'au moins un fond clair, un fond coloré vif et un fond sombre sont présents dans
      la rotation ; ajuster temporairement la durée de rotation à une valeur courte si besoin pour
      accélérer le test
- [ ] Contrainte à garder en tête pendant tout le test : `/tv` est un affichage **STATIQUE** —
      aucun scroll, aucun débordement ne doit jamais apparaître, quel que soit le scénario

---

## Scénario 1 — Cartes Memory occupent l'espace disponible (#108)

**Objectif** : Vérifier que les cartes Memory ne sont plus plafonnées artificiellement en taille
sur grand écran.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger la question MEMORY petite grille (3 paires / 6 cartes), LANCER | Grille visible sur `/tv`, cartes nettement plus grandes qu'avant #108 (occupent ~80% de la largeur de la zone de jeu, pas un petit bloc centré avec vide autour) | | |
| 2 | Comparer visuellement l'espace occupé par la grille vs l'espace total de la zone de jeu | Pas de vide résiduel important autour de la grille (le plafond `280px` ne doit plus être visible à l'œil) | | |
| 3 | Retourner une carte contenant un mot long (ex. "ELEPHANT") | Le texte occupe la pleine largeur de la carte, reste lisible, **aucune troncature** (pas de `...`, pas de texte coupé) | | |
| 4 | Observer les bords de chaque carte | Aucun débordement de texte hors de la carte | | |
| 5 | Observer `/tv` dans son ensemble pendant toute la question | Aucun scroll introduit, `overflow: hidden` respecté (contrainte TV statique) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Grande grille Memory (10 paires / 20 cartes) (#108)

**Objectif** : Vérifier que l'agrandissement des cartes reste correct à la borne haute du nombre
de cartes, sans casser la disposition en colonnes.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger la question MEMORY à 10 paires (20 cartes), LANCER | Grille en 5 colonnes × 4 rangées sur `/tv`, cartes agrandies (~80% de la largeur de colonne disponible) | | |
| 2 | Retourner plusieurs cartes à texte court et à texte long | Tout le texte reste lisible, pleine largeur de carte, sans troncature ni débordement | | |
| 3 | Observer l'ensemble de la grille | Toujours entièrement visible à l'écran, aucun scroll, aucune carte coupée en bas/à droite | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Lisibilité du type de question en phase READY, fond clair (#114)

**Objectif** : Vérifier le contraste du texte du type de question (`.ready-game-type`) sur un
fond clair.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Attendre (ou forcer via la config fonds d'écran) que `/tv` affiche un fond **clair** | Fond clair actif visible sur `/tv` | | |
| 2 | Charger une question QCM (ou tout autre type), rester en phase READY (avant LANCER) | Texte du type de question ("QCM", couleur `#34d399` verte, ou type courant) affiché | | |
| 3 | Observer le texte | Texte nettement lisible malgré le fond clair — contour noir visible sur les lettres | | |
| 4 | Observer la zone derrière le texte | Box légèrement assombrie visible derrière le texte (contraste supplémentaire) | | |
| 5 | Répéter avec une question SPEEDY, ARDOISE, MEMOTION (READY) sur le même fond clair | Même niveau de lisibilité et de contour pour les 4 types (couleurs différentes : SPEEDY bleu, ARDOISE jaune, QCM vert, MEMOTION rose) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Lisibilité du type de question en phase READY, fond coloré vif (#114)

**Objectif** : Vérifier le contraste sur un fond saturé, cas le plus difficile (couleur du texte
proche de la couleur de fond).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Attendre (ou forcer) l'affichage d'un fond **coloré vif** sur `/tv` | Fond vif actif visible | | |
| 2 | Charger successivement une question de chaque type (SPEEDY, ARDOISE, QCM, MEMOTION), rester en phase READY pour chacune | Texte du type de question lisible pour les 4 types, y compris quand la couleur du texte est proche de la teinte du fond (ex. rose MEMOTION sur fond rose/orange) | | |
| 3 | Observer le contour des lettres | Contour noir (ou blanc selon adaptation au fond) visible et net, pas seulement un flou (glow) | | |
| 4 | Observer la box derrière le texte | Assombrissement visible, texte clairement détaché du décor | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Lisibilité du type de question en phase READY, fond sombre (#114)

**Objectif** : Vérifier que le contour s'adapte (ne reste pas noir sur fond sombre, ce qui serait
invisible).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Attendre (ou forcer) l'affichage d'un fond **sombre** sur `/tv` | Fond sombre actif visible | | |
| 2 | Charger successivement une question de chaque type (SPEEDY, ARDOISE, QCM, MEMOTION), rester en phase READY | Texte lisible sur fond sombre pour les 4 types | | |
| 3 | Observer le contour des lettres | Contour clair/blanc adapté au fond sombre (pas un contour noir invisible), lettres nettement découpées | | |
| 4 | Observer la box derrière le texte | Assombrissement toujours présent et cohérent (pas d'effet indésirable de double-assombrissement illisible sur fond déjà sombre) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Non-régression

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go build ./...` puis `go test ./... -race` | Build OK, tous les tests PASS | | |
| 2 | `npm test` (suite React complète) | Tous les tests PASS | | |
| 3 | Dérouler une question QCM, SPEEDY, ARDOISE, MEMOTION complète (READY → STARTED → REVEALED) | Aucune régression visuelle en dehors de la phase READY (zones Timer, Réponses, Média inchangées) | | |
| 4 | Dérouler une question MEMORY complète (grille normale, hors #108/#114) | Comportement de jeu (retournement, paires, crédit) inchangé — seule la taille visuelle des cartes change | | |
| 5 | Observer `/tv` sur toute la durée du test | Jamais de scroll, jamais de débordement hors zone visible, quel que soit le type de question ou le fond actif | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Cartes Memory occupent ~80% de la largeur disponible, texte pleine largeur sans troncature,
      sur petite grille (Scénario 1) et grande grille (Scénario 2)
- [ ] Texte du type de question lisible avec contour net et box assombrie sur fond clair
      (Scénario 3)
- [ ] Idem sur fond coloré vif, cas le plus exigeant (Scénario 4)
- [ ] Idem sur fond sombre, avec contour adapté (clair) et non un contour noir invisible
      (Scénario 5)
- [ ] Aucune régression sur les autres écrans/types de question, aucun scroll introduit sur `/tv`
      (Scénario 6)

---

## Notes QA

[Espace pour observations]
