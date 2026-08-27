# Procédure de Test — Carte MEMOTION de type MEMORY (#187)

**Version** : v7.1.0 (branche `milestone/v7.1.0`)
**Date** : 2026-08-24, complétée 2026-08-25 (Scénarios 7-8, cycle 4 — asymétrie assumée des deux
sorties de fin de carte), complétée 2026-08-27 (Scénario 9, cycle 7 — TV public non-interactif,
VJoueur fonctionnel)
**Testeur** : QA
**Issue** : #187 — MEMORY imbriquée dans une carte MEMOTION : grille rejouable en carte (deuxième
type réellement jouable après QCM en #185), barème `STARS_PRORATA` (points au prorata des paires
trouvées), tour serveur seule autorité avec dérogation d'ignore silencieux hors tour.
**Référence** : `contracts/question-types.md` §5.4/§6.3/§7.3/§9, `contracts/websocket-actions.md`
fiche `FLIP_MEMORY_CARD`, `_work/handoff/dev-backend-20260824-171341.md`,
`_work/handoff/dev-frontend-20260824-170743.md`, `_work/handoff/dev-backend-20260825-213835.md`,
`_work/handoff/dev-frontend-20260825-213202.md`, plan
`_work/reports/plan-memotion-v710-memory-reveal-v2-20260825-212446.md` (cycle 4 — comportement final
de fin de carte MEMORY, validé utilisateur), `_work/handoff/dev-frontend-20260827-183745.md`
(cycle 7 — TV public non-interactif, VJoueur fonctionnel)

> **Voir aussi** : `tests/procedures/memotion-pause-resume-195.md` (cycle 6, #195) — bug PAUSE/
> CONTINUER pendant un chrono MEMOTION actif, découvert en QUALIF sur cette carte MEMORY mais
> **transverse** (tout type de carte + chrono MEMORIZE), traité en procédure séparée pour ne pas en
> réduire la portée réelle.

---

## ⚠️ Historique du Scénario 7 — trois cycles avant le comportement final

Le comportement de fin de carte MEMORY sur expiration du chrono a changé **deux fois** après la
première rédaction de cette procédure, à ne pas confondre en relisant d'anciens rapports :

1. **Cycle 2** (bug initial) : une carte MEMORY restait indéfiniment retournable après expiration de
   son propre chrono — détecté en validation manuelle QUALIF (SHA `b12915c1`).
2. **Cycle 3** (corrigé mais imparfait) : révélation automatique à l'expiration, comme une grille
   complète — corrigeait le bug mais harmonisait à tort les deux sorties. Sa diffusion dédiée ne
   suffisait pas à mettre à jour la grille côté `/anim` (défaut découvert en revue).
3. **Cycle 4** (final, validé utilisateur) : l'auto-révélation à l'expiration est **retirée**. Les
   deux sorties sont **délibérément asymétriques** — voir Scénario 7 ci-dessous. **Ne pas
   « harmoniser »** ce comportement en revue future : c'est un choix utilisateur explicite, pas un
   oubli (même précaution que la dérogation §9.2 sur le tour).

Si cette procédure est rejouée sur un build **antérieur** au cycle 4 (SHA `7682918b`), le Scénario 7
est censé **échouer** sur le cas (b) — ce n'est pas une régression de la procédure, c'est la
reproduction d'un comportement déjà remplacé.

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
- [ ] Au moins un VPlayer configuré et assigné à une équipe (pour les Scénarios 5, 6 et 7)

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

## Scénario 3 — Flip depuis `/anim` et depuis l'aperçu admin, révélation complète

**Objectif** : Vérifier que le flip fonctionne indifféremment depuis `/anim` (animateur) et l'aperçu
régie en iframe (`/tv?admin=true`), et que la révélation en fin de carte affiche toutes les paires.

> ⚠️ **Mise à jour cycle 7 (2026-08-27, SHA `8af17927`)** : ce scénario testait auparavant aussi le
> flip depuis **l'écran TV public** (`/tv` sans `?admin=true`) et le décrivait comme fonctionnel —
> **ce n'est plus le comportement voulu**. Décision utilisateur explicite en vérification finale
> QUALIF : l'écran TV public ne doit **jamais** être interactif. Ce scénario-ci ne teste donc plus
> que `/anim` et l'aperçu admin ; le **Scénario 9** ci-dessous couvre spécifiquement la
> non-interactivité de l'écran TV public (et l'a corrigée s'il ne l'était pas).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur la carte MEMORY en `QUESTION`, retourner une carte depuis `/anim` | La carte se retourne face visible sur `/anim` **et** `/tv` | | |
| 2 | Retourner une carte depuis l'aperçu régie en iframe (`/tv?admin=true`) | La carte se retourne face visible sur `/anim` **et** `/tv` — comportement identique au flip depuis `/anim` | | |
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

> ⚠️ **Historique — étape 3 affectée par une régression jusqu'au cycle 7** : jusqu'au correctif
> `_work/handoff/dev-frontend-20260827-183745.md` (SHA `8af17927`), `flipMemoryCard` (client)
> n'envoyait jamais l'identité du bumper émetteur (`ID`) — le serveur ne pouvait donc **jamais**
> résoudre l'équipe d'un VJoueur, et l'étape 3 échouait silencieusement même pour l'équipe active
> (dérogation §9.2 masquant le bug : « ignore hors tour » ne se distinguait pas de « identité non
> résolue »). Voir **Scénario 9** pour la confirmation dédiée de ce correctif.

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

## Scénario 7 — Les deux sorties asymétriques de fin de carte MEMORY (cycle 4, 2026-08-25)

**Objectif** : Vérifier le comportement **final, validé utilisateur** des deux façons dont une carte
MEMORY se termine — **délibérément différentes** l'une de l'autre (contrat à venir
§6.3/§7.3/§9, plan `plan-memotion-v710-memory-reveal-v2-20260825-212446.md` §1) :

| Sortie | Comportement attendu | Geste animateur |
|---|---|---|
| **(a) Grille complétée avant expiration** | Révélation **immédiate**, compteur remis à **0**, sans action | **Aucun** |
| **(b) Chrono expiré, grille incomplète** | Carte **figée** en `QUESTION` (plus aucun flip possible) | **RÉVÉLER (`MEMOTION_REVEAL`) requis** |

⚠️ **Ne pas signaler comme incohérence** que (a) soit automatique et (b) manuel — c'est le choix
utilisateur explicite du cycle 4, documenté ci-dessus (§ Historique). QA doit valider les **deux**
comportements tels quels, pas les uniformiser.

**Garde-fou déjà verrouillé par test automatisé**
(`internal/game/engine_memory_card_timer_expiry_187_test.go` :
`TestProcessMotionCardTick_MemoryCard_ExpiryLeavesCardInQuestion`,
`TestFlipMotionMemoryCard_IgnoredAfterTimerExpiry`,
`TestRevealMotionCard_AfterTimerExpiry_RevealsIncompleteGrid`,
`TestMemoryCard_CompleteGridBeforeExpiry_RevealsWithoutGesture`,
`TestProcessMotionCardTick_QCMCard_StaysInQuestionOnExpiry`,
`TestProcessMotionCardTick_SpeedyCard_StaysInQuestionOnExpiry` ; côté JS,
`AnimConductPanel.test.jsx` describe "L2, gain STARS_PRORATA affiché sur le bouton de crédit (#187
cycle 4, F1)") — ce scénario en est la **confirmation visuelle/manuelle**.

### (a) Grille complétée avant expiration du chrono

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sélectionner une carte MEMORY avec un chrono de carte assez long (ex: 30s) pour trouver toutes les paires avant expiration, DÉMARRER | Sous-phase `QUESTION`, chrono en décompte sur `/anim` et `/tv` | | |
| 2 | Trouver **toutes** les paires de la grille avant que le chrono n'atteigne 0 | Dès la dernière paire trouvée : passage **immédiat** en révélation, **sans appuyer sur RÉVÉLER** | | |
| 3 | Observer le chrono au moment de la révélation | Le **compteur est remis à 0** immédiatement (pas de décompte résiduel qui continue en arrière-plan) | | |
| 4 | Observer le bouton d'attribution de points sur `/anim` | Montant affiché = **valeur nominale pleine** de la carte (paires trouvées = paires totales ⇒ le prorata rend exactement le montant plein, cf. Scénario 6) | | |
| 5 | Désigner l'équipe gagnante | Carte `DONE`, équipe créditée du montant plein affiché | | |

**Verdict (a)** : [ ] PASS  [ ] FAIL

### (b) Chrono expiré, grille incomplète

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sélectionner une carte MEMORY à 4+ paires avec un chrono court (ex: 10s), DÉMARRER | Sous-phase `QUESTION`, chrono en décompte | | |
| 2 | Trouver 1 ou 2 paires sur les 4+ (grille volontairement **incomplète**), sans taper RÉVÉLER | Les paires trouvées restent visibles/matched, le reste de la grille face cachée | | |
| 3 | Laisser le chrono descendre jusqu'à 0 sans action de l'animateur | Le chrono affiche 0, mais **aucune révélation automatique** — la grille reste dans le même état (paires trouvées visibles, reste caché), la carte reste en `QUESTION` | | |
| 4 | Tenter de retourner une carte encore face cachée depuis `/anim` | **Aucun effet** : la tentative de flip est ignorée, aucune carte supplémentaire ne change d'état, aucune erreur affichée | | |
| 5 | Même tentative depuis un poste VJoueur (si applicable) | **Aucun effet**, identique à l'étape 4 | | |
| 6 | Sur `/anim`, observer le bouton RÉVÉLER | Il est actif/allumé (chrono à zéro) — c'est le geste attendu, pas une option secondaire | | |
| 7 | Taper RÉVÉLER | La grille se révèle **maintenant** : les paires non trouvées apparaissent face visible, aux côtés des paires déjà trouvées, sur `/anim` **et** `/tv` | | |
| 8 | Observer le bouton d'attribution de points sur `/anim` **après** ce RÉVÉLER | Montant affiché = **montant au PRORATA** des paires réellement trouvées (ex: 1 ou 2 paires trouvées sur 4+ → montant partiel, pas la valeur nominale pleine de la carte) — cf. formule Scénario 6 | | |
| 9 | Désigner l'équipe gagnante | Carte `DONE`, équipe créditée du montant **prorata** affiché à l'étape 8, pas du montant plein | | |
| 10 | Non-régression — répéter le principe (chrono à 0, grille/réponse incomplète) sur une carte **QCM** puis une carte **SPEEDY** de la même manche | Comportement **inchangé** : QCM/SPEEDY restent en `QUESTION` à l'expiration, l'animateur agit manuellement (RÉVÉLER/SANS VAINQUEUR) — ce comportement était déjà celui d'avant #187, désormais **règle générale explicite** pour tous les types de carte | | |

**Verdict (b)** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Carte MEMORY sur question sans chrono configuré : reste jouable indéfiniment (piège B3)

**Objectif** : Vérifier le piège identifié par le planner (cycle 4, §3.1 B3) : une question MEMOTION
**sans chrono configuré** (`TIME=0`, aucune limite de temps) ne démarre jamais de minuteur de carte —
`CURRENT_TIME` y reste à 0 en permanence, la **même valeur** qu'afficherait une carte dont le chrono
vient d'expirer. Une garde naïve basée uniquement sur `CURRENT_TIME==0` rendrait une telle carte
**injouable dès le premier instant, silencieusement**. Ce scénario vérifie que ce n'est pas le cas :
l'absence de chrono doit être distinguée de son expiration.

**Garde-fou déjà verrouillé par test automatisé**
(`TestFlipMotionMemoryCard_NoConfiguredTimer_StaysPlayableIndefinitely`,
`internal/game/engine_memory_card_timer_expiry_187_test.go`) — ce scénario en est la **confirmation
visuelle/manuelle**.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer (ou utiliser) une question MEMOTION **sans limite de temps** (`TIME` à 0/vide dans l'éditeur — pas de chrono de manche), contenant une carte MEMORY à 4+ paires | Aucun chrono affiché/décompté sur `/anim` ni `/tv` pour cette question | | |
| 2 | Lancer la carte MEMORY | Sous-phase `QUESTION`, grille face cachée, **aucun chrono visible ou à 0 fixe** — pas de décompte | | |
| 3 | Retourner une première paire (match ou mismatch) | Comportement normal (flip, match/mismatch, flip-back si mismatch) | | |
| 4 | Attendre significativement (ex: 30-60s) sans agir, puis retourner une nouvelle carte | La carte se retourne **normalement** — aucune raison pour laquelle l'attente aurait "fermé" la manche | | |
| 5 | Trouver progressivement toutes les paires, avec des pauses irrégulières entre les flips | Chaque flip reste accepté jusqu'à la dernière paire — jamais un flip refusé silencieusement en cours de route | | |
| 6 | Trouver la dernière paire | Grille complète → révélation immédiate (sortie (a) du Scénario 7), comme toute carte MEMORY à grille complète | | |
| 7 | Désigner l'équipe gagnante | Carte `DONE`, montant plein attribué (grille complète) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — TV public non-interactif, VJoueur fonctionnel, aperçu admin inchangé (cycle 7, 2026-08-27)

**Objectif** : Vérifier les deux correctifs du cycle 7 (`_work/handoff/dev-frontend-20260827-183745.md`,
SHA `8af17927`), rapportés par l'utilisateur en vérification finale QUALIF v7.1.0.6 sur une carte
MEMORY :

| Bug | Nature | Correctif |
|---|---|---|
| **(a) Écran TV public cliquable** | Préexistant au MEMORY classique, **révélé** (pas introduit) par #187 | `canClick` exige désormais `isVPlayer` ou `isAdminPreview` — la TV publique seule ne suffit plus |
| **(b) VJoueur ne peut pas flipper une carte** | **Vraie régression #187** (cycle 4) — latente depuis la création de `flipMemoryCard`, invisible tant que le serveur ne vérifiait aucun tour | `flipMemoryCard` envoie désormais l'`ID` du bumper émetteur (client), résolution serveur 3 passes désormais satisfaite |

⚠️ Ce scénario **change le comportement attendu** par rapport à ce que d'anciens rapports QA (ou une
lecture rapide du contrat `websocket-actions.md` actuel, pas encore mis à jour) pourraient laisser
penser : **le clic d'un spectateur sur l'écran TV public n'est plus une capacité légitime.** Ne pas
« corriger » ce scénario en sens inverse.

**Garde-fou déjà verrouillé par test automatisé** (`useWebSocket.flipMemoryCard.test.js` (5 tests),
`PlayerDisplay.memotion.test.jsx` (3 tests : TV public non cliquable + `flipMemoryCard` jamais
appelé, VJoueur cliquable + `ID` envoyé, aperçu admin cliquable sans `ID`), `VPlayerPage.test.jsx`
(2 tests : `playerId` transmis = clé du bumper, survit à une reconnexion)) — ce scénario en est la
**confirmation visuelle/manuelle**.

### (a) Écran TV public : aucun effet au clic

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/tv` (sans `?admin=true` — l'écran public, celui projeté en salle) pendant une carte MEMORY classique (question hôte) en `QUESTION` | Grille face cachée affichée normalement | | |
| 2 | Cliquer/taper directement sur une carte face cachée depuis cet écran | **Aucun effet** : la carte reste face cachée, aucun flip envoyé, aucun changement visible sur `/anim` ni sur aucun autre écran | | |
| 3 | Répéter sur une carte MEMORY **imbriquée dans une carte MEMOTION** en `QUESTION` | **Aucun effet**, identique à l'étape 2 (les deux grilles — question-scopée et carte MEMOTION — sont concernées par le correctif) | | |
| 4 | Répéter plusieurs clics successifs sur des cartes différentes | Toujours aucun effet, aucune erreur affichée, aucun état qui se dégrade | | |

**Verdict (a)** : [ ] PASS  [ ] FAIL

### (b) VJoueur de l'équipe active peut désormais flipper une carte MEMORY en carte MEMOTION

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Carte MEMORY imbriquée dans une carte MEMOTION, active en `QUESTION`, au moins 2 équipes participantes | Grille visible sur `/anim`/`/tv`, une équipe désignée comme équipe active | | |
| 2 | Depuis un poste VJoueur (téléphone) assigné à l'équipe **active**, taper une carte face cachée | La carte se retourne **normalement**, visible sur `/anim` **et** `/tv` — c'est le comportement qui était cassé avant ce correctif (cf. note du Scénario 5) | | |
| 3 | Faire trouver une paire complète en alternant les flips depuis ce même poste VJoueur | Match détecté normalement, comportement identique à un flip envoyé depuis `/anim` | | |
| 4 | Déconnecter puis reconnecter ce VJoueur (fermer/rouvrir l'onglet ou recharger la page) pendant la manche | Après reconnexion, le flip depuis ce VJoueur fonctionne toujours (l'identité du bumper survit à la reconnexion) | | |
| 5 | Depuis un VJoueur d'une équipe **hors tour**, taper une carte | **Aucun effet** — dérogation §9.2 toujours en vigueur, non affectée par ce correctif (cf. Scénario 5) | | |

**Verdict (b)** : [ ] PASS  [ ] FAIL

### (c) Aperçu admin en iframe : inchangé, toujours cliquable

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/tv?admin=true` (aperçu régie, typiquement en iframe dans `/admin`) pendant une carte MEMORY en `QUESTION` | Grille visible, identique à l'écran public en apparence | | |
| 2 | Cliquer sur une carte face cachée depuis cet aperçu | La carte se retourne **normalement** — comportement inchangé par ce cycle (seul l'écran public a perdu l'interactivité, pas l'aperçu admin) | | |

**Verdict (c)** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Carte MEMORY créée et enregistrée correctement dans l'éditeur (sous-éditeur dédié, pas de champ de barème)
- [ ] Grille MEMORY d'une carte MEMOTION conduite de bout en bout, correspondance positionnelle exacte `/anim` ↔ `/tv`
- [ ] Flip fonctionnel depuis `/anim` ET depuis l'aperçu admin (`/tv?admin=true`), révélation complète en fin de carte
- [ ] Aucune régression sur les cartes SPEEDY/QCM d'une même manche mixte
- [ ] Dérogation sécurité confirmée : tap VJoueur hors tour ignoré sans effet visible, aucune tempête de mise à jour
- [ ] Barème STARS_PRORATA conforme : points proportionnels aux paires trouvées, cas "5 points / 8 paires → 2" vérifié
- [ ] Sortie (a) grille complète avant expiration : révélation immédiate sans geste, compteur à 0, montant plein
- [ ] Sortie (b) chrono expiré grille incomplète : carte figée (flip ignoré), RÉVÉLER manuel requis, montant au prorata sur le bouton de crédit — sans régression QCM/SPEEDY
- [ ] Carte MEMORY sur question sans chrono configuré : reste jouable indéfiniment (piège B3)
- [ ] Écran TV public non-interactif (aucun effet au clic, question hôte ET carte MEMOTION) — cycle 7
- [ ] VJoueur de l'équipe active peut flipper une carte MEMORY en carte MEMOTION, y compris après reconnexion — cycle 7
- [ ] Aperçu admin en iframe (`/tv?admin=true`) reste cliquable — inchangé par le cycle 7

---

## Non-Régression (suite automatisée, à exécuter avant validation manuelle)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go build ./... && go test ./... -race` | Build OK, tous les tests PASS, y compris les suites `*_187_test.go` (`engine_memory_card_187_test.go`, `flip_memory_card_turn_187_test.go`, `flip_memory_card_broadcast_187_test.go`, `engine_memory_card_timer_expiry_187_test.go`) | | |
| 2 | `go test ./internal/game/... -run 'MemoryCard\|StarsProrata' -v` | Cas nommé `5 points / 8 pairs` PASS côté moteur | | |
| 3 | `go test ./cmd/server/... -run 'FlipMemoryCard' -race -v` | Dérogation de tour (ignore silencieux + zéro broadcast) et portée de carte (refus explicite) PASS | | |
| 4 | `go test ./internal/game/... -run 'MotionCardTick_MemoryCard\|MotionCardTick_QCMCard\|MotionCardTick_SpeedyCard\|IgnoredAfterTimerExpiry\|NoConfiguredTimer\|CompleteGridBeforeExpiry\|AfterTimerExpiry' -v` | Les 6 tests du cycle 4 (Scénarios 7-8) PASS : sortie (a) grille complète, sortie (b) chrono expiré figé + RÉVÉLER manuel, carte sans chrono jouable indéfiniment, non-régression QCM/SPEEDY | | |
| 5 | `cd server-go/web && npx vitest run` | Tous les tests PASS, y compris `motionGrid.test.js` (describe `computeStarsProrataPoints`, cas `STARS_PRORATA — 5 points / 8 pairs`), `PlayerDisplay.memotion.test.jsx` (describe "carte MEMOTION de type MEMORY (#187)" — TV public non cliquable + `flipMemoryCard` jamais appelé, VJoueur cliquable + `ID` envoyé, aperçu admin cliquable sans `ID`), `QuestionsPage.motionMemory.test.jsx`, `AnimConductPanel.test.jsx` (describe "L2, gain STARS_PRORATA affiché sur le bouton de crédit (#187 cycle 4, F1)"), `useWebSocket.flipMemoryCard.test.js` (5 tests, payload exact selon les arguments), `VPlayerPage.test.jsx` (`playerId` transmis, survit à une reconnexion) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations]
