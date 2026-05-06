# Procédure de Test — MEMOTION Bugfix v5.0.4

**Version** : 5.0.4
**Date** : 2026-05-06
**Commit** : `21cb26e`
**Branche** : main
**Testeur** : QA

---

## Contexte du Bugfix

Six corrections dans `PlayerDisplay.jsx`, `PlayerDisplay.css` et `GamePage.jsx` :

| # | Composant | Symptôme avant fix | Fix appliqué |
|---|-----------|-------------------|--------------|
| 1 | Phase READY | La grille de cartes s'affichait en READY (confusant) ; pas de team bar | Early return READY dédié : ✋ + "PRÉPAREZ-VOUS" animé + team bar, **sans** grille |
| 2 | Footer des cartes (grille GRID) | Thème débordait sur les étoiles/pts ; étoiles trop grandes | Thème `flex:1` + `ellipsis` ; étoiles/pts réduits + `flex-shrink:0` ; footer `space-between` + fond `rgba(0,0,0,0.45)` |
| 3 | Zoom GRID → SELECTED | Zoom `layoutId` ne fonctionnait pas (liaison cassée entre grille et fullscreen) | `LayoutGroup` ajouté en wrapper ; `scale` retiré de `initial/animate` des cartes (interférait avec la mesure `layoutId`) |
| 4 | Fond sous-phase REVEAL | Fond transparent (gradient vert diffus) pendant la phase REVEAL | `.memotion-tv-reveal` → `background: #1a1a2e` (fond sombre opaque cohérent avec SELECTED/QUESTION) |
| 5 | Bouton "RÉVÉLER" (admin) | Le bouton "RÉVÉLER" était cliquable pendant le décompte du timer | Bouton masqué tant que `timer > 0` (conditionné sur `!(timer > 0)` dans `GamePage.jsx`) |
| 6 | Animation DONE (fin de REVEAL) | Après REVEAL, la carte disparaissait sans animation de retour vers la grille | REVEAL reçoit `layoutId={memotion-card-${selectedId}}` → zoom de retour vers la position DONE dans la grille |

---

## Prérequis

- [ ] Environnement : **LOCAL** (ou QUALIF)
- [ ] Serveur BuzzControl v5.0.4 démarré (`go build` depuis `server-go/`)
- [ ] Admin connecté sur `http://localhost/admin`
- [ ] TV ouverte sur `http://localhost/tv` (plein écran recommandé, zoom navigateur 100 %)
- [ ] Jeu créé contenant au moins une question de type **MEMOTION** avec grille de cartes
  - Certaines cartes doivent avoir une **image**
  - Certaines cartes doivent être **sans image** (thème seul)
  - Au moins une carte avec un **thème long** (> 15 caractères, pour tester l'ellipsis)
- [ ] Timer configuré sur la question MEMOTION (ex : 30 s) pour tester le scénario 5

---

## Scénario 1 — Phase READY : message + team bar, sans grille (Fix 1)

**Objectif** : Vérifier qu'en phase READY la TV affiche uniquement le message
"PRÉPAREZ-VOUS" et la barre d'équipes, sans la grille de cartes.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis l'admin, charger une question MEMOTION puis passer en phase **READY** | La TV affiche la vue READY | | |
| 2 | Observer la zone principale | La **grille de cartes est absente** — aucune carte MEMOTION n'est visible | | |
| 3 | Observer le centre de l'écran | L'emoji ✋ pulse en boucle (`scale: 1 → 1.3 → 1`) | | |
| 4 | Observer le texte | "PRÉPAREZ-VOUS" clignote (`opacity: 1 → 0.2 → 1`) | | |
| 5 | Observer le bas de l'écran | La **barre d'équipes** participantes est affichée (chips colorées avec le nom de chaque équipe) | | |
| 6 | Admin démarre la partie (passe en GRID) | La grille de cartes apparaît ; le message READY disparaît | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Footer des cartes équilibré (Fix 2)

**Objectif** : Vérifier que le footer de chaque carte de la grille GRID est équilibré :
thème tronqué si long, étoiles et points clairement plus petits.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la partie MEMOTION (sous-phase GRID) | La grille de cartes s'affiche sur la TV | | |
| 2 | Observer le footer d'une carte avec image | Le footer occupe toute la largeur de la carte (`width: 100%`), fond semi-transparent (`rgba(0,0,0,0.45)`) | | |
| 3 | Observer une carte avec un thème **court** (≤ 10 caractères) | Le thème s'affiche en entier ; les étoiles et les points sont alignés à droite | | |
| 4 | Observer une carte avec un thème **long** (> 15 caractères) | Le thème est **tronqué avec `…`** — il ne déborde pas sur les étoiles ni les points | | |
| 5 | Observer les étoiles et les points sur n'importe quelle carte | Les étoiles et les points sont **nettement plus petits** que le thème ; ils ne se compressent pas (`flex-shrink: 0`) | | |
| 6 | Observer la carte sans image (`.memotion-card-theme-solo`) | Le thème occupe toute la hauteur de la carte, centré, en grand — sans footer de thème | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Zoom GRID → SELECTED via LayoutGroup (Fix 3)

**Objectif** : Vérifier que la sélection d'une carte produit un zoom fluide depuis sa
position exacte dans la grille jusqu'au plein écran.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis la sous-phase GRID, admin clique "Sélectionner" sur une carte en **haut à gauche** | La carte zoome progressivement depuis sa position jusqu'au plein écran — animation visible | | |
| 2 | Observer la fluidité | L'animation est fluide, sans saut, sans apparition/disparition instantanée de la carte | | |
| 3 | Retour en GRID (annuler ou revenir), puis sélectionner une carte en **bas à droite** | Le zoom part bien depuis la position réelle de cette carte (position différente de la précédente) | | |
| 4 | Observer l'absence d'artefact de scale | Les cartes de la grille n'ont pas d'animation de scale parasite au démarrage (scale retiré de `initial/animate`) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Fond opaque en sous-phase REVEAL (Fix 4)

**Objectif** : Vérifier que la vue REVEAL a un fond sombre opaque cohérent avec
les autres sous-phases plein écran.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis QUESTION, admin valide la réponse → sous-phase REVEAL | La transition flip QUESTION → REVEAL se produit | | |
| 2 | Observer le fond de la vue REVEAL en plein écran | Le fond est **sombre et opaque** (`#1a1a2e`, bleu nuit) — pas de gradient vert ni de transparence | | |
| 3 | Comparer visuellement avec SELECTED et QUESTION | Les trois vues (SELECTED, QUESTION, REVEAL) ont un fond sombre identique et cohérent | | |
| 4 | Observer pendant le flip QUESTION → REVEAL | Le fond sombre est visible entre les deux vues (quand la carte est de profil) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Bouton "RÉVÉLER" masqué pendant le timer (Fix 5)

**Objectif** : Vérifier que l'admin ne peut pas cliquer "RÉVÉLER" tant que le timer
est en cours de décompte.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer la question MEMOTION avec un timer > 0 (ex : 30 s) | Timer configuré | | |
| 2 | Lancer la partie, admin passe en sous-phase QUESTION (après avoir retourné une carte) | La sous-phase QUESTION démarre avec le timer en décompte | | |
| 3 | Observer l'interface admin pendant que `timer > 0` | Le bouton **"RÉVÉLER" est absent** (caché, non désactivé) — l'admin ne peut pas cliquer dessus | | |
| 4 | Attendre que le timer atteigne 0 | Le bouton **"RÉVÉLER" apparaît** automatiquement dans l'interface admin | | |
| 5 | Cliquer "RÉVÉLER" | La sous-phase passe en REVEAL normalement | | |
| 6 | Tester avec timer = 0 (ou pas de timer) | Le bouton "RÉVÉLER" est immédiatement visible dès l'entrée en QUESTION | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Zoom retour REVEAL → position DONE dans la grille (Fix 6)

**Objectif** : Vérifier qu'après la sous-phase REVEAL, la carte zoome de retour
vers sa position DONE dans la grille (animation `layoutId` inverse).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Effectuer un cycle complet jusqu'à la fin de REVEAL | La vue REVEAL est affichée en plein écran | | |
| 2 | Admin clique pour terminer REVEAL (retour en GRID) | La carte **zoome depuis le plein écran vers sa position dans la grille** (animation inverse du zoom GRID → SELECTED) | | |
| 3 | Observer la fluidité | L'animation est fluide, sans saut, sans disparition instantanée | | |
| 4 | Observer la carte une fois en grille | La carte affiche le marquage DONE (✓ + équipe gagnante) à sa position originale | | |
| 5 | Répéter avec une deuxième carte | Le zoom de retour part bien depuis le plein écran vers la position réelle de chaque carte concernée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénarios de Non-Régression (depuis v5.0.3)

### Scénario NR-1 — Phase READY : message animé (régression v5.0.3)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Passer en phase READY avec une question MEMOTION | ✋ pulse, "PRÉPAREZ-VOUS" clignote — même rendu qu'en v5.0.3 (sans grille cette fois) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-2 — Layout carte sans image (régression v5.0.3)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Observer les cartes sans image dans la grille GRID | Thème occupe tout l'espace vertical, centré, en grand (`clamp(0.7rem, 2.2cqmin, 1.5rem)`) | | |
| 2 | Vérifier qu'aucune carte ne déborde de son conteneur | Aucun overflow visible à aucune résolution | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-3 — Flip SELECTED → QUESTION fluide et opaque (régression v5.0.3)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Admin sélectionne une carte puis clique "Retourner" | Flip 3D coordonné : SELECTED sort à droite (`rotateY: 90`), QUESTION entre par la gauche (`rotateY: -90`) | | |
| 2 | Observer l'opacité pendant le flip | La carte reste opaque pendant toute la transition — aucun moment de transparence | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-4 — Direction flip QUESTION → REVEAL (régression v5.0.3)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Admin valide depuis QUESTION → REVEAL | QUESTION sort à droite (`rotateY: 90`), REVEAL entre par la gauche (`rotateY: -90`) — cohérent avec le sens SELECTED→QUESTION | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-5 — Mode MEMORY non impacté

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer/charger une partie MEMORY et la lancer | La grille MEMORY s'affiche correctement ; les animations de retournement fonctionnent | | |
| 2 | Terminer la partie MEMORY | Score affiché ; retour à IDLE sans erreur ; console navigateur sans erreur JS | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (pas de question MEMORY disponible)

---

## Critères de Validation

**Nouveaux (v5.0.4) :**
- [ ] **Scénario 1** : phase READY → grille absente, ✋ + "PRÉPAREZ-VOUS" animé + team bar
- [ ] **Scénario 2** : thème tronqué avec `…` si long ; étoiles/pts nettement plus petits et stables
- [ ] **Scénario 3** : zoom GRID → SELECTED fluide depuis la position réelle (LayoutGroup actif, pas d'artefact scale)
- [ ] **Scénario 4** : fond REVEAL sombre opaque `#1a1a2e`, cohérent avec SELECTED et QUESTION
- [ ] **Scénario 5** : bouton "RÉVÉLER" absent tant que `timer > 0`, visible dès `timer = 0`
- [ ] **Scénario 6** : après REVEAL, la carte zoome vers sa position DONE dans la grille (animation inverse fluide)

**Non-régression (v5.0.3) :**
- [ ] **NR-1** : READY animé fonctionnel (✋ + clignotement)
- [ ] **NR-2** : carte sans image — thème grand centré, aucun débordement
- [ ] **NR-3** : flip SELECTED → QUESTION opaque, coordonné (droite → gauche)
- [ ] **NR-4** : flip QUESTION → REVEAL dans le bon sens (QUESTION droite, REVEAL gauche)
- [ ] **NR-5** : mode MEMORY non régressé
- [ ] Aucune erreur JavaScript dans la console navigateur (TV et Admin) pendant toute la session

---

## Notes QA

[Espace pour observations, version du binaire testé, résolution d'écran utilisée, timer configuré, logs relevés]
