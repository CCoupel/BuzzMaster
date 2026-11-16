import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, fireEvent } from '@testing-library/react'
import RoueTv from './RoueTv'
import RoueGameBoard from './RoueGameBoard'
import { roueAnnouncement, makeRoueActions, roueGates } from './roueState'

afterEach(() => cleanup())

const base = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_PRESENTATION: 'Chanson de Line Renaud', ROUE_MODE: 'TANT_QUE_JE_GAGNE' },
  ROUE_SUBPHASE: 'LETTER', ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_TEAM_BANK: { Rouges: 1300, Bleus: 400 }, ROUE_CONSONANTS_LEFT: true, ROUE_SHOW_LETTERS: true,
  ROUE_BOARD: { COLS: 14, ROWS: ['______N ___R', '__ ____N __N_', '__ __TR_N_'] },
  ROUE_LETTERS: [{ LETTER: 'T', TEAM: 'Bleus', HIT: true }, { LETTER: 'G', TEAM: 'Rouges', HIT: false }],
  ROUE_WHEEL: { LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, FLIP_INTERVAL_MS: 600, CURRENT: 1, STOP: null,
    SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'value', VALUE: 500 }, { ID: 's2', KIND: 'bankrupt', VALUE: 0 }] },
  ROUE_LAST_EVENT: { SEQ: 3, KIND: 'LAND_VALUE', TEAM: 'Rouges', LETTER: '', COUNT: 0, VALUE: 500, GAIN: 0 },
  ROUE_WINNER: '', ROUE_AWARDS: {},
  ...over,
})
const teams = { Rouges: { COLOR: [239, 68, 68] }, Bleus: { COLOR: [59, 130, 246] } }

describe('RoueTv — TV (#144, M1)', () => {
  it('équipes avec cagnotte, présentation, annonce de la case, lettres proposées et roue', () => {
    const { container, getByText } = render(<RoueTv gameState={base()} teams={teams} catMeta={{ icon: '🐾', label: 'Animaux' }} />)
    expect(getByText('Rouges')).toBeTruthy()
    expect(container.querySelector('.roue-bank.active .n').textContent).toBe('Rouges')
    expect(container.querySelector('.roue-bank.active .v').textContent).toMatch(/1\s?300/)
    expect(getByText('Chanson de Line Renaud')).toBeTruthy()
    expect(container.querySelector('.roue-event').textContent).toBe('Case 500 — proposez une consonne')
    expect(container.querySelector('.roue-letters .miss').textContent).toBe('G')
    expect(container.querySelector('[data-testid="fortune-wheel"]')).not.toBeNull()
  })
  it('ROUE_SHOW_LETTERS=false : aucune lettre proposée affichée', () => {
    const { container } = render(<RoueTv gameState={base({ ROUE_SHOW_LETTERS: false })} teams={teams} />)
    expect(container.querySelector('.roue-letters b')).toBeNull()
  })
  it('affiche 8 équipes en mode dense', () => {
    const roster = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H']
    const { container } = render(<RoueTv gameState={base({ ROUE_TEAMS: roster })} teams={teams} />)
    expect(container.querySelector('.roue-banks.dense')).not.toBeNull()
    expect(container.querySelectorAll('.roue-bank').length).toBe(8)
  })
})

describe('RoueTv — tablette (#144, M5)', () => {
  it('STOP actif seulement pour l\'équipe qui a la main pendant SPINNING', () => {
    const onStop = vi.fn()
    const { container, rerender } = render(<RoueTv gameState={base({ ROUE_SUBPHASE: 'SPINNING' })} teams={teams} isVPlayer teamName="Rouges" onStop={onStop} />)
    fireEvent.click(container.querySelector('button.roue-vp-stop'))
    expect(onStop).toHaveBeenCalledTimes(1)
    rerender(<RoueTv gameState={base({ ROUE_SUBPHASE: 'SPINNING' })} teams={teams} isVPlayer teamName="Bleus" onStop={onStop} />)
    expect(container.querySelector('button.roue-vp-stop')).toBeNull()
    expect(container.querySelector('.roue-vp-stop.off').textContent).toMatch(/pas votre tour/)
  })
  it('ne contient jamais de phrase : seul le plateau est rendu', () => {
    const { container } = render(<RoueTv gameState={base({ question: { TYPE: 'ROUE', ROUE_PHRASE: 'SECRET' } })} teams={teams} isVPlayer teamName="Bleus" />)
    expect(container.textContent).not.toMatch(/SECRET/)
  })
})

describe('RoueTv — fin de manche', () => {
  it('après le STOP (état ROUE vidé), garde le dernier plateau résolu', () => {
    const solved = base({ ROUE_SUBPHASE: 'SOLVED', ROUE_WINNER: 'Rouges', ROUE_AWARDS: { Rouges: 1300 }, ROUE_BOARD: { COLS: 14, ROWS: ['ABC'] } })
    const { container, rerender } = render(<RoueTv gameState={solved} teams={teams} />)
    rerender(<RoueTv gameState={base({ phase: 'STOPPED', ROUE_SUBPHASE: '', ROUE_TEAMS: [], ROUE_BOARD: { COLS: 14, ROWS: [] }, ROUE_TEAM_BANK: {} })} teams={teams} />)
    expect(container.querySelectorAll('.rgb-cell.shown').length).toBe(3)
    expect(container.querySelectorAll('.roue-bank').length).toBe(2)
  })
})

describe('RoueGameBoard', () => {
  it('cases : cachée, éclairée, visible', () => {
    const { container } = render(<RoueGameBoard board={{ COLS: 14, ROWS: ['*_AB'] }} />)
    expect(container.querySelectorAll('.rgb-cell.lit').length).toBe(1)
    expect(container.querySelectorAll('.rgb-cell.tile').length).toBe(1)
    expect(container.querySelectorAll('.rgb-cell.shown').length).toBe(2)
    expect(container.querySelectorAll('.rgb-cell').length).toBe(14 * 4)
  })
  it('retournement : * → lettre anime les cases, et les conserve après re-rendu', () => {
    const { container, rerender } = render(<RoueGameBoard board={{ COLS: 14, ROWS: ['*_*'] }} />)
    rerender(<RoueGameBoard board={{ COLS: 14, ROWS: ['C_C'] }} />)
    expect(container.querySelectorAll('.rgb-cell.flip').length).toBe(2)
    rerender(<RoueGameBoard board={{ COLS: 14, ROWS: ['C_C'] }} />)
    expect(container.querySelectorAll('.rgb-cell.flip').length).toBe(2)
  })
  it('montage direct sur un plateau révélé : aucune animation', () => {
    const { container } = render(<RoueGameBoard board={{ COLS: 14, ROWS: ['ABC'] }} />)
    expect(container.querySelectorAll('.rgb-cell.flip').length).toBe(0)
  })
  it('ne modifie pas le tableau reçu', () => {
    const rows = ['AB']
    render(<RoueGameBoard board={{ COLS: 14, ROWS: rows }} />)
    expect(rows).toEqual(['AB'])
  })
})

describe('roueState', () => {
  it('annonces : banqueroute, lettre absente, solved', () => {
    const txt = (gs) => roueAnnouncement(gs).map(p => p.text ?? p.strong).join('')
    expect(txt(base({ ROUE_SUBPHASE: 'TURN_END', ROUE_WHEEL: { ...base().ROUE_WHEEL, CURRENT: 2 } }))).toMatch(/BANQUEROUTE/)
    expect(txt(base({ ROUE_SUBPHASE: 'TURN_END' }))).toBe('G : absente')
    expect(txt(base({ ROUE_SUBPHASE: 'SOLVED', ROUE_WINNER: 'Rouges', ROUE_AWARDS: { Rouges: 1300 } }))).toMatch(/ROUGES trouvent la phrase : \+1\s?300 points/)
  })
  it('actions : payloads du contrat §8.1', () => {
    const send = vi.fn()
    const a = makeRoueActions(send)
    a.spin(); a.letter('C'); a.next('NEXT'); a.next(); a.setTeams(['A', 'B'])
    expect(send.mock.calls).toEqual([['ROUE_SPIN', {}], ['ROUE_LETTER', { LETTER: 'C' }], ['ROUE_NEXT', { HAND: 'NEXT' }], ['ROUE_NEXT', { HAND: 'SAME' }], ['ROUE_SET_TEAMS', { TEAMS: ['A', 'B'] }]])
  })
  it('portes selon la sous-phase ; SOLO : pas de passe ni d\'équipe suivante', () => {
    expect(roueGates(base({ ROUE_SUBPHASE: 'TURN' })).canSpin).toBe(true)
    expect(roueGates(base({ ROUE_SUBPHASE: 'LIT' })).canFlip).toBe(true)
    const solo = roueGates(base({ ROUE_SUBPHASE: 'TURN_END', question: { TYPE: 'ROUE', ROUE_MODE: 'SOLO' } }))
    expect(solo.canSame).toBe(true)
    expect(solo.canNextTeam).toBe(false)
    expect(roueGates(base({ ROUE_SUBPHASE: 'TURN', ROUE_CONSONANTS_LEFT: false })).canAnswer).toBe(true)
  })
})
