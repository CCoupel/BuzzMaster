# Dev Frontend - Feature Tri par Rapidité de Réponse (v2.44.1)

Tu es l'agent `dev-frontend` responsable d'implémenter la feature "Tri par rapidité de réponse" dans BuzzControl.

## CONTEXTE

- Projet : BuzzControl (système de buzzers pour quiz)
- Feature : Tri dynamique équipes/joueurs par temps de buzz
- Branche : feature/tri-rapidite-reponse
- Version cible : v2.44.1
- Type : Frontend React uniquement
- Plan : PLAN_TRI_RAPIDITE_v2.44.1.md

## POINTS CRITIQUES

- Tri actif UNIQUEMENT en phases STARTED/PAUSED/REVEALED
- Tri stable : préserver ordre relatif si temps égaux
- Équipes/joueurs non buzzés (TIME=0) toujours en bas
- Calcul temps : (entity.TIME - gameState.GAME_TIME) / 1000 en ms
- Animations : framer-motion layoutId + spring transitions (300ms)
- Flash : nouveau buzz trigger animation (500ms)

---

## TÂCHE 1 : TRI DES ÉQUIPES

**Fichier** : `server-go/web/src/pages/GamePage.jsx`

Modifier la fonction `sortedTeams` (ligne 64) :

**ANCIEN CODE (actuel)** :
```javascript
const sortedTeams = useMemo(() => {
  return Object.entries(teams)
    .map(([name, data]) => ({
      name,
      ...data,
      buzzers: teamBumpers[name] || [],
    }))
    .sort((a, b) => {
      const scoreA = a.SCORE ?? 0
      const scoreB = b.SCORE ?? 0
      if (scoreB !== scoreA) return scoreB - scoreA
      const timeA = a.TIME ?? Infinity
      const timeB = b.TIME ?? Infinity
      return timeA - timeB
    })
}, [teams, teamBumpers])
```

**NOUVEAU CODE** :
```javascript
const sortedTeams = useMemo(() => {
  const teamsList = Object.entries(teams)
    .map(([name, data]) => ({
      name,
      ...data,
      buzzers: teamBumpers[name] || [],
    }))

  // Tri par temps de réponse si en STARTED/PAUSED/REVEALED
  if (['STARTED', 'PAUSED', 'REVEALED'].includes(gameState.PHASE)) {
    // Séparer équipes buzzées et non-buzzées
    const buzzedTeams = teamsList.filter(t => (t.TIME ?? 0) > 0)
    const nonBuzzedTeams = teamsList.filter(t => (t.TIME ?? 0) === 0)

    // Trier équipes buzzées par temps croissant (plus rapide en haut)
    buzzedTeams.sort((a, b) => a.TIME - b.TIME)

    // Garder l'ordre original des non-buzzés
    return [...buzzedTeams, ...nonBuzzedTeams]
  } else {
    // Tri par score hors phases de jeu actif (STOP, PREPARE, READY)
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

**CRITICAL**: Ajouter `gameState.PHASE` aux dépendances useMemo !

---

## TÂCHE 2 : PASSER LES PARAMÈTRES À TEAMCARD

**Fichier** : `server-go/web/src/pages/GamePage.jsx`

Dans le rendu de TeamCard (ligne ~250), passer les paramètres supplémentaires :

```javascript
<TeamCard
  key={team.name}
  name={team.name}
  color={team.COLOR}
  score={team.SCORE || 0}
  teamPoints={team.TEAM_POINTS || 0}
  ready={team.STATUS === 'READY'}
  active={team.TIME !== undefined && team.TIME > 0}
  timestamp={team.TIME}
  gameTime={gameState.GAME_TIME}
  gamePhase={gameState.PHASE}  // NOUVEAU
  rank={sortedTeams.findIndex(t => t.name === team.name) + 1}  // NOUVEAU
  showResponseTime={['STARTED', 'PAUSED', 'REVEALED'].includes(gameState.PHASE)}  // NOUVEAU
  buzzers={team.buzzers}
  onClick={() => handleTeamClick(team.name)}
  onTeamClick={team.buzzers.length > 0 ? () => setTeamPoints(team.name) : null}
  onPlayerClick={(bumperMac) => setBumperPoints(bumperMac)}
  className={team.STATUS === 'PAUSE' ? 'paused' : ''}
  waitingForReady={gameState.PHASE === 'PREPARE'}
  waitingForBuzz={gameState.PHASE === 'STARTED'}
  pointsTarget={gameState.question?.POINTS_TARGET || 'PLAYER'}
  qcmPenaltyConfig={gameState.question?.QCM_HINTS_ENABLED ? {
    penalty1: gameState.question.QCM_PENALTY_1 || 0.67,
    penalty2: gameState.question.QCM_PENALTY_2 || 0.33,
  } : null}
/>
```

---

## TÂCHE 3 : IMPLÉMENTER TEAMCARD.JSX

**Fichier** : `server-go/web/src/components/TeamCard.jsx`

**1. Ajouter les nouveaux props** :
```javascript
export default function TeamCard({
  name,
  color,
  score = 0,
  teamPoints = 0,
  ready = false,
  active = false,
  timestamp,
  gameTime,
  gamePhase,  // NOUVEAU
  rank,  // NOUVEAU (1, 2, 3, ou plus)
  showResponseTime,  // NOUVEAU (boolean)
  buzzers = [],
  onClick,
  onTeamClick,
  onPlayerClick,
  className = '',
  waitingForReady = false,
  waitingForBuzz = false,
  pointsTarget = null,
  qcmPenaltyConfig = null,
}) {
```

**2. Ajouter calcul temps et badge** :
```javascript
// Calcul du temps de réponse en ms
const responseTime = timestamp && gameTime
  ? Math.round((timestamp - gameTime) / 1000)
  : null

// Badge de classement
const getRankBadge = (r) => {
  if (r === 1) return '🏆'
  if (r === 2) return '🥈'
  if (r === 3) return '🥉'
  return null
}

const rankBadge = rank && showResponseTime ? getRankBadge(rank) : null
```

**3. Trier les joueurs DANS TeamCard** :
```javascript
// Tri des joueurs au sein de l'équipe
const sortedBuzzers = useMemo(() => {
  if (!['STARTED', 'PAUSED', 'REVEALED'].includes(gamePhase)) {
    return buzzers || []
  }

  const buzzed = (buzzers || []).filter(b => (b.timestamp ?? 0) > 0)
  const notBuzzed = (buzzers || []).filter(b => (b.timestamp ?? 0) === 0)

  // Tri stable : trier par timestamp croissant
  buzzed.sort((a, b) => a.timestamp - b.timestamp)

  return [...buzzed, ...notBuzzed]
}, [buzzers, gamePhase])
```

Importer `useMemo` depuis React si pas déjà fait.

**4. Modifier le header TeamCard pour badge et temps** :

ANCIEN (ligne ~83) :
```javascript
<h3 className="team-name">{name}</h3>
```

NOUVEAU :
```javascript
<div className="team-header-content">
  {rankBadge && <span className="rank-badge">{rankBadge}</span>}
  <h3 className="team-name">{name}</h3>
  {showResponseTime && responseTime !== null && (
    <span className="team-response-time">{responseTime}ms</span>
  )}
</div>
```

**5. Modifier rendu joueurs pour utiliser `sortedBuzzers`** :

Remplacer :
```javascript
{buzzers.map((buzzer) => (
```

Par :
```javascript
{sortedBuzzers.map((buzzer) => (
```

**6. Ajouter temps du joueur et animations** :

Pour chaque ligne joueur (ligne ~130+), modifier la structure :

```javascript
<motion.div
  key={`${buzzer.mac}-${buzzer.timestamp}`}
  layoutId={`buzzer-${buzzer.mac}`}
  layout
  initial={{ scale: 0.95, opacity: 0.8 }}
  animate={{ scale: 1, opacity: 1 }}
  transition={{ type: 'spring', stiffness: 300, damping: 30 }}
  className="buzzer-row"
>
  {/* Contenu existant du joueur */}
  <span className="buzzer-name">{buzzer.name}</span>

  {/* NOUVEAU : Afficher temps réponse joueur */}
  {showResponseTime && buzzer.timestamp > 0 && (
    <span className="buzzer-response-time">
      {Math.round((buzzer.timestamp - gameTime) / 1000)}ms
    </span>
  )}

  {/* ...reste du contenu (score, couleur QCM, etc.) */}
</motion.div>
```

**7. Ajouter layoutId à la carte équipe** :

Modifier la première `<motion.div` (ligne 70) :

```javascript
<motion.div
  layoutId={`team-${name}`}
  layout
  className={`team-card ...`}
  style={{ '--team-color': rgbColor }}
  initial={{ opacity: 0, y: 20 }}
  animate={{ opacity: 1, y: 0 }}
  transition={{ type: 'spring', stiffness: 300, damping: 30 }}
>
```

---

## TÂCHE 4 : STYLES GAMEPAGE.CSS

**Fichier** : `server-go/web/src/pages/GamePage.css`

Ajouter à la fin du fichier :

```css
/* Tri par rapidité de réponse - Styles équipes */

.team-header-content {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex: 1;
}

.rank-badge {
  font-size: 1.5rem;
  line-height: 1;
  margin-right: 0.25rem;
}

.team-response-time {
  font-size: 0.85rem;
  color: var(--gray-400);
  margin-left: auto;
  padding-right: 0.5rem;
  font-weight: 500;
  white-space: nowrap;
}

/* Couleur progressive pour temps (vert → gris) */
.game-page .teams-grid .team-card:nth-child(1) .team-response-time {
  color: var(--success);
  font-weight: 600;
}

.game-page .teams-grid .team-card:nth-child(2) .team-response-time {
  color: var(--success-light, #86efac);
}

.game-page .teams-grid .team-card:nth-child(3) .team-response-time {
  color: var(--warning-light, #fcd34d);
}
```

---

## TÂCHE 5 : STYLES TEAMCARD.CSS

**Fichier** : `server-go/web/src/components/TeamCard.css`

Ajouter à la fin du fichier :

```css
/* Tri par rapidité de réponse - Styles joueurs */

.buzzer-response-time {
  font-size: 0.75rem;
  color: var(--gray-400);
  margin-left: auto;
  padding: 0.25rem 0.5rem;
  font-weight: 400;
  white-space: nowrap;
}

/* Animation réorganisation joueurs */
.buzzer-row {
  transition: all 0.3s cubic-bezier(0.4, 0.0, 0.2, 1);
}

@keyframes buzz-flash {
  0% {
    background-color: rgba(34, 197, 94, 0.2);
    scale: 0.95;
  }
  50% {
    background-color: rgba(34, 197, 94, 0.1);
  }
  100% {
    background-color: transparent;
    scale: 1;
  }
}

/* Responsive : adapter taille police temps sur petits écrans */
@media (max-width: 768px) {
  .team-response-time {
    font-size: 0.75rem;
  }

  .buzzer-response-time {
    font-size: 0.65rem;
  }
}

@media (max-width: 480px) {
  .team-response-time {
    font-size: 0.7rem;
  }

  .buzzer-response-time {
    font-size: 0.6rem;
  }
}
```

---

## TÂCHE 6 : TESTS UNITAIRES

**Créer fichier** : `server-go/web/src/pages/GamePage.test.jsx`

```javascript
import { render, screen } from '@testing-library/react'
import GamePage from './GamePage'

describe('GamePage - Tri par rapidité de réponse', () => {
  // Test 1 : Calcul temps en ms
  test('Calcul temps: (team.TIME - gameState.GAME_TIME) / 1000', () => {
    const gameTime = 1000000000
    const teamTime = 1000100000
    const expected = 100  // ms

    const result = Math.round((teamTime - gameTime) / 1000)
    expect(result).toBe(expected)
  })

  // Test 2 : Tri par temps (plus rapide en haut)
  test('Équipes triées par temps croissant (rapide → lent)', () => {
    const teams = [
      { TIME: 1000150000 },    // 150ms
      { TIME: 1000100000 },    // 100ms (plus rapide)
      { TIME: 1000200000 },    // 200ms
    ]

    const sorted = teams.sort((a, b) => a.TIME - b.TIME)

    expect(sorted[0].TIME).toBe(1000100000)  // 100ms en premier
    expect(sorted[1].TIME).toBe(1000150000)  // 150ms en second
    expect(sorted[2].TIME).toBe(1000200000)  // 200ms en dernier
  })

  // Test 3 : Équipes non buzzées en bas
  test('Équipes avec TIME=0 toujours en bas', () => {
    const buzzedTeams = [{ TIME: 1000100000 }]
    const nonBuzzedTeams = [{ TIME: 0 }]

    const result = [...buzzedTeams, ...nonBuzzedTeams]

    expect(result[0].TIME).toBeGreaterThan(0)
    expect(result[1].TIME).toBe(0)
  })

  // Test 4 : Tri stable - même temps = ordre préservé
  test('Tri stable: même temps conserve l\'ordre', () => {
    const teams = [
      { name: 'A', TIME: 1000100000 },
      { name: 'B', TIME: 1000100000 },
      { name: 'C', TIME: 1000100000 },
    ]

    const sorted = [...teams].sort((a, b) => a.TIME - b.TIME)

    expect(sorted[0].name).toBe('A')
    expect(sorted[1].name).toBe('B')
    expect(sorted[2].name).toBe('C')
  })

  // Test 5 : Phase-aware - tri uniquement en STARTED/PAUSED/REVEALED
  test('Tri actif UNIQUEMENT en STARTED/PAUSED/REVEALED', () => {
    const phases = ['STARTED', 'PAUSED', 'REVEALED']
    phases.forEach(phase => {
      expect(['STARTED', 'PAUSED', 'REVEALED'].includes(phase)).toBe(true)
    })

    const excludedPhases = ['STOP', 'PREPARE', 'READY']
    excludedPhases.forEach(phase => {
      expect(['STARTED', 'PAUSED', 'REVEALED'].includes(phase)).toBe(false)
    })
  })
})
```

---

## TÂCHE 7 : TESTS E2E

**Créer fichier** : `server-go/tests/e2e/tri-rapidite-reponse.md`

```markdown
# Tests E2E : Tri par Rapidité de Réponse (v2.44.1)

## Prérequis
- Serveur démarré sur http://localhost
- Admin connecté à /admin
- TV affichage sur /tv
- 3-4 équipes créées avec joueurs

## Scénario 1 : Buzz première équipe
### Étapes
1. Sélectionner une question
2. Cliquer START (30s)
3. Après ~2s, cliquer sur équipe "Les Rouges" → buzz

### Vérification
- [ ] "Les Rouges" remonte en haut
- [ ] Badge 🏆 avant le nom
- [ ] Temps affiché : ~2000ms (±500ms)
- [ ] Animation fluide

## Scénario 2 : Buzz deuxième équipe
### Étapes
1. Après buzz Les Rouges
2. Attendre ~3s
3. Cliquer sur "Les Bleus" → buzz

### Vérification
- [ ] Les Bleus se place après Les Rouges
- [ ] Badge 🥈 avant Les Bleus
- [ ] Temps correct : ~5000ms
- [ ] Animation fluide

## Scénario 3 : Tri joueurs au sein équipe
### Étapes
1. Vérifier tri joueurs dans chaque équipe
2. Cliquer sur joueur → buzz
3. Vérifier réorganisation

### Vérification
- [ ] Joueur apparaît en haut de sa liste
- [ ] Temps joueur affiché
- [ ] Flash animation (500ms)
- [ ] Autres joueurs en bas

## Scénario 4 : Phase PAUSED
### Étapes
1. Après START, cliquer PAUSE
2. Vérifier tri

### Vérification
- [ ] Équipes restent triées (pas retour au score)
- [ ] Temps et badges visibles
- [ ] Tri stable

## Scénario 5 : Phase REVEALED
### Étapes
1. Cliquer REPONSE (REVEALED)
2. Vérifier tri

### Vérification
- [ ] Tri persiste
- [ ] Temps et badges visibles

## Scénario 6 : Retour à STOP
### Étapes
1. Cliquer STOP
2. Sélectionner nouvelle question

### Vérification
- [ ] Équipes triées par SCORE (ancien comportement)
- [ ] Temps masqués
- [ ] Badges disparaissent

## Scénario 7 : Responsive - Tablet
### Étapes
1. Redimensionner à 768x1024
2. Vérifier affichage en STARTED

### Vérification
- [ ] Temps visible
- [ ] Pas de débordement
- [ ] Animations fluides

## Scénario 8 : Responsive - Mobile
### Étapes
1. Redimensionner à 320x640
2. Vérifier affichage

### Vérification
- [ ] Temps visible
- [ ] Pas de débordement
- [ ] Animations fluides
```

---

## RÈGLES CRITIQUES À RESPECTER

1. ❌ NE PAS modifier les contrats API (websocket-actions.md, game-state.md)
2. ❌ NE PAS modifier le backend (Go)
3. ✅ Frontend UNIQUEMENT : React, CSS, tests E2E
4. ✅ Commits atomiques avec messages clairs
5. ✅ useMemo avec dépendances correctes
6. ✅ Tests unitaires + E2E complets
7. ✅ Responsive : mobile 320px+

---

## COMMITS À FAIRE

Après chaque tâche majeure :

```bash
git add [fichiers modifiés]
git commit -m "feat(tri-rapidite): [Description]

- Détail 1
- Détail 2

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

**Exemples de commits** :
1. feat(tri-rapidite): Implémenter tri équipes par temps de buzz
2. feat(tri-rapidite): Afficher temps réponse et badges classement
3. feat(tri-rapidite): Implémenter tri joueurs avec animations
4. feat(tri-rapidite): Ajouter styles CSS temps et animations
5. test(tri-rapidite): Ajouter tests unitaires et E2E

---

## LIVRABLES ATTENDUS

1. ✅ Code modifié :
   - GamePage.jsx (sortedTeams + paramètres TeamCard)
   - TeamCard.jsx (tri joueurs + temps + animations)
   - GamePage.css (styles équipes)
   - TeamCard.css (styles joueurs + animations)

2. ✅ Tests :
   - GamePage.test.jsx (tests unitaires)
   - tests/e2e/tri-rapidite-reponse.md (scénarios E2E)

3. ✅ Git history :
   - 5-6 commits atomiques
   - Messages clairs
   - Tous les tests passants

4. ✅ Version :
   - config.json à 2.44.1
   - Prêt pour REVIEW

---

**Commence maintenant ! Bonne chance !**
