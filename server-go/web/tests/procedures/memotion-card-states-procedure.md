# Procédure de Test — États Visuels MEMOTION (Bugfix #77)

**Version** : 5.1.x  
**Date** : 2026-05-13  
**Testeur** : QA  
**Issue** : #77 — Correction des 5 états visuels des cards MEMOTION sur l'affichage TV

---

## Prérequis

- [ ] Environnement : LOCAL ou QUALIF
- [ ] Serveur BuzzControl démarré (version post-bugfix #77)
- [ ] Navigateur ouvert sur `http://localhost/tv` (affichage TV) — onglet 1
- [ ] Navigateur ouvert sur `http://localhost` (interface admin) — onglet 2
- [ ] Jeu configuré avec une question de type **MEMOTION** et au moins 4 cartes
- [ ] Au moins 2 équipes configurées avec des couleurs distinctes (ex: violet + jaune)
- [ ] Jeu démarré sur une question MEMOTION (phase STARTED)

### Données de test recommandées

Préparer une question MEMOTION avec 4 cartes :
- **Carte A** : RECTO_IMAGE + QUESTION_TEXT + QUESTION_IMAGE + ANSWER_TEXT + ANSWER_IMAGE (carte complète)
- **Carte B** : RECTO_IMAGE + QUESTION_TEXT (sans image question) + ANSWER_TEXT (sans image réponse)
- **Carte C** : pas de RECTO_IMAGE + QUESTION_TEXT + QUESTION_IMAGE + ANSWER_IMAGE (sans texte réponse)
- **Carte D** : RECTO_IMAGE + sans QUESTION_TEXT + QUESTION_IMAGE + ANSWER_TEXT

---

## Scenarios

### Scenario 1 — Carte UNPLAYED (État neutre dans la grille)

**Objectif** : Vérifier que les cartes non jouées affichent le RECTO sur fond violet avec titre, image et étoiles.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Observer la grille MEMOTION sur l'écran TV (subphase GRID) | Fond violet (#1e1b4b → #4c1d95) sur chaque carte | | |
| 2 | Observer la zone haute (1/6) de chaque carte | Titre de la catégorie (RECTO_THEME) centré, texte blanc | | |
| 3 | Observer la zone centrale (4/6) de la Carte A | Image RECTO_IMAGE affichée (object-fit: contain) | | |
| 4 | Observer la zone centrale (4/6) de la Carte B | Image RECTO_IMAGE affichée | | |
| 5 | Observer la zone basse (1/6) de chaque carte | Étoiles ★ correspondant à la difficulté (1, 2 ou 3) | | |
| 6 | Vérifier l'absence de points dans le footer | Aucun texte "pt" ou nombre de points affiché | | |
| 7 | Vérifier la structure grille | Pas de flip, pas de scale, carte visible en entier | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Carte SELECTED dans la grille

**Objectif** : Vérifier que la carte sélectionnée porte la classe visuelle correcte sans flip.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis l'admin, sélectionner une carte MEMOTION (clic ou bouton) | La carte sélectionnée se met en surbrillance dans la grille | | |
| 2 | Observer la carte sélectionnée | Classe visuelle "selected" visible (bordure ou highlight) | | |
| 3 | Vérifier l'absence de flip | La carte ne se retourne PAS à la sélection | | |
| 4 | Vérifier que les autres cartes restent non-sélectionnées | Les autres cartes restent sans highlight | | |
| 5 | Vérifier l'apparition de l'overlay | L'overlay fullscreen SELECTED s'affiche sur la TV | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Overlay SELECTED fullscreen (Layout RECTO 3 rows)

**Objectif** : Vérifier que l'overlay SELECTED affiche le layout RECTO (titre/image/étoiles) sans header "theme+pts+équipe".

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sélectionner la Carte A (avec RECTO_IMAGE) depuis l'admin | Overlay fullscreen SELECTED s'ouvre avec animation clip-path | | |
| 2 | Observer la zone haute (row 1 = 1/6) | RECTO_THEME centré uniquement — PAS de "★★★ 5pt Équipe X" | | |
| 3 | Observer la zone centrale (row 2 = 4/6) | RECTO_IMAGE affichée (object-fit: contain), pleine zone | | |
| 4 | Observer la zone basse (row 3 = 1/6) | Étoiles ★★ (difficulté) centrées | | |
| 5 | Vérifier le fond de l'overlay | Fond violet (dégradé #1e1b4b → #312e81 → #4c1d95) | | |
| 6 | Vérifier l'absence du nom d'équipe dans l'overlay | Aucun nom d'équipe visible dans l'overlay SELECTED | | |
| 7 | Vérifier l'absence des points dans l'overlay | Aucun texte "pt" ou score visible | | |
| 8 | Sélectionner la Carte C (sans RECTO_IMAGE) | Zone centrale affiche le RECTO_THEME en texte large | | |
| 9 | Vérifier les étoiles pour la Carte C | Row 3 affiche les étoiles correspondant à la difficulté de C | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Overlay QUESTION (QUESTION_TEXT en row 1)

**Objectif** : Vérifier que l'overlay QUESTION affiche le texte de la question en haut et l'image en zone centrale.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis l'admin, passer de SELECTED à QUESTION (flip card) | Overlay QUESTION s'affiche avec animation rotateY | | |
| 2 | Observer la zone haute (row 1 = 1/6) de la Carte A | QUESTION_TEXT affiché en blanc, taille réduite (max 2 lignes) | | |
| 3 | Vérifier le contenu de la zone haute | UNIQUEMENT le texte de la question — PAS de titre thème, étoiles, pts, équipe | | |
| 4 | Observer la zone centrale (row 2 = 4/6) de la Carte A | QUESTION_IMAGE affichée en plein centre | | |
| 5 | Observer la zone basse (row 3 = 1/6) | Zone vide (espace libre pour les joueurs) | | |
| 6 | Tester la Carte B (QUESTION_TEXT sans QUESTION_IMAGE) | Row 1 = texte question, row 2 = vide, row 3 = vide | | |
| 7 | Tester la Carte D (QUESTION_IMAGE sans QUESTION_TEXT) | Row 1 = vide, row 2 = image question, row 3 = vide | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Overlay REVEAL (QUESTION_TEXT rappel en row 1)

**Objectif** : Vérifier que l'overlay REVEAL affiche le rappel de la question en haut (atténué), la réponse image au centre, et le texte réponse en bas.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis l'admin, passer de QUESTION à REVEAL | Overlay REVEAL s'affiche avec animation rotateY | | |
| 2 | Observer la zone haute (row 1 = 1/6) de la Carte A | QUESTION_TEXT affiché en version atténuée (opacity ~65%) | | |
| 3 | Vérifier que la zone haute NE montre PAS le RECTO_THEME | Aucun titre de thème — uniquement le rappel de la question | | |
| 4 | Vérifier que la zone haute NE montre PAS de pts/équipe | Aucun score, aucun nom d'équipe | | |
| 5 | Observer la zone centrale (row 2 = 4/6) de la Carte A | ANSWER_IMAGE affichée en plein centre | | |
| 6 | Observer la zone basse (row 3 = 1/6) de la Carte A | ANSWER_TEXT affiché (Carte A a image+texte) | | |
| 7 | Tester la Carte B (ANSWER_TEXT sans ANSWER_IMAGE) | Row 1 = rappel question, row 2 = ANSWER_TEXT en vert, row 3 = vide | | |
| 8 | Tester la Carte C (ANSWER_IMAGE sans texte réponse) | Row 1 = rappel question, row 2 = image réponse, row 3 = vide | | |
| 9 | Vérifier que le footer est toujours présent | Row 3 visible même vide (structure grille CSS cohérente) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Carte DONE dans la grille (fond équipe, scale 0.8)

**Objectif** : Vérifier que les cartes jouées (DONE) affichent la couleur de l'équipe gagnante, rétrécissent à 80% et montrent le nom de l'équipe dans le footer.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis l'admin, attribuer une carte à une équipe (valider la réponse) | La carte passe à l'état DONE dans la grille | | |
| 2 | Observer la carte DONE sur la TV | La carte rétrécit à 80% de sa taille initiale (animation scale) | | |
| 3 | Vérifier l'absence de flip | La carte NE se retourne PAS — elle reste sur le RECTO | | |
| 4 | Observer le fond de la carte DONE | Fond couleur de l'équipe gagnante (ex: violet si Équipe A est violette) | | |
| 5 | Observer le footer de la carte DONE | Étoiles + nom de l'équipe gagnante (ex: "★★ Équipe A") | | |
| 6 | Vérifier l'absence de .memory-card-front visible | Le recto est toujours affiché (pas de retournement côté "check" + winner) | | |
| 7 | Attribuer une autre carte à l'autre équipe | Sa carte DONE affiche la couleur de l'autre équipe | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Carte DONE sans équipe assignée (winnerTeam null)

**Objectif** : Vérifier que si aucune équipe n'est assignée à une carte DONE, le footer n'affiche que les étoiles.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer un état DONE sans équipe (annuler l'attribution si possible) | La carte a l'état DONE sans équipe gagnante | | |
| 2 | Observer le fond de la carte | Fond neutre (blanc semi-transparent, ~40%) | | |
| 3 | Observer le footer de la carte | Uniquement les étoiles ★ — PAS de nom d'équipe | | |
| 4 | Vérifier que la carte reste à scale 0.8 | La carte garde sa taille réduite malgré l'absence d'équipe | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Non-régression MODE MEMORY (cartes non affectées)

**Objectif** : Vérifier que le mode MEMORY est inchangé après bugfix #77.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer une question de type MEMORY (jeu de mémoire classique) | Grille MEMORY s'affiche normalement | | |
| 2 | Observer le dos des cartes MEMORY | Fond uni avec lettre (A, B, C...) — PAS de titre/étoiles MEMOTION | | |
| 3 | Retourner une paire de cartes MEMORY | Animation flip rotateY 180° visible (contrairement à MEMOTION) | | |
| 4 | Valider une paire MEMORY | Cartes restent retournées (face avant visible) avec couleur équipe | | |
| 5 | Vérifier l'absence d'overlay MEMOTION | Aucun .memotion-tv-fullscreen visible en mode MEMORY | | |
| 6 | Observer les cartes matched MEMORY | Flip 180° + face avant (image + texte) inchangés | | |
| 7 | Observer les scores MEMORY | Barres d'équipes et scores inchangés en bas de l'écran | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] SC1 : Cartes UNPLAYED — fond violet + titre + image + étoiles (layout 1/6-4/6-1/6)
- [ ] SC2 : Carte SELECTED — classe visuelle selected, pas de flip
- [ ] SC3 : Overlay SELECTED — 3 rows (titre/image/étoiles), PAS de pts/équipe dans row1
- [ ] SC4 : Overlay QUESTION — QUESTION_TEXT en row1, image en row2, row3 vide
- [ ] SC4b : QUESTION sans image — row2 vide
- [ ] SC4c : QUESTION sans texte — row1 vide
- [ ] SC5 : Overlay REVEAL — rappel question en row1 (atténué), image réponse en row2, texte en row3
- [ ] SC5b : REVEAL sans image — texte réponse en vert dans row2, row3 vide
- [ ] SC6 : Carte DONE — scale 0.8, pas de flip, fond couleur équipe, footer stars+équipe
- [ ] SC7 : Carte DONE sans équipe — fond neutre, footer stars uniquement
- [ ] SC8 : Non-régression MEMORY — flip 180° + face avant inchangés, pas d'overlay MEMOTION

## Notes QA

[Espace pour observations et captures d'écran]
