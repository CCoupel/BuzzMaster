# Procédure de Test — Conduite MEMOTION sur l'interface animateur (#160)

**Version** : v6.2.x (branche `feature/anim-question-display`)
**Date** : 2026-08-17
**Testeur** : QA
**Issue** : #160 — cinq sous-phases MEMOTION (`MEMORIZE` → `GRID` → `SELECTED` → `QUESTION` → `REVEAL`)
conduisibles depuis `/anim`, sans recours à `/admin` ni à l'aperçu TV
**Référence** : Plan `_work/reports/plan-20260817-160500.md`, maquette `docs/mockups/anim-memotion-160.html`,
`contracts/websocket-actions.md` §"Sécurité — Allow-list entrante", `docs/GAME_STATE_MACHINE.md`

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Un quiz contenant au moins :
  - une question MEMOTION en **mode normal** (`MOTION_MEMORIZE_DURATION` = 0 ou absent), 6 cartes
  - une question MEMOTION en **mode Secret** (`MOTION_MEMORIZE_DURATION` > 0), 6 cartes
  - une question MEMOTION à 20 cartes (grande grille, borne haute de `motionGrid.js`)
- [ ] Trois postes/onglets ouverts : `/anim` (tablette), `/tv` (affichage), `/admin` (régie)
- [ ] Au moins 2 équipes actives avec bumpers assignés (mode "chacun son tour" ou "tant que je gagne")
- [ ] Build QUALIF incluant B1 (liste blanche `ClientTypeAnim` élargie aux 5 actions MEMOTION) — sans
      B1, `/anim` ne peut émettre AUCUNE action MEMOTION (rejetée silencieusement, log `WARN` serveur)

---

## Scénario 1 — Correspondance `/anim` ↔ `/tv` (scénario central, risque R1)

**Objectif** : Vérifier que la grille MEMOTION de `/anim` désigne EXACTEMENT les mêmes cartes, aux
mêmes positions, avec les mêmes coordonnées et les mêmes points que `/tv` pour la même question.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger la question MEMOTION 6 cartes (mode normal), LANCER | Grille visible sur `/anim` ET `/tv`, **même nombre de colonnes** sur les deux (3 colonnes pour 6 cartes) | | |
| 2 | Ouvrir `/anim` et `/tv` côte à côte | Les deux grilles affichent les cartes dans le **même ordre** (pas de mélange local sur `/anim`, contrairement à MEMORY) | | |
| 3 | Sélectionner la carte en position (rangée 1, colonne 1) depuis `/anim` | La MÊME carte (même thème) passe au premier plan sur `/anim` ET apparaît en cours de jeu sur `/tv` | | |
| 4 | Relever les points annoncés pour cette carte sur `/anim` (SELECTED) | Montant identique à celui que `/tv`/`/admin` afficherait pour la même difficulté (barème `MOTION_CONFIG` ou repli 1/3/5) | | |
| 5 | Répéter sur la question MEMOTION à 20 cartes | Grille en 5 colonnes sur `/anim` ET `/tv` (borne haute, PLAFOND à 5 colonnes — contrairement à MEMORY qui irait à 6) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Manche complète en mode normal, "chacun son tour", 2 équipes

**Objectif** : Vérifier l'enchaînement des cinq sous-phases pour une manche standard, entièrement
conduite depuis `/anim` (AC1).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger la question MEMOTION mode normal, LANCER | Sous-phase `GRID` immédiate (pas de `MEMORIZE` en mode normal), grille active, bandeau "au tour de <Équipe 1>" | | |
| 2 | Taper une carte libre (thème visible) | Sous-phase `SELECTED` : carte au premier plan (thème + points), boutons DÉMARRER/ANNULER | | |
| 3 | Taper DÉMARRER | Sous-phase `QUESTION` : face question affichée, chrono de la carte démarre, boutons STOP CHRONO/RÉVÉLER (éteint)/SANS VAINQUEUR | | |
| 4 | Attendre la fin du chrono (ou taper STOP CHRONO) | RÉVÉLER devient cliquable (vert) dès le chrono à 0 | | |
| 5 | Taper RÉVÉLER | Sous-phase `REVEAL` : réponse affichée, boutons 🏆 <Équipe 1> / PERSONNE | | |
| 6 | Taper le bouton équipe (🏆 <Équipe 1>) | Carte marquée `DONE`, couleur de l'équipe posée sur la grille ; score de l'équipe incrémenté du montant annoncé ; retour à `GRID` | | |
| 7 | Observer la zone équipes | Le tour est passé à l'Équipe 2 (mode "chacun son tour" après attribution) | | |
| 8 | Recommencer jusqu'à ce que toutes les cartes soient `DONE` | Voir Scénario 6 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Manche complète en mode Secret

**Objectif** : Vérifier la sous-phase `MEMORIZE` et l'affichage en coordonnées.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger la question MEMOTION mode Secret (`MOTION_MEMORIZE_DURATION` > 0), LANCER | Sous-phase `MEMORIZE` : grille visible avec thèmes + étoiles (éteinte, non cliquable), chrono de mémorisation affiché, **aucun bouton en L2** (bandeau d'attente uniquement) | | |
| 2 | Observer `/tv` en parallèle | Mêmes cartes/thèmes affichés côté joueurs pendant `MEMORIZE` | | |
| 3 | Attendre l'expiration du chrono de mémorisation | Bascule AUTOMATIQUE vers `GRID` (aucune action de l'animateur ne peut l'écourter — comportement connu, non corrigé par #160) | | |
| 4 | Observer la grille en `GRID` | Les cartes n'affichent plus le thème : **coordonnée seule** (A1, A2...), **sans étoiles** (la difficulté ne doit pas trahir la carte) | | |
| 5 | Sélectionner puis jouer une carte jusqu'à `REVEAL` (comme Scénario 2) | Déroulé identique, la carte gagnée porte la couleur de l'équipe sur la grille (toujours en coordonnées) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Annulation depuis `SELECTED`

**Objectif** : Vérifier que l'annulation rend la carte libre, sans conséquence (AC3, geste "optional").

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis `GRID`, sélectionner une carte (par erreur) | Sous-phase `SELECTED`, carte au premier plan | | |
| 2 | Taper ANNULER | Retour immédiat à `GRID`, la carte redevient `UNPLAYED` (cliquable, couleur neutre), **aucun point attribué**, **aucune rotation d'équipe** | | |
| 3 | Sélectionner la MÊME carte à nouveau | Fonctionne normalement, comme si l'annulation n'avait jamais eu lieu | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — "Sans vainqueur" depuis `QUESTION` puis depuis `REVEAL`

**Objectif** : Vérifier la clôture sans attribution et la rotation d'équipe conforme au mode de tour.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Amener une carte en `QUESTION`, taper SANS VAINQUEUR (sans avoir révélé) | Retour direct à `GRID`, carte `DONE` **sans gagnant** (grise, "–"), **aucun point** | | |
| 2 | Observer la zone équipes (mode "chacun son tour") | Le tour est passé à l'équipe suivante | | |
| 3 | Amener une AUTRE carte jusqu'à `REVEAL`, taper PERSONNE | Carte `DONE` sans gagnant, aucun point, retour à `GRID` | | |
| 4 | Observer la zone équipes en mode "tant que je gagne" | "Sans vainqueur"/"Personne" fait passer la main (contrairement à une victoire, qui la garde) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Fin de manche : toutes cartes `DONE` → arrêt automatique

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Jouer/annuler/attribuer toutes les cartes de la question (mix gagnées/sans vainqueur) | Dès la dernière carte close, la question s'arrête automatiquement (comportement moteur, pas un geste `/anim`) | | |
| 2 | Observer `/anim` | Retour à l'état "à suivre"/RÉPONSE selon la phase atteinte, grille finale visible (toutes les cartes colorées ou grises) | | |
| 3 | Observer `/tv` | Même état final affiché | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Non-régression `/admin` (conduite MEMOTION complète depuis la régie seule)

**Objectif** : Vérifier qu'`/admin` reste pleinement fonctionnelle sur MEMOTION, sans interférence des
nouveaux droits accordés à `/anim` (B1 est un élargissement, pas un retrait).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Fermer/ignorer `/anim`, conduire une manche MEMOTION complète depuis `/admin` uniquement (sélection carte via l'aperçu TV en iframe, flip/stop/reveal/done via les contrôles régie) | Déroulé strictement identique à avant #160, aucune régression | | |
| 2 | Vérifier que `MEMOTION_SET_TEAMS` (composition des équipes participantes) reste accessible UNIQUEMENT depuis `/admin` | `/anim` ne propose aucun moyen de configurer les équipes participantes MEMOTION (périmètre explicite, comme MEMORY en #159) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Non-régression `/tv` (affichage MEMOTION inchangé, aucun scroll)

**Objectif** : Vérifier que le refactor F0/F1 (extraction `motionGrid.js`, consommée par
`PlayerDisplay.jsx`) n'a AUCUN effet visible sur `/tv` (risque R2 du plan).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dérouler une manche MEMOTION complète (mode normal ET Secret) en observant `/tv` SEUL, sans jamais utiliser `/anim` | Affichage strictement identique à avant #160 : mêmes colonnes, mêmes coordonnées, mêmes points, mêmes couleurs de carte `DONE` | | |
| 2 | Question à 20 cartes sur `/tv` | Grille entièrement visible, **aucun scroll** (contrainte STATIQUE de `/tv`, CLAUDE.md) | | |
| 3 | Redimensionner la fenêtre `/tv` (simuler différentes résolutions) | Toujours aucun scroll, disposition adaptée (`overflow: hidden`, unités viewport) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Conduite à deux postes simultanés (`/admin` et `/anim`)

**Objectif** : Vérifier que le serveur reste l'autorité unique quand deux interfaces peuvent agir sur
la même manche MEMOTION en parallèle (risque de double-action, pas de double-crédit — AC7).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/admin` et `/anim` simultanément sur la même partie, sélectionner une carte depuis `/anim` | La sélection apparaît immédiatement sur `/admin` (aperçu TV) — état serveur unique, pas de logique locale dupliquée | | |
| 2 | Démarrer la carte (DÉMARRER) depuis `/admin`, observer `/anim` | `/anim` bascule en `QUESTION` en temps réel | | |
| 3 | Taper RÉVÉLER depuis `/anim` PENDANT qu'un animateur sur `/admin` tente aussi de révéler (quasi-simultané) | Un seul passage en `REVEAL` effectif côté moteur, aucune erreur, aucun état incohérent | | |
| 4 | Créditer l'équipe gagnante depuis `/anim` (bouton 🏆 équipe) | Score incrémenté UNE SEULE FOIS (vérifier le score final), visible identiquement sur `/admin` et `/tv` | | |
| 5 | **AC7 — vérification critique** : à AUCUN moment de la manche (quelle que soit la sous-phase), `/anim` ne doit afficher de bouton de crédit générique ("+N pts"/"0 pt") sur les cartes équipe — seul le geste MEMOTION_DONE (🏆 équipe / PERSONNE) attribue des points | | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] `/anim` et `/tv` désignent toujours la même carte, à la même position, avec les mêmes points (Scénario 1)
- [ ] Manche complète mode normal, "chacun son tour", conduite intégralement depuis `/anim` (Scénario 2)
- [ ] Manche complète mode Secret : `MEMORIZE` (auto), coordonnées sans étoiles en `GRID` (Scénario 3)
- [ ] Annulation depuis `SELECTED` sans conséquence (Scénario 4)
- [ ] "Sans vainqueur"/"Personne" clôt sans attribution et fait tourner l'équipe selon le mode (Scénario 5)
- [ ] Fin de manche automatique quand toutes les cartes sont `DONE` (Scénario 6)
- [ ] `/admin` reste pleinement fonctionnelle seule, `MEMOTION_SET_TEAMS` reste régie-only (Scénario 7)
- [ ] `/tv` strictement inchangée, aucun scroll même à 20 cartes (Scénario 8)
- [ ] Conduite à deux postes cohérente, **AUCUN bouton de crédit générique visible pendant une manche MEMOTION** (Scénario 9, AC7)

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go build ./... && go test ./... -race` | Build OK, tous les tests PASS, y compris `inbound_allowlist_test.go`, `inbound_allowlist_anim_test.go` (T8a/T8b), `memotion_anim_test.go` (T8c, NOUVEAU) | | |
| 2 | `cd server-go/web && npm test` (suite Vitest complète) | Tous les tests PASS, y compris `motionGrid.test.js`, `motionRules.test.js`, `AnimMotionGrid.test.jsx`, `AnimMotionCard.test.jsx`, `AnimMotionActions.test.jsx`, `AnimConductPanel.test.jsx`, `AnimPage.test.jsx` | | |
| 3 | **T10** — `PlayerDisplay.*.test.jsx` et `GamePage.*.test.jsx` passent **SANS AUCUNE MODIFICATION** de leur côté | Si un de ces tests devait changer pour repasser au vert, F1 (extraction `motionGrid.js`) n'était pas un refactor pur — à signaler comme régression, pas à "corriger" le test | | |
| 4 | Manche QCM/SPEEDY/ARDOISE/MEMORY sur `/anim` (hors MEMOTION) | Aucune régression : zone conduite (L2 reste vide/réservée hors MEMOTION), crédit, colonne équipes inchangés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
