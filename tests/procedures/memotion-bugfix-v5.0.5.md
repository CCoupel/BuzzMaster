# Procédure de Test — MEMOTION Bugfix v5.0.5

**Version** : 5.0.5
**Date** : 2026-05-06
**Branche** : main
**Testeur** : QA

---

## Contexte du Bugfix

Deux corrections visuelles dans `PlayerDisplay.jsx` et `PlayerDisplay.css` :

| # | Composant | Symptôme avant fix | Fix appliqué |
|---|-----------|-------------------|--------------|
| 1 | Grille MEMOTION (GRID) | Les cartes étaient contraintes à une forme carrée (aspect-ratio 1/1 ou équivalent), laissant des zones vides dans la zone de jeu | Cartes **rectangulaires** : elles remplissent toute la zone de jeu comme les cartes MEMORY, sans contrainte de ratio |
| 2 | Zoom GRID → SELECTED | La transition utilisait une technique CSS qui pouvait produire des sauts ou un rendu incohérent selon la position de la carte | Zoom via **clip-path CSS + `getBoundingClientRect`** : la carte s'étire progressivement depuis sa position réelle dans la grille jusqu'au plein écran ; au retour (DONE), le mouvement inverse vers la grille est animé |

---

## Prérequis

- [ ] Environnement : **LOCAL** (ou QUALIF)
- [ ] Serveur BuzzControl v5.0.5 démarré (`go build` depuis `server-go/`)
- [ ] Admin connecté sur `http://localhost/admin`
- [ ] TV ouverte sur `http://localhost/tv` (plein écran recommandé, zoom navigateur 100 %)
- [ ] Jeu créé contenant au moins une question de type **MEMOTION** avec une grille de cartes (min. 4 cartes, idéalement 6 ou 9)
  - Certaines cartes doivent avoir une **image**
  - Certaines cartes doivent être **sans image** (thème seul)
- [ ] Tester avec différentes tailles de grille si possible (2×2, 3×2, 3×3)

---

## Scénario 1 — Grille MEMOTION remplie : cartes rectangulaires

**Objectif** : Vérifier que la grille MEMOTION couvre toute la zone de jeu et que
les cartes sont rectangulaires (non contraintes à un carré).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis l'admin, charger une question MEMOTION puis passer en phase **GRID** | La grille de cartes s'affiche sur la TV | | |
| 2 | Observer la zone de jeu dans son ensemble | La grille **remplit toute la zone de jeu disponible** — aucune bande vide ni espace non utilisé au-dessus/en dessous/sur les côtés | | |
| 3 | Observer la forme des cartes individuelles | Les cartes sont **rectangulaires** (plus hautes que larges ou proportion libre selon la grille) — elles ne sont pas contraintes à être carrées | | |
| 4 | Comparer visuellement avec une grille MEMORY | La densité d'occupation de l'écran est similaire au mode MEMORY | | |
| 5 | Tester avec une grille de taille différente (ex : 3×3 si 2×3 précédemment) | La grille s'adapte ; les cartes restent rectangulaires et remplissent toujours toute la zone | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Zoom avant clip-path : GRID → SELECTED

**Objectif** : Vérifier que la sélection d'une carte produit une animation de zoom
progressive via clip-path depuis sa position exacte dans la grille.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis la sous-phase GRID, admin clique "Sélectionner" sur une carte en **haut à gauche** de la grille | La TV lance l'animation de zoom | | |
| 2 | Observer le démarrage de l'animation | L'animation **part de la position réelle de la carte** (haut gauche) et s'élargit progressivement vers le plein écran via clip-path | | |
| 3 | Observer la fluidité | L'animation est **continue et progressive** — pas de saut, pas d'apparition/disparition instantanée | | |
| 4 | Observer la fin de l'animation | La carte occupe tout l'écran en plein écran (sous-phase SELECTED) | | |
| 5 | Revenir en GRID (annuler si possible), puis sélectionner une carte en **bas à droite** | L'animation part bien depuis la position en bas à droite (différente visuellement de l'étape 1–2) | | |
| 6 | Sélectionner une carte **au centre** de la grille | L'animation part du centre — le clip-path s'ouvre depuis la zone centrale | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Zoom arrière clip-path : SELECTED/REVEAL → position DONE dans la grille

**Objectif** : Vérifier que lorsque l'admin clôture un cycle (attribution des points → DONE),
la carte revient animativement vers sa position dans la grille.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Effectuer un cycle complet : GRID → SELECTED → QUESTION → REVEAL | La vue REVEAL est affichée en plein écran | | |
| 2 | Admin clique pour terminer (retour en GRID / DONE) | La carte **zoome depuis le plein écran vers sa position dans la grille** — animation inverse du zoom avant | | |
| 3 | Observer la fluidité | L'animation est fluide via clip-path, sans saut, sans disparition instantanée | | |
| 4 | Observer la carte une fois en grille | La carte affiche le marquage DONE (✓ + équipe gagnante) à sa position originale | | |
| 5 | Répéter avec une deuxième carte à une position différente | Le zoom de retour cible bien la position réelle de chaque carte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Non-régression : SELECTED → QUESTION (flip 3D)

**Objectif** : Vérifier que la transition SELECTED → QUESTION conserve le flip 3D
attendu et n'est pas altérée par les modifications du zoom clip-path.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Admin sélectionne une carte (SELECTED en plein écran) puis clique "Retourner" (passe en QUESTION) | La transition **flip 3D** s'effectue : SELECTED sort à droite (`rotateY: 90`), QUESTION entre par la gauche (`rotateY: -90`) | | |
| 2 | Observer l'opacité pendant le flip | La carte reste **opaque** pendant toute la transition — aucun moment de transparence | | |
| 3 | Observer la fin de la transition | La vue QUESTION est affichée proprement en plein écran, question et réponse masquée visibles | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Non-régression : RÉVÉLER (flip 3D vers la réponse)

**Objectif** : Vérifier que l'action RÉVÉLER affiche le flip 3D vers la réponse,
avec fond opaque cohérent.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis QUESTION, admin clique "RÉVÉLER" | Le **flip 3D** QUESTION → REVEAL se produit (QUESTION sort, REVEAL entre) | | |
| 2 | Observer le fond de la vue REVEAL | Le fond est **sombre et opaque** (`#1a1a2e`) — cohérent avec SELECTED et QUESTION | | |
| 3 | Observer la vue REVEAL | La réponse est affichée clairement sur fond opaque | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénarios de Non-Régression (depuis v5.0.4)

### Scénario NR-1 — Phase READY : message animé + team bar, sans grille

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Passer en phase READY avec une question MEMOTION | ✋ pulse, "PRÉPAREZ-VOUS" clignote, team bar visible — **aucune carte MEMOTION dans la grille** | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-2 — Footer des cartes équilibré

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Observer les cartes en grille GRID | Thème tronqué avec `…` si long ; étoiles et points petits et stables (`flex-shrink: 0`) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-3 — Bouton "RÉVÉLER" masqué pendant le timer

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer un timer > 0 sur la question MEMOTION, passer en QUESTION | Le bouton "RÉVÉLER" est **absent** tant que `timer > 0` | | |
| 2 | Attendre que le timer atteigne 0 | Le bouton "RÉVÉLER" **apparaît** automatiquement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-4 — Mode MEMORY non impacté

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer/charger une partie MEMORY et la lancer | La grille MEMORY s'affiche correctement ; les animations de retournement fonctionnent | | |
| 2 | Terminer la partie MEMORY | Score affiché ; retour à IDLE sans erreur ; console navigateur sans erreur JS | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (pas de question MEMORY disponible)

---

## Critères de Validation

**Nouveaux (v5.0.5) :**
- [ ] **Scénario 1** : grille MEMOTION occupe toute la zone de jeu ; cartes rectangulaires (non carrées)
- [ ] **Scénario 2** : zoom avant clip-path — animation progressive depuis la position réelle de la carte, sans saut
- [ ] **Scénario 3** : zoom arrière clip-path — retour animé vers la position DONE dans la grille, sans saut

**Non-régression (v5.0.4) :**
- [ ] **Scénario 4** : flip 3D SELECTED → QUESTION opaque, coordonné (droite → gauche)
- [ ] **Scénario 5** : RÉVÉLER flip 3D, fond opaque `#1a1a2e`
- [ ] **NR-1** : phase READY animée sans grille
- [ ] **NR-2** : footer cartes équilibré (thème tronqué, étoiles stables)
- [ ] **NR-3** : bouton "RÉVÉLER" masqué pendant timer
- [ ] **NR-4** : mode MEMORY non régressé
- [ ] Aucune erreur JavaScript dans la console navigateur (TV et Admin) pendant toute la session

---

## Notes QA

[Espace pour observations, version du binaire testé, résolution d'écran utilisée, taille de grille testée, navigateur utilisé, logs relevés]
