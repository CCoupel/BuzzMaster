# Procédure de Test — MEMOTION Bugfix v5.0.2

**Version** : 5.0.2
**Date** : 2026-05-06
**Commit** : `4fc81f36`
**Branche** : main
**Testeur** : QA

---

## Contexte du Bugfix

Trois régressions visuelles corrigées dans `PlayerDisplay.jsx` / `PlayerDisplay.css` :

| # | Composant | Symptôme avant fix | Fix appliqué |
|---|-----------|-------------------|--------------|
| 1 | Cartes MEMOTION (grille GRID) | Image occupait ≤ 35 % de la carte ; texte trop petit ; étoiles mal positionnées | `flex: 1 1 0 / max-height: 75 %` sur `.memotion-card-img`, taille texte augmentée, étoiles poussées en bas via `margin-top: auto` |
| 2 | Animation SELECTED → QUESTION | Pas d'animation flip 3D ; les deux vues s'affichaient en même temps ou se superposaient | `AnimatePresence` unique partagé ; SELECTED sort sur `rotateY: 90`, QUESTION entre sur `rotateY: -90` |
| 3 | Image plein-écran (sous-phase SELECTED) | Image trop petite (`max-height: 50 %`) | Classe surcharge `.memotion-tv-selected .memotion-tv-fs-img` à `max-height: 70 %` |

---

## Prérequis

- [ ] Environnement : **LOCAL** (ou QUALIF)
- [ ] Serveur BuzzControl v5.0.2 démarré (`go build` depuis `server-go/`)
- [ ] Admin connecté sur `http://localhost/admin`
- [ ] TV ouverte sur `http://localhost/tv` (plein écran recommandé, ou zoom navigateur 100 %)
- [ ] Jeu créé contenant au moins une question de type **MEMOTION** avec grille de cartes
  - Certaines cartes doivent avoir une **image** (pour tester les scénarios image + texte)
  - Certaines cartes peuvent être **sans image** (texte seul)

---

## Scénario 1 — Contenu des cartes dans la grille (Fix 1)

**Objectif** : Vérifier que l'image et le thème occupent correctement l'espace disponible
dans chaque carte de la grille MEMOTION (sous-phase GRID).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la partie MEMOTION depuis l'admin | La TV passe en sous-phase GRID ; la grille de cartes s'affiche | | |
| 2 | Observer une carte **avec image** | L'image occupe la majorité verticale de la carte (> 50 % de hauteur visible, pas de espace vide excessif au-dessus) | | |
| 3 | Observer le texte du thème sur la même carte | Le thème est lisible, corps de texte nettement plus grand qu'avant le fix (`clamp(0.7rem…)`) | | |
| 4 | Observer les étoiles et les points de la carte | Les étoiles et les points sont **en bas** de la carte, pas intercalés entre image et thème | | |
| 5 | Observer une carte **sans image** | Le thème occupe le centre, les étoiles/points sont en bas, aucun débordement | | |
| 6 | Redimensionner la fenêtre TV à une résolution différente (ex : 1280×720) | Aucune carte ne déborde de son conteneur ; les images restent `object-fit: contain` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Transition flip 3D SELECTED → QUESTION (Fix 2)

**Objectif** : Vérifier que la transition entre les sous-phases SELECTED et QUESTION
produit un effet flip 3D fluide et coordonné.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis la sous-phase GRID, admin clique "Sélectionner" sur une carte | La TV passe en sous-phase SELECTED ; la carte sélectionnée zoome jusqu'au plein écran (animation `layoutId`) | | |
| 2 | Observer la vue SELECTED en plein écran | Le recto de la carte (image ou texte du thème) s'affiche correctement ; la vue est stable, sans clignotement | | |
| 3 | Admin clique "Retourner" | La vue SELECTED **part vers la droite** (`rotateY: 0 → 90`) avant de disparaître | | |
| 4 | (suite) | La vue QUESTION **arrive depuis la gauche** (`rotateY: -90 → 0`) juste après | | |
| 5 | (suite) | L'effet flip 3D est fluide, coordonné, sans superposition visible des deux vues | | |
| 6 | Observer la vue QUESTION | La question (texte et/ou image) s'affiche correctement en plein écran | | |
| 7 | Répéter le test avec une carte **sans image** | La sous-phase SELECTED affiche le texte du thème en recto (fallback `RECTO_THEME`) ; le flip vers QUESTION fonctionne identiquement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Image plein-écran en sous-phase SELECTED (Fix 3)

**Objectif** : Vérifier que l'image affichée en plein écran lors de la sous-phase SELECTED
est suffisamment grande (≥ 70 % de hauteur disponible).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sélectionner une carte **avec image** depuis GRID | La TV passe en SELECTED, l'image de la carte s'affiche en plein écran | | |
| 2 | Observer la proportion verticale de l'image | L'image occupe au moins 70 % de la hauteur de la zone plein écran (nettement plus grande qu'avant le fix qui était à 50 %) | | |
| 3 | Vérifier que l'image reste contenue (pas de dépassement) | L'image ne déborde pas de son conteneur ; `object-fit: contain` respecté | | |
| 4 | Observer le texte du thème sous l'image | Le thème reste lisible en dessous de l'image, pas caché | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Non-régression MEMOTION (autres sous-phases)

**Objectif** : Vérifier que les autres sous-phases MEMOTION ne sont pas régressées.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis QUESTION, admin valide la réponse | La TV passe en sous-phase REVEAL ; le flip final s'affiche correctement (verso → recto ou recto → verso selon config) | | |
| 2 | Observer la grille GRID après un REVEAL | La carte retournée affiche le **checkmark** (✓) ou marquage DONE ; les autres cartes sont inchangées | | |
| 3 | Refaire un second cycle complet (GRID → SELECTED → QUESTION → REVEAL) | Aucune erreur, animations fluides, pas de freeze de l'interface TV | | |
| 4 | Vérifier l'interface **admin** (`/admin`) pendant toute la session | Les actions admin (Sélectionner, Retourner, Valider) répondent normalement ; aucun log d'erreur dans la console navigateur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Non-régression MEMORY (mode distinct)

**Objectif** : Vérifier que le mode MEMORY (jeu de paires) n'est pas impacté par les
changements CSS/JSX de MEMOTION.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer/charger une partie de type **MEMORY** | Partie MEMORY démarre normalement | | |
| 2 | Lancer la phase MEMORY | La grille de cartes MEMORY s'affiche correctement sur la TV | | |
| 3 | Retourner une paire de cartes | L'animation de retournement MEMORY fonctionne ; les cartes trouvées restent visibles | | |
| 4 | Terminer la partie MEMORY | Le score s'affiche correctement ; retour à l'état IDLE sans erreur | | |

**Verdict** : [ ] PASS  [ ] FAIL  [ ] N/A (pas de question MEMORY disponible)

---

## Critères de Validation

- [ ] **Scénario 1** : l'image occupe > 50 % de la carte dans la grille GRID
- [ ] **Scénario 1** : le texte du thème est lisible et nettement plus grand qu'en v5.0.1
- [ ] **Scénario 1** : étoiles et points en bas de chaque carte, pas de débordement
- [ ] **Scénario 2** : flip SELECTED → QUESTION est un effet 3D fluide et coordonné (sortie droite, entrée gauche)
- [ ] **Scénario 2** : aucune superposition des deux vues pendant la transition
- [ ] **Scénario 3** : image SELECTED occupe ≥ 70 % de la hauteur disponible
- [ ] **Scénario 4** : sous-phase REVEAL fonctionne, grille GRID met à jour les cartes DONE
- [ ] **Scénario 5** : mode MEMORY non régressé
- [ ] Aucune erreur JavaScript dans la console navigateur (TV et Admin) pendant toute la session

---

## Notes QA

[Espace pour observations, version du binaire testé, résolution d'écran utilisée, logs relevés]
