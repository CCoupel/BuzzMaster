import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// AnimPage — RAFALE, milestone v8.0.0 #16/#107/#199, révision maquette
// rafale-v8.html §9.2/§9.3 (SHA 7d5320ac) : encart question+réponse fusionné
// (.rafale-anim-qcard, plus de AnimAnswerZone/hold-to-peek pour ce type),
// panneau équipes enrichi (3 compteurs : bonnes/RAFALE_TEAM_COUNTERS,
// mauvaises/RAFALE_TEAM_ERRORS, série/RAFALE_TEAM_STREAK).
//
// Fichier absent au moment où ce lot est livré (signalé explicitement par
// dev-frontend) — le nouvel encart + panneau n'avaient qu'une couverture
// indirecte (build/lint). Même patron de mocks que AnimPage.test.jsx :
// AnimConductPanel/AnimAnswerZone mockés (couverture exhaustive ailleurs),
// AnimTeamCard réel (déjà le cas dans AnimPage.test.jsx).
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    constructor() { this.isEnabled = false }
    enable() { this.isEnabled = true; return Promise.resolve() }
    disable() { this.isEnabled = false }
  },
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(),
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime, phase }) => <div data-testid="timer" data-phase={phase}>{currentTime}</div>,
}))

// #166/F5 — mock de câblage (même patron que AnimPage.test.jsx) : capture
// les props RAFALE reçues (data-attributes + boutons) pour vérifier le
// câblage vers rafaleValidate/rafaleInvalidate, sans dépendre du rendu réel
// d'AnimConductPanel (sa propre couverture exhaustive vit dans
// AnimConductPanel.test.jsx / AnimRafaleActions.rafale.test.jsx).
// #198 (retour QUALIF 8.0.0.13) — le rendu visuel de l'encart
// question+réponse a déménagé dans AnimRafaleQuestion.jsx (zone centrale,
// montée PAR AnimConductPanel), sa propre couverture vit désormais dans
// AnimConductPanel.test.jsx. Ce mock expose donc `props.rafale` tel quel
// via data-attributes : ce fichier vérifie que AnimPage.jsx calcule et
// transmet les BONNES valeurs (câblage), pas le rendu final.
vi.mock('../components/AnimConductPanel', () => ({
  default: (props) => (
    <div
      data-testid="conduct-panel"
      data-phase={props.phase}
      data-rafale-disabled={String(!!props.rafaleDisabled)}
      data-rafale-question={props.rafale?.current?.QUESTION ?? ''}
      data-rafale-answer={props.rafale?.answerValue ?? ''}
      data-rafale-team={props.rafale?.teamName ?? ''}
      data-rafale-team-color-css={props.rafale?.teamColorCss ?? ''}
      data-rafale-cat-label={props.rafale?.catMeta?.label ?? ''}
      data-rafale-difficulty={props.rafale?.current?.DIFFICULTY ?? ''}
    >
      <button onClick={() => props.onRafaleValidate?.()}>MOCK_RAFALE_VALIDATE</button>
      <button onClick={() => props.onRafaleInvalidate?.()}>MOCK_RAFALE_INVALIDATE</button>
    </div>
  ),
}))

vi.mock('../components/AnimAnswerZone', () => ({
  default: (props) => (
    <div data-testid="answer-zone" data-question-id={props.question?.ID ?? ''} data-revealed={String(!!props.revealed)} />
  ),
}))

vi.mock('../components/AnimArdoiseList', () => ({
  default: () => <div data-testid="ardoise-list" />,
}))

// ⚠️ `...overrides` spreadé EN PREMIER — voir le piège documenté dans
// GamePage.rafaleStartGate.test.jsx : un `overrides.gameState` partiel ne
// doit jamais écraser silencieusement la clé `gameState` fusionnée juste
// après.
function makeGameMock(overrides = {}) {
  return {
    ...overrides,
    status: 'connected',
    gameState: {
      phase: 'STARTED',
      timer: 90,
      totalTime: 120,
      gameTime: 0,
      question: { ID: 'rq1', TYPE: 'RAFALE', RAFALE_QUESTION_TIME: 3 },
      RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
      RAFALE_QUESTION_TIME: 2,
      RAFALE_CURRENT_TEAM: '',
      RAFALE_CURRENT_TEAM_COLOR: [],
      RAFALE_PARTICIPATING_TEAMS: [],
      RAFALE_TEAM_COUNTERS: {},
      RAFALE_TEAM_ERRORS: {},
      RAFALE_TEAM_STREAK: {},
      RAFALE_ASKED_COUNT: 0,
      ...overrides.gameState,
    },
    teams: overrides.teams ?? { 'Équipe A': { NAME: 'Équipe A', COLOR: [99, 102, 241] } },
    bumpers: overrides.bumpers ?? { 'AA:00': { TEAM: 'Équipe A', NAME: 'J1' } },
    nextQuestion: null,
    questionPosition: { position: 0, total: 0 },
    awardedTeams: {},
    creditPoints: 0,
    startGame: vi.fn(),
    stopGame: vi.fn(),
    pauseGame: vi.fn(),
    continueGame: vi.fn(),
    revealAnswer: vi.fn(),
    selectQuestion: vi.fn(),
    setTeamPoints: vi.fn(),
    setBumperPoints: vi.fn(),
    flipMemoryCard: vi.fn(),
    regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' },
    clearRegieMessage: vi.fn(),
    selectMotionCard: vi.fn(),
    flipMotionCard: vi.fn(),
    stopMotionTimer: vi.fn(),
    revealMotionCard: vi.fn(),
    doneMotionCard: vi.fn(),
    // RAFALE (v8.0.0, #16/#107, contrat rafale.md §5.1/§5.2)
    rafaleAnswer: overrides.rafaleAnswer ?? { ID: 'r-042', ANSWER: 'Rome' },
    rafaleValidate: overrides.rafaleValidate ?? vi.fn(),
    rafaleInvalidate: overrides.rafaleInvalidate ?? vi.fn(),
  }
}

beforeEach(() => {
  useGame.mockReturnValue(makeGameMock())
  useCategories.mockReturnValue({ categories: [] })
})

afterEach(() => {
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Encart question+réponse fusionné (§9.2) — plus de AnimAnswerZone/
// hold-to-peek pour RAFALE.
// ---------------------------------------------------------------------------

describe('AnimPage — RAFALE : câblage du prop `rafale` vers AnimConductPanel (§9.2, zone centrale)', () => {
  // #198 (retour QUALIF 8.0.0.13) — l'encart lui-même (.rafale-anim-qcard)
  // est désormais rendu par AnimRafaleQuestion.jsx, monté PAR
  // AnimConductPanel (mocké ici, cf. commentaire du mock ci-dessus) : ce
  // fichier vérifie que AnimPage.jsx calcule et transmet les bonnes
  // valeurs dans `props.rafale`, pas le rendu visuel final (couvert par
  // AnimConductPanel.test.jsx).
  it('phase STARTED, TYPE=RAFALE : PAS de AnimAnswerZone (masquage hold-to-peek retiré pour ce type)', () => {
    useGame.mockReturnValue(makeGameMock())
    render(<AnimPage />)

    expect(screen.queryByTestId('answer-zone')).not.toBeInTheDocument()
  })

  it('transmet la question ET la réponse ENSEMBLE, sans masquage (rafaleAnswer déjà résolu)', () => {
    render(<AnimPage />)

    const panel = screen.getByTestId('conduct-panel')
    expect(panel).toHaveAttribute('data-rafale-question', 'Capitale de l\'Italie ?')
    expect(panel).toHaveAttribute('data-rafale-answer', 'Rome')
  })

  it('rafaleAnswer.ID ne correspond PAS à la question courante (réponse pas encore reçue) : answerValue vide, la question reste transmise', () => {
    useGame.mockReturnValue(makeGameMock({ rafaleAnswer: { ID: 'r-999', ANSWER: 'Berlin' } }))
    render(<AnimPage />)

    const panel = screen.getByTestId('conduct-panel')
    expect(panel).toHaveAttribute('data-rafale-question', 'Capitale de l\'Italie ?')
    expect(panel).toHaveAttribute('data-rafale-answer', '')
  })

  it('transmet catégorie et difficulté résolues', () => {
    useGame.mockReturnValue(makeGameMock())
    useCategories.mockReturnValue({ categories: [] })
    render(<AnimPage />)

    const panel = screen.getByTestId('conduct-panel')
    expect(panel.getAttribute('data-rafale-cat-label')).toMatch(/Geographie/)
    expect(panel).toHaveAttribute('data-rafale-difficulty', '2')
  })

  it('câble RafaleTimers avec le timer de manche et le timer de question (Timer mocké — même patron que AnimPage.test.jsx, valeurs brutes transmises)', () => {
    const { container } = render(<AnimPage />)
    const displays = Array.from(container.querySelectorAll('[data-testid="timer"]')).map((el) => el.textContent)
    expect(displays).toContain('90') // gameState.timer (manche)
    expect(displays).toContain('2') // RAFALE_QUESTION_TIME (question)
  })

  it('mode multi, équipe active définie : son nom est transmis dans `rafale.teamName`', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { RAFALE_CURRENT_TEAM: 'Équipe A', RAFALE_CURRENT_TEAM_COLOR: [99, 102, 241] },
    }))
    render(<AnimPage />)
    const panel = screen.getByTestId('conduct-panel')
    expect(panel).toHaveAttribute('data-rafale-team', 'Équipe A')
    expect(panel).toHaveAttribute('data-rafale-team-color-css', 'rgb(99,102,241)')
  })
})

// ---------------------------------------------------------------------------
// Non-régression AnimAnswerZone (hold-to-peek) — les AUTRES types continuent
// de passer par AnimAnswerZone sans changement (retrait scopé RAFALE
// uniquement).
// ---------------------------------------------------------------------------

describe('AnimPage — non-régression : les autres types continuent d\'utiliser AnimAnswerZone', () => {
  it('TYPE=QCM : AnimAnswerZone est monté, aucun rafale.current transmis (rafale=null)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        question: { ID: 'q1', TYPE: 'QCM' },
        RAFALE_CURRENT_QUESTION: {},
      },
    }))
    render(<AnimPage />)

    expect(screen.getByTestId('answer-zone')).toBeInTheDocument()
    expect(screen.getByTestId('answer-zone')).toHaveAttribute('data-question-id', 'q1')
    expect(screen.getByTestId('conduct-panel')).toHaveAttribute('data-rafale-question', '')
  })

  it('TYPE=SPEEDY : AnimAnswerZone est monté, .rafale-anim-qcard absent', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        question: { ID: 'q2', TYPE: 'SPEEDY' },
        RAFALE_CURRENT_QUESTION: {},
      },
    }))
    render(<AnimPage />)

    expect(screen.getByTestId('answer-zone')).toBeInTheDocument()
    expect(screen.getByTestId('answer-zone')).toHaveAttribute('data-question-id', 'q2')
  })
})

// ---------------------------------------------------------------------------
// Panneau équipes enrichi (maquette §3) — 3 compteurs par équipe.
// ---------------------------------------------------------------------------

describe('AnimPage — RAFALE : panneau équipes enrichi (3 compteurs)', () => {
  it('mode SOLO (une seule équipe) : une seule carte équipe, avec ses 3 compteurs', () => {
    useGame.mockReturnValue(makeGameMock({
      teams: { 'Solo Team': { NAME: 'Solo Team', COLOR: [16, 185, 129] } },
      bumpers: { 'AA:00': { TEAM: 'Solo Team', NAME: 'J1' } },
      gameState: {
        RAFALE_PARTICIPATING_TEAMS: [],
        RAFALE_TEAM_COUNTERS: { 'Solo Team': 4 },
        RAFALE_TEAM_ERRORS: { 'Solo Team': 1 },
        RAFALE_TEAM_STREAK: { 'Solo Team': 2 },
      },
    }))
    const { container } = render(<AnimPage />)

    const stats = container.querySelectorAll('.rafale-anim-team-stats')
    expect(stats).toHaveLength(1)
    expect(container.querySelector('.rafale-anim-team-stat-good').textContent).toContain('4')
    expect(container.querySelector('.rafale-anim-team-stat-bad').textContent).toContain('1')
    expect(container.querySelector('.rafale-anim-team-stat-streak').textContent).toContain('2')
  })

  it('mode multi : chaque équipe participante a sa propre carte avec ses propres compteurs', () => {
    useGame.mockReturnValue(makeGameMock({
      teams: {
        'Équipe A': { NAME: 'Équipe A', COLOR: [99, 102, 241] },
        'Équipe B': { NAME: 'Équipe B', COLOR: [234, 179, 8] },
      },
      bumpers: {
        'AA:00': { TEAM: 'Équipe A', NAME: 'J1' },
        'BB:00': { TEAM: 'Équipe B', NAME: 'J2' },
      },
      gameState: {
        RAFALE_PARTICIPATING_TEAMS: ['Équipe A', 'Équipe B'],
        RAFALE_TEAM_COUNTERS: { 'Équipe A': 3, 'Équipe B': 1 },
        RAFALE_TEAM_ERRORS: { 'Équipe A': 0, 'Équipe B': 2 },
        RAFALE_TEAM_STREAK: { 'Équipe A': 3, 'Équipe B': 0 },
      },
    }))
    const { container } = render(<AnimPage />)

    expect(container.querySelectorAll('.rafale-anim-team-stats')).toHaveLength(2)
  })

  it('compteurs à 0 : la carte affiche quand même ses 3 stats (à 0), pas de repli "ne participe pas"', () => {
    useGame.mockReturnValue(makeGameMock({
      teams: { 'Solo Team': { NAME: 'Solo Team' } },
      bumpers: { 'AA:00': { TEAM: 'Solo Team', NAME: 'J1' } },
      gameState: { RAFALE_PARTICIPATING_TEAMS: [] },
    }))
    const { container } = render(<AnimPage />)

    const stats = container.querySelector('.rafale-anim-team-stats')
    expect(stats).not.toBeNull()
    expect(container.querySelector('.rafale-anim-team-stat-good').textContent).toContain('0')
  })
})

// ---------------------------------------------------------------------------
// Boutons VALIDE/INVALIDE (câblage AnimConductPanel -> rafaleValidate/
// rafaleInvalidate, useWebSocket.js).
// ---------------------------------------------------------------------------

describe('AnimPage — RAFALE : boutons VALIDE/INVALIDE (câblage vers rafaleValidate/rafaleInvalidate)', () => {
  it('clic sur MOCK_RAFALE_VALIDATE appelle rafaleValidate()', () => {
    const rafaleValidate = vi.fn()
    useGame.mockReturnValue(makeGameMock({ rafaleValidate }))
    render(<AnimPage />)

    fireEvent.click(screen.getByText('MOCK_RAFALE_VALIDATE'))
    expect(rafaleValidate).toHaveBeenCalledTimes(1)
  })

  it('clic sur MOCK_RAFALE_INVALIDATE appelle rafaleInvalidate()', () => {
    const rafaleInvalidate = vi.fn()
    useGame.mockReturnValue(makeGameMock({ rafaleInvalidate }))
    render(<AnimPage />)

    fireEvent.click(screen.getByText('MOCK_RAFALE_INVALIDATE'))
    expect(rafaleInvalidate).toHaveBeenCalledTimes(1)
  })
})
