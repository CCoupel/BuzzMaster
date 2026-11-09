import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, within, waitFor } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// #290 (v11.6.0, LOT-4) — éditeur de manches : bouton 🎛 « Ambiance » par manche (pas sur « Sans manche »), badge
// « Ambiance : N réglages », fenêtre « Ambiance de la manche » (mode override), enregistrement ⇒ ROUNDS_UPDATE COMPLET
// qui transporte AMBIANCE (les autres manches la conservent). Maquette admin_questions_list__round_ambiance_override.html ;
// contrats ambiance-settings.md §6, websocket-actions.md §ROUNDS_UPDATE. Même harnais que QuestionsPage.rounds.245.test.jsx.

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

// #215 — QuestionsPage appelle désormais useNavigate()/useSearchParams()
// (onglets Questions/Rafale, lien "configurer le quiz" → navigation vers
// Backstage) : ces tests ne montent aucun Router réel, donc mock plutôt que
// de faire porter <MemoryRouter> à chacun des (nombreux) render() ci-dessous.
vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
  useSearchParams: () => [new URLSearchParams(), vi.fn()],
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, type, ...rest }) => (
    <button onClick={onClick} disabled={disabled} type={type || 'button'} {...rest}>
      {children}
    </button>
  ),
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
  CardHeader: ({ children }) => <div className="card-header">{children}</div>,
  CardBody: ({ children }) => <div className="card-body">{children}</div>,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div
      data-testid={`qcard-${question.ID}`}
      onClick={() => onClick && onClick(question)}
    />
  ),
  CATEGORIES: {
    CULTURE: { label: 'Culture', icon: '🎭', color: '#8b5cf6' },
    SPORT:   { label: 'Sport',   icon: '⚽', color: '#22c55e' },
  },
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css',    () => ({}))

// ---------------------------------------------------------------------------
// Import useGame après mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Helper : mock minimal useGame
// ---------------------------------------------------------------------------

const Q = (id, order, round, over = {}) => ({ ID: String(id), TYPE: 'SPEEDY', QUESTION: `Question ${id}`, ANSWER: 'a', POINTS: '1', TIME: '30', CATEGORY: 'CULTURE', ORDER: order, ...(round ? { ROUND_ID: round } : {}), ...over })
const QUESTIONS = {
  1: Q(1, 0, 'r1'), 2: Q(2, 1, 'r1'), 3: Q(3, 0, 'r2'), 4: Q(4, 0), 5: Q(5, 1),
  6: Q(6, 2, 'r1', { TYPE: 'ENTRACTE', QUESTION: 'Fin de la manche Un' }),
}
const AMB_R1 = { sound: { countdown: { enabled: false }, cues: { 'qcm-indice': false } }, light: { mode: 'ON' } }
const ROUNDS = [{ ID: 'r1', NAME: 'Un', AMBIANCE: AMB_R1 }, { ID: 'r2', NAME: 'Deux', AMBIANCE: {} }]

let game
const mount = (over = {}, rounds = ROUNDS, questions = QUESTIONS) => {
  game = {
    questions, fsInfo: { used: 0, total: 100 }, deleteQuestion: vi.fn(), sendMessage: vi.fn(),
    gameState: { phase: 'STOPPED', question: null, ROUNDS: rounds },
    newGame: vi.fn(), roundsUpdate: vi.fn(() => true), roundsUpdateResult: null, clearRoundsUpdateResult: vi.fn(), ...over,
  }
  useGame.mockReturnValue(game)
  return render(<QuestionsPage />)
}
// le nom accessible d'un groupe peut préfixer le rang (« Manche 1 · Un ») : on retrouve la région par son nom de manche
const group = (name) => {
  const hit = screen.getAllByRole('region').find((r) => { const l = r.getAttribute('aria-label') || ''; return l === name || l.endsWith(` ${name}`) || l.endsWith(`· ${name}`) })
  if (!hit) throw new Error(`groupe « ${name} » introuvable : ${screen.getAllByRole('region').map((r) => r.getAttribute('aria-label')).join(' | ')}`)
  return hit
}
const lastPayload = () => game.roundsUpdate.mock.calls[game.roundsUpdate.mock.calls.length - 1][0]
const cited = (p) => [...p.unassigned, ...p.rounds.flatMap((r) => r.questions)].map(String).sort()
const ALL_IDS = ['1', '2', '3', '4', '5', '6']

// stockage mémoire : le préférence de repli est locale au navigateur (localStorage), absente de cet environnement de test
const memoryStorage = () => {
  const m = new Map()
  return { getItem: (k) => (m.has(k) ? m.get(k) : null), setItem: (k, v) => { m.set(k, String(v)) }, removeItem: (k) => { m.delete(k) }, clear: () => m.clear() }
}
beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal('localStorage', memoryStorage())
})
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })


const BASE = {
  sound: { muted: false, countdown: { enabled: true, loop: true }, cues: Object.fromEntries(['depart','temps-ecoule','gagne','perdu','reveal','entracte-debut','entracte-fin','qcm-indice','memory-carte','memory-paire','memotion-selection','memotion-fin-carte','rafale-bonne','rafale-mauvaise','rafale-fin-lot'].map((c) => [c, true])) },
  light: { mode: 'AUTO' },
}
const TYPES = { 'qcm-indice': 'QCM', 'memory-carte': 'MEMORY', 'memory-paire': 'MEMORY', 'memotion-selection': 'MEMOTION', 'memotion-fin-carte': 'MEMOTION', 'rafale-bonne': 'RAFALE', 'rafale-mauvaise': 'RAFALE', 'rafale-fin-lot': 'RAFALE' }
const SOUNDS = { cues: Object.keys(BASE.sound.cues).map((cue) => ({ cue, type: TYPES[cue] || '', enabled: true, custom: false, duration_seconds: 0.5, path: `/files/sounds/${cue}.wav` })) }

beforeEach(() => {
  global.fetch = vi.fn(async (url) => {
    const ok = (body) => ({ ok: true, status: 200, json: async () => body, text: async () => JSON.stringify(body) })
    if (url === '/api/ambiance/settings') return ok({ global: BASE, effective: BASE, sources: {}, round_id: '', round_override: {} })
    if (url === '/api/sounds') return ok(SOUNDS)
    return ok([])
  })
})

const ambiBtn = (name) => within(group(name)).getByRole('button', { name: /ambiance/i })

describe('Éditeur en manches — bouton 🎛 et badge Ambiance (#290)', () => {
  it('chaque manche porte un bouton 🎛 ; « Sans manche » n\'en a pas', () => {
    mount()
    expect(ambiBtn('Un').textContent).toContain('🎛')
    expect(ambiBtn('Deux').textContent).toContain('🎛')
    expect(within(group('Sans manche')).queryByRole('button', { name: /ambiance/i })).toBeNull()
  })

  it('badge « Ambiance : 3 réglages » sur la manche surchargée (cues comptées une à une) ; aucun badge si héritée', () => {
    mount()
    expect(within(group('Un')).getByText(/Ambiance : 3 réglages/)).toBeInTheDocument()
    expect(within(group('Deux')).queryByText(/Ambiance : \d/)).toBeNull()
  })

  it('serveur antérieur (ROUNDS sans AMBIANCE) : aucun badge, aucune erreur', () => {
    mount({}, [{ ID: 'r1', NAME: 'Un' }, { ID: 'r2', NAME: 'Deux' }])
    expect(screen.queryByText(/Ambiance : \d/)).toBeNull()
  })
})

describe('Éditeur en manches — fenêtre « Ambiance de la manche » (#290)', () => {
  it('🎛 ouvre la fenêtre de la bonne manche, avec la surcharge courante (3 réglages surchargés)', async () => {
    mount()
    fireEvent.click(ambiBtn('Un'))
    expect(await screen.findByRole('dialog', { name: /Ambiance de la manche « Un »/ })).toBeInTheDocument()
    expect(screen.getByText(/3 réglages surchargés/)).toBeInTheDocument()
  })

  it('Critical — Enregistrer envoie un ROUNDS_UPDATE COMPLET : l\'AMBIANCE de la manche modifiée, celle des autres conservée', async () => {
    mount()
    fireEvent.click(ambiBtn('Deux'))
    const dlg = await screen.findByRole('dialog', { name: /Ambiance de la manche « Deux »/ })
    fireEvent.click(within(within(dlg).getByRole('radiogroup', { name: /^Son$/i })).getByRole('radio', { name: 'Coupé' }))
    fireEvent.click(within(dlg).getByRole('button', { name: 'Enregistrer' }))
    await waitFor(() => expect(game.roundsUpdate).toHaveBeenCalledTimes(1))
    const p = lastPayload()
    expect(cited(p)).toEqual(ALL_IDS)
    expect(p.rounds.find((r) => r.id === 'r2').ambiance).toEqual({ sound: { muted: true } })
    expect(p.rounds.find((r) => r.id === 'r1').ambiance).toEqual(AMB_R1)
  })

  it('« Tout remettre à Hérité » puis Enregistrer envoie AMBIANCE {} (efface la surcharge)', async () => {
    mount()
    fireEvent.click(ambiBtn('Un'))
    const dlg = await screen.findByRole('dialog', { name: /Ambiance de la manche « Un »/ })
    fireEvent.click(within(dlg).getByRole('button', { name: /Tout remettre à/ }))
    fireEvent.click(within(dlg).getByRole('button', { name: 'Enregistrer' }))
    await waitFor(() => expect(game.roundsUpdate).toHaveBeenCalledTimes(1))
    expect(lastPayload().rounds.find((r) => r.id === 'r1').ambiance).toEqual({})
  })

  it('Annuler n\'envoie rien (et ferme)', async () => {
    mount()
    fireEvent.click(ambiBtn('Un'))
    const dlg = await screen.findByRole('dialog', { name: /Ambiance de la manche « Un »/ })
    fireEvent.click(within(dlg).getByRole('button', { name: 'Annuler' }))
    expect(game.roundsUpdate).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog', { name: /Ambiance de la manche/ })).toBeNull()
  })

  it('un geste ordinaire (▼ sur une manche) conserve les surcharges de TOUTES les manches', () => {
    mount()
    fireEvent.click(within(group('Un')).getByRole('button', { name: /▼|descendre/i }))
    const p = lastPayload()
    expect(p.rounds.find((r) => r.id === 'r1').ambiance).toEqual(AMB_R1)
    expect(p.rounds.find((r) => r.id === 'r2').ambiance).toEqual({})
  })
})
