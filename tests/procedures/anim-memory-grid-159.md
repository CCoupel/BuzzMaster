# Procédure de Test — Grille MEMORY sur l'interface animateur (#159)

**Version** : v6.2.x (branche `feature/anim-question-display`)
**Date** : 2026-08-16
**Testeur** : QA
**Issue** : #159 — grille MEMORY tactile sur `/anim`, correspondance positionnelle avec `/tv`
**Référence** : Plan `_work/reports/plan-20260816-224500.md` §7, `docs/GAME_STATE_MACHINE.md` (MEMOTION/MEMORY)

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Un quiz contenant au moins une question MEMORY à 3 paires (6 cartes) et une question MEMORY à
      10 paires (20 cartes)
- [ ] Deux navigateurs/onglets ouverts : `/anim` (tablette) et `/tv` (affichage), plus `/admin` sur un
      troisième poste
- [ ] Au moins 2 équipes actives avec bumpers assignés, pour tester le mode "chacun son tour"
- [ ] Redimensionnement de fenêtre possible pour simuler 1280×800 et 1024×768 (ou deux tablettes
      physiques de tailles différentes)

---

## Scénario 1 — Correspondance positionnelle `/anim` ↔ `/tv`

**Objectif** : Vérifier que la grille MEMORY de `/anim` désigne EXACTEMENT les mêmes cartes, aux mêmes
positions, que `/tv` pour la même question — c'est le risque central du plan (R9).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger une question MEMORY à 3 paires (6 cartes), lancer (STARTED) | Grille visible sur `/anim` ET `/tv`, 2 colonnes × 3 rangées sur les deux | | |
| 2 | Ouvrir `/anim` et `/tv` **côte à côte** à l'écran | Les deux grilles ont le même nombre de colonnes | | |
| 3 | Retourner une carte à la position (rangée 1, colonne 1) depuis `/anim` | La MÊME carte (même contenu) se retourne à la position (rangée 1, colonne 1) sur `/tv` | | |
| 4 | Répéter pour au moins 3 positions différentes (coins + centre) | Correspondance exacte à chaque fois, aucun décalage | | |
| 5 | Recharger `/anim` (F5) sans changer de question | La grille se reconstruit **à l'identique** (même ordre) — pas de re-mélange au rechargement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Partie SOLO complète

**Objectif** : Vérifier le déroulé complet d'une manche MEMORY sans équipes (mode solo/partie globale).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger la question MEMORY 3 paires, aucune équipe/tour configuré, LANCER | Grille active sur `/anim`, bandeau **sans** "au tour de" | | |
| 2 | Retourner deux cartes formant une paire | Les deux cartes restent visibles ("trouvées"), compteur "paires" passe à 1/3 | | |
| 3 | Retourner deux cartes NE formant PAS une paire | Bref délai d'affichage puis retour automatique face cachée, compteur "erreurs" incrémenté de 1 | | |
| 4 | Terminer les 3 paires | Compteur "paires" affiche "complète", plus aucune carte cliquable | | |
| 5 | Arrêter la question (STOP) | Grille passe à l'état inerte (cartes non cliquables, grisées) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Mode "chacun son tour" (multi-équipes)

**Objectif** : Vérifier la bascule de tour, l'attribution par équipe et l'identification de l'équipe
active.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer 2 équipes participantes, LANCER une question MEMORY | Bandeau affiche "au tour de <Équipe 1>" | | |
| 2 | Observer la colonne équipes | La carte de l'équipe active a un contour visuel distinct (active) | | |
| 3 | Équipe 1 trouve une paire | Reste au tour de l'Équipe 1 (retour, rejoue), compteur "paires" de l'Équipe 1 s'incrémente dans sa carte équipe | | |
| 4 | Équipe 1 échoue une paire | Bascule automatique : bandeau affiche "au tour de <Équipe 2>", contour actif se déplace sur la carte de l'Équipe 2 | | |
| 5 | Continuer jusqu'à ce que toutes les paires soient trouvées | Bandeau affiche "complète", plus de bascule de tour | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Équipe non participante

**Objectif** : Vérifier l'affichage "en retrait" d'une équipe présente au roster mais exclue de la
question MEMORY courante (`MEMORY_PARTICIPATING_TEAMS`).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer une question MEMORY avec une liste de participation restreinte (ex. 2 équipes sur 3 depuis `/admin`) | | | |
| 2 | Observer la colonne équipes sur `/anim` | L'équipe exclue reste visible (score conservé) mais visuellement en retrait, libellé "ne participe pas" | | |
| 3 | Observer le bandeau/tour | L'équipe exclue n'est jamais désignée comme équipe active | | |
| 4 | Zone de crédit de l'équipe exclue en fin de question | Seul "0 pt" proposé, jamais de motif "pas de buzz"/"pas de réponse" à côté | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Grille inerte hors phase STARTED

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question MEMORY chargée, phase READY (avant LANCER) | Emplacement réservé ou grille visible mais non cliquable | | |
| 2 | LANCER puis PAUSE | Grille visible, cartes grisées, aucun clic possible | | |
| 3 | CONTINUER | Grille redevient cliquable, état (cartes déjà trouvées/retournées) inchangé | | |
| 4 | STOP puis RÉPONSE (REVEALED) | Grille reste inerte, dernier état visible (pas de re-mélange) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Crédit MEMORY comparé à la régie

**Objectif** : Vérifier que le montant crédité sur `/anim` est identique à celui que `/admin`
afficherait pour les mêmes compteurs.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Terminer une question MEMORY avec un nombre de paires/erreurs connu (ex. 3 paires, 1 erreur) | Question STOPPED | | |
| 2 | Relever le montant "+N pts" proposé sur `/anim` pour l'équipe | Noter la valeur | | |
| 3 | Relever le montant que `/admin` afficherait/créditerait pour les mêmes compteurs | Valeurs **identiques** entre `/anim` et `/admin` | | |
| 4 | Créditer depuis `/anim` | Score de l'équipe incrémenté du montant affiché, ligne verrouillée (même comportement que #170) | | |
| 5 | Observer `/admin` après ce crédit | Verrouillage synchronisé, montant identique visible côté régie | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Aperçu TV depuis `/admin` toujours fonctionnel

**Objectif** : Non-régression — l'aperçu TV intégré à `/admin` (iframe, geste de clic régie) reste
utilisable en parallèle de `/anim`, comme avant #159.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/admin` avec une question MEMORY en cours, retourner une carte via l'aperçu TV intégré | Fonctionne comme avant #159, carte retournée visible sur `/tv` | | |
| 2 | Observer `/anim` simultanément | La même carte apparaît retournée sur `/anim` (état serveur partagé, pas de logique locale dupliquée) | | |
| 3 | Retourner une carte depuis `/anim` | Visible en retour sur l'aperçu `/admin` et sur `/tv` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Grande grille (10 paires / 20 cartes)

**Objectif** : Vérifier la disposition à la borne haute (5 colonnes) et la lisibilité sur petit écran.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger la question MEMORY à 10 paires, LANCER | Grille en 5 colonnes × 4 rangées sur `/anim` ET `/tv` | | |
| 2 | Redimensionner la fenêtre `/anim` à 1280×800 | Grille entièrement visible, cartes lisibles, aucun scroll horizontal | | |
| 3 | Redimensionner à 1024×768 | Grille toujours entièrement visible (le nombre de colonnes NE CHANGE PAS, seule la taille des cartes s'adapte, cf. #171) — si la hauteur manque, c'est le bloc central de conduite qui défile, pas la grille elle-même | | |
| 4 | Vérifier la correspondance positionnelle (comme Scénario 1) sur cette grille à 20 cartes | Toujours exacte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Non-régression

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go build ./...` puis `go test ./... -race` | Build OK, tous les tests PASS, y compris `inbound_allowlist_test.go`, `inbound_allowlist_anim_test.go`, `flip_memory_card_anim_test.go` | | |
| 2 | `npm test` (suite React complète) | Tous les tests PASS, y compris `memoryGrid.test.js`, `AnimMemoryGrid.test.jsx`, `AnimConductPanel.test.jsx`, `AnimTeamCard.test.jsx`, `AnimPage.test.jsx` | | |
| 3 | Manche MEMORY complète sur `/tv` / vue joueur, SANS passer par `/anim` | Comportement strictement identique à avant #159 (extraction pure vers `utils/memoryGrid.js`, aucune régression possible côté joueur) | | |
| 4 | Manche QCM, SPEEDY, ARDOISE sur `/anim` (hors MEMORY) | Aucune régression — zone conduite, crédit, colonne équipes inchangés pour ces types | | |
| 5 | Vérifier qu'aucune équipe non-MEMORY n'affiche jamais de contour actif/en retrait | Classes `active`/`dimmed` de `AnimTeamCard` absentes hors question MEMORY | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] `/anim` et `/tv` désignent toujours la même carte à la même position, y compris après rechargement (Scénario 1)
- [ ] Partie SOLO complète : détection de paire, erreurs, complétion (Scénario 2)
- [ ] Bascule de tour multi-équipes correcte, équipe active identifiable (Scénario 3)
- [ ] Équipe non participante affichée en retrait, jamais de motif "pas de buzz/réponse" (Scénario 4)
- [ ] Grille inerte en dehors de STARTED (Scénario 5)
- [ ] Montant crédité identique entre `/anim` et `/admin`, verrouillage synchronisé (Scénario 6)
- [ ] Aperçu TV intégré à `/admin` toujours fonctionnel en parallèle de `/anim` (Scénario 7)
- [ ] Grille à 10 paires lisible et correcte à 1280×800 et 1024×768 (Scénario 8)
- [ ] Aucune régression `/tv`/vue joueur ni sur les autres types de question sur `/anim` (Scénario 9)

---

## Notes QA

[Espace pour observations]
