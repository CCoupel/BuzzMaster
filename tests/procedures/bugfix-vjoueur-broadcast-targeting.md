# Procédure de Test — Ciblage des broadcasts hors PREPARE/READY (#129)

**Version** : à définir (branche `bugfix/vjoueur-broadcast-targeting`)
**Date** : 2026-08-03
**Branche** : bugfix/vjoueur-broadcast-targeting
**Testeur** : QA

---

## Contexte du Bug

Prolongement direct de #127. Trois événements propres à **un seul** participant — sa connexion, sa
déconnexion, chacune de ses frappes ARDOISE — déclenchaient chacun un `UPDATE` complet vers **tous**
les VJoueurs, à n'importe quel moment de la partie (pas seulement pendant PREPARE/READY, fenêtre déjà
traitée par #127). Un audit consommateur par consommateur a montré que les VJoueurs n'exploitent
**aucune** de ces données — hors l'écho de leur propre état lors de leur reconnexion. Un quatrième
site relève du même principe : la rafale per-PONG conservée par #127 pour Admin+TV+Buzzer atteint
encore la TV, qui n'affiche pourtant qu'un libellé statique pendant PREPARE.

**Correctif attendu** (`contracts/vplayer-payload-filter.md` §5) :
- Déconnexion (`onPlayerDisconnected`) : plus aucun envoi vers les VJoueurs (Admin/TV/Buzzer inchangés).
- Connexion/reconnexion (`handlePlayerConnect`) : plus d'envoi vers les **autres** VJoueurs, mais le
  participant qui se (re)connecte reçoit un **`UPDATE` ciblé** portant son propre état (`CONNECTED`,
  équipe, score) — obligatoire, sous peine de régresser #118/#120/#122.
- Saisie ARDOISE (`ARDOISE_INPUT`) : seul l'admin continue de recevoir l'`UPDATE` — plus personne
  d'autre (TV, VJoueurs, buzzers). Ferme au passage une fuite d'équité : le texte que les autres
  équipes saisissent n'arrive plus dans le navigateur de chaque joueur.
- Rafale per-PONG résiduelle (site 4) : la TV n'en reçoit plus, ne conservant que les deux bornes de
  la fenêtre PREPARE→READY (comme le VJoueur depuis #127). Les buzzers physiques restent servis
  (seul signal de phase disponible pour le firmware).
- *(Phase 2, séparable)* Regroupement des `UPDATE` ARDOISE sur une fenêtre ≤ 150 ms, avec vidage
  immédiat à tout changement de phase — ne change rien pour les VJoueurs (déjà à zéro après la
  phase 1), vise la contention du verrou moteur côté serveur.
- *(Phase 3, séparable)* Même ciblage pour les réponses BUTTON/QCM : chaque joueur reçoit la
  confirmation de **sa** réponse, les autres VJoueurs ne reçoivent plus rien de ce fait.

**Objectif affiché du correctif : aucun changement visible, sur aucune interface.** Toute différence
d'affichage constatée pendant cette procédure est un défaut à signaler, jamais un effet attendu.

Maquette de référence : `_work/mockups/129-broadcast-targeting.md` (diagrammes de séquence
AVANT/APRÈS, matrice « qui reçoit quoi » §4, points de contrôle W1→W12 repris ci-dessous).

> **Portée de cette procédure au moment de sa rédaction** : seule la **Phase 1** (ciblage) est
> confirmée livrée (commit `96a1d09`). Les scénarios 2 (partie « latence ≤ 150 ms ») et 7 (QCM ciblé,
> phase 3) sont à exécuter seulement une fois les phases correspondantes livrées — à vérifier auprès
> du CDP avant de les cocher FAIL en leur absence.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `bugfix/vjoueur-broadcast-targeting`)
- [ ] Au moins 8 appareils/onglets distincts pouvant ouvrir `/player` — les scénarios de charge
      ciblent 8 VJoueurs / 8 équipes
- [ ] Un accès admin (`/game`) et un accès TV (`/tv`) ouverts en parallèle, visibles pendant tout le
      test
- [ ] Au moins une question ARDOISE et une question QCM disponibles
- [ ] Au moins 2 buzzers physiques appairés (scénario 6, W12)
- [ ] Outils navigateur (onglet réseau / console) sur au moins un appareil VJoueur, pour observer les
      trames WebSocket reçues (et confirmer l'absence de `ARDOISE_ANSWERS`, scénario 4)
- [ ] Accès aux logs serveur (pour confirmer l'absence d'erreur/`panic`)
- [ ] Moyen de couper le réseau d'un ou plusieurs appareils VJoueur à la demande (mode avion, ou
      déconnexion Wi-Fi du point d'accès)

---

## Scénario 1 — Déconnexions/reconnexions en masse (CA1/CA2/CA3, W1/W4/W5/W8)

**Objectif** : Vérifier que la (dé)connexion d'un participant n'atteint plus les autres VJoueurs, que
celui qui se reconnecte retrouve bien son état, et que l'admin garde une vue en direct.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Inscrire 8 VJoueurs, tous en jeu (au moins une manche jouée, scores non nuls) | Les 8 apparaissent côté admin avec leurs scores | | |
| 2 | Ouvrir l'onglet réseau (WebSocket) sur UN appareil VJoueur qui restera connecté tout du long | Prêt à observer l'absence de trames | | |
| 3 | Couper le réseau des 8 autres VJoueurs quasi simultanément (mode avion / coupure Wi-Fi) | **W1** : côté admin, les badges de connexion passent orange **un par un, en direct** — pas tous d'un coup à la fin | | |
| 4 | Sur l'appareil observé à l'étape 2, compter les trames `UPDATE` reçues pendant les 8 déconnexions | **0 trame `UPDATE`** liée à ces déconnexions (CA1) | | |
| 5 | **W5** : observer l'écran de l'appareil resté connecté pendant les 8 déconnexions | Aucun changement visible — ni classement, ni équipe, ni score qui bougent | | |
| 6 | Rétablir le réseau des 8 appareils, quasi simultanément | **W1** (suite) : côté admin, les badges reviennent à la normale un par un, en direct | | |
| 7 | Sur l'appareil observé à l'étape 2, recompter les trames `UPDATE` reçues pendant les 8 reconnexions | Toujours **0 trame `UPDATE`** liée à ces reconnexions des autres (CA1) | | |
| 8 | **W4** : sur CHACUN des 8 appareils qui se reconnectent, vérifier l'état retrouvé | Nom, équipe, couleur et score identiques à avant la coupure — **aucune** régression #118/#120/#122 | | |
| 9 | Sur un des 8 appareils qui se reconnecte, avec l'onglet réseau ouvert AVANT la reconnexion | Reçoit **exactement 1** `UPDATE` (l'écho ciblé, CA2), contenant son propre bumper avec `CONNECTED=true` | | |
| 10 | **W8** : observer les badges de connexion de tous les VJoueurs après reconnexion | Aucune dégradation orange/rouge parasite — comportement #109/#118 inchangé | | |
| 11 | Vérifier les logs serveur | Aucune erreur, aucun `panic` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Saisie ARDOISE en rafale (CA4/CA5, W2/W6)

**Objectif** : Vérifier que la saisie ARDOISE de 8 équipes n'atteint plus les VJoueurs ni la TV, et
que l'admin garde une vue en direct.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Préparer une question ARDOISE, 8 équipes avec au moins un VJoueur chacune | | | |
| 2 | Lancer READY → START, ouvrir l'onglet réseau sur un appareil VJoueur | Prêt à observer l'absence de trames | | |
| 3 | Faire taper les 8 équipes simultanément pendant ~45 secondes | **W2** : côté admin, les réponses de chaque équipe s'affichent et se mettent à jour **en direct** | | |
| 4 | **W6** : sur un appareil VJoueur, observer sa propre saisie pendant que les autres tapent | Saisie fluide, locale, jamais écrasée par le texte des autres | | |
| 5 | Sur l'appareil VJoueur observé à l'étape 2, compter les trames `UPDATE` reçues pendant toute la saisie | **0 trame `UPDATE`** issue d'`ARDOISE_INPUT` (CA4) | | |

**Verdict** : [ ] PASS  [ ] FAIL

### Variante — latence du regroupement (Phase 2, à exécuter seulement si livrée)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Répéter l'étape 3 ci-dessus en chronométrant, à l'œil, le délai entre une frappe et son apparition côté admin | Délai supplémentaire **imperceptible** (≤ 150 ms, CA5) | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (phase 2 non livrée)

---

## Scénario 3 — REVEAL ARDOISE : dernières frappes présentes (CA6, W3/W7/W10)

**Objectif** : Vérifier que malgré le ciblage (et le regroupement en phase 2), aucune réponse n'est
perdue au moment du résultat.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question ARDOISE, 8 équipes en train de taper | | | |
| 2 | **W3** : observer la TV pendant toute la saisie | Rien ne s'affiche côté TV pendant la saisie (comportement déjà en place aujourd'hui) | | |
| 3 | Faire taper **une dernière modification** sur plusieurs équipes, puis cliquer STOP/REVEAL **immédiatement après** (viser la fenêtre la plus étroite possible, idéalement < 150 ms après la dernière frappe) | | | |
| 4 | **W10** : observer l'admin au moment du résultat | **Toutes** les réponses, y compris les toutes dernières frappes, sont présentes — aucune réponse tronquée ou manquante | | |
| 5 | **W3/W7** : observer la TV puis le VJoueur au REVEAL | Toutes les réponses affichées correctement des deux côtés, comme avant le correctif | | |
| 6 | Répéter 2-3 fois avec un timing différent | Comportement identique à chaque fois | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Fuite `ARDOISE_ANSWERS` fermée (équité)

**Objectif** : Confirmer que le texte saisi par les autres équipes n'est plus jamais présent dans le
navigateur d'un VJoueur pendant la saisie — correction d'équité obtenue au passage par le ciblage.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question ARDOISE, au moins 2 équipes, onglet réseau/console ouvert sur un appareil VJoueur | | | |
| 2 | Faire taper une réponse distinctive sur une AUTRE équipe que celle de l'appareil observé | | | |
| 3 | Inspecter le trafic WebSocket reçu par l'appareil VJoueur observé (onglet réseau) pendant la saisie | Aucun message ne contient `ARDOISE_ANSWERS`, ni le texte saisi par l'autre équipe, sous quelque forme que ce soit (CA7) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Rafale per-PONG résiduelle retirée de la TV (CA12, W11)

**Objectif** : Vérifier que le site 4 (TV retirée du per-PONG) ne provoque aucun retard ni
clignotement visible sur l'écran TV.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Inscrire 10 VJoueurs, ouvrir la TV (`/tv`) | | | |
| 2 | Depuis l'admin, sélectionner une question et cliquer READY | **W11** : la TV affiche « 🔔 NOUVELLE QUESTION » dès l'entrée en PREPARE, sans délai perceptible | | |
| 3 | Laisser les 10 VJoueurs répondre (PONG) jusqu'au passage en READY, en observant la TV en continu | Aucun clignotement, aucun gel, aucun rafraîchissement visible de l'écran TV pendant la rafale | | |
| 4 | Observer la bascule au passage en READY | **W11** (suite) : bascule sur l'écran READY sans retard ni clignotement | | |
| 5 | Sur un onglet réseau ouvert côté TV, compter les trames `UPDATE` reçues sur toute la fenêtre PREPARE→READY | **2 trames `UPDATE`** exactement (entrée + transition), quel que soit N — comme le VJoueur depuis #127 (CA12) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Non-régression buzzers physiques (W12)

**Objectif** : Vérifier que le ciblage, centré sur VJoueur/TV, ne change rien au comportement des
buzzers physiques — en particulier ils gardent leur cadence per-PONG (seul signal de phase dont
dispose le firmware).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 2 buzzers physiques appairés, mélangés avec des VJoueurs | | | |
| 2 | Lancer READY, laisser la rafale de PONG se dérouler | **W12** : assignation d'équipe et rotation grise des LED inchangées — comportement identique à avant le correctif | | |
| 3 | Vérifier que les buzzers physiques répondent bien à chaque cycle PONG (pas de gel) | Cadence per-PONG inchangée | | |
| 4 | Question ARDOISE, équipes tapant : observer les buzzers physiques (s'ils sont assignés à une équipe qui tape) | Aucun effet visible — les buzzers n'ont jamais reçu ni consommé `ARDOISE_ANSWERS` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — QCM ciblé (Phase 3, à exécuter seulement si livrée) (CA10, W9)

**Objectif** : Vérifier que chaque joueur reçoit la confirmation de sa propre réponse QCM, sans que
les autres VJoueurs ne reçoivent de mise à jour à ce titre — et que `PAUSE` continue d'atteindre tout
le monde (pilotage LED).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question QCM, au moins 5 VJoueurs, STARTED | | | |
| 2 | Un VJoueur répond, onglet réseau ouvert sur un AUTRE VJoueur qui n'a pas encore répondu | **W9** : celui qui répond voit la confirmation de sa réponse s'afficher normalement | | |
| 3 | Sur l'appareil observé (celui qui n'a pas répondu), vérifier les trames reçues suite à la réponse du premier | Aucun `UPDATE` reçu de ce fait (CA10) — seul le `PAUSE` qui accompagne le buzz doit être visible (pilotage LED, inchangé) | | |
| 4 | Faire répondre plusieurs VJoueurs quasi simultanément | Chacun voit sa propre confirmation ; aucun VJoueur ne voit la réponse d'un autre avant STOP/REVEAL | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (phase 3 non livrée)

---

## Scénario 8 — Reprise d'identité par NOM, sans `vplayer_id` (CA9, non-régression)

**Objectif** : Vérifier que `sendStateToClient()` (émis sur `HELLO`) continue d'envoyer le payload
complet non réduit à la connexion — c'est la seule source permettant à une session sans `vplayer_id`
de retrouver son identité par balayage `NAME`. Ce chemin n'est pas modifié par #129, mais le contrat
§5 le fige explicitement : à revérifier après le ciblage des trois autres sites.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un VJoueur est inscrit et a déjà joué (nom mémorisé) | | | |
| 2 | Vider uniquement l'identifiant stocké (`localStorage`) de cet appareil, en conservant le pseudo | | | |
| 3 | Recharger la page (F5) | Le joueur retrouve son identité (équipe, score) via le payload complet reçu à la connexion — aucune régression | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] 8 déconnexions + 8 reconnexions : 0 `UPDATE` reçu par un VJoueur resté connecté (scénario 1, CA1)
- [ ] Le VJoueur qui se reconnecte reçoit exactement 1 `UPDATE` ciblé avec son état à jour, sans
      régression #118/#120/#122 (scénario 1, CA2)
- [ ] L'admin garde une vue en direct des (dé)connexions et des réponses ARDOISE (scénario 1/2, CA3)
- [ ] 0 `UPDATE` issu d'`ARDOISE_INPUT` reçu par un VJoueur ou la TV pendant la saisie (scénario 2, CA4)
- [ ] Aucune réponse ARDOISE perdue au REVEAL malgré le ciblage/regroupement (scénario 3, CA6)
- [ ] `ARDOISE_ANSWERS` absent de tout message reçu par un client VJoueur en phase STARTED (scénario 4, CA7)
- [ ] Aucune dégradation parasite du badge de connexion sur le chemin d'écho ciblé (scénario 1, CA8)
- [ ] La TV reçoit exactement 2 `UPDATE` sur la fenêtre PREPARE→READY, sans retard ni clignotement
      visible (scénario 5, CA12)
- [ ] Aucune régression sur les buzzers physiques : cadence per-PONG et LED inchangées (scénario 6)
- [ ] *(Phase 2, si livrée)* Latence du regroupement ARDOISE imperceptible (≤ 150 ms) côté admin (scénario 2 variante, CA5)
- [ ] *(Phase 3, si livrée)* Chaque joueur reçoit sa propre confirmation QCM, jamais celle d'un autre
      avant STOP/REVEAL ; `PAUSE` toujours diffusé à tous (scénario 7, CA10)
- [ ] Reprise d'identité par NOM (session sans `vplayer_id`) toujours fonctionnelle (scénario 8, CA9)
- [ ] Aucune erreur/`panic` dans les logs serveur pendant toute la procédure
- [ ] Suite automatisée complète (Go + React) au vert avant validation finale (CA11), avec attention
      particulière aux suites listées dans le plan (`main_broadcast_127_test.go`,
      `player_connect_connstate*_test.go`, `onplayerdisconnected_ghost_test.go`,
      `player_evicted*_test.go`, `name_recovery_test.go`)

## Notes QA

[Espace pour observations]

> Note pour QA : au moment de la rédaction, seule la Phase 1 (ciblage) est confirmée livrée
> (commit `96a1d09`). Les scénarios 2 (variante latence) et 7 (QCM ciblé) sont marqués N/A si les
> phases 2/3 correspondantes ne sont pas encore livrées — vérifier auprès du CDP avant de les cocher
> FAIL en leur absence.
