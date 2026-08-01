# Procédure de Test — Notification d'éviction absente et motif erroné (#123)

**Version** : à définir (branche `bugfix/vjoueur-evicted-notification`)
**Date** : 2026-07-30
**Branche** : bugfix/vjoueur-evicted-notification (empilée sur bugfix/vjoueur-reconnect-mismatch,
#120 + #118)
**Testeur** : QA

---

## Contexte du Bug

Supprimer un VJoueur connecté depuis l'écran d'administration ne produisait **aucune**
notification visible sur son téléphone (rien, pas même un écran d'attente), et un rechargement
ultérieur affichait à tort « Une nouvelle partie a commencé », alors qu'aucune partie n'avait été
relancée.

**Cause racine (plan `_work/reports/plan-20260730-094500.md`)** — trois défauts :

- **A1** : l'écran d'administration (`TeamsPage.jsx`) ne supprime jamais un joueur via l'action
  `DELETE_BUMPER` (celle qui notifie, corrigée en #120) — il envoie un `UPDATE` générique portant
  un roster amputé. Le mécanisme de notification de #120 était donc correct mais branché sur une
  action que personne n'envoie.
- **A2** : `VPlayerPage.jsx` ne remettait jamais l'état React `bumper` à `null` quand celui-ci
  disparaissait du roster — l'écran continuait d'afficher un joueur périmé, et le filet de
  sécurité `SESSION_EXPIRED` de #118 restait neutralisé (il garde sur `bumper` resté « vrai » pour
  toujours).
- **B** : un ID périmé alors que les inscriptions sont fermées était systématiquement traduit en
  « nouvelle partie », qu'il s'agisse réellement d'une purge ou d'une simple suppression
  individuelle.

**Fix attendu** :
- Le serveur notifie désormais sur **constat** de disparition du roster (peu importe l'action
  responsable), en plus de corriger l'action réellement utilisée par l'admin.
- L'écran du joueur cesse d'afficher un bumper périmé.
- Un registre serveur mémorise brièvement (~1h) le motif de disparition, pour répondre
  correctement à une reconnexion ultérieure au lieu de deviner.

**Les textes des bandeaux ne changent pas** (validés au GATE 2 de #120) — seule la
**correspondance** entre la situation réelle et le motif affiché est corrigée.

Maquette de référence (matrice scénario → motif) :
https://claude.ai/code/artifact/cc6ff96e-a907-4035-b99d-73c82bbdb47a

---

## Prérequis

- [ ] Environnement : QUALIF (serveur accessible sur le réseau local)
- [ ] Au moins 2 VJoueurs inscrits (téléphones réels ou onglets séparés)
- [ ] Accès admin (`/game` pour la partie, `/teams` pour la gestion des joueurs)
- [ ] Un buzzer physique déjà appairé, pour le scénario de non-régression

---

## Scénario 1 — Reproduction exacte de l'issue

**Objectif** : Vérifier que la suppression d'un VJoueur connecté affiche immédiatement le bon
motif, et que ce motif reste cohérent après un rechargement de page.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un VJoueur (« Alice ») est inscrit et en jeu | Écran de jeu normal | | |
| 2 | Depuis l'écran d'administration (`/teams`), cliquer sur le bouton « × » pour supprimer Alice, confirmer | **Immédiatement**, sur le téléphone d'Alice : bandeau « Ta place a été libérée par l'animateur. Tu peux te réinscrire. » — pas un écran vide, pas rien | | |
| 3 | Attendre le retour automatique à l'inscription | Retour à l'écran d'inscription, avec le même bandeau affiché au-dessus du formulaire | | |
| 4 | **Recharger la page** d'inscription (F5) | Le motif affiché (s'il réapparaît via un lien direct rechargé pendant la fenêtre de lecture) reste cohérent — **jamais** « Une nouvelle partie a commencé » | | |
| 5 | Se réinscrire avec le même pseudo « Alice » | Accepté sans `NAME_TAKEN` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Suppression subie hors ligne

**Objectif** : Vérifier qu'un joueur déconnecté au moment de sa suppression reçoit le bon motif à
son retour (et non un `ENROLLMENT_CLOSED` générique ni une déduction erronée).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un VJoueur (« Bob ») est inscrit et en jeu | Écran de jeu normal | | |
| 2 | Couper le réseau du téléphone de Bob (WiFi coupé) | Le badge de connexion admin passe orange/rouge | | |
| 3 | Pendant que Bob est hors ligne, le supprimer depuis `/teams` | Aucun effet visible immédiat côté Bob (hors ligne) | | |
| 4 | Rétablir le réseau de Bob | À la reconnexion : bandeau « Ta place a été libérée par l'animateur. Tu peux te réinscrire. » — **pas** « Une nouvelle partie a commencé » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Nouvelle partie (non-régression)

**Objectif** : Vérifier qu'une vraie purge `NEW_GAME` continue d'afficher le bon motif, en direct
comme après une déconnexion.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Au moins 2 VJoueurs inscrits et en jeu, l'un d'eux hors ligne (WiFi coupé) | Un joueur en ligne, un hors ligne | | |
| 2 | Lancer une nouvelle partie (NEW_GAME) depuis l'admin | Le joueur en ligne voit immédiatement « Une nouvelle partie a commencé » | | |
| 3 | Rétablir le réseau du joueur hors ligne | À sa reconnexion : même motif « Une nouvelle partie a commencé » — pas de confusion avec une suppression individuelle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Session périmée sans suppression (sixième cas de la maquette)

**Objectif** : Vérifier qu'un retour avec une session périmée, **hors de tout contexte de
suppression**, alors que les inscriptions sont fermées, affiche le motif littéral et non une
déduction erronée.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Simuler une session VJoueur périmée n'ayant jamais existé côté serveur (ex. `localStorage` manuellement altéré, ou un très vieil onglet jamais reconnecté), inscriptions actuellement fermées | — | | |
| 2 | Ouvrir/recharger la page VJoueur avec cette session | Bandeau « Les inscriptions sont fermées » (sens littéral) — **jamais** « Une nouvelle partie a commencé » | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Suppression d'un buzzer physique (non-régression)

**Objectif** : Vérifier qu'aucune notification VJoueur n'est déclenchée par la suppression d'un
buzzer physique.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un buzzer physique et un VJoueur sont tous deux connectés | État normal des deux | | |
| 2 | Supprimer le buzzer physique depuis `/teams` | Le buzzer disparaît de la liste ; **aucun effet** sur l'écran du VJoueur (pas de bandeau, pas de renvoi) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Une suppression individuelle affiche **immédiatement** le bon bandeau (scénario 1)
- [ ] Le motif reste cohérent après rechargement — jamais « nouvelle partie » à tort (scénario 1)
- [ ] Un joueur supprimé hors ligne reçoit le bon motif à son retour (scénario 2)
- [ ] Une vraie purge `NEW_GAME` affiche toujours « nouvelle partie », en direct et après
      déconnexion (scénario 3)
- [ ] Une session périmée hors suppression, inscriptions fermées, affiche le motif littéral
      (scénario 4)
- [ ] La suppression d'un buzzer physique n'a aucun effet sur les VJoueurs (scénario 5)
- [ ] Un joueur supprimé peut se réinscrire avec le même pseudo sans `NAME_TAKEN` (scénario 1)

## Notes QA

[Espace pour observations]
