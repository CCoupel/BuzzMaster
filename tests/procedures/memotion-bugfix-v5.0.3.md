# Procédure de Test — MEMOTION Bugfix v5.0.3

**Version** : 5.0.3
**Date** : 2026-05-06
**Commit** : `91ea21c`
**Branche** : main
**Testeur** : QA

---

## Contexte du Bugfix

Cinq régressions visuelles corrigées dans `PlayerDisplay.jsx` / `PlayerDisplay.css` :

| # | Composant | Symptôme avant fix | Fix appliqué |
|---|-----------|-------------------|--------------|
| 1 | Phase READY avec question MEMOTION | La grille n'apparaissait pas en READY ; pas de message "PRÉPAREZ-VOUS" | Condition `showGameContent \|\| showReady` + bloc "PREPAREZ-VOUS" animé (✋ pulsant + texte clignotant) |
| 2 | Cartes de la grille GRID | Mise en page identique image/sans image ; thème trop petit en mode solo | Layout dual : **avec image** → cover + footer (thème + étoiles + pts) ; **sans image** → `.memotion-card-theme-solo` flex:1 centré (`clamp(0.9→2rem)`) |
| 3 | Zoom GRID → SELECTED | Animation zoom bloquée (`transformStyle: preserve-3d` empêchait la layout transition Framer Motion) | Suppression de `transformStyle: 'preserve-3d'` sur le div SELECTED |
| 4 | Opacité pendant le flip | Carte transparente (invisible) pendant la transition flip SELECTED → QUESTION | Fond `#1a1a2e` sur `.memotion-tv-fullscreen` ; suppression des `opacity` dans les variants SELECTED exit et QUESTION initial/animate |
| 5 | Direction flip QUESTION → REVEAL | REVEAL entrait depuis la droite au lieu de la gauche (incohérent avec le flip SELECTED→QUESTION) | `exit: { rotateY: 90 }` sur QUESTION ; `initial: { rotateY: -90 }` sur REVEAL |

---

## Prérequis

- [ ] Environnement : **LOCAL** (ou QUALIF)
- [ ] Serveur BuzzControl v5.0.3 démarré (`go build` depuis `server-go/`)
- [ ] Admin connecté sur `http://localhost/admin`
- [ ] TV ouverte sur `http://localhost/tv` (plein écran recommandé, zoom navigateur 100 %)
- [ ] Jeu créé contenant au moins une question de type **MEMOTION** avec grille de cartes
  - Certaines cartes doivent avoir une **image** (pour tester les scénarios image/sans image)
  - Certaines cartes doivent être **sans image** (texte seul)

---

## Scénario 1 — Phase READY avec question MEMOTION (Fix 1)

**Objectif** : Vérifier que la TV affiche la grille MEMOTION et le message animé
"PRÉPAREZ-VOUS" lorsque la phase est READY (avant que l'admin lance la partie).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis l'admin, charger une question MEMOTION puis passer en phase **READY** (sans encore démarrer le jeu) | La TV affiche la **grille de cartes** MEMOTION en arrière-plan | | |
| 2 | Observer la zone centrale de la TV | Le message **"PREPAREZ-VOUS"** est visible, avec l'emoji ✋ qui pulse et le texte qui clignote | | |
| 3 | Observer l'emoji ✋ | L'emoji grossit et rétrécit en boucle (`scale: 1 → 1.3 → 1`, durée ~0.4 s) | | |
| 4 | Observer le texte "PREPAREZ-VOUS" | Le texte alterne entre opaque et presque invisible (`opacity: 1 → 0.2 → 1`, durée ~0.6 s) | | |
| 5 | Admin démarre la partie (passe en GRID) | Le message "PREPAREZ-VOUS" disparaît ; la grille reste affichée, prête à l'interaction | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Lisibilité des cartes dans la grille (Fix 2)

**Objectif** : Vérifier que les cartes avec image et sans image ont chacune un layout
adapté et lisible dans la grille GRID.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la partie MEMOTION (sous-phase GRID) | La grille de cartes s'affiche sur la TV | | |
| 2 | Observer une carte **avec image** | L'image couvre toute la surface de la carte (`object-fit: cover`, sans bordure ni marge) | | |
| 3 | (suite) | Un footer semi-transparent apparaît **en bas** de la carte avec : thème (texte compact), étoiles et points | | |
| 4 | (suite) | Aucun débordement ; la carte reste dans son conteneur | | |
| 5 | Observer une carte **sans image** | Le thème occupe **tout l'espace vertical** de la carte, centré horizontalement et verticalement, en grand (`clamp(0.9rem, 2.8cqmin, 2rem)`) | | |
| 6 | (suite) | Un footer compact en bas affiche étoiles et points | | |
| 7 | Comparer visuellement les deux types de cartes | Les deux layouts sont clairement différenciés et lisibles, sans confusion | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Zoom GRID → SELECTED (Fix 3)

**Objectif** : Vérifier que la sélection d'une carte produit un zoom fluide depuis sa
position dans la grille jusqu'au plein écran.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis la sous-phase GRID, admin clique "Sélectionner" sur une carte | L'animation démarre immédiatement | | |
| 2 | Observer l'animation de transition | La carte **zoome depuis sa position dans la grille** en s'agrandissant progressivement jusqu'au plein écran (animation `layoutId` Framer Motion) | | |
| 3 | Observer la fluidité | L'animation est fluide, sans saut, sans freeze, sans disparition/réapparition de la carte | | |
| 4 | Observer la carte en fin d'animation | La carte occupe le plein écran en sous-phase SELECTED ; le thème et l'image (si présente) sont lisibles | | |
| 5 | Répéter avec une carte dans un coin opposé de la grille | Le zoom part bien depuis la position réelle de cette carte (pas depuis le centre ou la position précédente) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Opacité pendant le flip SELECTED → QUESTION (Fix 4)

**Objectif** : Vérifier que la carte reste **visible et opaque** pendant toute la durée
de la transition flip SELECTED → QUESTION, et que le fond sombre est bien présent.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | En sous-phase SELECTED (plein écran), observer le fond de la zone fullscreen | Le fond est **sombre** (`#1a1a2e`, bleu nuit) — visible derrière la carte | | |
| 2 | Admin clique "Retourner" | La transition flip commence | | |
| 3 | Observer la carte pendant toute la durée du flip (~0.35 s) | La carte reste **opaque** (entièrement visible) pendant tout le flip — aucun moment où elle devient transparente ou invisible | | |
| 4 | (suite) | Le fond sombre `#1a1a2e` est visible pendant le retournement (quand la carte est de profil à ~90°) | | |
| 5 | En fin de transition, observer la vue QUESTION | La vue QUESTION est opaque et stable dès son apparition | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Direction du flip QUESTION → REVEAL (Fix 5)

**Objectif** : Vérifier que la transition QUESTION → REVEAL produit un flip 3D cohérent
avec le flip SELECTED → QUESTION (QUESTION part à droite, REVEAL arrive de gauche).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | En sous-phase QUESTION (plein écran affiché), observer la vue | La question est affichée correctement | | |
| 2 | Admin valide la réponse (passage en sous-phase REVEAL) | La transition flip commence | | |
| 3 | Observer le sens de sortie de QUESTION | La vue QUESTION **part vers la droite** (`rotateY: 0 → 90`) avant de disparaître | | |
| 4 | Observer l'entrée de REVEAL | La vue REVEAL **arrive depuis la gauche** (`rotateY: -90 → 0`) | | |
| 5 | Observer la cohérence globale | Le flip QUESTION → REVEAL est dans le **même sens** que SELECTED → QUESTION (cohérence : chaque vue sort à droite, la suivante entre par la gauche) | | |
| 6 | Observer le fond pendant le flip | Le fond sombre `#1a1a2e` est visible entre les deux vues | | |
| 7 | Observer la vue REVEAL en fin d'animation | Le verso de la carte (réponse) s'affiche correctement, opaque, sans clignotement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénarios de Non-Régression (depuis v5.0.2)

### Scénario NR-1 — Contenu des cartes (régression v5.0.2)

**Objectif** : Vérifier que les corrections de layout de v5.0.2 ne sont pas régressées.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Observer la grille GRID | Les cartes restent dans leur conteneur, aucun débordement | | |
| 2 | Observer le footer des cartes avec image | Thème, étoiles et points sont groupés en bas de la carte | | |
| 3 | Redimensionner la fenêtre TV (ex : 1280×720) | Aucune carte ne déborde, les images restent contenues | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-2 — Flip SELECTED → QUESTION (régression v5.0.2)

**Objectif** : Vérifier que la transition flip 3D SELECTED → QUESTION reste fluide.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Admin sélectionne une carte → admin retourne | L'effet flip 3D est fluide et coordonné (sortie droite, entrée gauche) | | |
| 2 | Répéter avec carte sans image | SELECTED affiche le texte du thème en recto ; le flip vers QUESTION fonctionne identiquement | | |
| 3 | Observer l'absence de superposition | Les deux vues ne se superposent jamais pendant la transition | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-3 — Image plein-écran SELECTED (régression v5.0.2)

**Objectif** : Vérifier que l'image affichée en SELECTED reste grande (≥ 70 % hauteur).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sélectionner une carte avec image | La TV passe en SELECTED plein écran | | |
| 2 | Observer la proportion de l'image | L'image occupe au moins 70 % de la hauteur de la zone ; pas de dépassement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-4 — Cycle MEMOTION complet (régression générale)

**Objectif** : Vérifier qu'un cycle complet GRID → SELECTED → QUESTION → REVEAL se
déroule sans erreur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Effectuer un cycle complet | Toutes les transitions s'enchaînent ; la grille GRID met à jour la carte avec le marquage DONE (✓) | | |
| 2 | Refaire un second cycle sur une autre carte | Aucune erreur, animations fluides, pas de freeze | | |
| 3 | Vérifier la console navigateur (TV + Admin) | Aucune erreur JavaScript | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario NR-5 — Mode MEMORY non impacté

**Objectif** : Vérifier que le mode MEMORY (jeu de paires) n'est pas régressé.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer/charger une partie MEMORY et la lancer | La grille de cartes MEMORY s'affiche correctement sur la TV | | |
| 2 | Retourner une paire de cartes | L'animation de retournement MEMORY fonctionne ; les cartes trouvées restent visibles | | |
| 3 | Terminer la partie MEMORY | Le score s'affiche ; retour à IDLE sans erreur | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (pas de question MEMORY disponible)

---

## Critères de Validation

**Nouveaux (v5.0.3) :**
- [ ] **Scénario 1** : grille MEMOTION visible en phase READY, message "PREPAREZ-VOUS" animé (✋ pulsant, texte clignotant)
- [ ] **Scénario 2** : carte avec image → image `cover` + footer en bas ; carte sans image → thème centré grand
- [ ] **Scénario 3** : zoom GRID → SELECTED fluide depuis la position réelle de la carte (aucun saut)
- [ ] **Scénario 4** : carte opaque pendant tout le flip SELECTED → QUESTION ; fond sombre `#1a1a2e` visible
- [ ] **Scénario 5** : QUESTION sort à droite, REVEAL entre par la gauche — flip cohérent dans les deux sens

**Non-régression (v5.0.2) :**
- [ ] **NR-1** : aucun débordement dans la grille GRID
- [ ] **NR-2** : flip SELECTED → QUESTION fluide et coordonné
- [ ] **NR-3** : image SELECTED ≥ 70 % de hauteur disponible
- [ ] **NR-4** : cycle complet MEMOTION sans erreur, carte DONE marquée après REVEAL
- [ ] **NR-5** : mode MEMORY non régressé
- [ ] Aucune erreur JavaScript dans la console (TV et Admin) pendant toute la session

---

## Notes QA

[Espace pour observations, version du binaire testé, résolution d'écran utilisée, logs relevés]
