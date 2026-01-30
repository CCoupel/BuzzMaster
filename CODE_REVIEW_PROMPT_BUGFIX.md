# Code Review Prompt - Bug Fix v2.44.2

## Contexte
Correction du bug d'animation dans la feature tri-rapidite-reponse (v2.44.1).

## Problème
Les animations de réorganisation des équipes et joueurs ne sont pas visibles lors des changements de position.

## Cause
Le CSS de TeamCard.jsx avait `overflow: hidden` qui crée un stacking context bloquant les animations framer-motion.

## Changements
### 1. server-go/config.json
- Version: 2.44.1 → 2.44.2

### 2. server-go/web/src/components/TeamCard.css
- Ligne 10: `.team-card` - `overflow: hidden` → `overflow: visible`
- Ligne 34: `.team-card-header` - `overflow: hidden` → `overflow: visible`

## Analyse à effectuer

### Vérifier
1. **Impact CSS** : Les animations de layout framer-motion sont maintenant visibles ?
   - Animation spring 300ms au réarrangement des joueurs ✓
   - Animation spring 300ms au réarrangement des équipes ✓

2. **Débordement de contenu** : Pas de texte débordant du container ?
   - `.team-name` utilise `text-overflow: ellipsis` ✓
   - Pas d'effet visuel indésirable ✓

3. **Non-régression** : Les autres animations continuent de fonctionner ?
   - Animations enfants (badges, score flash) ✓
   - CSS transitions (hover) ✓
   - Autre contenu du TeamCard ✓

4. **GamePage.css** : Les overrides sont cohérents ?
   - Ligne 276 force déjà `overflow: visible` - OK ✓
   - Ligne 275 container scrollable - pas d'impact ✓

## Verdict attendu
APPROVED - Correction minimale et ciblée du problème

## Test Manuel
1. Lancer le serveur : `go run ./cmd/server`
2. Naviguer vers `/admin` (GamePage)
3. Lancer un jeu (START)
4. Lancer plusieurs buzzers (Ctrl+Clic sur joueurs)
5. Observer : Les joueurs s'animent-ils lors du réarrangement ?

Expected: Les joueurs glissent en douceur vers le haut (animation spring)
