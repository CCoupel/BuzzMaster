# Procédure de Test — Libérer la place d'un VJoueur connecté (#134)

**Version** : à définir (branche `feature/vjoueur-reassign-slot`)
**Date** : 2026-08-04
**Branche** : feature/vjoueur-reassign-slot
**Testeur** : QA

---

## Contexte de la Feature

Aujourd'hui, cliquer « Réinscription » sur un joueur **encore connecté** ne produit aucun effet
observable — ni côté joueur (session intacte), ni côté admin (aucune carte ne bouge). Le bouton n'a
jamais été conçu pour ce cas : c'est une autorisation différée pour un joueur **déjà tombé** (#122).

**Comportement attendu** (`contracts/seat-release.md`) :
- Sur un joueur **connecté**, « Réinscription » l'évince désormais, le notifie, et libère sa place —
  **sans perdre son score**. Le libellé du bouton ne change pas ; seuls deux avertissements
  apparaissent dans la modale de retrait, selon l'état du joueur.
- Le mécanisme est un **re-clé** (même struct `*Bumper`, nouvelle clé `ID`) : c'est le même
  mécanisme qui préserve déjà le score en #122, réutilisé tel quel — pas un concept nouveau.
- N'importe quel joueur peut reprendre le siège libéré, pas seulement l'occupant précédent : le
  siège porte le score, pas la personne (comportement **volontaire**, à faire valider en QA — Y3).
- Sur un joueur **déconnecté**, le comportement est **strictement inchangé** (#122) : c'est la
  non-régression la plus critique de ce lot.
- La connexion WebSocket n'est **jamais** fermée de force par le serveur — le client se retire de
  lui-même après avoir traité la notification.

Maquette de référence : `_work/mockups/134-seat-release.md` (maquettes d'écran, textes exacts,
chronogramme, machine à états du siège, points de contrôle Y1→Y10 repris ci-dessous).

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `feature/vjoueur-reassign-slot`)
- [ ] Accès admin (`/game` ou `/`) avec la modale de retrait d'un VJoueur
- [ ] Au moins 3 appareils/onglets `/player` distincts, dont un qui restera déconnecté volontairement
- [ ] Un moyen de couper le réseau d'un appareil à la demande (mode avion) — scénario 4 (Y5)
- [ ] Un accès pour ouvrir/fermer les inscriptions depuis l'admin
- [ ] Accès à `/tv` pour vérifier l'affichage du score pendant la libération (scénario 6, Y7)
- [ ] Accès aux logs serveur

---

## Scénario 1 — Réinscription sur un joueur DÉCONNECTÉ (Y1, non-régression #122)

**Objectif** : Vérifier que le comportement historique (#122) n'a **strictement pas changé** — c'est
la non-régression la plus critique du lot.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un VJoueur (« Marie ») est inscrit puis se déconnecte (fermer l'onglet ou mode avion), badge orange/rouge côté admin | | | |
| 2 | Depuis l'admin, ouvrir la modale de retrait de Marie et cliquer « Réinscription » | **Aucun** des deux avertissements « connectée » ne doit apparaître (elle est déconnectée) — comportement de modale identique à avant #134 | | |
| 3 | Observer la carte de Marie côté admin juste après le clic | **Aucun changement visible** : pas d'éviction, pas de re-clé, la carte reste telle quelle | | |
| 4 | Retenter une connexion sur l'appareil de Marie, sans identifiant stocké, même pseudo | Elle reprend son siège normalement (comportement #122 inchangé) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Réinscription sur un joueur CONNECTÉ, inscriptions ouvertes (Y2/Y3)

**Objectif** : Vérifier le cœur de la feature — éviction, conservation du score, reprise par
n'importe quel joueur. **Vérifier explicitement que l'avertissement/sous-titre est vu avant le clic**
(R6 du plan) — un tiers qui hérite du score est un comportement voulu, mais qui doit être visible et
compris par l'animateur avant qu'il ne confirme.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « Marie » est inscrite, connectée, a un score non nul (ex. 42 pts) sur une équipe | | | |
| 2 | Depuis l'admin, ouvrir la modale de retrait de Marie | Le sous-titre **« Retrouve son score et son équipe »** est visible sous l'option Réinscription, **et** l'avertissement « ⚠ Marie est connectée : elle sera renvoyée à l'inscription tout de suite. » apparaît — **avant** tout clic (CA8) | | |
| 3 | Cliquer « Réinscription » | Chez Marie : écran de motif affiché en **moins d'1 seconde**, texte « Ta place a été libérée par l'animateur. Réinscris-toi avec le même pseudo : tu retrouveras ton score et ton équipe. », puis redirection vers l'inscription | | |
| 4 | Côté admin, observer la carte de Marie | Elle passe déconnectée, **score toujours affiché (42 pts, pas remis à zéro)** | | |
| 5 | Sur l'appareil de Marie, se réinscrire avec **le même pseudo** | Elle retrouve son score (42 pts) et son équipe | | |
| 6 | **Variante Y3** : répéter les étapes 1-4, puis réinscrire un **AUTRE** appareil avec le pseudo « Marie » (pas celui qui était connecté) | Ce nouvel appareil retrouve le score et l'équipe de Marie (42 pts) — comportement voulu : le siège porte le score, pas la personne | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Réinscription sur un joueur CONNECTÉ, inscriptions FERMÉES (Y4)

**Objectif** : Vérifier le second avertissement — piège identifié dans le plan (l'avertissement
« inscriptions fermées » n'existait auparavant que sous Suppression totale).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Fermer les inscriptions depuis l'admin | | | |
| 2 | « Marie » est connectée, en jeu | | | |
| 3 | Ouvrir la modale de retrait de Marie | **Les deux avertissements** apparaissent sous Réinscription : « connectée : renvoyée tout de suite » **et** « ⚠ Inscriptions fermées : Marie ne pourra pas se réinscrire tant qu'elles le sont. » (CA8) | | |
| 4 | Cliquer « Réinscription » | Marie voit l'écran de motif puis atterrit sur l'écran d'attente (inscriptions fermées) — pas de formulaire, pas de blocage silencieux | | |
| 5 | Depuis l'admin, rouvrir les inscriptions | | | |
| 6 | Sur l'appareil de Marie | Elle peut désormais se réinscrire avec son pseudo et reprend son siège (score conservé) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Notification perdue (Y5)

**Objectif** : Vérifier le filet de sécurité — si `PLAYER_EVICTED` ne parvient jamais au joueur
(réseau coupé au mauvais moment), il ne doit **jamais** se reconnecter silencieusement sur l'ancien
identifiant, ni rester bloqué sur un écran figé indéfiniment.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « Marie » est connectée, en jeu | | | |
| 2 | Couper le réseau de son appareil (mode avion) **juste avant** que l'admin clique « Réinscription » | | | |
| 3 | Depuis l'admin, cliquer « Réinscription » sur Marie (elle ne reçoit rien, hors ligne) | Côté admin, la carte de Marie passe quand même déconnectée avec score conservé (le serveur ne dépend pas de l'accusé de réception) | | |
| 4 | Rétablir le réseau de l'appareil de Marie | Au pire, l'appareil reste figé sur l'ancien écran de jeu au maximum **~10 secondes** (filet client), puis se rétablit — jamais indéfiniment | | |
| 5 | Observer le comportement final sur l'appareil de Marie | Il finit par afficher l'écran de motif (ou directement l'écran d'inscription) avec le bon texte `SEAT_RELEASED` — **jamais** de reconnexion silencieuse avec effet de bord (pas de partie continuée comme si de rien n'était) | | |
| 6 | Se réinscrire avec le même pseudo | Score et équipe retrouvés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Reconnexion avec l'ancien identifiant juste après libération (Y6)

**Objectif** : Vérifier que la course qui motive le re-clé est bien fermée — l'ancien ID ne doit
jamais permettre une reconnexion « comme si de rien n'était ».

> Nécessite un moyen technique de rejouer une requête `PLAYER_CONNECT` avec un ID explicite (outils
> dev / un ancien build client qui a gardé l'ID en mémoire avant la libération) — à défaut, ce
> scénario peut être délégué aux tests automatisés (CA5) et coché N/A ici.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Libérer la place d'un joueur connecté (comme scénario 2) | | | |
| 2 | Immédiatement après, tenter une reconnexion avec l'**ancien** identifiant (avant que le client n'ait purgé le sien) | La reconnexion est **refusée** avec le motif `SEAT_RELEASED` — jamais un retour silencieux dans la partie | | |
| 3 | Réessayer normalement (sans ID, même pseudo) | Reprise du siège fonctionne, score conservé — l'autorisation n'a **pas** été annulée par la tentative refusée | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (délégué aux tests automatisés CA5, pas d'outil disponible)

---

## Scénario 6 — Score et équipe visibles pendant la libération, admin et TV (Y7)

**Objectif** : Confirmer qu'aucun score n'est remis à zéro pendant la manipulation, ni côté admin ni
côté TV.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « Marie » connectée, score non nul, TV (`/tv`) ouverte sur un écran de classement | | | |
| 2 | Libérer sa place depuis l'admin | | | |
| 3 | Observer la TV immédiatement après | Le score de Marie reste affiché tel quel (elle passe déconnectée mais son score n'est pas remis à zéro ni retiré) | | |
| 4 | Observer la carte admin | Idem — score conservé, carte simplement marquée déconnectée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Suppression totale, non-régression (Y8)

**Objectif** : Confirmer que l'option « Suppression totale » (destruction du score) n'est en rien
affectée par ce lot.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un VJoueur connecté, score non nul | | | |
| 2 | Ouvrir la modale de retrait, choisir « Suppression totale » | Comportement identique à avant #134 : bumper supprimé, score **perdu** | | |
| 3 | Côté joueur | Message `PLAYER_REMOVED` (pas `SEAT_RELEASED`) — texte inchangé | | |
| 4 | Se réinscrire avec le même pseudo | Nouvelle inscription à zéro, aucun score hérité | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Libération pendant la phase PREPARE (Y9)

**Objectif** : Vérifier que libérer un joueur pendant PREPARE ne bloque **pas** la partie —
c'était le risque le plus important identifié dans le plan (R1).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 3 VJoueurs inscrits sur 2 équipes, tous connectés | | | |
| 2 | Depuis l'admin, sélectionner une question et cliquer READY (entrée en PREPARE) | Les cartes commencent à passer « prêt » au fur et à mesure des PONG | | |
| 3 | **Avant** que tous les joueurs aient répondu, libérer la place d'un des joueurs connectés (« Réinscription ») | Le joueur libéré est évincé normalement | | |
| 4 | Laisser les autres joueurs répondre (PONG) | La partie **passe bien en READY** malgré le joueur libéré qui ne répondra jamais — **pas de blocage** | | |
| 5 | Vérifier les logs serveur | Aucune erreur, aucun blocage silencieux | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 9 — Client antérieur face au nouveau serveur (Y10)

**Objectif** : Vérifier la dégradation propre pour un client qui ne connaît pas encore le motif
`SEAT_RELEASED`.

> Si un client antérieur (build précédant #134) n'est pas disponible pour ce test, cocher N/A et le
> signaler — ce scénario peut être couvert par les tests automatisés (CA10) à défaut d'un
> environnement dédié.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Connecter un client antérieur à #134 (VJoueur, build précédent) | | | |
| 2 | Libérer sa place depuis l'admin (nouveau serveur) | Le joueur est bien renvoyé à l'inscription, avec un texte générique (motif non reconnu par ce client) — **jamais bloqué**, jamais d'écran figé | | |
| 3 | Se réinscrire | Fonctionne normalement | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (pas de client antérieur disponible)

---

## Critères de Validation

- [ ] Joueur déconnecté : comportement #122 strictement inchangé, aucune éviction (scénario 1, Y1)
- [ ] Joueur connecté, inscriptions ouvertes : éviction en < 1 s, score et équipe retrouvés à la
      réinscription — vu et compris par l'animateur AVANT confirmation (sous-titre + avertissement
      visibles) (scénario 2, Y2/Y3)
- [ ] Un tiers peut reprendre un siège libéré et hérite du score — comportement voulu, validé en QA
      (scénario 2 variante, Y3)
- [ ] Inscriptions fermées : les deux avertissements apparaissent, le joueur atterrit sur l'écran
      d'attente, reprend son siège à la réouverture (scénario 3, Y4)
- [ ] Notification perdue : jamais de reconnexion silencieuse, filet client ~10 s, réinscription
      possible ensuite (scénario 4, Y5)
- [ ] Reconnexion avec l'ancien ID refusée sans annuler l'autorisation de reprise (scénario 5, Y6)
- [ ] Score jamais remis à zéro pendant la libération, sur admin et TV (scénario 6, Y7)
- [ ] Suppression totale inchangée : score perdu, message `PLAYER_REMOVED` (scénario 7, Y8)
- [ ] Libération pendant PREPARE ne bloque pas le passage en READY (scénario 8, Y9)
- [ ] Client antérieur dégradé proprement, jamais bloqué (scénario 9, Y10)
- [ ] Aucune erreur/`panic` dans les logs serveur pendant toute la procédure
- [ ] Suite automatisée complète (Go + React) au vert avant validation finale (CA12), en particulier
      `player_evicted_test.go`, `player_evicted_roster_diff_test.go`, `name_recovery_test.go`,
      `reconnect_id_test.go`, `VPlayerPage.name-recovery.test.jsx`, `EnrollPage.redirect-banner.test.jsx`

## Notes QA

[Espace pour observations]

> Note pour QA : les scénarios 5 (Y6) et 9 (Y10) peuvent être cochés N/A si l'environnement/outillage
> nécessaire n'est pas disponible — ils sont alors couverts par les tests automatisés (CA5/CA10). Ne
> pas les cocher FAIL par défaut dans ce cas.
