# Procédure de Test — Crédit synchronisé entre animateurs (#170)

**Version** : v6.2.x (branche `feature/anim-question-display`)
**Date** : 2026-08-16
**Testeur** : QA
**Issue** : #170 — crédit synchronisé, verrouillage du double-crédit, refus à 0 point
**Référence** : Plan `_work/reports/plan-20260816-125123.md` §5, contrat
`contracts/websocket-actions.md` §"Animateur" AWARDED_TEAMS

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] **Deux navigateurs/onglets `/anim`** ouverts simultanément (deux "tablettes"), plus `/admin`
      sur un troisième poste
- [ ] Un quiz avec au moins une question SPEEDY et une QCM
- [ ] Au moins 2 équipes actives (bumpers assignés), dont une avec un bumper ayant buzzé (SPEEDY)

---

## Scénario 1 — Verrouillage croisé entre deux tablettes

**Objectif** : Vérifier qu'un crédit posé sur une tablette verrouille immédiatement l'autre.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir `/anim` sur les tablettes A et B, charger et arrêter une question SPEEDY (STOPPED) sur les deux | Les deux tablettes affichent les gestes "+N pts" et "0 pt" pour chaque équipe | | |
| 2 | Sur la tablette A, créditer une équipe ("+N pts") | La ligne se verrouille sur A : affiche "✓ +N pts", plus de bouton | | |
| 3 | Observer la tablette B **sans rien faire** | La ligne se verrouille **immédiatement** sur B aussi, même montant affiché | | |
| 4 | Tenter de créditer à nouveau cette équipe depuis B | Impossible — aucun bouton cliquable sur cette ligne | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Le refus (0 pt) verrouille comme un crédit

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur une question STOPPED, cliquer "0 pt" pour une équipe depuis la tablette A | La ligne se verrouille sur A : affiche "✓ 0 pt" (PAS "+0 pts") | | |
| 2 | Observer la tablette B | Même verrouillage immédiat, "✓ 0 pt" affiché | | |
| 3 | Vérifier le score de l'équipe refusée avant/après | **Score inchangé** | | |
| 4 | Observer les LED des buzzers de l'équipe refusée | **Aucune LED de victoire (comet)** ne s'allume | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Crédit depuis la régie

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur une question STOPPED, créditer une équipe depuis `/admin` | Crédit effectué normalement sur `/admin` (aucune restriction) | | |
| 2 | Observer les tablettes A et B | L'équipe se verrouille sur **les deux** tablettes, montant affiché correspond à celui crédité par la régie | | |
| 3 | Recréditer la même équipe depuis `/admin` (la régie n'a pas de garde) | `/admin` l'autorise sans restriction | | |
| 4 | Observer à nouveau les tablettes A et B | Le montant affiché est la **somme** des deux crédits régie | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — SPEEDY : crédit via un joueur verrouille l'équipe

**Objectif** : Vérifier le regroupement par équipe (TeamName), pas par bumper (WinnerID) — le
chemin nominal SPEEDY.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question SPEEDY, un joueur a buzzé le premier de son équipe | Le crédit "+N pts" cible ce joueur (cible PLAYER par défaut) | | |
| 2 | Créditer depuis la tablette A | La ligne de L'ÉQUIPE (pas seulement du joueur) se verrouille | | |
| 3 | Observer la tablette B | Équipe verrouillée également, malgré un crédit ciblant un bumper individuel | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Rejouer une question déjà créditée

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créditer une équipe sur une question, l'observer verrouillée | Verrou actif | | |
| 2 | Depuis `/admin`, relancer la MÊME question (LANCER à nouveau) | Question repart en STARTED | | |
| 3 | Arrêter la question à nouveau (STOPPED) | Retour en STOPPED | | |
| 4 | Observer l'équipe précédemment créditée | **Le crédit redevient possible** — la ligne n'est plus verrouillée (le verrou de la partie précédente ne doit pas persister) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Connexion tardive d'une troisième tablette

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avec A et B déjà connectées et une équipe déjà créditée | État verrouillé sur A et B | | |
| 2 | Ouvrir `/anim` sur une TROISIÈME tablette (C), après coup | C affiche immédiatement l'état à jour : l'équipe créditée apparaît verrouillée dès le chargement, **sans action de sa part** | | |
| 3 | Vérifier qu'aucun `localStorage`/`sessionStorage` n'est utilisé (outils dev) | Aucune trace de persistance locale du verrouillage — l'état vient uniquement du serveur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Un refus n'apparaît jamais au PALMARÈS

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Refuser (0 pt) au moins une équipe sur une question, créditer normalement une autre équipe sur une autre question | Deux événements enregistrés | | |
| 2 | Consulter le PALMARÈS (TV ou `/admin`) | Seule l'équipe créditée positivement apparaît — l'équipe refusée est absente | | |
| 3 | Consulter l'historique détaillé de la régie (`/admin` → historique) | La ligne "+0 pts" du refus **est visible** — trace assumée (arbitrage C1), distincte d'une équipe jamais examinée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Non-régression

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go build ./...` puis `go test ./... -race` | Build OK, tous les tests PASS | | |
| 2 | `npm test` (suite React complète) | Tous les tests PASS, y compris `AnimCreditControl.test.jsx`, `useWebSocket.awardedTeams.test.js` | | |
| 3 | Manche SPEEDY complète sur `/anim` (hors scénarios de double-tablette) | Cible et montant du crédit identiques à avant #170 | | |
| 4 | Manche QCM avec indices, vérifier le montant par équipe (calcQcmTeamAward) | Montant inchangé (#157) | | |
| 5 | `/admin` : créditer et recréditer librement | Aucune restriction nouvelle | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Crédit depuis une tablette verrouille toutes les tablettes (Scénario 1)
- [ ] Refus (0 pt) verrouille comme un crédit, aucune LED, score inchangé (Scénario 2)
- [ ] Crédit régie verrouille les tablettes, sommé si recrédité (Scénario 3)
- [ ] Crédit SPEEDY via un joueur verrouille l'équipe entière (Scénario 4)
- [ ] Rejouer une question déjà créditée libère le verrou (Scénario 5)
- [ ] Connexion tardive reçoit l'état à jour sans stockage local (Scénario 6)
- [ ] Un refus n'apparaît jamais au PALMARÈS mais reste dans l'historique régie (Scénario 7)
- [ ] Aucune régression SPEEDY/QCM/`/admin` (Scénario 8)

---

## Notes QA

[Espace pour observations]
