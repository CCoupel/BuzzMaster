# Procédure de Test — Reconnexion VJoueur après coupure réseau (#118)

**Version** : à définir (branche `bugfix/vjoueur-reconnect-mismatch`)
**Date** : 2026-07-29
**Branche** : bugfix/vjoueur-reconnect-mismatch (rebasée sur bugfix/vplayer-enroll-redirect, #120)
**Testeur** : QA

---

## Contexte du Bug

Un VJoueur qui perd sa connexion réseau en cours de partie (WiFi coupé, point d'accès hors de
portée) reste bloqué sur l'écran de jeu, sans aucun indicateur, et ne se reconnecte **jamais**
automatiquement — seul un rechargement complet de la page (F5) rétablit la connexion.

**Cause racine (plan `_work/reports/plan-20260729-190000.md`)** : le serveur détecte déjà une
liaison morte (ping/pong protocolaire + délai de lecture 5s), mais **le client n'a aucune preuve
de vie exploitable** — le navigateur répond aux trames ping du protocole automatiquement, sans
jamais exposer d'événement JavaScript pour elles. Sur une vraie coupure réseau, la trame de
fermeture du serveur ne traverse jamais un lien coupé : `onclose` ne se déclenche pas côté client,
le socket devient un « zombie » (`readyState` toujours `OPEN`), et la garde de reconnexion refuse
justement de rouvrir une connexion tant que ce zombie existe. Le chemin de reconnexion est fermé
deux fois.

**⚠️ Point d'attention critique pour cette validation** : ce défaut n'a **jamais pu être détecté
en sandbox** — il nécessite une vraie coupure réseau physique (WiFi coupé sur un téléphone réel),
qu'aucun environnement de test automatisé ne peut simuler. C'est exactement ce qui a laissé passer
ce défaut lors de #120 (rapport `workflow-state.json` : « QUALIFIÉ AVEC RÉSERVES — scénario
multi-appareils/buzzer physique non exécutable en sandbox »). **Si cette procédure ne peut pas être
exécutée avec une vraie coupure réseau, #118 ne doit pas être déclaré résolu.**

**Fix attendu** :
- Le serveur émet un battement applicatif `HEARTBEAT` toutes les 3s (en plus du ping protocolaire
  existant, inchangé).
- Le client surveille l'arrivée de n'importe quel message ; au-delà de 3× la cadence annoncée sans
  nouvelle, il considère la liaison morte, ferme le socket zombie lui-même, et se reconnecte —
  sans rechargement de page.
- Le buzzer reste **actif** pendant la coupure (jamais désactivé) ; un appui est mis en file et
  envoyé à la reconnexion, sauf si la question a changé entre-temps (abandon silencieux).
- Un bandeau informe de l'état de la liaison (perdue / rétablie).

Maquette de référence : https://claude.ai/code/artifact/118bdb47-49c3-487c-b372-782a12ce385b

---

## Prérequis

- [ ] Environnement : QUALIF (serveur accessible sur le réseau local)
- [ ] **Un smartphone réel** connecté en WiFi, avec accès aux réglages réseau (pouvoir couper/
      rétablir le WiFi à la demande) — impératif, aucun simulateur ne remplace ce scénario
- [ ] Un jeu en cours (phase STARTED sur une question) avec au moins un VJoueur inscrit
- [ ] Accès admin (`/game`) pour observer le badge de connexion et les scores
- [ ] Idéalement : un second appareil VJoueur et/ou un buzzer physique pour le scénario de charge

---

## Scénario 1 — Cas nominal : coupure réseau brève en cours de partie

**Objectif** : Vérifier la reconnexion automatique après une coupure réseau réelle, sans
rechargement de page.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | VJoueur inscrit et en jeu (phase STARTED), score non nul si possible | Écran de jeu normal, aucun bandeau | | |
| 2 | Couper le WiFi du téléphone (mode avion, ou désactivation WiFi) | Après quelques secondes (jusqu'à ~9-15s), un bandeau orange « Connexion perdue — reconnexion… » apparaît, avec un point clignotant | | |
| 3 | Observer le buzzer pendant la coupure | Le bouton buzzer reste visuellement actif (pas grisé, pas de curseur "interdit") | | |
| 4 | Rétablir le WiFi après ~30 secondes | Le bandeau passe au vert « Connexion rétablie » puis disparaît après ~2 secondes, **sans aucun rechargement de page** | | |
| 5 | Vérifier côté admin | Le badge de connexion du joueur revient à l'état normal ; score et équipe du joueur sont **intacts** (aucune perte) | | |
| 6 | Le joueur peut à nouveau buzzer normalement après reconnexion | Un buzz après reconnexion est pris en compte côté admin | | |

**Verdict** : [ ] PASS  [ ] FAIL

### Variante — coupure longue (5 minutes)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Répéter le scénario 1 avec une coupure de 5 minutes au lieu de 30s | Même résultat : reconnexion automatique au retour, sans rechargement, score/équipe conservés | | |

**Verdict** : [ ] PASS  [ ] FAIL

### Variante — lien instable (coupures rapprochées)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Activer/désactiver le WiFi plusieurs fois de suite rapidement (quelques secondes entre chaque bascule) | La reconnexion finit par aboutir malgré les coupures répétées — pas de blocage définitif | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Cas critique : buzz différé pendant un changement de question

**Objectif** : Vérifier que le buzz mis en attente pendant une coupure ne « fuite » jamais sur une
question différente de celle où il a été pressé — le point de vigilance le plus important du
correctif (F7).

### 2a — Buzz différé, cas nominal (même question)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | VJoueur en jeu, question N en phase STARTED | Écran de jeu normal | | |
| 2 | Couper le WiFi du téléphone | Bandeau orange affiché après quelques secondes | | |
| 3 | Pendant la coupure, appuyer sur le buzzer | Aucun retour visuel de buzz confirmé (le serveur ne l'a pas reçu) — l'appui est mémorisé localement | | |
| 4 | Rétablir le WiFi **avant que la question N ne se termine** | Reconnexion automatique, puis le buzz apparaît côté admin comme si le joueur avait buzzé normalement | | |
| 5 | Vérifier côté admin | Le buzz est comptabilisé sur la question N, avec un temps de réaction cohérent | | |

**Verdict** : [ ] PASS  [ ] FAIL

### 2b — Buzz différé, cas critique (la question change pendant la coupure)

**C'est le scénario qui valide le point de vigilance principal du plan** : sans la double garde
(purge sur `PREPARE` observé + validation de l'identité de question au moment du vidage), un buzz
périmé pourrait être compté à tort sur la question suivante.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | VJoueur en jeu, question N en phase STARTED | Écran de jeu normal | | |
| 2 | Couper le WiFi du téléphone | Bandeau orange affiché après quelques secondes | | |
| 3 | Pendant la coupure, appuyer sur le buzzer | Aucun retour visuel de confirmation | | |
| 4 | **Depuis l'admin (sur un autre appareil)**, laisser la question N se terminer normalement (STOP → REVEAL) puis lancer la question N+1 (READY → START) — **le tout pendant que le téléphone du joueur reste hors ligne** | La question N+1 démarre normalement pour les autres participants | | |
| 5 | Rétablir le WiFi du téléphone du joueur, une fois la question N+1 déjà en cours | Le téléphone se reconnecte automatiquement (bandeau vert puis disparition) | | |
| 6 | Vérifier côté admin | **Aucun buzz n'apparaît pour ce joueur sur la question N+1** — le buzz appuyé pendant la coupure a été silencieusement abandonné | | |
| 7 | Le joueur peut buzzer normalement sur la question N+1 depuis ce point | Un nouveau buzz sur N+1 est pris en compte normalement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Une coupure réseau réelle (WiFi coupé/rétabli sur téléphone) déclenche une reconnexion
      automatique, sans aucun rechargement de page (scénario 1)
- [ ] Le bandeau de connexion (orange puis vert) s'affiche conformément à la maquette (scénario 1)
- [ ] Le buzzer reste actif visuellement pendant toute la coupure (scénario 1, étape 3)
- [ ] Score et équipe du joueur sont conservés après reconnexion (scénario 1)
- [ ] Une coupure longue (5 min) et un lien instable (coupures rapprochées) n'empêchent pas la
      reconnexion (scénario 1, variantes)
- [ ] Un buzz pendant une coupure brève (même question au retour) est bien pris en compte (2a)
- [ ] **Un buzz pendant une coupure couvrant tout un changement de question n'apparaît JAMAIS sur
      la nouvelle question** (2b — le point de vigilance principal)
- [ ] Non-régression #120 : le comportement d'inscription VJoueur reste inchangé
- [ ] **Cette procédure a été exécutée avec une vraie coupure réseau physique** — si ce n'est pas
      le cas, l'indiquer explicitement dans les notes ci-dessous et ne pas considérer #118 comme
      validé (cf. avertissement en tête de document)

## Notes QA

[Espace pour observations — préciser notamment si la coupure réseau a été simulée ou réelle]
