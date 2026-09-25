# Procédure de Test — Forcer l'affichage « Jeu » au lancement d'une manche (#240)

**Version** : 11.1.0.14+ (build QUALIF)
**Date** : 2026-09-25
**Testeur** : Utilisateur (validation manuelle — navigateur réel requis)
**Issue** : #240 — **Plan** : `_work/handoff/plan-240-20260925-171000.md`

## Règle à valider

Le sélecteur d'affichage de la carte « TV » de `/admin` (**Jeu / Equipes / Joueurs / Palmarès**) est remis sur **Jeu** par le serveur :
- à la **sélection d'une question** (entrée en PREPARE) ;
- au clic **START** ;
- au **CONTINUER** après une PAUSE ;
- *(hypothèse du plan, non confirmée explicitement — à valider ici)* au départ d'une **carte MEMOTION** et au tirage d'une **carte RAFALE**.

Ce n'est **pas un verrou** : l'animateur peut ensuite rebasculer manuellement sur Equipes / Joueurs / Palmarès.
Pas de forçage sur PAUSE, REVEAL, STOP, retour automatique READY → PREPARE, NOUVELLE PARTIE.

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows), serveur démarré
- [ ] Onglets : `/admin` (carte « TV » visible), `/tv`, et **`/player` (VPlayer)** — idéalement côte à côte
- [ ] Une partie avec au moins 2 équipes, buzzers/VJoueurs connectés, un quiz avec plusieurs questions **QCM/SPEEDY**, une question **MEMORY**, une question **MEMOTION** (avec une carte RAFALE si possible)
- [ ] Repère : le **bouton actif** de la carte TV de `/admin` reflète l'affichage courant

## Scénarios

### Scénario 1 — TV laissée sur Equipes/Palmarès après un REVEAL, puis sélection de la question suivante (cas de l'issue)

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Jouer une question jusqu'au REVEAL | Réponse affichée sur `/tv` | | |
| 2 | Sur `/admin`, carte TV : cliquer **Equipes** | `/tv` affiche les scores ; bouton « Equipes » actif | | |
| 3 | Sélectionner la question suivante (clic dans la liste) | **`/tv` bascule seule sur le jeu** (« PRÉPAREZ-VOUS »), bouton **« Jeu »** devient actif sur `/admin`, **sans action** de l'animateur | | |
| 4 | `/player` (VPlayer) | Affiche aussi la vue de jeu (plus la vue Equipes) | | |
| 5 | Refaire les étapes 1-3 avec **Palmarès** puis **Joueurs** à la place de Equipes | Même résultat à chaque fois | | |
| 6 | Refaire avec le bouton **« à suivre »** au lieu du clic dans la liste | Même résultat | | |
| 7 | Sélectionner une question depuis la tablette **/anim** (si disponible) | Même résultat | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — START avec la TV remise sur les scores pendant la préparation

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sélectionner une question ; attendre **READY** (buzzers prêts) | TV sur le jeu (« PRÉPAREZ-VOUS »), bouton Jeu actif | | |
| 2 | Carte TV : cliquer **Joueurs** (ou Equipes / Palmarès) | La TV obéit (pas de verrou), bouton actif = celui choisi | | |
| 3 | Cliquer **START** | **La TV repasse seule sur le jeu** (compte à rebours puis question/chrono), bouton **Jeu** actif ; le VPlayer aussi | | |
| 4 | Recommencer avec chacune des 3 vues | Même résultat | | |
| 5 | Répéter avec une question **MEMORY** | Même résultat au START | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Le forçage n'est pas un verrou

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sélectionner une question (TV forcée sur Jeu) puis, **en PREPARE**, cliquer **Equipes** | La TV affiche les scores et **y reste** (pas de retour automatique) | | |
| 2 | Passer en READY (buzzers prêts) | La vue **Equipes reste** (READY ne re-force pas) | | |
| 3 | Cliquer START | Retour au jeu (forcé) | | |
| 4 | Pendant STARTED, cliquer **Palmarès** | La TV affiche le palmarès et **y reste** jusqu'à nouveau choix / prochaine sélection / START / CONTINUER | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — PAUSE / CONTINUER

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question en cours (STARTED), cliquer **PAUSE** avec la TV sur Jeu | Pas de changement d'affichage à cause de la pause | | |
| 2 | Pendant la pause, cliquer **Equipes** | La TV affiche les scores | | |
| 3 | Cliquer **CONTINUER** | **La TV repasse seule sur le jeu**, chrono reprend ; VPlayer idem | | |
| 4 | Recommencer avec Joueurs puis Palmarès | Même résultat | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — AUCUN forçage sur les transitions qui ne lancent pas de manche

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | STARTED, TV sur **Equipes**, cliquer **PAUSE** | La TV **reste** sur Equipes | | |
| 2 | Depuis STOPPED avec la TV sur Equipes, cliquer **REVEAL** | La TV **reste** sur Equipes (pas de retour au jeu) | | |
| 3 | STARTED, TV sur **Joueurs**, cliquer **STOP** | La TV **reste** sur Joueurs | | |
| 4 | READY, TV sur Equipes ; provoquer le retour automatique READY → PREPARE (retirer une équipe participante en MEMORY/MEMOTION SOLO) | La TV **reste** sur Equipes | | |
| 5 | Palmarès affiché, lancer **NOUVELLE PARTIE** | Pas de forçage dû à cette action (la vue reste celle choisie) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Cartes MEMOTION et tirage RAFALE *(hypothèse à confirmer)*

> Le plan propose de forcer aussi le départ d'une carte MEMOTION et le tirage RAFALE ; l'utilisateur ne l'a pas confirmé explicitement. Si le comportement observé ne convient pas, le noter en « Notes QA » : le test automatisé associé devra être ajusté.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une manche MEMOTION (grille de cartes affichée), TV passée sur **Equipes** | La TV reste sur Equipes (pas de forçage en simple affichage de la grille) | | |
| 2 | Sélectionner une carte de la grille | **La TV repasse sur le jeu** (la carte est visible des joueurs) ; bouton Jeu actif | | |
| 3 | Sur une carte **RAFALE**, retourner la carte, repasser la TV sur Equipes, puis lancer le tirage RAFALE | **La TV repasse sur le jeu** au tirage de la première question | | |
| 4 | Vérifier `/player` à chaque étape | Suit la TV | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — VPlayer et non-régression

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `/player` ouvert pendant les scénarios 1, 2 et 4 | Il suit systématiquement la vue forcée (jeu) | | |
| 2 | Buzzers physiques pendant une manche | Comportement inchangé (LEDs, buzz, aucun effet nouveau) | | |
| 3 | `/anim` pendant une manche | Inchangé (n'utilise pas ce sélecteur) | | |
| 4 | Recharger `/tv` (F5) pendant PREPARE puis STARTED | Affiche directement la vue courante correcte (jeu) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Après REVEAL puis Equipes/Joueurs/Palmarès, la sélection de la question ramène seule la TV sur Jeu (scénario 1)
- [ ] Le START ramène la TV sur Jeu depuis les 3 vues (scénario 2)
- [ ] CONTINUER après PAUSE ramène la TV sur Jeu (scénario 4)
- [ ] Le forçage n'est jamais un verrou (scénario 3)
- [ ] Aucun forçage sur PAUSE, REVEAL, STOP, retour READY → PREPARE, NOUVELLE PARTIE (scénario 5)
- [ ] Le bouton « Jeu » de `/admin` s'active en même temps que la TV
- [ ] VPlayer conforme ; buzzers et /anim inchangés
- [ ] Comportement MEMOTION/RAFALE (hypothèse) validé ou remonté

## Notes QA

[Espace pour observations]
