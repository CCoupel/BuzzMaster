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

---

## Notes QA

[Espace pour observations, capture d'écran, timing précis observé, version du binaire testé]
