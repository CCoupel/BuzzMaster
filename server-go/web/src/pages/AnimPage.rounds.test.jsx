import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// #245 (lot V3-3) — anim : pastille de manche « Manche 2/3 · nom · 3/5 », « Terminer l'entracte » (ENTRACTE_SET false),
// aucun bouton de déclenchement manuel. Maquette (g‴).
vi.mock('nosleep.js', () => ({ default: class NoSleep { constructor() { this.isEnabled = false } enable() { return Promise.resolve() } disable() {} } }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({ useCategories: vi.fn() }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/AnimConductPanel', () => ({
  default: ({ entracteActive, onEndEntracte }) => (
    <div data-testid="conduct-panel">
      {entracteActive && <button onClick={onEndEntracte}>Terminer l'entracte</button>}
    </div>
  ),
}))
vi.mock('../components/AnimAnswerZone', () => ({ default: () => <div /> }))
vi.mock('../components/AnimArdoiseList', () => ({ default: () => <div /> }))
vi.mock('./AnimPage.css', () => ({}))
vi.mock('../components/LaunchGameHost.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

const ROUNDS = [{ ID: 'r1', NAME: 'Échauffement' }, { ID: 'r2', NAME: 'Culture générale' }, { ID: 'r3', NAME: 'Finale' }]
const make = (gameState = {}, over = {}) => ({
  status: 'connected',
  gameState: {
    phase: 'STOPPED', timer: 30, totalTime: 30, gameTime: 0, entracte: false,
    question: { ID: '9', TYPE: 'QCM', QUESTION: 'Fleuve ?', POINTS: 3, TIME: 30 },
    entracteConfig: { TITLE: 'E', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
    ROUNDS, ROUND_CONTEXT: { ROUND_ID: 'r2', ROUND_INDEX: 2, ROUND_COUNT: 3, POSITION: 3, TOTAL: 5, FIRST_OF_ROUND: false },
    ...gameState,
  },
  teams: {}, bumpers: {}, nextQuestion: null, questionPosition: { position: 0, total: 0 }, awardedTeams: {}, creditPoints: 0,
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
  setTeamPoints: vi.fn(), setBumperPoints: vi.fn(), flipMemoryCard: vi.fn(), setEntracte: vi.fn(),
  regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' }, clearRegieMessage: vi.fn(),
  selectMotionCard: vi.fn(), flipMotionCard: vi.fn(), stopMotionTimer: vi.fn(), revealMotionCard: vi.fn(), doneMotionCard: vi.fn(),
  ...over,
})
beforeEach(() => { useCategories.mockReturnValue({ categories: [] }) })

describe('AnimPage — manches (#245, lot V3-3)', () => {
  it('Critical — pastille « Manche 2/3 · nom · 3/5 »', () => {
    useGame.mockReturnValue(make())
    render(<AnimPage />)
    expect(screen.getByText('Manche 2/3 · Culture générale · 3/5')).toBeTruthy()
  })

  it('question hors manche alors que le quiz en compte : « Sans manche · 2/4 »', () => {
    useGame.mockReturnValue(make({ ROUND_CONTEXT: { ROUND_ID: '', ROUND_INDEX: 0, ROUND_COUNT: 3, POSITION: 2, TOTAL: 4, FIRST_OF_ROUND: false } }))
    render(<AnimPage />)
    expect(screen.getByText('Sans manche · 2/4')).toBeTruthy()
  })

  it('entracte de la manche (POSITION 0) : « · entracte »', () => {
    useGame.mockReturnValue(make({ ROUND_CONTEXT: { ROUND_ID: 'r2', ROUND_INDEX: 2, ROUND_COUNT: 3, POSITION: 0, TOTAL: 5, FIRST_OF_ROUND: false } }))
    render(<AnimPage />)
    expect(screen.getByText('Manche 2/3 · Culture générale · entracte')).toBeTruthy()
  })

  it('Critical — sans manche : aucune pastille de manche', () => {
    useGame.mockReturnValue(make({ ROUNDS: [], ROUND_CONTEXT: { ROUND_ID: '', ROUND_INDEX: 0, ROUND_COUNT: 0, POSITION: 0, TOTAL: 0, FIRST_OF_ROUND: false } }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-chip-round')).toBeNull()
  })

  it('Critical — entracte en cours : « Terminer l’entracte » envoie ENTRACTE_SET{ACTIVE:false} (setEntracte(false)) ; indicateur « Entracte en cours »', () => {
    const g = make({ entracte: true })
    useGame.mockReturnValue(g)
    render(<AnimPage />)
    expect(screen.getByText(/Entracte en cours/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: "Terminer l'entracte" }))
    expect(g.setEntracte).toHaveBeenCalledTimes(1)
    expect(g.setEntracte).toHaveBeenCalledWith(false)
  })

  it('hors entracte : aucun bouton « Terminer » ; jamais de bouton de déclenchement manuel', () => {
    useGame.mockReturnValue(make())
    render(<AnimPage />)
    expect(screen.queryByRole('button', { name: /entracte/i })).toBeNull()
  })
})
