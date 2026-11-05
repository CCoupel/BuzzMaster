# Procédure de Test — RAFALE « Alphabétique » : tirage par passes et roue des lettres (#282)

**Version** : v11.4.0 (LOT-2)
**Date** : 2026-11-05
**Testeur** : QA (scénarios AUTO) / utilisateur (scénarios HUMAIN)
**Contrats** : `contracts/rafale.md` §15, `contracts/wheel-component.md`
**Maquettes** : `docs/mockup/v11.4.0/ui/tv_rafale_started__letter_wheel.html`, `admin_question_editor__rafale_alphabetical.html`

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL ; admin sur `/admin`, TV réelle (ou fenêtre 1920×1080 plein écran) sur `/tv`, `/anim` ouvert
- [ ] Réservoir RAFALE : au moins 60 questions couvrant la plupart des lettres, avec des réponses à article (« La Seine », « L'Égypte », « Les Misérables »), accentuées (« Élan »), un chiffre (« 1984 ») et plusieurs réponses par lettre
- [ ] Une manche RAFALE existante (non alphabétique) servant de témoin

## Scénarios

### Scénario 1 — Tirage par passes A→Z (AUTO)

**Validation** : AUTO — **Issues** : #282
Équivalents automatisés : `TestRafaleAlpha282_Draw_*`, `TestRafaleAlpha282_Normalization_ObservedLetters`, `TestRafaleAlpha282_Letters_ShrinkAsWordsArePlayed`, `TestRafaleAlpha282_Next_MatchesNextQueueEntry` (`internal/game/rafale_alpha_282_test.go`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go && go test ./internal/game -run RafaleAlpha282 -race -v` | Tous verts : passes, lettre unique, passe k+1 à la première lettre pourvue, `#` en fin de passe, normalisation, `LETTERS` qui se réduit, `NEXT` exact | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Persistance de l'option et validation multipart (AUTO)

**Validation** : AUTO — **Issues** : #282
Équivalents : `TestRafaleAlpha282_Upload_*`, `TestRafaleAlpha282_Pool_*` (`internal/server/rafale_alpha_282_test.go`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/server -run RafaleAlpha282 -race -v` | Les deux champs survivent à l'enregistrement ; layout invalide ⇒ 400 ; `LETTERS` du pool conforme | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Aucune fuite de la réponse vers TV / VPlayer (AUTO)

**Validation** : AUTO — **Issues** : #282
Équivalents : `internal/protocol/rafale_alpha_282_test.go`, `cmd/server/rafale_alpha_fanout_282_test.go`, tests de fuite existants.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go test ./internal/protocol ./cmd/server -run 'RafaleAlpha282\|RafaleAnswer\|RafaleCurrent' -race` | Verts : lettre (1 caractère) et liste reçues par admin/anim/TV/VPlayer, jamais par les buzzers ; réponse absente de TV/VPlayer | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 4 — Composant Wheel : géométrie et « 100 mouvements » (AUTO)

**Validation** : AUTO — **Issues** : #282
Équivalents : `components/wheel/geometry.test.js`, `components/wheel/Wheel.test.jsx`, `components/RafaleLetterWheel.test.jsx`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `cd server-go/web && npx vitest run src/components/wheel src/components/RafaleLetterWheel.test.jsx` | Verts : position normalisée dans [0, n) après 1 000 mouvements ; 100 mises à jour ⇒ cellules toutes rendues, `translateX` bornée, courant présent (strip et arc, 26 et 24 segments) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 5 — Éditeur : interrupteur et choix Bandeau / Roue (HUMAIN)

**Validation** : HUMAIN — **Issues** : #282

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Questions, type RAFALE, nouvelle manche | Interrupteur « 🔤 Alphabétique » entre le mode de jeu et les temps, désactivé ; aucune vignette | | |
| 2 | Activer | Texte de règle affiché ; vignettes « Bandeau » (cochée) et « Roue » ; message du réservoir : « N questions · L lettres couvertes (A–Z sauf …) » | | |
| 3 | Choisir « Roue », enregistrer, rouvrir la manche | Interrupteur actif et « Roue » sélectionnée (rien perdu à l'enregistrement) | | |
| 4 | Désactiver, enregistrer, rouvrir | Option désactivée ; message du réservoir sans mention de lettres | | |
| 5 | Éditer une carte RAFALE dans une question MEMOTION | Aucun interrupteur « Alphabétique » dans le sous-éditeur de carte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 6 — Manche alphabétique sur TV réelle : bandeau puis roue (HUMAIN)

**Validation** : HUMAIN — **Issues** : #282

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une manche alphabétique « Bandeau », TV plein écran | Bandeau de lettres en bas de l'écran (≈ 16 % de la hauteur), lettre courante sous le pointeur dans la couleur de l'équipe ; rien ne défile | | |
| 2 | Valider 5 réponses | À chaque question la lettre courante change (A, B, C… puis cycle) ; le contenu glisse, pointeur fixe, pas de pause de 0,6 s gênante | | |
| 3 | Lecture à 3 m de l'écran | La lettre courante est lisible sans effort ; les lettres sans mot restant sont nettement grisées | | |
| 4 | Relancer la manche en « Roue » | Arc en bas de l'écran (≈ 30 %), même comportement ; questions et zones existantes toujours lisibles, rien ne déborde | | |
| 5 | Réponse « 1984 » (lettre `#`) | Aucune lettre en surbrillance, la roue reste affichée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 7 — Roue lisible après 100 mouvements (HUMAIN, complète le sc. 4)

**Validation** : HUMAIN — **Issues** : #282

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Réservoir ≥ 100 questions, plafond 100, manche alphabétique en bandeau, valider/invalider rapidement jusqu'à 100 questions | À tout moment le bandeau est plein (aucun trou, aucune zone vide), le courant est sous le pointeur, y compris après 100 mouvements | | |
| 2 | Même essai en « Roue » | Aucun saut d'angle, toutes les lettres lisibles à la 100ᵉ question | | |
| 3 | Passer 3 passes complètes A→Z | Aucune dérive visible entre la lettre annoncée et la lettre sous le pointeur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 8 — Admin et animateur : chip, pastille, « Suivante (lettre X) » (HUMAIN)

**Validation** : HUMAIN — **Issues** : #282

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Page Jeu (admin) avant lancement | Chip « 🔤 Alphabétique » parmi les chips de configuration ; message du réservoir avec lettres couvertes | | |
| 2 | Manche en cours | Pastille de la lettre courante devant la question ; ligne « Suivante (lettre S) : … » avec la réponse suivante | | |
| 3 | Page `/anim` | Chip et pastille identiques ; la réponse est visible (comme avant) | | |
| 4 | Ouvrir `/player` (VPlayer) | Aucun rendu nouveau, la réponse n'apparaît nulle part | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 9 — Mode classique et carte MEMOTION inchangés (HUMAIN)

**Validation** : HUMAIN — **Issues** : #282

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Manche RAFALE témoin sans l'option | Aucune roue, aucune chip, tirage stratifié comme en v11.3.x ; TV identique | | |
| 2 | Carte RAFALE d'une question MEMOTION | Fonctionne comme avant ; aucune lettre ni roue | | |
| 3 | Épuiser le réservoir en mode alphabétique | Fin normale (« réservoir épuisé ») sans erreur ; lettres vidées ensuite | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Ordre de tirage par passes A→Z conforme, normalisation (accents, casse, articles) conforme
- [ ] Roue lisible, statique, dans la couleur de l'équipe, après 100 mouvements
- [ ] Aucune fuite de la réponse hors admin/anim
- [ ] Mode classique et cartes MEMOTION inchangés

## Notes QA

