# Plan d'Implémentation : Tri par Rapidité de Réponse (v2.44.1)

**Statut** : EN PLANIFICATION
**Branche** : `feature/tri-rapidite-reponse`
**Version cible** : 2.44.1
**Type** : Frontend React - Tri dynamique et animations

---

## Résumé Exécutif

Feature de tri dynamique des équipes et joueurs par temps de buzz (rapidité de réponse) sur la page Jeu admin. Affichage du temps réaction en ms avec badges de classement (🏆 🥈 🥉) et animations fluides lors des réorganisations. Scope limité au frontend React, aucune modification backend.

**Phases** :
- Phase 1 : Tri des équipes par temps de buzz
- Phase 2 : Tri des joueurs au sein de chaque équipe
- Phase 3 : Animations fluides (framer-motion)

---

## Phase 1 : ANALYSE PRÉALABLE

### Analyse de la structure actuelle

| Élément | État actuel | Impact |
|---------|-----------|--------|
| **GamePage.jsx** | ✅ Utilise `useMemo` pour tri équipes par score | Adapter pour tri par temps |
| **TeamCard.jsx** | ✅ Accepte `timestamp` et `gameTime` | Paramètres présents, prêt pour affichage |
| **Framer-motion** | ✅ Déjà utilisé pour animations | Utiliser `layoutId` pour réorganisations |
| **GAME_TIME** | ✅ Disponible dans gameState | Timestamp référence en microsecondes |
| **team.TIME** | ✅ Timestamp du premier buzz d'équipe | Clé de tri |
| **bumper.TIME** | ✅ Timestamp du buzz du joueur | Clé de tri joueurs |

### Timestamps disponibles

- `gameState.GAME_TIME` : Timestamp serveur au démarrage du jeu (microsecondes)
- `team.TIME` : Timestamp du premier buzz de l'équipe (microsecondes, 0 si pas buzzé)
- `bumper.TIME` : Timestamp du buzz du joueur (microsecondes, 0 si pas buzzé)

### Phases de jeu pertinentes

- ❌ STOP/PREPARE/READY : Pas de tri, ordre par défaut
- ✅ STARTED : Tri actif, affichage des temps
- ✅ PAUSED : Tri actif, affichage des temps
- ✅ REVEALED : Tri actif, affichage des temps

### Points critiques

1. **Tri stable** : Équipes avec temps égaux préservent l'ordre relatif
2. **Dépendances useMemo** : Ajouter `gameState.PHASE` pour ne trier qu'en STARTED/PAUSED/REVEALED
3. **Calcul temps** : `(team.TIME - gameState.GAME_TIME) / 1000` en millisecondes
4. **layoutId framer-motion** : Clé stable pour chaque équipe/joueur

---

## Phase 2 : TRI DES ÉQUIPES (v2.44.1)

### Tâche 2.1 : Logique de tri dans GamePage.jsx

**Objectif** : Modifier la fonction `sortedTeams` pour trier par temps au lieu du score en phases STARTED/PAUSED/REVEALED.

**Détail** :
```javascript
// Tri tri dépendant de la phase du jeu
const sortedTeams = useMemo(() => {
  const teamsList = Object.entries(teams)
    .map(([name, data]) => ({
      name,
      ...data,
      buzzers: teamBumpers[name] || [],
    }))

  // Tri par temps de réponse si en STARTED/PAUSED/REVEALED
  if (['STARTED', 'PAUSED', 'REVEALED'].includes(gameState.PHASE)) {
    // Séparer équipes buzzées et non buzzées
    const buzzedTeams = teamsList.filter(t => (t.TIME ?? 0) > 0)
    const nonBuzzedTeams = teamsList.filter(t => (t.TIME ?? 0) === 0)

    // Trier équipes buzzées par temps croissant (plus rapide en haut)
    buzzedTeams.sort((a, b) => a.TIME - b.TIME)

    // Garder l'ordre des non-buzzés
    return [...buzzedTeams, ...nonBuzzedTeams]
  } else {
    // Tri par score hors phases de jeu actif
    teamsList.sort((a, b) => {
      const scoreA = a.SCORE ?? 0
      const scoreB = b.SCORE ?? 0
      if (scoreB !== scoreA) return scoreB - scoreA
      const timeA = a.TIME ?? Infinity
      const timeB = b.TIME ?? Infinity
      return timeA - timeB
    })
    return teamsList
  }
}, [teams, teamBumpers, gameState.PHASE])
```

**Fichier** : `server-go/web/src/pages/GamePage.jsx`
**Effort** : 30 min
**Dépendance** : Tâche 2.2 (affichage du temps)

---

### Tâche 2.2 : Affichage du temps dans TeamCard.jsx

**Objectif** : Ajouter l'affichage du temps de réponse en ms à côté du nom d'équipe.

**Détail** :
- Calcul : `timeMs = Math.round((team.TIME - gameState.GAME_TIME) / 1000)`
- Format : `XXXms` en gris clair si équipe buzzée
- Masqué si équipe non buzzée ou phase STOP/PREPARE/READY
- Couleur dégradée : vert pour le plus rapide (classement) → gris pour le reste

**Paramètres à ajouter** :
- `responseTime` : Temps en ms (null si non buzzé)
- `rank` : Rang de classement (1, 2, 3, ou null)
- `showResponseTime` : Boolean pour l'affichage selon phase

**Fichier** : `server-go/web/src/components/TeamCard.jsx`
**Effort** : 45 min
**Dépendance** : Tâche 2.3 (styles)

---

### Tâche 2.3 : Badges de classement

**Objectif** : Ajouter les badges 🏆 🥈 🥉 pour les 3 premiers.

**Détail** :
```javascript
const getRankBadge = (rank) => {
  if (rank === 1) return '🏆'
  if (rank === 2) return '🥈'
  if (rank === 3) return '🥉'
  return null
}
```

**Position** : Avant le nom d'équipe, remplace le numéro d'ordre

**Fichier** : `server-go/web/src/components/TeamCard.jsx`
**Effort** : 20 min
**Dépendance** : Tâche 2.2

---

### Tâche 2.4 : Styles temps/badges dans CSS

**Objectif** : Ajouter les styles pour le temps de réponse et les badges.

**Classes CSS** :
- `.team-response-time` : Temps en ms à droite du nom
- `.rank-badge` : Badges 🏆 🥈 🥉
- `.response-time-rank-1` : Couleur verte (plus rapide)
- `.response-time-rank-2+` : Couleur gris clair

**Fichier** : `server-go/web/src/components/TeamCard.css`
**Effort** : 20 min
**Dépendance** : Tâche 2.3

---

## Phase 3 : TRI DES JOUEURS (v2.44.1)

### Tâche 3.1 : Tri des joueurs dans TeamCard.jsx

**Objectif** : Trier dynamiquement les joueurs au sein de chaque équipe par temps de buzz.

**Détail** :
- Utiliser `useMemo` dans TeamCard
- Séparer joueurs buzzés et non-buzzés
- Trier joueurs buzzés par `bumper.TIME` croissant
- Afficher joueurs non-buzzés en bas

**Code** :
```javascript
const sortedBuzzers = useMemo(() => {
  if (!['STARTED', 'PAUSED', 'REVEALED'].includes(gamePhase)) {
    return buzzers || []
  }

  const buzzed = (buzzers || []).filter(b => (b.timestamp ?? 0) > 0)
  const notBuzzed = (buzzers || []).filter(b => (b.timestamp ?? 0) === 0)

  buzzed.sort((a, b) => a.timestamp - b.timestamp)

  return [...buzzed, ...notBuzzed]
}, [buzzers, gamePhase])
```

**Fichier** : `server-go/web/src/components/TeamCard.jsx`
**Effort** : 30 min
**Dépendance** : Tâche 3.2

---

### Tâche 3.2 : Affichage temps joueur

**Objectif** : Afficher le temps de réponse du joueur en ms.

**Détail** :
- Même calcul que temps équipe : `(bumper.TIME - gameState.GAME_TIME) / 1000` ms
- Taille police plus petite que le temps équipe
- Masqué si phase STOP/PREPARE/READY

**Fichier** : `server-go/web/src/components/TeamCard.jsx`
**Effort** : 20 min
**Dépendance** : Tâche 3.3

---

### Tâche 3.3 : Styles joueurs avec temps

**Objectif** : Ajouter styles pour affichage temps joueur.

**Classes CSS** :
- `.buzzer-response-time` : Temps joueur (plus petit que équipe)
- `.buzzer-row-response-time` : Ligne joueur avec temps

**Fichier** : `server-go/web/src/components/TeamCard.css`
**Effort** : 15 min
**Dépendance** : Tâche 3.2

---

## Phase 4 : ANIMATIONS (v2.44.1)

### Tâche 4.1 : Animation réorganisation équipes

**Objectif** : Animer la réorganisation des équipes quand l'ordre change.

**Détail** :
- Ajouter `layoutId={`team-${team.name}`}` à chaque `motion.div` d'équipe
- Utiliser `sharedLayoutAnimation` de framer-motion
- Transition : spring avec stiffness 300, damping 30
- Durée : 300ms

**Code** :
```javascript
<motion.div
  layoutId={`team-${team.name}`}
  layout
  transition={{ type: 'spring', stiffness: 300, damping: 30 }}
>
  {/* Contenu équipe */}
</motion.div>
```

**Fichier** : `server-go/web/src/pages/GamePage.jsx` + `TeamCard.jsx`
**Effort** : 30 min
**Dépendance** : Tâche 4.2

---

### Tâche 4.2 : Animation réorganisation joueurs

**Objectif** : Animer la réorganisation des joueurs au sein de l'équipe.

**Détail** :
- Ajouter `layoutId={`buzzer-${bumper.mac}`}` à chaque ligne joueur
- Même transition que les équipes : spring 300/30
- Durée : 300ms

**Fichier** : `server-go/web/src/components/TeamCard.jsx`
**Effort** : 20 min
**Dépendance** : Tâche 4.3

---

### Tâche 4.3 : Flash de highlight nouveau buzz

**Objectif** : Animer l'arrivée d'un nouveau buzz avec un flash visuel.

**Détail** :
- Détecter quand `bumper.timestamp` change (nouveau buzz)
- Animation : scale 0.95 → 1.0, couleur accent (500ms)
- Utiliser framer-motion `Animate` avec `key` changeant

**Code** :
```javascript
<motion.div
  key={`${bumper.mac}-${bumper.timestamp}`}
  initial={{ scale: 0.95, opacity: 0.8 }}
  animate={{ scale: 1, opacity: 1 }}
  transition={{ duration: 0.5 }}
>
  {/* Contenu joueur */}
</motion.div>
```

**Fichier** : `server-go/web/src/components/TeamCard.jsx`
**Effort** : 25 min
**Dépendance** : Tâche 4.4

---

### Tâche 4.4 : Tests visuels animations

**Objectif** : Tester les animations sur différentes résolutions et navigateurs.

**Détail** :
- Tester sur Desktop (1920x1080), Laptop (1366x768), Tablet (768x1024)
- Vérifier la fluidité des transitions (60fps)
- Vérifier que les animations ne causent pas de layout shift
- Tester avec DevTools "Reduce motion" activé

**Fichier** : Tests manuels via navigateur
**Effort** : 30 min
**Dépendance** : Aucune

---

## Phase 5 : TESTS & VALIDATION (v2.44.1)

### Tâche 5.1 : Tests unitaires - Tri stable

**Objectif** : Valider que le tri est stable et les calculs de temps corrects.

**Détail** :
- Test 1 : Tri stable - équipes avec temps égaux conservent l'ordre
- Test 2 : Calcul du temps en ms : `(team.TIME - gameState.GAME_TIME) / 1000`
- Test 3 : Équipes non buzzées en bas (TIME = 0)
- Test 4 : Tri joueurs au sein équipe (même logique)

**Fichier** : `server-go/web/src/pages/GamePage.test.jsx`
**Effort** : 45 min

---

### Tâche 5.2 : Tests E2E - Buzz → Tri → Animation

**Objectif** : Tests complets du workflow buzz → réorganisation → animation.

**Scénarios** :
1. **Scenario 1** : Équipe A buze (342ms) → apparaît en haut avec badge 🏆
2. **Scenario 2** : Équipe B buze (456ms) → réorganisation, Équipe A toujours en haut
3. **Scenario 3** : Joueur Alice buzz → s'affiche avec son temps, animation flash
4. **Scenario 4** : Phase REVEALED → tri persiste, temps visibles

**Fichier** : `tests/e2e/tri-rapidite-reponse.md` (scénarios Chrome/MCP)
**Effort** : 60 min

---

### Tâche 5.3 : Tests visuels - Responsive et readability

**Objectif** : Valider l'affichage sur différentes résolutions.

**Breakpoints** :
- Desktop 1920px : Temps et badges lisibles
- Laptop 1366px : Pas de débordement
- Tablet 768px : Colonne équipes rétrécie, temps toujours visible
- Mobile 320px : Responsive adapté

**Fichier** : Tests manuels + screenshots
**Effort** : 40 min

---

## Phase 6 : DOCUMENTATION (v2.44.1)

### Tâche 6.1 : Mise à jour CHANGELOG.md

**Objectif** : Documenter la feature dans le changelog.

**Contenu** :
```markdown
### v2.44.1 - Tri par rapidité de réponse (2024-01-30)

**Nouvelle fonctionnalité**
- Tri dynamique des équipes par temps de buzz sur page Jeu admin
- Tri des joueurs au sein de chaque équipe par temps de réponse
- Affichage du temps de réponse en ms avec couleur dégradée
- Badges de classement (🏆 🥈 🥉) pour les 3 premiers
- Animations fluides lors des réorganisations (framer-motion 300ms)
- Flash highlight sur nouveau buzz (500ms)

**Comportement**
- Tri actif en phases STARTED/PAUSED/REVEALED
- Équipes/joueurs non buzzés en bas de liste
- Temps masqué en phases STOP/PREPARE/READY

**Fichiers modifiés**
- GamePage.jsx : Logique tri équipes
- TeamCard.jsx : Affichage temps, tri joueurs, animations
- GamePage.css, TeamCard.css : Styles nouveau

**Tests**
- Tests unitaires tri stable
- Tests E2E buzz → tri → animation
- Tests responsive et visuels
```

**Fichier** : `CHANGELOG.md`
**Effort** : 15 min

---

### Tâche 6.2 : Commentaires code explicatifs

**Objectif** : Documenter les fonctions clés dans le code source.

**Zones à commenter** :
1. Fonction `sortedTeams` dans GamePage.jsx
2. Fonction `sortedBuzzers` dans TeamCard.jsx
3. Fonctions calcul du temps `getResponseTime()`
4. Fonction badge classement `getRankBadge()`

**Exemple** :
```javascript
// Tri dépendant de la phase du jeu
// En STARTED/PAUSED/REVEALED : tri par temps de buzz (plus rapide en haut)
// Équipes/joueurs non buzzés (TIME=0) en bas
// Tri stable : préserve l'ordre relatif si temps égaux
const sortedTeams = useMemo(() => {
  // ... tri logic
}, [teams, teamBumpers, gameState.PHASE])
```

**Fichier** : `GamePage.jsx`, `TeamCard.jsx`
**Effort** : 20 min

---

### Tâche 6.3 : Notes techniques dans CLAUDE.md

**Objectif** : Documenter l'implémentation technique pour futures références.

**Contenu** :
```markdown
### Tri par Rapidité de Réponse (v2.44.1)

**Principe** :
- Tri dynamique des équipes/joueurs par `TIME` (timestamp du buzz)
- Actif uniquement en phases STARTED/PAUSED/REVEALED
- Équipes/joueurs buzzés (TIME > 0) en haut, triés par temps croissant
- Équipes/joueurs non buzzés (TIME = 0) en bas

**Calcul du temps en ms** :
```
timeMs = Math.round((entity.TIME - gameState.GAME_TIME) / 1000)
```

**Implémentation** :
- useMemo dans GamePage.jsx pour tri équipes
- useMemo dans TeamCard.jsx pour tri joueurs
- layoutId framer-motion pour animations réorganisation (300ms)
- Flash animation sur nouveau buzz (500ms)

**Badges classement** :
- Rang 1 : 🏆
- Rang 2 : 🥈
- Rang 3 : 🥉
- Rang 4+ : pas de badge

**Phase-specific behavior** :
| Phase | Tri | Affichage temps |
|-------|-----|-----------------|
| STOP/PREPARE/READY | Non | Masqué |
| STARTED | Oui | Visible |
| PAUSED | Oui | Visible |
| REVEALED | Oui | Visible |
```

**Fichier** : `CLAUDE.md` (section "Tri par Rapidité de Réponse")
**Effort** : 20 min

---

## Dépendances entre tâches

```
Tâche 2.1 (Tri équipes logic)
    ↓
Tâche 2.2 (Affichage temps) → Tâche 2.3 (Badges) → Tâche 2.4 (CSS)
    ↓
Tâche 3.1 (Tri joueurs) → Tâche 3.2 (Temps joueur) → Tâche 3.3 (CSS joueur)
    ↓
Tâche 4.1 (Anim équipes) → Tâche 4.2 (Anim joueurs) → Tâche 4.3 (Flash buzz)
    ↓
Tâche 4.4 (Tests visuels)
    ↓
Tâche 5.1 (Tests unitaires)
    ↓
Tâche 5.2 (Tests E2E)
    ↓
Tâche 5.3 (Tests responsive)
    ↓
Tâche 6.1 (CHANGELOG) → Tâche 6.2 (Comments) → Tâche 6.3 (CLAUDE.md)
```

---

## Points Critiques et Risques

| Risque | Probabilité | Impact | Mitigation |
|--------|------------|--------|-----------|
| **Tri non-stable lors de temps égaux** | Faible | Moyen | Tests unitaires tri stable |
| **Animations cause layout shift** | Moyen | Moyen | Tester avec DevTools, utiliser layoutId |
| **Temps non visible en petites résolutions** | Faible | Moyen | Tests responsive, adapter font-size |
| **Calcul temps incorrect (unité)** | Très faible | Élevé | Double-check conversion µs → ms |
| **Performance useMemo dépendances** | Très faible | Faible | Vérifier que dépendances correctes |

---

## Estimation d'Effort

| Phase | Tâches | Durée |
|-------|--------|-------|
| **2. Tri équipes** | 2.1-2.4 | 2h 15min |
| **3. Tri joueurs** | 3.1-3.3 | 1h 25min |
| **4. Animations** | 4.1-4.4 | 1h 45min |
| **5. Tests** | 5.1-5.3 | 2h 25min |
| **6. Documentation** | 6.1-6.3 | 55min |
| **TOTAL** | **16 tâches** | **8h 45min** |

---

## Livrables Finaux

1. ✅ **Code modifié** :
   - GamePage.jsx (tri équipes + layoutId)
   - TeamCard.jsx (tri joueurs + affichage temps + animations)
   - GamePage.css (styles temps équipes)
   - TeamCard.css (styles temps joueurs + animations)

2. ✅ **Tests** :
   - `GamePage.test.jsx` (tri stable, calculs temps)
   - `tests/e2e/tri-rapidite-reponse.md` (scénarios E2E Chrome)

3. ✅ **Documentation** :
   - CHANGELOG.md (v2.44.1)
   - CLAUDE.md (notes techniques)
   - Commentaires inline (fonctions clés)

4. ✅ **Build** :
   - Version 2.44.1
   - Branche `feature/tri-rapidite-reponse`
   - Tests passants (100% coverage nouveau code)

---

## Prochaines Étapes (après PLAN validation)

1. **Phase 1 (PLAN)** → Validation utilisateur ✅ (ce document)
2. **Phase 2 (DEV)** → dev-frontend implémente toutes les tâches
3. **Phase 3 (REVIEW)** → code-reviewer valide code quality
4. **Phase 4 (QA)** → QA exécute tests unitaires + E2E
5. **Phase 5 (DOC)** → doc-updater finalise documentation
6. **Phase 6 (QUALIF)** → deploy crée archive QUALIF

---

**Plan créé** : 2024-01-30
**Chef de projet** : Claude Code (CDP)
**Version cible** : 2.44.1
**Branche** : feature/tri-rapidite-reponse
