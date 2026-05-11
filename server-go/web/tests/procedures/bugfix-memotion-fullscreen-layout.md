# Procédure de Test — Bugfix Layout Fullscreen MEMOTION

**Version** : 5.1.3
**Date** : 2026-05-11
**Testeur** : QA
**Bug corrigé** : Layout fullscreen MEMOTION ne respectait pas la structure 1/6 + 4/6 + 1/6

## Contexte du Bug

Les cards MEMOTION en mode plein écran (sous-phases SELECTED, QUESTION, REVEAL) n'appliquaient
pas le CSS grid `1fr 4fr 1fr`. La zone Body (image) prenait tout l'espace disponible sans
réserver les zones Header et Footer, et le chronomètre était mal positionné.

**Layout attendu après le fix :**
```
┌─────────────────────────────────────────┐  ← 1/6 hauteur zone jeu
│  HEADER : thème + étoiles + équipe      │
├─────────────────────────────────────────┤  ← 4/6 hauteur zone jeu
│                                         │
│   BODY : image (object-fit: contain)    │
│   ou texte thème si pas d'image         │
│                                         │
├─────────────────────────────────────────┤  ← 1/6 hauteur zone jeu
│  FOOTER : chronomètre (QUESTION) / vide │
└─────────────────────────────────────────┘
```

## Prérequis

- [ ] Environnement : LOCAL ou QUALIF
- [ ] Serveur BuzzControl démarré
- [ ] TV affichée sur `/tv` (ou `/player` sur un second écran)
- [ ] Admin ouvert sur `/` (interface GamePage)
- [ ] Une partie MEMOTION configurée avec au moins 4 cartes dont :
  - 2 cartes **avec** image recto (`RECTO_IMAGE` renseignée)
  - 2 cartes **sans** image recto (texte seulement)
- [ ] Résolution TV : 1920×1080 ou équivalent plein écran

## Scénarios

---

### Scénario 1 — SELECTED avec image recto

**Objectif** : Vérifier que le header occupe 1/6, le body 4/6, sans timer

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une partie MEMOTION | La grille de cartes s'affiche sur TV | | |
| 2 | Admin : sélectionner une carte **avec image** | La carte zoome en plein écran sur TV | | |
| 3 | Observer le header (zone haute) | Thème + étoiles + nom équipe — occupe ~1/6 de la hauteur | | |
| 4 | Observer le body (zone centrale) | L'image est centrée, proportionnelle, non recadrée (`object-fit: contain`) — occupe ~4/6 | | |
| 5 | Observer la zone basse | Vide — pas de chronomètre en SELECTED — occupe ~1/6 | | |
| 6 | Vérifier les bords | L'image ne déborde PAS hors de la zone body | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — SELECTED sans image recto

**Objectif** : Vérifier que le texte RECTO_THEME s'affiche dans le body à la place de l'image

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Admin : sélectionner une carte **sans image** | Carte zoome en plein écran | | |
| 2 | Observer le header | Thème + étoiles + équipe — ~1/6 hauteur | | |
| 3 | Observer le body | Le **RECTO_THEME** s'affiche en grand texte centré — ~4/6 | | |
| 4 | Vérifier qu'aucune image cassée n'apparaît | Aucun carré blanc/icône brisée | | |
| 5 | Observer la zone basse | Vide (~1/6) — pas de chronomètre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — QUESTION (avec chronomètre)

**Objectif** : Vérifier que le chronomètre s'affiche dans la zone basse (1/6) et non dans le header

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis SELECTED, admin passe en phase QUESTION | Animation flip vers la question | | |
| 2 | Observer la zone haute (header, ~1/6) | Thème + étoiles + points + équipe — PAS de chronomètre dans cette zone | | |
| 3 | Observer la zone centrale (body, ~4/6) | Image de question + texte question — bien centré | | |
| 4 | Observer la zone basse (~1/6) | **Le chronomètre** s'affiche ici (identique aux autres types de jeux) | | |
| 5 | Laisser le timer tourner | Le chronomètre décompte correctement | | |
| 6 | Vérifier proportion | Body occupe bien ~4× la hauteur du header | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — REVEAL (réponse)

**Objectif** : Vérifier header 1/6 + body réponse 4/6 + zone basse vide 1/6, sans timer

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Admin passe en phase REVEAL | Animation flip vers la réponse | | |
| 2 | Observer le header (~1/6) | Thème + étoiles + points — **PAS d'équipe, PAS de timer** | | |
| 3 | Observer le body (~4/6) | Image réponse + texte réponse en **vert** | | |
| 4 | Vérifier l'image réponse | Centrée, proportionnelle, non recadrée | | |
| 5 | Observer la zone basse (~1/6) | Vide — pas de chronomètre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Non-régression : autres types de questions

**Objectif** : Vérifier que les types QUIZ, QCM, MEMORY ne sont pas affectés

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une partie QUIZ | Affichage TV normal — Timer + Question + Réponses | | |
| 2 | Vérifier l'absence d'overlay MEMOTION | Aucune div `.memotion-tv-fullscreen` visible | | |
| 3 | Lancer une partie QCM | Affichage TV normal avec choix QCM | | |
| 4 | Lancer une partie MEMORY | Grille MEMORY normale | | |
| 5 | Vérifier les scores sur TV | Scores affichés normalement dans le bandeau | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Validation visuelle proportions TV 1080p

**Objectif** : Vérifier visuellement les proportions sur écran 16:9 plein écran

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Passer la TV en plein écran (F11 ou bouton ⛶) | Affichage plein écran sans barre navigateur | | |
| 2 | Phase QUESTION : mesurer visuellement les zones | Header ≈ 180px, Body ≈ 720px, Timer ≈ 180px (sur 1080px) | | |
| 3 | Vérifier que l'image ne sort PAS de la zone body | Aucun débordement visible, marges internes respectées | | |
| 4 | Tester avec une image en format portrait (tall) | L'image est letterboxée horizontalement, centrée verticalement | | |
| 5 | Tester avec une image en format paysage (wide) | L'image est letterboxée verticalement, centrée horizontalement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les scénarios nominaux (SC1–SC6) passent
- [ ] Le chronomètre apparaît **uniquement** en QUESTION, dans la zone basse (1/6)
- [ ] L'image de carte respecte `object-fit: contain` (pas de recadrage)
- [ ] Les proportions 1/6 + 4/6 + 1/6 sont visuellement respectées à l'œil
- [ ] Aucune régression sur les types QUIZ, QCM, MEMORY
- [ ] Aucune régression sur les bugs corrigés en SC7 (pas de `.memotion-card-pts`)

## Notes QA

[Espace pour observations, captures d'écran, commentaires]

---

*Procédure générée par test-writer — BuzzControl v5.1.3*
