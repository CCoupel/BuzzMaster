# Procédure de Test — Icône de déconnexion pour les VJoueurs (#109)

**Version** : à définir (branche `bugfix/vjoueur-disconnect-icon`)
**Date** : 2026-07-11
**Branche** : bugfix/vjoueur-disconnect-icon
**Testeur** : QA

---

## Contexte du Bug

Un VJoueur (joueur virtuel connecté via `/ws/player`, ex. depuis un smartphone) qui perd sa
connexion (onglet fermé, réseau coupé) ne fait apparaître **aucune icône de déconnexion**
côté admin (`GamePage`) ni dans `TeamsPage`, contrairement à un buzzer physique.

**Cause racine (cumulative)** :
1. **Backend** : rien ne mettait jamais `Bumper.Connected` à `true`/`false` pour un VPlayer
   (ni à la connexion, ni à la déconnexion WebSocket réelle).
2. **Frontend** : le badge de déconnexion excluait explicitement les VJoueurs
   (`!buzzer.isVPlayer && !buzzer.isVirtual`), masquant l'icône même si `Connected` était correct.

**Fix attendu** :
- Backend : `Connected=true` à la création/reconnexion d'un VJoueur, `Connected=false` à la
  déconnexion WebSocket réelle (avec garde anti-"flash zombie" en cas de reconnexion rapide).
- Frontend : affichage de l'icône pour tout bumper (physique ou VJoueur) avec `connected === false`.

Cette procédure couvre le comportement fonctionnel de bout en bout, une fois les 3 batches du
plan livrés (backend Phase 1+2, frontend Phase 3). Voir le plan complet :
`_work/reports/plan-20260711-160927.md`.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `bugfix/vjoueur-disconnect-icon`)
- [ ] Un jeu en phase `ENROLL` actif (pour pouvoir enrôler un VJoueur) puis en jeu
- [ ] Un navigateur/onglet mobile (ou second onglet desktop) pour simuler le VJoueur
- [ ] Accès admin (`/admin` ou `/game`) et à `TeamsPage`
- [ ] Optionnel : un buzzer physique connecté pour le scénario de non-régression

---

## Scénario 1 — Connexion d'un VJoueur : aucune icône affichée

**Objectif** : Vérifier qu'un VJoueur fraîchement connecté (ou reconnecté) n'affiche jamais
l'icône de déconnexion.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la page d'enrôlement VJoueur (`/player` ou équivalent) sur un second appareil/onglet | Formulaire d'enrôlement affiché | | |
| 2 | Saisir un nom (ex. "Alice") et valider | VJoueur enrôlé, redirigé vers l'écran de jeu VJoueur | | |
| 3 | Sur l'admin (`GamePage`), localiser la carte/équipe du VJoueur "Alice" | Aucune icône de déconnexion visible sur "Alice" | | |
| 4 | Sur `TeamsPage`, localiser "Alice" | Aucune icône de déconnexion visible sur "Alice" | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Déconnexion d'un VJoueur : icône apparaît

**Objectif** : Vérifier qu'une perte de connexion WebSocket réelle du VJoueur fait apparaître
l'icône côté admin, dans un délai raisonnable (quelques secondes).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | VJoueur "Alice" connecté (suite du Scénario 1) | — | | |
| 2 | Fermer complètement l'onglet/l'appareil du VJoueur "Alice" (pas juste le mettre en veille) | Connexion WebSocket coupée côté serveur | | |
| 3 | Observer `GamePage` pendant quelques secondes | L'icône de déconnexion apparaît sur "Alice", même style visuel qu'un buzzer physique déconnecté | | |
| 4 | Observer `TeamsPage` | L'icône de déconnexion apparaît également sur "Alice" | | |
| 5 | Noter le délai approximatif entre la fermeture et l'apparition de l'icône | Délai raisonnable (quelques secondes, cohérent avec le timeout WebSocket) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Reconnexion rapide : pas de flash parasite

**Objectif** : Vérifier qu'une reconnexion rapide du même VJoueur (nouvelle connexion WS avant
expiration de l'ancienne) ne provoque pas d'affichage "déconnecté" parasite (garde anti-zombie).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | VJoueur "Bob" connecté et enrôlé | Aucune icône affichée | | |
| 2 | Rafraîchir immédiatement l'onglet du VJoueur "Bob" (F5, reconnexion quasi instantanée) | Nouvelle connexion WS établie sous la même identité "Bob" | | |
| 3 | Observer `GamePage` en continu pendant la manipulation | **Aucun flash** de l'icône de déconnexion sur "Bob" (ou flash imperceptible < 1s) | | |
| 4 | Observer `TeamsPage` en continu pendant la manipulation | Idem : pas de flash visible | | |
| 5 | Une fois la reconnexion stabilisée | Aucune icône affichée sur "Bob" | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Non-régression : buzzer physique

**Objectif** : Vérifier que le comportement de déconnexion/reconnexion d'un buzzer physique
n'est pas affecté par ce fix (le hub VJoueur et le hub buzzer sont séparés, mais à valider
fonctionnellement).

**Prérequis** : Au moins un buzzer BuzzClick physique disponible.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Connecter un buzzer physique, vérifier sa présence dans `GamePage`/`TeamsPage` | Aucune icône de déconnexion | | |
| 2 | Éteindre/débrancher le buzzer | Icône de déconnexion apparaît après le délai habituel, comportement inchangé par rapport à avant le fix | | |
| 3 | Rallumer/reconnecter le buzzer rapidement | Pas de flash parasite (comportement déjà existant, doit rester identique) | | |
| 4 | Vérifier qu'aucune icône "VJoueur déconnecté" distincte n'apparaît sur un buzzer physique | Le libellé/tooltip reste "Buzzer déconnecté" générique, inchangé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Non-régression : clients admin/TV

**Objectif** : Vérifier qu'aucune régression n'affecte le comptage ou le comportement des
clients admin/TV (hub WebSocket partagé avec les VJoueurs).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir plusieurs onglets admin et un affichage TV (`/tv`) en parallèle d'un ou plusieurs VJoueurs | Comptage de connexions correct (admin/tv/vplayer) | | |
| 2 | Connecter/déconnecter un VJoueur pendant que admin/TV restent ouverts | Aucun impact sur l'affichage admin/TV (pas de déconnexion parasite, pas d'erreur console) | | |
| 3 | Fermer un onglet admin ou TV | Comptage mis à jour normalement, aucune icône de déconnexion erronée n'apparaît sur les VJoueurs ou buzzers | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénario 1 : aucune icône affichée à la connexion d'un VJoueur
- [ ] Scénario 2 : icône affichée après déconnexion réelle, sur `GamePage` ET `TeamsPage`
- [ ] Scénario 3 : aucun flash parasite lors d'une reconnexion rapide
- [ ] Scénario 4 : comportement des buzzers physiques strictement inchangé
- [ ] Scénario 5 : aucune régression sur le comptage/comportement admin/TV
- [ ] Le style visuel de l'icône VJoueur déconnecté est identique à celui du buzzer physique
      (pas de nouveau libellé "VJoueur déconnecté" introduit — hors scope)
- [ ] Scénario 6 : les 3 couleurs de badge (orange/rouge/vert) s'affichent correctement
- [ ] Scénario 7 : les compteurs Navbar `vjoueur`/`buzzer` (X/Y) et leur coloration sont corrects
- [ ] Scénario 8 : un bumper non assigné à une équipe n'affiche jamais de badge et n'est jamais compté
- [ ] Scénario 9 : l'écran TV (`/tv`) n'affiche aucun badge ni compteur de connexion (périmètre clos)

---

## Ajout — Batch 1 (25/07/2026) : badge 4 états + compteurs Navbar

> Complète les scénarios 1-5 ci-dessus (toujours valides) suite à l'extension du plan
> (`_work/reports/planner-20260725-105503-final.md`) : badge de connexion à 4 états
> (caché/orange/rouge/vert) mutualisé dans `ConnectionBadge.jsx`, filtre "participants
> uniquement" (assignés à une équipe), et compteurs Navbar `vjoueur`/`buzzer` au format X/Y
> avec coloration par sévérité. **Écran TV : aucun changement (périmètre clos).**

### Scénario 6 — Les 3 couleurs de badge (orange / rouge / vert)

**Objectif** : Vérifier que le badge de connexion distingue bien 3 situations, pas seulement
"connecté/déconnecté".

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Déconnecter un VJoueur ou un buzzer participant (sans qu'aucun message ne lui soit destiné pendant la coupure) | Badge **orange** | | |
| 2 | Pendant la coupure, déclencher un envoi qui lui est destiné (ex. LED_SET pour un buzzer, ou un changement d'état de jeu pour un VJoueur) | Badge passe au **rouge** | | |
| 3 | Reconnecter le bumper | Badge passe au **vert** brièvement | | |
| 4 | Attendre quelques secondes sans provoquer de nouvelle coupure | Badge disparaît (retour à l'état caché) | | |
| 5 | Re-déconnecter le bumper pendant qu'il est encore vert (juste après reconnexion) | Badge repasse à **orange** (pas de retour direct à caché) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Compteurs Navbar : format X/Y et coloration

**Objectif** : Vérifier les compteurs `vjoueur` et `buzzer` de la Navbar admin (format
connectés/participants) et leur coloration par sévérité.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Assigner 2 VJoueurs et 2 buzzers physiques à des équipes, tous connectés | Navbar affiche `vjoueur 2/2` et `buzzer 2/2`, sans coloration | | |
| 2 | Déconnecter un des VJoueurs (sans message perdu) | Compteur `vjoueur` passe à `1/2`, chip orange | | |
| 3 | Provoquer une perte de message pour ce même VJoueur | Chip `vjoueur` passe au rouge (le compteur reste `1/2`) | | |
| 4 | Reconnecter le VJoueur | Chip redevient neutre une fois l'état stabilisé, compteur revient à `2/2` | | |
| 5 | Vérifier les compteurs `admin`/`tv` pendant toute la manipulation | Inchangés (nombre brut, jamais colorés) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Filtre "participants uniquement"

**Objectif** : Vérifier qu'un bumper non assigné à une équipe (buzzer qui vient de faire HELLO
mais pas encore glissé dans une équipe, ou VJoueur en attente d'assignation) n'affiche jamais de
badge et n'entre jamais dans les compteurs Navbar, quel que soit son état de connexion réel.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Connecter un buzzer physique sans l'assigner à une équipe | Aucun badge, non compté dans `buzzer X/Y` | | |
| 2 | Déconnecter ce même buzzer non assigné | Toujours aucun badge, compteur Navbar inchangé | | |
| 3 | Assigner le buzzer (toujours déconnecté) à une équipe | Le badge orange apparaît immédiatement, le compteur `buzzer` passe à son total +1 | | |
| 4 | Retirer l'assignation d'équipe d'un bumper actuellement en badge rouge/orange | Le badge disparaît immédiatement (retour à l'état caché), sorti des compteurs | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 9 — Non-régression écran TV (périmètre clos)

**Objectif** : Confirmer qu'aucun badge ni compteur de connexion n'apparaît sur l'écran TV
(décision finale utilisateur — périmètre TV clos à zéro changement).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/tv` en vue PLAYERS pendant qu'un ou plusieurs bumpers sont déconnectés (orange/rouge) | Aucune icône de déconnexion sur les avatars joueurs | | |
| 2 | Observer l'ensemble des vues TV (scores, PLAYERS, etc.) | Aucun compteur `vjoueur`/`buzzer` ou équivalent affiché nulle part sur `/tv` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

---

## Correction Scénario 15 — Purge roster VJoueur (v5.7.20)

**Scénario original (obsolète)** : Purge à la réouverture d'un enrôlement (`StartEnrollment`).

**Design corrigé** : La purge des VJoueurs déconnectés (et connectés) a **lieu sur `InitGame`/`NEW_GAME`** (inconditionnelle), **pas sur `StartEnrollment`**. Clarification produit : une partie démarre toujours avec un roster VJoueur vierge — il n'existe pas de "VJoueur legacy" à purger à la réouverture d'un enrôlement (qui peut légitimement être rouvert **en cours de partie**, sans évincer un joueur actif temporairement coupé).

### Scénario 15 (corrigé) — Purge à InitGame/NEW_GAME

**Objectif** : Vérifier que la purge du roster VJoueur libère les noms pour la prochaine session.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Session 1 active : enrôler 2 VJoueurs ("Alice", "Bob"), les assigner à des équipes, puis terminer la partie | VJoueurs "Alice" et "Bob" en jeu | | |
| 2 | Démarrer une nouvelle partie (`InitGame`/`NEW_GAME`) | Message `ENROLLMENT_UPDATE` envoyé, indiquant purge du roster | | |
| 3 | Observer la console server (`log` ou debug) ou vérifier le modele Bumper : tous les VJoueurs doivent avoir été supprimés | VJoueurs "Alice" et "Bob" supprimés du roster ; aucun bumper fantôme restant | | |
| 4 | Enrôler à nouveau un VJoueur nommé "Alice" | Accepté sans conflit — le nom "Alice" est libre après purge | | |
| 5 | Vérifier qu'aucun score/équipe legacy de la session 1 n'est préservé pour ce nouvel "Alice" | "Alice" débute avec score 0, aucune équipe assignée | | |
| 6 | Vérifier qu'un buzzer physique enrôlé en session 1 n'a **pas** été purgé | Buzzer physique toujours présent, équipe et score intacts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Ajout — Fix R1 (Identité par ID) — Scénarios 10-15

> Complète les scénarios 1-9 ci-dessus (toujours valides) suite au fix R1 de la branche
> (`_work/reports/dev-backend-20260725-...-r1-fix.md`) : identité de reconnexion par ID
> (localStorage côté VJoueur) élimine tout risque de perte/fusion de données sur collision
> de nom. Nouveaux codes de rejet : `PLAYER_REJECTED` avec raison `NAME_TAKEN` si nom en
> conflit. **Tous les scénarios 10-15 couverts par tests automatisés** — procédure manuelle
> utile pour les futurs smoke tests QUALIF/PROD.

### Scénario 10 — Reconnexion par ID (même appareil)

**Objectif** : Vérifier qu'un VJoueur reconnaît son identité par ID (localStorage) et préserve automatiquement équipe/score sans créer de nouveau bumper.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Enrôler un VJoueur "Alice" sur un appareil mobile (note l'ID généré, stocké en localStorage) | VJoueur reçoit un ID unique (ex: `f8d7c6b5-...`) | | |
| 2 | Assigner "Alice" à l'équipe "Les Rouges", lui attribuer 10 points | Score visible dans l'admin : Alice 10 pts, équipe Les Rouges | | |
| 3 | Rafraîchir l'appareil mobile (F5 ou fermer/rouvrir l'onglet) | Nouvelle connexion WebSocket, `PLAYER_CONNECT` envoyé avec ID + nom "Alice" | | |
| 4 | Observer l'admin : vérifier qu'"Alice" n'apparaît qu'**une seule fois** (pas de doublon) | Aucun second bumper "Alice" créé ; équipe Les Rouges, score 10 pts **préservés** | | |
| 5 | Observer la console du VPlayer après reconnexion | Message `PLAYER_CONNECTED` reçu (pas `PLAYER_REJECTED`), ID et équipe confirmés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 11 — Rejet si nom déjà pris (VJoueur connecté)

**Objectif** : Vérifier qu'un second VJoueur utilisant le même nom "Alice" est rejeté si le premier est encore connecté.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Session active : VJoueur "Alice" connecté sur appareil A | "Alice" visible dans l'admin | | |
| 2 | Ouvrir un second navigateur/appareil (appareil B), tenter d'enrôler un nouveau VJoueur "Alice" | Nouveau VJoueur "Alice" tente `PLAYER_CONNECT` avec un ID différent | | |
| 3 | Observer l'appareil B | Écran bloquant `PLAYER_REJECTED` affiché : "Le nom Alice est déjà utilisé" (ou équivalent) | | |
| 4 | Observer l'admin | Un seul "Alice" affiché (celui de l'appareil A), aucun bumper créé/fusionné/supprimé | | |
| 5 | Attendre 3 secondes ou cliquer "Rejoindre à nouveau" | Redirection auto vers `/enroll`, possibilité de choisir un autre nom | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 12 — Rejet si nom déjà pris (VJoueur déconnecté)

**Objectif** : Vérifier qu'un nom en conflit reste protégé même si le VJoueur original est déconnecté.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | VJoueur "Alice" enrôlé et assigné à l'équipe "Les Rouges" (score 10 pts) | Alice visible dans l'admin | | |
| 2 | Déconnecter "Alice" (fermer l'onglet du VJoueur, réseau coupé) | Badge orange apparaît sur Alice côté admin | | |
| 3 | Ouvrir un second appareil, tenter d'enrôler un nouveau VJoueur "Alice" | Nouveau VJoueur "Alice" avec un ID différent tente `PLAYER_CONNECT` | | |
| 4 | Observer l'appareil en enrôlement | Écran bloquant `PLAYER_REJECTED` : "Le nom Alice est déjà utilisé" | | |
| 5 | Observer l'admin | Toujours un seul "Alice" (déconnecté, badge orange), score et équipe intacts, **aucune duplication ni fusion** | | |
| 6 | Retour sur l'appareil de l'Alice déconnecté : reconnecter (F5) | "Alice" se reconnecte par ID, badge disparaît, score/équipe restaurés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 13 — ID périmé → écran d'erreur + redirection

**Objectif** : Vérifier qu'un VJoueur avec un ID obsolète (bumper supprimé/purgé ou ID non résolu) reçoit un écran d'erreur clair et une redirection auto.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Enrôler VJoueur "Charlie" sur l'appareil A (note l'ID) | "Charlie" enrôlé, localStorage contient l'ID | | |
| 2 | Purger le roster (démarrer une nouvelle partie `InitGame`/`NEW_GAME`) | "Charlie" supprimé du roster | | |
| 3 | Reconnecter l'appareil A du VJoueur "Charlie" (utilise l'ID obsolète stocké localement) | VJoueur envoie `PLAYER_CONNECT` avec ID périmé | | |
| 4 | Observer l'appareil A | Écran bloquant `PLAYER_REJECTED` s'affiche immédiatement | | |
| 5 | Observer le message d'erreur | Message clair (ex: "Votre session a expiré" ou "Le joueur a été supprimé") | | |
| 6 | Attendre 3 secondes sans action | Redirection auto vers `/enroll` (possibilité d'une nouvelle inscription) | | |
| 7 | Cliquer le bouton "Rejoindre à nouveau" (s'il existe) | Navigation manuelle vers `/enroll` possible également | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 14 — EnrollPage attend réponse serveur (pas de faux départ)

**Objectif** : Vérifier que `EnrollPage` n'affiche pas prématurément le message de succès avant la réponse du serveur (aucune navigation optimiste).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la page `/enroll` | Formulaire d'enrôlement affiché | | |
| 2 | Saisir un nom "David" et cliquer "Valider" | Bouton désactivé/affichage "En attente de réponse..." | | |
| 3 | Observer la barre d'adresse/URL | Pas de navigation vers `/player` avant la réponse | | |
| 4 | Sur une connexion lente (ou simulée), attendre la réponse du serveur | Navigation vers `/player` intervient **après** réception de `PLAYER_CONNECTED` (pas avant) | | |
| 5 | En cas de rejet serveur (ex: `PLAYER_REJECTED`), vérifier que le formulaire redevient actif | Possibilité de ressaisir un autre nom sans rechargement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Notes QA

[Espace pour observations, capture d'écran, timing précis observé, version du binaire testé, date de test]
