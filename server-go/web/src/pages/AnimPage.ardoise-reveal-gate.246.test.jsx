import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, fireEvent, within } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// #246 (v11.2, Batch 2) — liste ARDOISE et bloc de conduite de /anim selon la maquette VALIDÉE
// docs/mockup/v11.2/ui/anim_ardoise_list__reveal_gate.html : mêmes états et mêmes règles que l'admin
// (pastilles, compteur « n/N reçues », RÉPONSE désactivé + motif, « Révéler quand même… » + confirmation,
// « non confirmée »). Composants réels (AnimArdoiseList, AnimConductPanel) : boîte noire par les textes.

vi.mock('nosleep.js', () => ({ default: class { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({ useCategories: vi.fn() }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))

const mk = (team, id, connected = true) => ({
  [id]: { TEAM: team, NAME: `VJ-${team}`, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: connected },
})
const TEAMS = {
  'Les Bleus': { NAME: 'Les Bleus', SCORE: 0, COLOR: [0, 0, 255] },
  'Les Verts': { NAME: 'Les Verts', SCORE: 0, COLOR: [0, 255, 0] },
  'Les Oranges': { NAME: 'Les Oranges', SCORE: 0, COLOR: [255, 128, 0] },
  'Les Gris': { NAME: 'Les Gris', SCORE: 0, COLOR: [128, 128, 128] }, // aucune tablette : jamais attendue
}
const BUMPERS = { ...mk('Les Bleus', 'b1'), ...mk('Les Verts', 'b2', false), ...mk('Les Oranges', 'b3') }
const ANSWERS = {
  'Les Bleus': { TEXT: 'PARIS', STARTED_AT: 12000000, SUBMITTED_AT: 13000000, RECEIVED: true, RECEIVED_HASH: '246f9d40' },
  'Les Verts': { TEXT: 'MARSEILLE', STARTED_AT: 12500000, SUBMITTED_AT: 13500000, RECEIVED: false, RECEIVED_HASH: '' },
  'Les Oranges': { TEXT: 'LYON', STARTED_AT: 12600000, SUBMITTED_AT: 13600000, RECEIVED: false, RECEIVED_HASH: '' },
}
const ALL = Object.fromEntries(Object.entries(ANSWERS).map(([k, v]) => [k, { ...v, RECEIVED: true, RECEIVED_HASH: 'x' }]))

const mkGame = (phase, answers) => ({
  status: 'connected',
  gameState: {
    phase, timer: phase === 'STARTED' ? 18 : 0, totalTime: 30, gameTime: 10000000,
    question: { ID: '7', TYPE: 'ARDOISE', STATUS: phase, QUESTION: 'Capitale ?', ANSWER: 'Paris', POINTS: '2' },
    ARDOISE_ANSWERS: answers,
  },
  teams: TEAMS, bumpers: BUMPERS, nextQuestion: null, questionPosition: { position: 1, total: 3 },
  awardedTeams: {}, creditPoints: 2,
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(),
  selectQuestion: vi.fn(), setTeamPoints: vi.fn(), setBumperPoints: vi.fn(), flipMemoryCard: vi.fn(),
  regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' }, clearRegieMessage: vi.fn(),
  selectMotionCard: vi.fn(), flipMotionCard: vi.fn(), stopMotionTimer: vi.fn(), revealMotionCard: vi.fn(), doneMotionCard: vi.fn(),
})

const text = (c) => c.textContent.replace(/\s+/g, ' ')
const btn = (c, re) => [...c.querySelectorAll('button')].find((b) => re.test(b.textContent.trim()))

describe('AnimPage — liste ARDOISE et garde REVEAL (#246, maquette anim)', () => {
  beforeEach(() => { useGame.mockReset(); useCategories.mockReturnValue({ categories: [] }) })

  it('Critical — après STOP : « 1/3 reçues », pastilles reçue / en attente / hors ligne', () => {
    useGame.mockReturnValue(mkGame('STOPPED', ANSWERS))
    const { container } = render(<AnimPage />)
    const t = text(container)
    expect(t).toMatch(/1\/3 reçues/)
    expect(t).toMatch(/✓ reçue/)
    expect(t).toMatch(/⏳ en attente/)
    expect(t).toMatch(/⚠ hors ligne/)
  })

  it('Critical — RÉPONSE désactivé, motif « En attente de 2 tablettes » avec la tablette hors ligne signalée', () => {
    useGame.mockReturnValue(mkGame('STOPPED', ANSWERS))
    const { container } = render(<AnimPage />)
    expect(btn(container, /RÉPONSE/).disabled).toBe(true)
    const t = text(container)
    expect(t).toMatch(/En attente de 2 tablettes/)
    expect(t).toMatch(/Les Verts[^]*hors ligne/)
    expect(t).toMatch(/Les Oranges/)
  })

  it('tout reçu : « 3/3 reçues », RÉPONSE actif, pas de motif ni de forçage ; RÉPONSE n’est pas un REVEAL forcé', () => {
    const game = mkGame('STOPPED', ALL)
    useGame.mockReturnValue(game)
    const { container } = render(<AnimPage />)
    expect(text(container)).toMatch(/3\/3 reçues/)
    expect(btn(container, /RÉPONSE/).disabled).toBe(false)
    expect(btn(container, /Révéler quand même…/)).toBeUndefined()
    fireEvent.click(btn(container, /RÉPONSE/))
    expect(game.revealAnswer).toHaveBeenCalledTimes(1)
    expect(game.revealAnswer.mock.calls[0][0]).not.toBe(true)
  })

  it('pendant la question : « Saisie en cours », pas de pastille ni de motif', () => {
    useGame.mockReturnValue(mkGame('STARTED', ANSWERS))
    const { container } = render(<AnimPage />)
    const t = text(container)
    expect(t).toMatch(/Saisie en cours/)
    expect(t).not.toMatch(/✓ reçue|⏳ en attente|⚠ hors ligne|En attente de/)
  })

  it('« Révéler quand même… » → confirmation (équipes manquantes) ; « Attendre » annule', () => {
    const game = mkGame('STOPPED', ANSWERS)
    useGame.mockReturnValue(game)
    const { container, baseElement } = render(<AnimPage />)
    fireEvent.click(btn(container, /Révéler quand même…/))

    const dlg = baseElement.querySelector('[role="dialog"]')
    expect(dlg, 'la confirmation doit être un role="dialog"').toBeTruthy()
    expect(text(dlg)).toMatch(/Révéler sans toutes les réponses \?/)
    expect(text(dlg)).toMatch(/Les Verts[^]*hors ligne/)
    expect(text(dlg)).toMatch(/Les Oranges/)
    expect(text(dlg)).not.toMatch(/Les Bleus/)
    fireEvent.click(within(dlg).getByText('Attendre'))
    expect(game.revealAnswer).not.toHaveBeenCalled()
  })

  it('Critical — confirmer envoie revealAnswer(true)', () => {
    const game = mkGame('STOPPED', ANSWERS)
    useGame.mockReturnValue(game)
    const { container, baseElement } = render(<AnimPage />)
    fireEvent.click(btn(container, /Révéler quand même…/))
    fireEvent.click(within(baseElement.querySelector('[role="dialog"]')).getByRole('button', { name: 'Révéler quand même' }))
    expect(game.revealAnswer).toHaveBeenCalledWith(true)
  })

  it('REVEALED après forçage : « non confirmée » sur les équipes non reçues seulement, dernier texte conservé', () => {
    useGame.mockReturnValue(mkGame('REVEALED', ANSWERS))
    const { container } = render(<AnimPage />)
    const rows = [...container.querySelectorAll('.anim-ardoise-row')]
    const row = (n) => rows.find((r) => r.textContent.includes(n))
    expect(row('Les Verts').textContent).toMatch(/non confirmée/)
    expect(row('Les Verts').textContent).toMatch(/MARSEILLE/)
    expect(row('Les Oranges').textContent).toMatch(/non confirmée/)
    expect(row('Les Bleus').textContent).not.toMatch(/non confirmée/)
  })

  it('PAUSED : mêmes règles que STOPPED', () => {
    useGame.mockReturnValue(mkGame('PAUSED', ANSWERS))
    const { container } = render(<AnimPage />)
    expect(text(container)).toMatch(/1\/3 reçues/)
  })
})
