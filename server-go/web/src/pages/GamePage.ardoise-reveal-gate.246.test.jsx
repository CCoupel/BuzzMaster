import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, within } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #246 (v11.2, Batch 2) — zone ARDOISE de /admin/game selon la maquette VALIDÉE
// docs/mockup/v11.2/ui/admin_game_ardoise_zone__reveal_gate.html : pastilles reçue / en attente /
// hors ligne, compteur « n/N reçues », REPONSE désactivé avec motif, « Révéler quand même… » +
// confirmation listant les équipes, « non confirmée » après forçage.
// Contrats : websocket-actions.md §REVEAL (équipe attendue = clé de teams ayant ≥ 1 bumper IS_VPLAYER,
// hors ligne = tous ses VJoueurs CONNECTED=false, ne lève PAS la garde), game-state.md §ARDOISE_ANSWERS.
// Boîte noire : textes de la maquette, aucune dépendance aux classes CSS du dev.

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn((questions) => ({
    selectedCategories: new Set(),
    availableCategories: [],
    filteredQuestions: questions ? Object.values(questions) : [],
    toggleCategoryFilter: vi.fn(),
    clearCategoryFilters: vi.fn(),
  })),
}))

vi.mock('../components/Timer', () => ({
  default: ({ phase }) => <div data-testid="timer" data-phase={phase} />,
}))

vi.mock('../components/QuestionPreview', () => ({
  default: () => <div data-testid="question-preview" />,
}))

vi.mock('../components/TeamCard', () => ({
  default: ({ name }) => <div data-testid={`team-card-${name}`} />,
  OtaAllModal: () => null,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div
      data-testid={`question-card-${question.ID}`}
      onClick={() => onClick && onClick(question)}
    />
  ),
  CATEGORIES: {},
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, variant, size, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color || '#888')),
}))

vi.mock('./GamePage.css', () => ({}))


const mk = (team, mac, connected = true) => ({
  [mac]: { TEAM: team, NAME: `VJ-${team}`, SCORE: 0, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: connected, COLOR: [1, 2, 3] },
})
const TEAMS = {
  'Les Bleus': { NAME: 'Les Bleus', SCORE: 0, COLOR: [0, 0, 255] },
  'Les Verts': { NAME: 'Les Verts', SCORE: 0, COLOR: [0, 255, 0] },
  'Les Oranges': { NAME: 'Les Oranges', SCORE: 0, COLOR: [255, 128, 0] },
  'Les Gris': { NAME: 'Les Gris', SCORE: 0, COLOR: [128, 128, 128] }, // aucune tablette : jamais attendue
}
const BUMPERS = { ...mk('Les Bleus', 'b1'), ...mk('Les Verts', 'b2', false), ...mk('Les Oranges', 'b3') }
const ANSWERS = {
  'Les Bleus': { TEXT: 'PARIS', STARTED_AT: 2000000, SUBMITTED_AT: 3000000, RECEIVED: true, RECEIVED_HASH: '246f9d40' },
  'Les Verts': { TEXT: 'MARSEILLE', STARTED_AT: 2500000, SUBMITTED_AT: 3500000, RECEIVED: false, RECEIVED_HASH: '' },
  'Les Oranges': { TEXT: 'LYON', STARTED_AT: 2600000, SUBMITTED_AT: 3600000, RECEIVED: false, RECEIVED_HASH: '' },
}
const ALL_RECEIVED = Object.fromEntries(Object.entries(ANSWERS).map(([k, v]) => [k, { ...v, RECEIVED: true, RECEIVED_HASH: 'x' }]))

const makeGameMock = (phase, answers, extra = {}) => ({
  gameState: {
    phase,
    question: { ID: '10', TYPE: 'ARDOISE', QUESTION: 'Capitale ?', ANSWER: 'Paris', POINTS: '2', ARDOISE_KEYBOARD_TYPE: 'AZERTY' },
    remote: 'GAME', timer: phase === 'STARTED' ? 18 : 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [],
    ARDOISE_ANSWERS: answers, gameTime: 1000000,
  },
  teams: TEAMS, bumpers: BUMPERS,
  questions: { '10': { ID: '10', STATUS: phase, ORDER: 1 } },
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(),
  selectQuestion: vi.fn(), setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(),
  forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(), sendMessage: vi.fn(),
  ...extra,
})

const text = (c) => c.textContent.replace(/\s+/g, ' ')
const revealBtn = (c) => [...c.querySelectorAll('button')].find((b) => b.textContent.trim() === 'REPONSE')
const forceBtn = (c) => [...c.querySelectorAll('button')].find((b) => /Révéler quand même…/.test(b.textContent))

describe('GamePage — garde REVEAL ARDOISE : zone Réponses (#246, maquette admin)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — après STOP : compteur « 1/3 reçues » (équipes attendues seulement), pastilles reçue / en attente / hors ligne', () => {
    useGame.mockReturnValue(makeGameMock('STOPPED', ANSWERS))
    const { container } = render(<GamePage />)
    const t = text(container)

    expect(t).toMatch(/1\/3 reçues/)           // « Les Gris » (sans VJoueur) n'est pas attendue
    expect(t).not.toMatch(/\/4 reçues/)
    expect(t).toMatch(/✓ reçue/)
    expect(t).toMatch(/⏳ en attente/)
    expect(t).toMatch(/⚠ hors ligne/)
  })

  it('Critical — REPONSE désactivé avec le motif listant les équipes en attente (hors ligne signalée)', () => {
    useGame.mockReturnValue(makeGameMock('STOPPED', ANSWERS))
    const { container } = render(<GamePage />)

    expect(revealBtn(container).disabled).toBe(true)
    const t = text(container)
    expect(t).toMatch(/En attente des réponses de 2 tablettes/)
    expect(t).toMatch(/Les Verts[^]*hors ligne/)
    expect(t).toMatch(/Les Oranges/)
  })

  it('la tablette hors ligne ne lève PAS la garde (même règle que le serveur)', () => {
    const answers = { ...ANSWERS, 'Les Oranges': { ...ANSWERS['Les Oranges'], RECEIVED: true } }
    useGame.mockReturnValue(makeGameMock('STOPPED', answers)) // reste Les Verts : hors ligne, non reçue
    const { container } = render(<GamePage />)

    expect(revealBtn(container).disabled).toBe(true)
    expect(text(container)).toMatch(/2\/3 reçues/)
  })

  it('tout reçu : « 3/3 reçues », REPONSE actif, plus de motif ni de « Révéler quand même… » ; REPONSE appelle revealAnswer sans forçage', () => {
    const game = makeGameMock('STOPPED', ALL_RECEIVED)
    useGame.mockReturnValue(game)
    const { container } = render(<GamePage />)

    expect(text(container)).toMatch(/3\/3 reçues/)
    expect(revealBtn(container).disabled).toBe(false)
    expect(forceBtn(container)).toBeUndefined()
    expect(text(container)).not.toMatch(/En attente des réponses/)
    fireEvent.click(revealBtn(container))
    expect(game.revealAnswer).toHaveBeenCalledTimes(1)
    expect(game.revealAnswer.mock.calls[0][0]).not.toBe(true)
  })

  it('pendant la question (STARTED) : « Saisie en cours », aucune pastille ni légende, REPONSE désactivé sans motif d’attente', () => {
    useGame.mockReturnValue(makeGameMock('STARTED', ANSWERS))
    const { container } = render(<GamePage />)
    const t = text(container)

    expect(t).toMatch(/Saisie en cours/)
    expect(t).not.toMatch(/✓ reçue|⏳ en attente|⚠ hors ligne/)
    expect(t).not.toMatch(/En attente des réponses/)
    expect(forceBtn(container)).toBeUndefined()
  })

  it('« Révéler quand même… » ouvre la confirmation listant les équipes manquantes ; « Attendre » annule sans REVEAL', () => {
    const game = makeGameMock('STOPPED', ANSWERS)
    useGame.mockReturnValue(game)
    const { container, baseElement } = render(<GamePage />)

    fireEvent.click(forceBtn(container))

    const dlg = baseElement.querySelector('[role="dialog"]')
    expect(dlg, 'la confirmation doit être un role="dialog"').toBeTruthy()
    const dt = text(dlg)
    expect(dt).toMatch(/Révéler sans toutes les réponses \?/)
    expect(dt).toMatch(/Les Verts[^]*hors ligne/)
    expect(dt).toMatch(/Les Oranges/)
    expect(dt).not.toMatch(/Les Bleus/)
    expect(dt).toMatch(/non confirmée/)
    fireEvent.click(within(dlg).getByText('Attendre'))
    expect(game.revealAnswer).not.toHaveBeenCalled()
    expect(baseElement.querySelector('[role="dialog"]')).toBeNull()
  })

  it('Critical — confirmer « Révéler quand même » envoie le REVEAL forcé (revealAnswer(true))', () => {
    const game = makeGameMock('STOPPED', ANSWERS)
    useGame.mockReturnValue(game)
    const { container, baseElement } = render(<GamePage />)
    fireEvent.click(forceBtn(container))

    fireEvent.click(within(baseElement.querySelector('[role="dialog"]')).getByRole('button', { name: 'Révéler quand même' }))

    expect(game.revealAnswer).toHaveBeenCalledTimes(1)
    expect(game.revealAnswer).toHaveBeenCalledWith(true)
  })

  it('après forçage (REVEALED) : les équipes non reçues portent « non confirmée », les reçues non', () => {
    useGame.mockReturnValue(makeGameMock('REVEALED', ANSWERS))
    const { container } = render(<GamePage />)

    const rows = [...container.querySelectorAll('.ardoise-answer-row')]
    const row = (name) => rows.find((r) => r.textContent.includes(name))
    expect(row('Les Verts').textContent).toMatch(/non confirmée/)
    expect(row('Les Oranges').textContent).toMatch(/non confirmée/)
    expect(row('Les Bleus').textContent).not.toMatch(/non confirmée/)
    expect(row('Les Verts').textContent).toMatch(/MARSEILLE/) // dernier texte reçu conservé
  })

  it('REVEALED sans forçage (tout reçu) : aucune « non confirmée »', () => {
    useGame.mockReturnValue(makeGameMock('REVEALED', ALL_RECEIVED))
    const { container } = render(<GamePage />)
    expect(text(container)).not.toMatch(/non confirmée/)
  })

  it('PAUSED : mêmes pastilles et même garde que STOPPED', () => {
    useGame.mockReturnValue(makeGameMock('PAUSED', ANSWERS))
    const { container } = render(<GamePage />)
    expect(text(container)).toMatch(/1\/3 reçues/)
    expect(revealBtn(container).disabled).toBe(true)
  })

  it('non-régression hors ARDOISE : REPONSE inchangé, aucun compteur ni forçage', () => {
    const game = makeGameMock('STOPPED', {})
    game.gameState.question = { ID: '10', TYPE: 'SPEEDY', QUESTION: 'Q', ANSWER: 'A', POINTS: '2' }
    useGame.mockReturnValue(game)
    const { container } = render(<GamePage />)
    expect(text(container)).not.toMatch(/reçues/)
    expect(forceBtn(container)).toBeUndefined()
    expect(text(container)).not.toMatch(/En attente des réponses/)
  })
})
