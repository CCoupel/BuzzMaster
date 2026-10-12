# Procédure de Test — Sélecteur QCM : aucune réponse donnée (bug #255, v11.2)

**Version** : v11.2.x (QUALIF)
**Date** : 2026-10-12
**Testeur** : **l'utilisateur uniquement** (plusieurs équipes, tablettes/buzzers : ni `qa` ni `deployer` ne l'exécutent)
**Contrat** : `contracts/websocket-actions.md` §TEAM_POINTS_BATCH « QCM : clic sur une réponse »
**Tests automatiques** : `tests/INDEX.md` (ligne #255)

## Prérequis

- [ ] QUALIF, au moins 3 équipes avec un joueur chacune (tablette ou buzzer), un QCM disponible
- [ ] Idéalement une partie où un QCM précédent a déjà été joué (les joueurs gardent leur ancienne couleur de réponse)

## Scénario 1 — Personne ne répond

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Lancer un QCM (START), **personne ne répond**, STOP puis REVEAL | Sélecteur de réponses affiché | | |
| 2 | Lire le compte de chaque réponse | **« 0 équipe » pour les quatre réponses** (jamais « 3 équipes ») | | |
| 3 | Cliquer chaque réponse (bonne, puis mauvaises) | **Aucune** équipe sélectionnée ; « Attribuer les points (0) » / « Appliquer les pénalités (0) » désactivé | | |
| 4 | Même test sur un QCM réglé « Points : joueur » | Comptes « 0 joueur », aucune ligne cochée | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — Une équipe répond

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Nouveau QCM : **une seule équipe (A)** répond (ex. rouge), les autres non | | | |
| 2 | STOP, REVEAL, lire les comptes | Rouge : **1 équipe** ; les trois autres : 0 | | |
| 3 | Cliquer « rouge » | **Seule l'équipe A** est sélectionnée | | |
| 4 | Cliquer la bonne réponse (si A n'a pas la bonne) | Aucune sélection, bouton désactivé | | |
| 5 | Question suivante (QCM), aucune réponse | Comptes à **0** (l'ancienne couleur des joueurs n'est pas comptée) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Un joueur qui n'a pas répondu n'est jamais compté ni sélectionné
- [ ] Une réponse effective est comptée comme avant (y compris montants d'indices)
