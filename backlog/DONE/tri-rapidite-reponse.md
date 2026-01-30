# Tri par rapidité de réponse (Page Jeu Admin)

**Statut** : ✅ v2.44.1 - COMPLÉTÉ

**Implémentation** : 2026-01-30
**Branch** : feature/tri-rapidite-reponse
**Commits** : 5 commits (d3f746c...7b630ed)

## Description

Sur la page de jeu de l'animateur (`/admin` ou `/anim`), trier dynamiquement les équipes et les joueurs par rapidité de réponse/buzz. Permet à l'animateur de voir immédiatement qui a répondu le plus vite.

## Objectifs

- [x] Trier les équipes par temps de réponse (plus rapide en haut)
- [x] Trier les joueurs au sein de chaque équipe par temps de réponse
- [x] Affichage dynamique dès qu'un buzz est reçu
- [x] Indicateur visuel du temps de réponse
- [x] Phase-aware (tri OFF hors jeu, ON en STARTED/PAUSED/REVEALED)
- [x] Badges de classement (🏆🥈🥉)
- [x] Animation fluide (300ms spring + 500ms flash)
- [x] Responsive design (desktop, tablet, mobile)

## Design

### Colonne Équipes (page Jeu)

```
┌─────────────────────────┐
│ 🏆 Les Rouges    342ms  │  ← Équipe la plus rapide
│   • Alice       342ms   │  ← Joueur le plus rapide
│   • Bob         567ms   │
├─────────────────────────┤
│ 🥈 Les Bleus     456ms  │
│   • Charlie     456ms   │
│   • David       612ms   │
├─────────────────────────┤
│ ⏳ Les Verts     ---    │  ← Pas encore buzzé
│   • Eve         ---     │
└─────────────────────────┘
```

### Comportement

| Phase | Tri | Affichage |
|-------|-----|-----------|
| STOP/PREPARE/READY | Ordre par défaut (alphabétique ou création) | Temps masqué |
| STARTED | Équipes ayant buzzé en haut, triées par temps | Temps affiché |
| PAUSED | Idem STARTED | Temps affiché |
| REVEALED | Idem STARTED | Temps affiché |

## Tâches

### Phase 1 - Tri des équipes (v2.42.0)

- [ ] **Tri dynamique des équipes**
  - Utiliser `team.TIME` (timestamp du premier buzz de l'équipe)
  - Équipes avec `TIME > 0` triées par temps croissant (plus rapide en haut)
  - Équipes avec `TIME = 0` en bas (pas encore buzzé)
  - Recalculer le tri à chaque UPDATE reçu

- [ ] **Affichage du temps de réponse équipe**
  - Calcul : `(team.TIME - gameState.GAME_TIME) / 1000` ms
  - Format : `XXXms` à côté du nom de l'équipe
  - Couleur : vert pour le plus rapide, dégradé vers gris

- [ ] **Indicateur de classement**
  - Badge 🏆 pour l'équipe la plus rapide
  - Badge 🥈 pour la deuxième
  - Badge 🥉 pour la troisième
  - Pas de badge pour les suivantes

### Phase 2 - Tri des joueurs (v2.42.0)

- [ ] **Tri dynamique des joueurs dans chaque équipe**
  - Utiliser `bumper.TIME` (timestamp du buzz du joueur)
  - Joueurs avec `TIME > 0` triés par temps croissant
  - Joueurs avec `TIME = 0` en bas

- [ ] **Affichage du temps de réponse joueur**
  - Format : `XXXms` à côté du nom du joueur
  - Plus petit que le temps équipe

### Phase 3 - Animations (optionnel)

- [ ] **Animation de réorganisation**
  - Transition fluide quand l'ordre change (framer-motion)
  - Durée : 300ms

- [ ] **Highlight du nouveau buzz**
  - Flash sur la ligne du joueur qui vient de buzzer
  - Durée : 500ms

## Fichiers à modifier

| Fichier | Modification |
|---------|--------------|
| `GamePage.jsx` | Logique de tri des équipes et joueurs |
| `GamePage.css` | Styles pour temps et badges de classement |
| `TeamCard.jsx` | Affichage du temps et du badge |
| `TeamCard.css` | Styles pour le temps de réponse |

## Notes techniques

- Le tri doit être stable (préserver l'ordre relatif si temps égaux)
- Utiliser `useMemo` pour éviter les recalculs inutiles
- `GAME_TIME` est le timestamp serveur au démarrage du jeu (microsecondes)
- `team.TIME` et `bumper.TIME` sont les timestamps de buzz (microsecondes)

## Version cible

v2.42.0
