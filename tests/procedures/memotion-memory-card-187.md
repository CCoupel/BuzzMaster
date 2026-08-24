# Procédure de Test — Carte MEMOTION de type MEMORY (#187)

**Version** : v7.1.0 (branche `milestone/v7.1.0`)
**Date** : 2026-08-24
**Testeur** : QA
**Issue** : #187 — MEMORY imbriquée dans une carte MEMOTION : grille rejouable en carte (deuxième
type réellement jouable après QCM en #185), barème `STARS_PRORATA` (points au prorata des paires
trouvées), tour serveur seule autorité avec dérogation d'ignore silencieux hors tour.
**Référence** : `contracts/question-types.md` §5.4/§6.3/§7.3/§9, `contracts/websocket-actions.md`
fiche `FLIP_MEMORY_CARD`, `_work/handoff/dev-backend-20260824-171341.md`,
`_work/handoff/dev-frontend-20260824-170743.md`

---

## ⚠️ Prérequis obligatoire — non-régression #160/#184/#185

Avant de valider cette procédure, s'assurer que `tests/procedures/anim-memotion-160.md`,
`tests/procedures/memotion-card-type-184.md` et `tests/procedures/memotion-qcm-card-185.md` ont déjà
été rejouées et validées sur ce même build (ou les rejouer si ce n'est pas le cas). Cette
procédure-ci ne les remplace pas, elle **ajoute** la recette spécifique à #187 : une carte MEMORY
réellement en jeu au sein d'une manche MEMOTION.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Un quiz contenant une question MEMOTION **mixte** avec au moins : 1 carte SPEEDY, 1 carte QCM
      (cf. `memotion-card-type-184.md` Scénario 4) et 1 carte MEMORY à 2+ paires (créée dans le
      Scénario 1 ci-dessous)
- [ ] Trois postes/onglets ouverts : `/anim` (tablette), `/tv` (affichage), `/admin` (régie)
- [ ] Au moins 2 équipes actives avec bumpers assignés
- [ ] Au moins un VPlayer configuré et assigné à une équipe (pour les Scénarios 5 et 6)

---

## Scénario 1 — Création d'une carte MEMOTION MEMORY dans l'éditeur

**Objectif** : Vérifier que l'éditeur `/admin` → Questions permet de créer une carte MEMORY imbriquée
dans une question MEMOTION, avec le sous-éditeur dédié (`MotionCardMemoryEditor`) et sans exposer de
champ de barème (contrat §6.3 : `STARS_PRORATA` est imposé, sans `VALUE`).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir une question MEMOTION existante (ou en créer une), ajouter une carte, choisir le type `MEMORY` | Sous-éditeur MEMORY affiché (recto/thème + liste de paires), pas le formulaire SPEEDY/QCM | | |
| 2 | Observer les réglages de points de la carte | **Aucun** sélecteur de mode de barème, **aucun** champ VALUE — les 3 réglages de points MEMORY habituels (`POINTS_PER_PAIR` etc.) apparaissent neutralisés/grisés si affichés | | |
| 3 | Ajouter 2 paires (4 cartes) avec un texte ou une image distincts par paire | Les 2 paires sont enregistrées, prévisualisables dans l'éditeur | | |
| 4 | Ajouter une 3e paire pour disposer d'une grille impaire en nombre de colonnes/lignes (ex: 3 paires = 6 cartes) | La carte se sauvegarde sans erreur | | |
| 5 | Enregistrer la question, recharger la page | La carte MEMORY et ses paires sont conservées à l'identique (round-trip) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Manche mixte SPEEDY + QCM + MEMORY, correspondance positionnelle `/anim` ↔ `/tv`

**Objectif** : Vérifier qu'une carte MEMORY se comporte comme une grille MEMORY classique (référence
`anim-memory-grid-159.md`) une fois **imbriquée** dans une carte MEMOTION, et que la disposition de
la grille est identique sur `/anim` et `/tv` (risque R9 hérité de #159).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger la question MEMOTION mixte, LANCER | Grille de cartes MEMOTION visible sur `/anim` ET `/tv` (thème + étoiles, cartes SPEEDY/QCM/MEMORY indiscernables avant sélection) | | |
| 2 | Sélectionner la carte MEMORY depuis `/anim` | Sous-phase `SELECTED` : carte au premier plan (thème + points), boutons DÉMARRER/ANNULER — identiques aux autres types de carte | | |
| 3 | Taper DÉMARRER | Sous-phase `QUESTION` : la grille MEMORY de la carte (toutes cartes face cachée) s'affiche sur `/anim` **et** `/tv`, même nombre de colonnes/lignes des deux côtés | | |
| 4 | Retourner une carte à une position donnée (ex: coin haut-gauche) depuis `/anim` | La MÊME carte (même contenu) se retourne à la MÊME position sur `/tv` | | |
| 5 | Répéter pour 2-3 positions différentes | Correspondance exacte à chaque fois, aucun décalage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Flip depuis `/anim` et depuis `/tv`, révélation complète

**Objectif** : Vérifier que le flip fonctionne indifféremment depuis `/anim` (animateur) et `/tv`
(aperçu régie/spectateur), et que la révélation en fin de carte affiche toutes les paires (contrat
§7.3 : `anim` et `tv` n'ont pas d'équipe, la vérification de tour ne s'applique jamais à eux).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur la carte MEMORY en `QUESTION`, retourner une carte depuis `/anim` | La carte se retourne face visible sur `/anim` **et** `/tv` | | |
| 2 | Retourner une carte depuis `/tv` (clic direct sur l'affichage, ou aperçu régie `/tv?admin=true`) | La carte se retourne face visible sur `/anim` **et** `/tv` — comportement identique au flip depuis `/anim` | | |
| 3 | Retourner une paire qui ne correspond pas (mismatch) | Les 2 cartes restent visibles brièvement puis se retournent automatiquement face cachée (délai `MEMORY_CONFIG.FLIP_DELAY` de la carte) | | |
| 4 | Trouver toutes les paires de la carte | Dès la dernière paire trouvée, la carte passe en révélation (pas d'action ANNULER/STOP requise pour une grille complète) — ou taper RÉVÉLER pour une révélation manuelle si des paires manquent | | |
| 5 | Observer l'état de révélation | **Toutes** les paires de la carte sont visibles face retournée, sur `/anim` **et** `/tv` — pas de paire orpheline restée face cachée | | |
| 6 | Désigner l'équipe gagnante (🏆 <Équipe>) | Carte `DONE`, points attribués (voir Scénario 6), retour à `GRID` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Non-régression : cartes SPEEDY et QCM de la même manche

**Objectif** : Vérifier qu'une manche mixte à 3 types (SPEEDY/QCM/MEMORY) n'introduit aucune
régression sur les cartes SPEEDY et QCM qu'elle contient.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dans la même manche mixte, jouer une carte SPEEDY jusqu'à `DONE` | Déroulé strictement identique à avant #187 | | |
| 2 | Jouer une carte QCM jusqu'à `DONE` | Déroulé strictement identique à #185 (aucune entrée joueur, indices progressifs) | | |
| 3 | Alterner les 3 types de carte plusieurs fois de suite | Aucune confusion de contenu entre cartes (jamais la grille MEMORY sur une carte SPEEDY/QCM, et inversement) | | |
| 4 | Terminer la manche (toutes les cartes `DONE`) | Arrêt automatique de la question, comme pour une manche 100% SPEEDY | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Dérogation sécurité : VJoueur hors tour, tap ignoré sans effet visible

**Objectif** : Vérifier la dérogation documentée (contrat §9.2, `websocket-actions.md` fiche
`FLIP_MEMORY_CARD`) : un tap VJoueur hors tour est **ignoré silencieusement** — aucune mutation,
aucun message d'erreur, aucun effet visible sur aucun écran. C'est un comportement voulu, **pas un
bug** — à ne pas signaler comme régression si le tap ne fait rien.

**Garde-fou déjà verrouillé par test automatisé** (`TestFlipMemoryCard_Turn_VPlayerOffActiveTeam_IgnoredSilently`,
`TestFlipMemoryCard_CardScoped_VPlayerOffActiveTeam_IgnoredSilently`,
`TestFlipMemoryCard_Turn_VPlayerOffActiveTeam_NoBroadcastToOtherClients`,
`TestFlipMemoryCard_CardScoped_VPlayerOffActiveTeam_NoBroadcastToOtherClients`) — ce scénario en est
la **confirmation visuelle/manuelle**.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Carte MEMORY active en sous-phase `QUESTION`, au moins 2 équipes participantes, l'équipe active affichée sur `/anim`/`/tv` | Une équipe est désignée comme équipe courante (tour) | | |
| 2 | Depuis un poste VJoueur assigné à une équipe **hors tour**, taper une carte face cachée | **Aucun effet observable** : la carte reste face cachée sur `/anim`, `/tv` et le poste VJoueur lui-même ; aucun message d'erreur affiché au joueur | | |
| 3 | Depuis un poste VJoueur assigné à l'équipe **active** (dans le tour), taper une carte face cachée | La carte se retourne normalement, visible sur tous les écrans | | |
| 4 | Répéter l'étape 2 plusieurs fois (2-3 taps hors tour successifs) | Toujours aucun effet, aucune accumulation d'erreur ni de latence perceptible sur les autres clients (pas de tempête de mise à jour) | | |
| 5 | Comparer avec un flip depuis `/anim` ou `/tv` pendant que l'équipe active est différente de l'équipe du VJoueur qui vient de taper hors tour | `/anim` et `/tv` retournent la carte normalement — la vérification de tour ne s'applique **jamais** à `anim`/`tv` (ils n'ont pas d'équipe) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Barème STARS_PRORATA : carte incomplète au timer = points proportionnels

**Objectif** : Vérifier le barème par défaut d'une carte MEMORY (contrat §6.2/§6.3) :
`points_étoiles × paires_trouvées / paires_totales`, avec arrondi serveur, et un cas nommé de
référence identique côté Go et JS : **5 points pour 8 paires, 4 trouvées → 2 points**
(`5 × 4/8 = 2`).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer (ou utiliser) une carte MEMORY à 8 paires, valeur en étoiles réglée pour valoir 5 points au nominal (mêmes réglages `MotionConfig.Points1Star` que la manche) | Carte créée, barème `STARS_PRORATA` par défaut (aucun champ VALUE à saisir, cf. Scénario 1) | | |
| 2 | Lancer la carte, trouver exactement 4 des 8 paires (moitié), puis taper RÉVÉLER avant d'avoir trouvé les 4 autres | Sous-phase `REVEAL` : les 4 paires trouvées restent visibles, les 4 non trouvées se révèlent aussi (révélation totale, cf. Scénario 3) | | |
| 3 | Désigner l'équipe gagnante (🏆 <Équipe>) | Carte `DONE`, l'équipe reçoit **2 points** (5 × 4/8) — pas 5 points pleins, pas 0 | | |
| 4 | Répéter avec une carte où **toutes** les paires sont trouvées avant révélation | L'équipe reçoit la **valeur nominale exacte** de la carte (ex: 5 points), sans perte d'arrondi | | |
| 5 | Répéter avec une carte où **aucune** paire n'est trouvée (révélation immédiate) | L'équipe reçoit **0 point** | | |
| 6 | ⚠️ Vérifier qu'aucun réglage de points saisi côté animateur/régie n'influence le résultat | Le nombre de points attribué dépend uniquement des paires effectivement trouvées côté serveur — aucun champ éditable ne permet de "forcer" un score différent pour une carte MEMORY | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Carte MEMORY créée et enregistrée correctement dans l'éditeur (sous-éditeur dédié, pas de champ de barème)
- [ ] Grille MEMORY d'une carte MEMOTION conduite de bout en bout, correspondance positionnelle exacte `/anim` ↔ `/tv`
- [ ] Flip fonctionnel depuis `/anim` ET depuis `/tv`, révélation complète en fin de carte
- [ ] Aucune régression sur les cartes SPEEDY/QCM d'une même manche mixte
- [ ] Dérogation sécurité confirmée : tap VJoueur hors tour ignoré sans effet visible, aucune tempête de mise à jour
- [ ] Barème STARS_PRORATA conforme : points proportionnels aux paires trouvées, cas "5 points / 8 paires → 2" vérifié

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go build ./... && go test ./... -race` | Build OK, tous les tests PASS, y compris les suites `*_187_test.go` (`engine_memory_card_187_test.go`, `flip_memory_card_turn_187_test.go`, `flip_memory_card_broadcast_187_test.go`) | | |
| 2 | `go test ./internal/game/... -run 'MemoryCard\|StarsProrata' -v` | Cas nommé `5 points / 8 pairs` PASS côté moteur | | |
| 3 | `go test ./cmd/server/... -run 'FlipMemoryCard' -race -v` | Dérogation de tour (ignore silencieux + zéro broadcast) et portée de carte (refus explicite) PASS | | |
| 4 | `cd server-go/web && npx vitest run` | Tous les tests PASS, y compris `motionGrid.test.js` (describe `computeStarsProrataPoints`, cas `STARS_PRORATA — 5 points / 8 pairs`), `PlayerDisplay.memotion.test.jsx` (describe "carte MEMOTION de type MEMORY (#187)"), `QuestionsPage.motionMemory.test.jsx` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
