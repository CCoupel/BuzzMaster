# Procédure de Test — Reprise de place assistée par l'animateur (#122, couvre #124)

**Version** : à définir (branche `bugfix/vplayer-name-recovery`)
**Date** : 2026-07-30
**Branche** : bugfix/vplayer-name-recovery (empilée sur bugfix/vjoueur-evicted-notification,
#120 + #118 + #123)
**Testeur** : QA

---

## Contexte du Bug

Depuis #109, l'identité d'un VJoueur repose uniquement sur son ID de bumper. Un téléphone qui
perd cet ID (stockage vidé, changement d'appareil, rejet antérieur) ne peut plus jamais prouver
que sa place lui appartient : toute tentative sous le même pseudo tombe sur « Ce pseudo est déjà
utilisé, choisis-en un autre » — **le pire conseil possible** face à sa propre place, puisqu'il
pousse le joueur à en changer et à abandonner son score. Seule une suppression manuelle par
l'animateur débloquait la situation, sans que rien ne le lui signale.

**Fix attendu** (plan `_work/reports/plan-20260730-123000.md`) :
- Le message distingue désormais un homonyme connecté (« choisis-en un autre », inchangé) d'une
  reprise probable de sa propre place (« Cette place est peut-être la tienne. Demande à
  l'animateur de te la rendre... »).
- Sa fiche se signale côté animateur (« place demandée ») immédiatement après la tentative
  échouée — jamais pour une fiche simplement déconnectée.
- L'animateur dispose de **deux actions distinctes** : **Réinscription** (le joueur retrouve son
  score et son équipe, autorisation à usage unique) ou **Suppression totale** (réutilise #123 —
  score et équipe perdus, place libérée).
- **Piège à vérifier explicitement** : la suppression totale ne rend la place utilisable que si
  les inscriptions sont ouvertes ; sinon le joueur ne peut pas revenir avant leur réouverture. La
  fiche doit avertir de ce cas.

Maquette de référence : https://claude.ai/code/artifact/9fced780-fb98-40c3-9816-15939eec2a89

---

## Prérequis

- [ ] Environnement : QUALIF (serveur accessible sur le réseau local)
- [ ] Un VJoueur (téléphone réel ou onglet séparé) pouvant être inscrit, avec un score non nul et
      une équipe assignée
- [ ] Accès admin (`/teams` pour la gestion des joueurs, `/game` pour la partie)
- [ ] Pouvoir effacer le stockage local du téléphone/onglet du VJoueur (Réglages navigateur →
      Effacer les données du site, ou navigation privée pour simuler un nouvel appareil)

---

## Scénario A — Réinscription : le score et l'équipe sont conservés

**Objectif** : Reproduire le scénario original #122/#124 et vérifier que la réinscription
restitue la place avec son historique.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un VJoueur (« Emma ») est inscrit, assigné à une équipe, avec un score non nul (ex. via quelques buzz gagnants) | Écran de jeu normal, score visible côté admin | | |
| 2 | Effacer le stockage local du téléphone d'Emma (ou passer en navigation privée) | Le stockage `vplayer_id` est perdu | | |
| 3 | Rouvrir la page d'inscription et ressaisir le pseudo « Emma » | Message : « Cette place est peut-être la tienne. Demande à l'animateur de te la rendre, puis réessaie — tu retrouveras ton score. » — **pas** « choisis-en un autre » | | |
| 4 | Vérifier côté admin (`/teams`) | La fiche d'Emma affiche la pastille « place demandée » et **deux** boutons : « Réinscription » et « Suppression totale », chacun avec sa conséquence indiquée | | |
| 5 | Cliquer sur **Réinscription**, confirmer (le nom d'Emma doit apparaître dans la confirmation) | La pastille « place demandée » disparaît de la fiche | | |
| 6 | Sur le téléphone d'Emma, ressaisir le même pseudo « Emma » et valider | Accepté — Emma rejoint l'écran de jeu | | |
| 7 | Vérifier son score et son équipe | **Identiques** à ce qu'ils étaient avant l'étape 2 — rien n'a été perdu | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario B — Suppression totale : le score est perdu, la place dépend des inscriptions

**Objectif** : Vérifier l'autre action et le piège identifié par le plan.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Reproduire les étapes 1-4 du scénario A avec un autre joueur (« Léo »), inscriptions **fermées** au moment de la tentative | Fiche « place demandée » affichée pour Léo | | |
| 2 | Observer la fiche avant de cliquer | **L'avertissement contextuel est visible** : suppression totale empêchera Léo de revenir tant que les inscriptions ne sont pas rouvertes | | |
| 3 | Cliquer sur **Suppression totale**, confirmer (nom de Léo dans la confirmation) | Le bumper de Léo disparaît de l'admin et de la TV (notification #123 : bandeau « place libérée » si Léo était en train de regarder) | | |
| 4 | Sur le téléphone de Léo, tenter de se réinscrire avec le même pseudo, inscriptions toujours fermées | Rejeté : « Les inscriptions sont fermées » — **il ne peut pas revenir** | | |
| 5 | Rouvrir les inscriptions puis réessayer | Accepté normalement — **score reparti à zéro**, aucune équipe assignée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario C — Non-régressions

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Un second joueur tente un pseudo porté par un VJoueur **actuellement connecté** | Message inchangé : « Ce pseudo est déjà utilisé, choisis-en un autre » — aucune fiche ne se signale | | |
| 2 | Un VJoueur simplement déconnecté (coupure réseau normale, #118) sans qu'aucune reprise n'ait été tentée | Sa fiche n'affiche **aucune** des deux actions — uniquement le bouton « Supprimer » habituel, badge de connexion orange/rouge standard | | |
| 3 | Ce même joueur se reconnecte normalement (retour réseau) | Aucune fiche « place demandée » n'apparaît à aucun moment | | |
| 4 | Deux appareils tentent la reprise du même pseudo juste après une seule **Réinscription** | Un seul aboutit ; le second retombe sur un refus normal (le pseudo est de nouveau pris par un détenteur connecté) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Le message différencié apparaît pour une reprise probable (scénario A, étape 3)
- [ ] La fiche se signale immédiatement côté animateur, avec les deux actions et leurs
      conséquences annoncées (scénario A, étape 4)
- [ ] La réinscription conserve score et équipe, même en pleine partie (scénario A)
- [ ] La suppression totale perd le score et ne permet le retour que si les inscriptions sont
      ouvertes — l'avertissement est visible **avant** de cliquer (scénario B)
- [ ] Un homonyme connecté reçoit toujours le message inchangé (scénario C)
- [ ] Une fiche simplement déconnectée n'affiche jamais les deux actions (scénario C)
- [ ] Une seule reprise aboutit par autorisation accordée (scénario C)

## Notes QA

[Espace pour observations]
