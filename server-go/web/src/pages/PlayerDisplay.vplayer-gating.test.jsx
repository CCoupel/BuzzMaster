import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Tests : PlayerDisplay — gating des calculs de classement pour VJoueur (#127, T3.1)
//
// Cause racine (_work/reports/dev-frontend-investigation-r2-20260802-201500.md,
// _work/reports/planner-20260802-212049.md) : à la transition PREPARE→READY, chaque
// VJoueur reçoit une rafale de N messages UPDATE (un par PONG reçu côté serveur).
// Avant #127, sortedTeams/sortedPlayers (tri O(n log n) sur TOUTES les équipes/tous
// les joueurs) étaient recalculés à réception de CHACUN de ces messages, alors qu'un
// VJoueur n'affiche ce classement dans AUCUN cas courant.
//
// Point de vigilance découvert en cours d'implémentation (pas dans le plan initial) :
// les vues "Classement Equipes"/"Classement Joueurs" (remote=SCORE/PLAYERS,
// PlayerDisplay.jsx l.1012/1085) ne sont PAS gatées sur !isVPlayer — elles s'affichent
// aussi côté VJoueur quand l'animateur bascule le remote. Un gating inconditionnel sur
// isVPlayer aurait donc cassé cette vue existante. Le fix gate sortedTeams/sortedPlayers
// sur `isVPlayer && remote !== 'SCORE'/'PLAYERS'` — vide uniquement quand la donnée ne
// sera de toute façon jamais rendue. Idem pour la célébration "+X pts" (pointsAnimation,
// rendue SANS garde !isVPlayer en vue GAME) : découplée de sortedTeams/previousRanking
// dans un effet dédié basé sur `teams` brut, pour continuer à fonctionner côté VJoueur
// pendant la partie (remote='GAME') sans réintroduire de tri.
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    enable() { return Promise.resolve() }
    disable() {}
  },
}))

vi.mock('canvas-confetti', () => ({ default: vi.fn() }))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div>,
}))

vi.mock('../components/Podium', () => ({
  default: () => <div data-testid="podium" />,
}))

vi.mock('../components/QRCodeOverlay', () => ({
  default: () => null,
}))

vi.mock('../components/QRCodeDisplay', () => ({
  default: () => null,
}))

vi.mock('./QuestionsPage', () => ({
  CATEGORIES: [],
}))

vi.mock('../constants/colors', () => ({
  getCategoryColor: vi.fn(() => '#8b5cf6'),
}))

vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color)),
}))

vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))

// ---------------------------------------------------------------------------
// Import useGame après les mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Données de test
// ---------------------------------------------------------------------------

const TEAMS_2 = {
  'Équipe A': { NAME: 'Équipe A', SCORE: 10, COLOR: [99, 102, 241] },
  'Équipe B': { NAME: 'Équipe B', SCORE: 5, COLOR: [234, 179, 8] },
}

const BUMPERS_2 = {
  'AA:00:00:00:00:01': { TEAM: 'Équipe A', NAME: 'VJoueur-A', SCORE: 10, IS_VPLAYER: true, CONNECTED: true, COLOR: [99, 102, 241] },
  'AA:00:00:00:00:02': { TEAM: 'Équipe B', NAME: 'VJoueur-B', SCORE: 5, IS_VPLAYER: true, CONNECTED: true, COLOR: [234, 179, 8] },
}

// Simule le payload réduit à venir en Phase 2 backend (T2.1) : bumpers réduit au seul
// destinataire pendant PREPARE/READY. Le frontend ne doit jamais en dépendre pour ne
// pas planter, mais doit rester sain si le backend le fournit déjà.
const BUMPERS_REDUCED_TO_SELF = {
  'AA:00:00:00:00:01': { TEAM: 'Équipe A', NAME: 'VJoueur-A', SCORE: 10, IS_VPLAYER: true, CONNECTED: true, COLOR: [99, 102, 241] },
}

function makeMock({
  phase = 'PREPARE',
  remote = 'GAME',
  teams = TEAMS_2,
  bumpers = BUMPERS_2,
  question = null,
} = {}) {
  return {
    gameState: {
      phase,
      remote,
      timer: 0,
      totalTime: 30,
      question,
      backgrounds: [],
      currentBackgroundIndex: 0,
      newGameBackgrounds: [],
      memoryMatchedPairs: [],
      MEMORY_PARTICIPATING_TEAMS: [],
      MEMOTION_PARTICIPATING_TEAMS: [],
      MEMOTION_CARD_STATES: {},
      MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: null,
      MEMOTION_SELECTED: null,
      qcmInvalidated: [],
    },
    teams,
    bumpers,
    flipMemoryCard: vi.fn(),
    showQRCode: false,
    selectMotionCard: vi.fn(),
  }
}

const renderVPlayer = (overrides = {}) => {
  useGame.mockReturnValue(makeMock(overrides))
  return render(
    <PlayerDisplay
      isVPlayer={true}
      playerName="Joueur1"
      playerNameColor={[99, 102, 241]}
      teamName="Équipe A"
      teamColor={[99, 102, 241]}
    />
  )
}

const renderTV = (overrides = {}) => {
  useGame.mockReturnValue(makeMock(overrides))
  return render(<PlayerDisplay />)
}

beforeEach(() => {
  vi.clearAllMocks()
  // Évite l'erreur "Not implemented: requestFullscreen" dans jsdom (VPlayerPage
  // demande le plein écran au montage — cf. VPlayerPage.jsx, ici exercé via PlayerDisplay).
  Object.defineProperty(document.documentElement, 'requestFullscreen', {
    value: vi.fn().mockResolvedValue(undefined),
    writable: true,
    configurable: true,
  })
})

// ---------------------------------------------------------------------------
// T3.2 — aucun tri exécuté côté VJoueur hors vue SCORE/PLAYERS
// ---------------------------------------------------------------------------

describe('PlayerDisplay — gating VJoueur du classement (#127, CA8)', () => {
  it('isVPlayer=true, remote=GAME (rafale PREPARE→READY) : aucun tri exécuté au montage ni sur UPDATE suivant', () => {
    const sortSpy = vi.spyOn(Array.prototype, 'sort')

    const { rerender } = renderVPlayer({ phase: 'PREPARE', remote: 'GAME' })
    expect(sortSpy).not.toHaveBeenCalled()

    // Simule le message UPDATE suivant de la rafale (#127) : nouvelles références
    // teams/bumpers (comme après un JSON.parse frais), mêmes données modulo un score.
    useGame.mockReturnValue(makeMock({
      phase: 'PREPARE',
      remote: 'GAME',
      teams: { ...TEAMS_2, 'Équipe A': { ...TEAMS_2['Équipe A'], SCORE: 11 } },
      bumpers: { ...BUMPERS_2 },
    }))
    rerender(
      <PlayerDisplay
        isVPlayer={true}
        playerName="Joueur1"
        teamName="Équipe A"
      />
    )

    expect(sortSpy).not.toHaveBeenCalled()
    sortSpy.mockRestore()
  })

  it('isVPlayer=true, phase READY : toujours aucun tri (couvre la transition PREPARE→READY elle-même)', () => {
    const sortSpy = vi.spyOn(Array.prototype, 'sort')

    renderVPlayer({ phase: 'READY', remote: 'GAME' })

    expect(sortSpy).not.toHaveBeenCalled()
    sortSpy.mockRestore()
  })

  it('isVPlayer=true, remote=SCORE : le classement Equipes est bien calculé et affiché (pas de régression sur la vue remote existante)', () => {
    renderVPlayer({ phase: 'STOPPED', remote: 'SCORE' })

    expect(screen.getByTestId('podium')).toBeTruthy()
    expect(screen.getByText('Équipe A')).toBeTruthy()
    expect(screen.getByText('10')).toBeTruthy()
  })

  it('isVPlayer=true, remote=PLAYERS : le classement Joueurs est bien calculé et affiché (pas de régression sur la vue remote existante)', () => {
    renderVPlayer({ phase: 'STOPPED', remote: 'PLAYERS' })

    expect(screen.getByTestId('podium')).toBeTruthy()
    expect(screen.getByText('VJoueur-A')).toBeTruthy()
  })

  it('isVPlayer=false (TV) : le classement Equipes reste calculé, non régressé', () => {
    renderTV({ phase: 'STOPPED', remote: 'SCORE' })

    expect(screen.getByTestId('podium')).toBeTruthy()
    expect(screen.getByText('Équipe A')).toBeTruthy()
    expect(screen.getByText('10')).toBeTruthy()
  })

  it('isVPlayer=false (TV) : le tri s\'exécute toujours normalement en vue SCORE (comportement TV inchangé)', () => {
    const sortSpy = vi.spyOn(Array.prototype, 'sort')

    renderTV({ phase: 'STOPPED', remote: 'SCORE' })

    expect(sortSpy).toHaveBeenCalled()
    sortSpy.mockRestore()
  })

  it('bumpers réduit à 1 entrée (simulation payload backend réduit, PREPARE/READY) : aucun crash, aucun "undefined" affiché', () => {
    expect(() => {
      renderVPlayer({
        phase: 'READY',
        remote: 'GAME',
        bumpers: BUMPERS_REDUCED_TO_SELF,
        teams: TEAMS_2,
      })
    }).not.toThrow()

    expect(document.body.textContent).not.toMatch(/undefined/)
  })

  it('bumpers réduit à 1 entrée en phase PREPARE : aucun crash', () => {
    expect(() => {
      renderVPlayer({
        phase: 'PREPARE',
        remote: 'GAME',
        bumpers: BUMPERS_REDUCED_TO_SELF,
        teams: TEAMS_2,
      })
    }).not.toThrow()
  })
})

// ---------------------------------------------------------------------------
// Fix post-review — code-reviewer #127, Problème Majeur #2 : pendant PREPARE/READY,
// le serveur réduit `bumpers` au seul bumper du VJoueur destinataire (contrat
// vplayer-payload-filter.md §2). Rien n'empêche l'admin de basculer sur la vue
// "Joueurs" (remote=PLAYERS) précisément pendant ces deux phases (GamePage.jsx, bouton
// sans garde de phase) — sans précaution, sortedPlayers y serait recalculé sur ce
// payload réduit et n'afficherait qu'une seule entrée (le VJoueur lui-même), trompeur.
// `teams` n'est jamais réduit, donc remote='SCORE' (sortedTeams) n'est pas concerné.
// ---------------------------------------------------------------------------

describe('PlayerDisplay — fix régression classement Joueurs pendant PREPARE/READY (#127, code-reviewer Majeur #2)', () => {
  it('remote=PLAYERS, phase=PREPARE avec bumpers réduit à 1 entrée : réaffiche le dernier classement complet connu (pas de classement à 1 entrée trompeur)', () => {
    // 1) Vue "Joueurs" affichée normalement (hors PREPARE/READY) avec le classement
    //    complet — alimente le cache du dernier classement complet connu.
    const { rerender } = renderVPlayer({
      phase: 'STOPPED',
      remote: 'PLAYERS',
      teams: TEAMS_2,
      bumpers: BUMPERS_2,
    })
    expect(screen.getByText('VJoueur-A')).toBeTruthy()
    expect(screen.getByText('VJoueur-B')).toBeTruthy()

    // 2) Transition vers PREPARE : le serveur réduit bumpers au seul destinataire.
    //    Le classement affiché doit rester le classement COMPLET précédent, pas se
    //    réduire à la seule entrée reçue.
    useGame.mockReturnValue(makeMock({
      phase: 'PREPARE',
      remote: 'PLAYERS',
      teams: TEAMS_2,
      bumpers: BUMPERS_REDUCED_TO_SELF,
    }))
    rerender(<PlayerDisplay isVPlayer={true} playerName="Joueur1" teamName="Équipe A" />)

    expect(screen.getByText('VJoueur-A')).toBeTruthy()
    expect(screen.getByText('VJoueur-B')).toBeTruthy()
  })

  it('remote=PLAYERS, phase=READY avec bumpers réduit à 1 entrée : réaffiche le dernier classement complet connu', () => {
    const { rerender } = renderVPlayer({
      phase: 'STARTED',
      remote: 'PLAYERS',
      teams: TEAMS_2,
      bumpers: BUMPERS_2,
    })
    expect(screen.getByText('VJoueur-A')).toBeTruthy()
    expect(screen.getByText('VJoueur-B')).toBeTruthy()

    useGame.mockReturnValue(makeMock({
      phase: 'READY',
      remote: 'PLAYERS',
      teams: TEAMS_2,
      bumpers: BUMPERS_REDUCED_TO_SELF,
    }))
    rerender(<PlayerDisplay isVPlayer={true} playerName="Joueur1" teamName="Équipe A" />)

    expect(screen.getByText('VJoueur-A')).toBeTruthy()
    expect(screen.getByText('VJoueur-B')).toBeTruthy()
  })

  it('remote=PLAYERS, phase=PREPARE, aucun classement complet connu au préalable (montage direct) : aucun crash, aucun "undefined" (repli sûr sur liste vide plutôt qu\'un classement à 1 entrée trompeur)', () => {
    expect(() => {
      renderVPlayer({
        phase: 'PREPARE',
        remote: 'PLAYERS',
        teams: TEAMS_2,
        bumpers: BUMPERS_REDUCED_TO_SELF,
      })
    }).not.toThrow()

    expect(document.body.textContent).not.toMatch(/undefined/)
    // Ni classement complet (pas encore connu), ni classement trompeur à 1 entrée.
    expect(screen.queryByText('VJoueur-A')).toBeNull()
  })

  it('remote=SCORE, phase=PREPARE avec bumpers réduit : non concerné — teams n\'est jamais réduit, le classement Equipes reste correct', () => {
    renderVPlayer({
      phase: 'PREPARE',
      remote: 'SCORE',
      teams: TEAMS_2,
      bumpers: BUMPERS_REDUCED_TO_SELF,
    })

    expect(screen.getByText('Équipe A')).toBeTruthy()
    expect(screen.getByText('Équipe B')).toBeTruthy()
  })

  it('remote=SCORE, phase=READY avec bumpers réduit : non concerné — le classement Equipes reste correct', () => {
    renderVPlayer({
      phase: 'READY',
      remote: 'SCORE',
      teams: TEAMS_2,
      bumpers: BUMPERS_REDUCED_TO_SELF,
    })

    expect(screen.getByText('Équipe A')).toBeTruthy()
    expect(screen.getByText('Équipe B')).toBeTruthy()
  })

  it('remote=PLAYERS, phase=STARTED (hors PREPARE/READY) avec bumpers réduit : n\'est PAS concerné par le figeage — recalcule normalement (pas de garde superflue hors des deux phases visées)', () => {
    // Garde-fou de non-sur-correction : le figeage ne doit s'appliquer QU'à
    // PREPARE/READY. Ici bumpers est (artificiellement, hors contrat réel) réduit à 1
    // entrée en dehors de ces phases : le composant doit recalculer sur les données
    // reçues telles quelles (comportement normal), pas figer un ancien classement.
    renderVPlayer({
      phase: 'STARTED',
      remote: 'PLAYERS',
      teams: TEAMS_2,
      bumpers: BUMPERS_REDUCED_TO_SELF,
    })

    expect(screen.getByText('VJoueur-A')).toBeTruthy()
    expect(screen.queryByText('VJoueur-B')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Non-régression : célébration "+X pts" découplée du classement trié (cf. commentaire
// #127 dans PlayerDisplay.jsx au-dessus de previousTeamScoresRef) — doit continuer à
// fonctionner côté VJoueur pendant la partie (remote='GAME'), alors que sortedTeams y
// est désormais toujours vide pour ce client.
// ---------------------------------------------------------------------------

describe('PlayerDisplay — célébration de score indépendante du gating (#127, non-régression)', () => {
  it('isVPlayer=true, remote=GAME : la célébration "+X pts" se déclenche toujours quand une équipe marque', () => {
    const { rerender } = renderVPlayer({
      phase: 'STARTED',
      remote: 'GAME',
      teams: TEAMS_2,
    })

    expect(screen.queryByText(/\+\d+ pts/)).toBeNull()

    useGame.mockReturnValue(makeMock({
      phase: 'STARTED',
      remote: 'GAME',
      teams: { ...TEAMS_2, 'Équipe A': { ...TEAMS_2['Équipe A'], SCORE: 20 } },
    }))
    rerender(
      <PlayerDisplay
        isVPlayer={true}
        playerName="Joueur1"
        teamName="Équipe A"
      />
    )

    expect(screen.getByText('+10 pts')).toBeTruthy()
  })
})
