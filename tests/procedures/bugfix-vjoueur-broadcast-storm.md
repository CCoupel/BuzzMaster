# Procédure de Test — Rafale de broadcasts PREPARE→READY (#127)

**Version** : à définir (branche `bugfix/vjoueur-broadcast-storm-ready`)
**Date** : 2026-08-02
**Branche** : bugfix/vjoueur-broadcast-storm-ready
**Testeur** : QA

---

## Contexte du Bug

À chaque `PONG` reçu pendant la phase PREPARE (« préparez-vous »), le serveur rediffusait l'état
complet du jeu à **tous** les clients, y compris les VJoueurs. Pour une partie à N participants, un
VJoueur recevait N+2 payloads complets (jusqu'à 34 Ko en QCM, 164 Ko en MEMOTION) en quelques
centaines de millisecondes — exactement au moment où ces mêmes VJoueurs échangent leur propre trafic
`PLAYER_CONNECT`/`PONG`. Cette rafale consommait la marge de ping/pong et provoquait des
déconnexions/reconnexions au moment précis du passage en READY.

**Correctif attendu** (`contracts/vplayer-payload-filter.md`) :
- Les VJoueurs sont retirés de la rafale de PONG — seuls Admin, TV et les buzzers physiques
  continuent de recevoir un `UPDATE` à chaque PONG (l'admin doit garder sa progression « prêt »
  équipe par équipe, en direct).
- Le VJoueur ne reçoit plus que **2** `UPDATE` sur toute la fenêtre PREPARE→READY (entrée en
  PREPARE, puis transition en READY), quel que soit le nombre de participants.
- Pendant ces deux messages seulement, la carte `bumpers` envoyée à un VJoueur identifié est réduite
  à son seul bumper (`teams` et `GAME` restent complets). Dès la sortie de PREPARE/READY, la carte
  complète est restaurée avant tout affichage qui en dépend.

**Objectif affiché du correctif : aucun changement visible, sur aucune interface.** Toute différence
d'affichage constatée pendant cette procédure est un défaut à signaler, jamais un effet attendu.

Maquette de référence : `_work/mockups/127-broadcast-matrix.md` (diagrammes de séquence AVANT/APRÈS,
matrice « qui reçoit quoi », points de contrôle V1→V9 repris ci-dessous).

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `bugfix/vjoueur-broadcast-storm-ready`)
- [ ] Au moins 10 appareils/onglets distincts pouvant ouvrir `/player` (ou un mélange
      d'onglets + buzzers physiques) — le scénario de charge cible 10 VJoueurs
- [ ] Un accès admin (`/game`) et un accès TV (`/tv`) ouverts en parallèle, visibles pendant tout le
      test
- [ ] Au moins une question de chaque type disponible : QCM, MEMORY (2+ équipes), MEMOTION
- [ ] Outils navigateur (onglet réseau / console) sur au moins un appareil VJoueur, pour observer le
      nombre de trames WebSocket reçues pendant la fenêtre PREPARE→READY (scénario 1)
- [ ] Accès aux logs serveur (pour confirmer l'absence d'erreur/`panic` pendant la rafale)

---

## Scénario 1 — Rafale PREPARE→READY, 10 VJoueurs (cœur du bug, CA1/CA2)

**Objectif** : Vérifier que la rafale a disparu côté VJoueur et que l'admin garde sa progression
« prêt » en direct.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Inscrire 10 VJoueurs, répartis sur 2 équipes minimum | Les 10 apparaissent côté admin | | |
| 2 | Ouvrir l'onglet réseau (WebSocket) sur UN des appareils VJoueur, avant de lancer READY | Prêt à observer les trames reçues | | |
| 3 | Depuis l'admin, sélectionner une question et cliquer READY (« préparez-vous ») | Les 10 appareils VJoueur affichent l'écran PREPARE, sans délai perceptible | | |
| 4 | Laisser les 10 appareils répondre automatiquement (PONG) jusqu'au passage en READY | Côté admin (`/game`) : les cartes d'équipe passent « prêt » **une par une, en direct** — pas toutes d'un coup à la fin | | |
| 5 | Sur l'appareil observé à l'étape 2, compter les trames WebSocket reçues de type `UPDATE` pendant toute la fenêtre (entrée PREPARE → passage READY) | **2 trames `UPDATE`** au total (pas plus, indépendamment du nombre de VJoueurs) | | |
| 6 | Observer les 9 autres appareils VJoueur pendant la rafale | Aucun clignotement, aucun gel, aucune reconnexion visible | | |
| 7 | Vérifier les logs serveur | Aucune erreur, aucun `panic`, aucun warning de canal saturé | | |

**Verdict** : [ ] PASS  [ ] FAIL

> Si l'étape 5 montre plus de 2 trames `UPDATE`, ne pas cocher PASS — c'est précisément la
> régression que ce correctif doit éliminer (voir la suite automatisée
> `TestVPlayerBroadcastIntegration_PrepareToRevealSequence`, qui mesure ce même compte de façon
> reproductible).

### Variante — 1 seul VJoueur

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Répéter le scénario 1 avec un seul VJoueur inscrit | Même résultat : exactement 2 `UPDATE` reçus, passage en READY immédiat après son unique PONG | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — V1/V2/V3 : écrans admin/TV/VJoueur inchangés

**Objectif** : Confirmer qu'aucun changement visible n'apparaît sur les trois écrans pendant
PREPARE→READY (objectif affiché du correctif).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer READY avec plusieurs VJoueurs, en observant simultanément `/game`, `/tv` et un appareil VJoueur | **V1** (admin) : cartes d'équipe « prêt » une par une, en direct, comme avant | | |
| 2 | Observer l'écran TV pendant PREPARE puis READY | **V2** : écran « préparez-vous » puis READY, sans clignotement ni retard perceptible | | |
| 3 | Observer l'écran VJoueur pendant PREPARE puis READY | **V3** : bouton et libellés identiques ; aucune régression de nom d'équipe, de couleur, de score | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — V4 : MEMORY multi-équipes, barre d'équipes après READY

**Objectif** : Vérifier que la réduction de la carte `bumpers` pendant PREPARE/READY ne fait
**jamais** apparaître un chip d'équipe gris une fois la partie lancée (risque R2 du plan — `teams`
n'est volontairement pas réduit, seul `bumpers` l'est).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Préparer une question MEMORY avec 3 équipes ou plus, chacune avec au moins un VJoueur | | | |
| 2 | Lancer READY, laisser passer la rafale de PONG jusqu'à READY | | | |
| 3 | Lancer START (COUNTDOWN puis STARTED) | | | |
| 4 | Sur l'écran d'un VJoueur, observer la barre d'équipes participantes | **Toutes** les équipes apparaissent, chacune à sa couleur — **jamais** de chip gris/vide | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — V5 : MEMOTION, barre d'équipes après READY

**Objectif** : Même vérification que le scénario 3, pour une question MEMOTION.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Préparer une question MEMOTION avec 2 équipes ou plus, VJoueurs inclus | | | |
| 2 | Lancer READY, laisser passer la rafale jusqu'à READY, puis START | | | |
| 3 | Sur l'écran d'un VJoueur, observer la barre des équipes participantes | Toutes les équipes correctement colorées, comme avant le correctif | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — V6 : QCM, badges d'équipe sur les réponses (STOPPED/REVEALED)

**Objectif** : Vérifier l'invariant de restauration (CA6) au moment où il compte réellement : les
badges d'équipe sur chaque réponse QCM doivent afficher **toutes** les équipes ayant buzzé, jamais
une carte tronquée héritée de la phase PREPARE/READY.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Préparer une question QCM, plusieurs VJoueurs répartis sur plusieurs équipes | | | |
| 2 | Lancer READY → laisser passer la rafale → READY → START | | | |
| 3 | Faire buzzer plusieurs VJoueurs sur des réponses différentes, puis STOP | | | |
| 4 | Observer l'écran VJoueur en phase STOPPED | Badges d'équipe visibles sur chaque réponse buzzée, une entrée par équipe ayant répondu | | |
| 5 | Cliquer REVEAL | Badges toujours corrects, ordonnés par temps de réponse, aucune équipe manquante | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — V7 : session sans `vplayer_id` (repli carte complète)

**Objectif** : Vérifier la condition 3 du contrat (`contracts/vplayer-payload-filter.md` §2) : un
client dont le `PlayerID` n'est pas encore identifié côté serveur doit toujours recevoir la carte
complète, sous peine de ne plus jamais pouvoir retrouver son identité (balayage par nom,
`VPlayerPage.jsx`).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un VJoueur est inscrit et a déjà joué au moins une manche (son nom est mémorisé) | | | |
| 2 | Dans les outils navigateur, vider uniquement l'identifiant stocké (`localStorage`) de cet appareil, en conservant le pseudo | Prêt pour une reconnexion « sans ID » | | |
| 3 | Recharger la page (F5) pendant que le jeu est en PREPARE ou READY (viser la fenêtre la plus étroite) | Le joueur retrouve son identité (équipe, score) — le repli sur la carte complète fonctionne même dans cette fenêtre réduite | | |
| 4 | Répéter en rechargeant plutôt en dehors de PREPARE/READY (ex. STARTED) | Comportement identique, non-régression #109 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — V8 : badges de connexion, pas de dégradation parasite (CA7)

**Objectif** : Vérifier qu'une rafale de PONG des autres joueurs ne dégrade jamais le badge de
connexion (#109/#118) d'un VJoueur qui, lui, est réellement déconnecté — non-régression du
correctif #118 tout juste livré (risque R1 du plan).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 5 VJoueurs inscrits, un buzzer physique déjà appairé | | | |
| 2 | Couper le réseau (mode avion) d'un des VJoueurs, attendre que son badge passe orange côté admin | Badge orange visible sur sa fiche | | |
| 3 | Lancer READY, laisser les AUTRES VJoueurs répondre en rafale (PONG) | Pendant toute la rafale, le badge du joueur déconnecté reste orange — il ne doit **jamais** passer rouge simplement parce que d'autres PONGent | | |
| 4 | Rétablir le réseau du joueur déconnecté | Reconnexion normale, badge revient à l'état masqué, score/équipe intacts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — V9 : un joueur ne répond jamais PONG

**Objectif** : Vérifier que PREPARE reste actif tant que tous n'ont pas répondu, et que l'admin voit
qui manque — comportement inchangé par #127.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Inscrire plusieurs VJoueurs, en laisser un volontairement inactif (onglet en arrière-plan ou fermé) | | | |
| 2 | Lancer READY | Les autres joueurs passent « prêt » normalement ; le jeu reste en PREPARE tant que le joueur manquant n'a pas répondu | | |
| 3 | Observer l'admin | La carte du joueur manquant reste visiblement « non prêt » — l'animateur peut identifier qui bloque | | |
| 4 | Utiliser « Forcer prêt » (si disponible) ou faire répondre le joueur manquant | Le jeu passe en READY normalement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Non-régression buzzers physiques

**Objectif** : Vérifier que le correctif, entièrement centré sur les VJoueurs, ne change rien à la
cadence ni au contenu reçu par les buzzers physiques (hors scope du plan, mais à couvrir en
non-régression).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 2 buzzers physiques appairés, mélangés avec des VJoueurs | | | |
| 2 | Lancer READY, laisser la rafale de PONG se dérouler | Les LED des buzzers physiques réagissent normalement pendant PREPARE (pas de gel, pas de scintillement anormal) | | |
| 3 | Vérifier le badge OTA / firmware d'un buzzer physique côté admin pendant et après la rafale | Toujours affiché correctement (non affecté par le filtrage TV/VJoueur) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 10 — Vue « Joueurs » basculée pendant PREPARE/READY

**Objectif** : Non-régression du fix dev-frontend (code-reviewer, majeur #2, SHA `a69424c`) : le
classement « Joueurs » affichait une seule entrée si l'animateur basculait cette vue pendant
PREPARE/READY côté VJoueur — conséquence de la carte `bumpers` réduite à un seul destinataire
pendant ces deux phases (`sortedPlayers` fige désormais le dernier classement complet connu au lieu
de le recalculer sur la carte réduite).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 4 VJoueurs inscrits et ayant déjà marqué des points lors d'une manche précédente | Classement « Joueurs » complet visible (4+ entrées) | | |
| 2 | Depuis l'admin, sélectionner une nouvelle question et cliquer READY, en restant sur la vue « Joueurs » côté TV/VJoueur pendant toute la fenêtre PREPARE→READY | Le classement « Joueurs » continue d'afficher **toutes** les entrées précédentes — **jamais** une seule entrée ni un classement tronqué, y compris pendant la rafale de PONG | | |
| 3 | Laisser passer la transition en READY, observer encore quelques secondes | Classement toujours complet | | |
| 4 | Lancer START, faire marquer des points, puis STOP | Le classement se met à jour normalement avec les nouveaux scores dès la sortie de PREPARE/READY | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Un VJoueur reçoit exactement 2 `UPDATE` entre l'entrée en PREPARE et le passage en READY,
      pour N=1 et N=10 (scénario 1)
- [ ] L'admin garde sa progression « prêt » équipe par équipe, en direct, inchangée (scénario 1/2)
- [ ] Aucun changement visible sur `/game`, `/tv`, `/player` pendant PREPARE→READY (scénario 2)
- [ ] Aucun chip d'équipe gris en MEMORY multi-équipes après READY (scénario 3)
- [ ] Barre d'équipes MEMOTION correctement colorée après READY (scénario 4)
- [ ] Badges d'équipe QCM complets en STOPPED/REVEALED (scénario 5)
- [ ] Une session sans `vplayer_id` retrouve son identité même rechargée pendant PREPARE/READY
      (scénario 6)
- [ ] Aucune dégradation parasite du badge de connexion pendant la rafale de PONG des autres joueurs
      (scénario 7)
- [ ] Comportement inchangé quand un joueur ne répond jamais (scénario 8)
- [ ] Le classement « Joueurs » reste complet s'il est affiché pendant PREPARE/READY, jamais réduit
      à une seule entrée (scénario 10)
- [ ] Aucune régression sur les buzzers physiques (scénario 9)
- [ ] Aucune erreur/`panic` dans les logs serveur pendant toute la procédure
- [ ] Suite automatisée complète (Go + React, 649+ tests) au vert avant validation finale (CA9) —
      voir `server-go/cmd/server/vplayer_broadcast_integration_test.go` pour la mesure automatisée
      du scénario 1 (CA1/CA2/CA6) et `main_broadcast_127_test.go` pour la couverture unitaire

## Notes QA

[Espace pour observations]

> Historique : au moment de la rédaction initiale de cette procédure, la suite automatisée
> `TestVPlayerBroadcastIntegration_PrepareToRevealSequence` (CA1) échouait avec un compte de 4
> `UPDATE` au lieu de 2, à cause de deux appels redondants dans `handleReady()` et
> `broadcastReady()`/`sendLEDSetAllBuzzers()`, hors du périmètre initial T1.1-T1.3. Corrigé par
> dev-backend (SHA `2a39fef`) — CA1 est désormais vérifié à la valeur exacte du contrat. Si l'étape
> 5 du scénario 1 montre à nouveau plus de 2 trames, c'est une régression à signaler, pas un
> comportement connu.
