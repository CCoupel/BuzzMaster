# Procédure de Test — Audit broadcasts LED non ciblés (#132)

**Version** : 5.10.1+ (branche `bugfix/led-broadcast-audit`)
**Date** : 2026-08-04
**Branche** : bugfix/led-broadcast-audit
**Testeur** : QA

---

## Contexte

Pendant #127 et #129, le même défaut a été trouvé et corrigé deux fois : une fonction liée aux LED
se terminait par un `broadcastUpdate()` inconditionnel et non ciblé — renvoyant l'état complet à
tous les clients (VJoueur compris) même quand ce n'était pas nécessaire (le seul contenu réellement
neuf de ce broadcast, `ACK_PENDING`, n'est de toute façon jamais visible ni par TV ni par les
VJoueurs). #132 audite les 5 fonctions restantes de la même famille (`broadcastLEDSet`,
`sendLEDSetStop`, `sendLEDSetReveal`, `sendLEDSetToTeam`, `sendLEDSetComet`) et applique le même
correctif partout où le défaut était réellement présent.

**Ce changement est invisible pour un utilisateur normal** : aucune animation LED, aucun texte,
aucun comportement de jeu ne change. Seul le **volume réseau** vers TV/VJoueur diminue (un UPDATE
en moins à chaque STOP, REVEAL, et attribution de points avec effet COMET). La QA porte donc sur la
**non-régression fonctionnelle** des LED elles-mêmes, pas sur un nouveau comportement visible.

Rapport d'audit détaillé : `_work/reports/dev-backend-132-audit-20260804.md`.

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `bugfix/led-broadcast-audit`)
- [ ] Au moins 1 buzzer physique connecté (ou simulateur firmware), assigné à une équipe
- [ ] Accès admin (`/game`), accès `/tv`, au moins 1 onglet `/player`
- [ ] Une question QCM et une question NORMAL/points disponibles
- [ ] Accès aux logs serveur / outils réseau (onglet Network du navigateur sur `/tv` et `/player`)

---

## Scénario 1 — STOP : LED buzzer + absence de régression

**Objectif** : `sendLEDSetStop` (appelée par `broadcastStop`) doit toujours allumer les LED des
buzzers physiques en couleur d'équipe, et TV/VJoueur ne doivent plus recevoir l'UPDATE redondant qui
suivait (à vérifier via l'onglet Network, pas visuellement — c'est le point que ce lot change).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Partie en cours, cliquer "Stop" depuis l'admin | Buzzers physiques passent en SOLID couleur d'équipe (inchangé) | | |
| 2 | Observer l'onglet Network de `/tv` et `/player` au moment du clic | Une seule frame `UPDATE` doit apparaître (celle de `broadcastStop` lui-même), pas deux | | |
| 3 | Observer l'onglet Network de `/game` (admin) | L'UPDATE supplémentaire (spinner ACK_PENDING) doit toujours arriver côté admin | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — REVEAL : LED de feedback QCM

**Objectif** : `sendLEDSetReveal` doit toujours donner le bon feedback lumineux (bonne/mauvaise
réponse) sur les buzzers physiques après une question QCM.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question QCM, faire répondre au moins un buzzer physique, révéler la réponse | Feedback LED correct (couleur bonne réponse / mauvaise réponse) sur les buzzers, inchangé | | |
| 2 | Observer l'onglet Network de `/tv` et `/player` au moment de la révélation | Une seule frame `UPDATE` liée à REVEAL, pas de doublon | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Attribution de points avec effet COMET

**Objectif** : `sendLEDSetComet` (4 chemins d'appel : POINTS, BUMPER_POINTS, TEAM_POINTS, MEMOTION
gagnante) doit toujours déclencher l'animation COMET sur les buzzers de l'équipe gagnante, ET le
score doit toujours apparaître correctement sur TV/VJoueur (c'est un AUTRE broadcast, non touché par
#132, qui porte le score — à vérifier qu'il n'a pas été perdu par erreur).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Attribuer des points à une équipe ayant un buzzer physique (bouton +points admin) | Animation COMET (2 tours, ~4.8s) sur les buzzers de l'équipe, puis retour à l'état normal | | |
| 2 | Observer TV et `/player` pendant/après l'attribution | Le nouveau score s'affiche normalement (porté par le broadcastUpdate() de la fonction appelante, pas celui de sendLEDSetComet) | | |
| 3 | Répéter avec MEMOTION (carte gagnante) si accessible | Même comportement : COMET sur buzzers de l'équipe gagnante, score à jour partout | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Non-régression générale

**Objectif** : Vérifier qu'aucun autre point de la partie n'est affecté (les 2 autres fonctions
auditées, `broadcastLEDSet` et `sendLEDSetToTeam`, sont du code mort en production — aucun scénario
utilisateur ne les exerce, rien à tester manuellement pour elles).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Dérouler une partie complète normale (enrôlement, PREPARE/READY, buzz, réponse, points, stop) | Comportement LED et déroulé de partie identiques à avant #132, de bout en bout | | |
| 2 | Vérifier l'absence d'erreur dans les logs serveur pendant toute la partie | Aucune erreur/panic liée à `sendLEDSet*` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Synthèse

| Scénario | Verdict |
|----------|---------|
| 1 — STOP | |
| 2 — REVEAL | |
| 3 — COMET / points | |
| 4 — Non-régression générale | |

**Verdict global** : [ ] VALIDATED  [ ] NOT VALIDATED

**Notes complémentaires** :
