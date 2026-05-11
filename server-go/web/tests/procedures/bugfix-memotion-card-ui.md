# Procédure de Test — Bugfix UI Cartes MEMOTION (centrage titre + suppression pts)

**Version** : 5.1.2
**Date** : 2026-05-11
**Testeur** : QA
**Référence** : bugfix `PlayerDisplay.css` + `PlayerDisplay.jsx` — `.memotion-card-header` / `.memotion-card-title` / `.memotion-card-pts`

---

## Contexte du bugfix

Deux corrections visuelles sur les cartes de la grille MEMOTION (vue TV, phase GRID) :

1. **Centrage du titre** — Le texte du thème dans `.memotion-card-header` doit être centré horizontalement. Avant le fix : le titre pouvait être aligné à gauche selon la disposition flex.
2. **Suppression des points** — Le `<span class="memotion-card-pts">` (affichait ex : « 3 pts ») a été supprimé du footer. Seules les étoiles `★` restent visibles.

---

## Prérequis

- [ ] Environnement : **LOCAL** ou **QUALIF**
- [ ] Accès à la vue TV : `http://<serveur>/tv` (ouvrir dans un second onglet en plein écran)
- [ ] Une partie en cours avec au moins 2 cartes MEMOTION de difficultés différentes (1★ et 2★ ou 3★)
- [ ] Résolution écran ≥ 1280×720 ou TV connectée

---

## Scénarios

### Scénario 1 — Centrage du titre dans le header de la carte

**Objectif** : Vérifier que le RECTO_THEME est centré horizontalement dans la zone header de chaque carte MEMOTION (phase GRID).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une partie MEMOTION, attendre la phase GRID (grille de cartes visible) | La grille de cartes s'affiche sur l'écran TV | | |
| 2 | Observer la zone titre (bande supérieure) de chaque carte | Le texte du thème est **centré horizontalement** dans la bande sombre du header | | |
| 3 | Vérifier avec une carte dont le titre est court (ex : « Rock ») | Le texte court reste centré (pas aligné à gauche) | | |
| 4 | Vérifier avec une carte dont le titre est long (ex : « Cinéma des années 90 ») | Le texte long est centré et tronqué avec `…` si nécessaire (pas de débordement) | | |
| 5 | Inspecter avec les DevTools (`F12`) : sélectionner `.memotion-card-header` | Propriétés CSS : `justify-content: center`, `.memotion-card-title` a `text-align: center` et `width: 100%` | | |

**Description visuelle attendue** :

```
┌─────────────────────────┐
│   Thème Star Wars       │  ← texte CENTRÉ dans la bande sombre
├─────────────────────────┤
│                         │
│     [IMAGE RECTO]       │
│                         │
├─────────────────────────┤
│        ★★               │  ← étoiles seules (pas de "3 pts")
└─────────────────────────┘
```

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 2 — Absence des points (pts) dans le footer

**Objectif** : Vérifier que le footer des cartes MEMOTION n'affiche plus le texte de points (ex : « 1 pts », « 3 pts »), uniquement les étoiles ★.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | En phase GRID, observer le bas de chaque carte | Seules les étoiles `★` sont visibles dans la bande footer | | |
| 2 | Vérifier une carte 1 étoile (DIFFICULTY = 1) | Footer affiche : `★` — aucun texte supplémentaire | | |
| 3 | Vérifier une carte 2 étoiles (DIFFICULTY = 2) | Footer affiche : `★★` — aucun texte supplémentaire | | |
| 4 | Vérifier une carte 3 étoiles (DIFFICULTY = 3) | Footer affiche : `★★★` — aucun texte supplémentaire | | |
| 5 | Inspecter avec les DevTools : chercher `.memotion-card-pts` dans le DOM | **Aucun élément** `.memotion-card-pts` présent dans le DOM | | |
| 6 | Chercher le texte « pts » dans la page (Ctrl+F sur la vue TV) | **Aucune occurrence** de « pts » dans les footers des cartes GRID | | |

**Description visuelle attendue** :

```
Avant le fix :          Après le fix (attendu) :
┌──────────────┐        ┌──────────────┐
│ ★★  3 pts   │  →     │     ★★       │
└──────────────┘        └──────────────┘
```

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scénario 3 — Non-régression : la grille reste fonctionnelle

**Objectif** : Vérifier que les corrections CSS/JSX n'ont pas cassé l'interactivité ou l'affichage global de la grille.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | En phase GRID, cliquer sur une carte | La carte passe en mode SELECTED (fullscreen overlay) | | |
| 2 | Vérifier la phase SELECTED | L'overlay plein écran affiche le thème et l'image (comportement inchangé) | | |
| 3 | Une carte jouée (DONE) doit afficher ✓ et le nom d'équipe | La face avant (retournée) reste correcte | | |
| 4 | Vérifier que la barre d'équipes en bas de grille est présente | Les chips d'équipes sont toujours affichés | | |
| 5 | Naviguer entre phases (QUESTION, REVEAL) | Pas de régression visuelle dans les autres subphases | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Tous les titres de cartes sont centrés horizontalement (Scénario 1 complet)
- [ ] Aucun élément `.memotion-card-pts` n'est rendu dans le DOM (Scénario 2)
- [ ] Le texte « pts » est absent du footer en phase GRID (Scénario 2)
- [ ] Les étoiles s'affichent correctement selon la difficulté 1★/2★/3★ (Scénario 2)
- [ ] Aucune régression sur les autres fonctionnalités MEMOTION (Scénario 3)

## Notes QA

[Espace pour observations]
