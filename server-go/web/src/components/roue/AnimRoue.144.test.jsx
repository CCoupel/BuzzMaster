import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen } from '@testing-library/react'
import { AnimRoueView } from './AnimRoue'
import { makeRoueActions } from './roueState'

afterEach(() => cleanup())

const gs = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_PHRASE: 'COMBIEN POUR', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_PRESENTATION: 'Chanson' },
  ROUE_SUBPHASE: 'TURN', ROUE_STEP: 1, ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_NEXT_TEAM: 'Bleus', ROUE_NEXT_DEFAULT: '',
  ROUE_TEAM_BANK: { Rouges: 1300, Bleus: 400 }, ROUE_TURN_GAIN: 0, ROUE_CONSONANTS_LEFT: true,
  ROUE_LETTERS: [{ LETTER: 'C', TEAM: 'Bleus', HIT: true }, { LETTER: 'G', TEAM: 'Rouges', HIT: false }],
  ROUE_WHEEL: { SEGMENTS: [{ KIND: 'value', VALUE: 500 }], CURRENT: 0, SPIN_MAX_MS: 10000, STOP: null },
  ROUE_LAST_EVENT: { SEQ: 0, KIND: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }, ROUE_AWARDS: {}, ROUE_WINNER: '',
  ...over,
})
const teams = { Rouges: { COLOR: [239, 68, 68] }, Bleus: { COLOR: [59, 130, 246] } }
const setup = (state) => {
  const send = vi.fn()
  const utils = render(<AnimRoueView gameState={state} teams={teams} actions={makeRoueActions(send)} />)
  return { send, ...utils }
}

describe('AnimRoue — tablette animateur (#144, M7)', () => {
  it('étape 1 : seul « Lancer la roue » est actif et envoie ROUE_SPIN', () => {
    const { send, container } = setup(gs())
    const enabled = [...container.querySelectorAll('.anim-roue-steps button')].filter(b => !b.disabled)
    expect(enabled.length).toBe(1)
    fireEvent.click(enabled[0])
    expect(send).toHaveBeenCalledWith('ROUE_SPIN', {})
  })
  it('étape 4 : le clavier sert à la saisie, les consonnes jouées sont colorées par équipe', () => {
    const { send, container } = setup(gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 }))
    const c = screen.getByRole('button', { name: /^C/ })
    expect(c.disabled).toBe(true)
    expect(c.style.getPropertyValue('--c')).toBe('rgb(59,130,246)')
    fireEvent.click(screen.getByRole('button', { name: /^K/ }))
    fireEvent.click(screen.getByText(/lettre K/))
    expect(send).toHaveBeenCalledWith('ROUE_LETTER', { LETTER: 'K' })
    expect(container.querySelector('.roue-vow')).toBeNull()
  })
  it('étape 6 → réponse validée ; deux boutons d\'étape 7 actifs, le conseillé plein', () => {
    const { send, container } = setup(gs({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_NEXT_DEFAULT: 'NEXT' }))
    fireEvent.click(screen.getByText(/Réponse/, { selector: 'button' }))
    expect(send).toHaveBeenCalledWith('ROUE_ANSWER', {})
    const btns = [...container.querySelectorAll('.anim-roue-steps button')]
    const same = btns.find(b => /Tour/.test(b.textContent))
    const next = btns.find(b => /Équipe suivante/.test(b.textContent))
    expect(same.disabled || next.disabled).toBe(false)
    expect(next.className).toMatch(/cur/)
    expect(same.className).toMatch(/alt/)
    fireEvent.click(next)
    expect(send).toHaveBeenCalledWith('ROUE_NEXT', { HAND: 'NEXT' })
  })
  it('n\'expose jamais ROUE_SET_TEAMS, ROUE_PASS ni ROUE_REVEAL (admin seulement)', () => {
    const { send, container } = setup(gs())
    container.querySelectorAll('button').forEach(b => { if (!b.disabled) fireEvent.click(b) })
    const sent = send.mock.calls.map(c => c[0])
    expect(sent.filter(a => ['ROUE_SET_TEAMS', 'ROUE_PASS', 'ROUE_REVEAL'].includes(a))).toEqual([])
    expect(container.textContent).not.toMatch(/Passer la main|Dévoiler/)
  })
  it('SOLO : pas de bouton « Équipe suivante »', () => {
    setup(gs({ ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7, question: { TYPE: 'ROUE', ROUE_MODE: 'SOLO', ROUE_PHRASE: 'A' } }))
    expect(screen.queryByText(/Équipe suivante/)).toBeNull()
  })
  it('phrase : lettres trouvées à la couleur de l\'équipe', () => {
    const { container } = setup(gs())
    const f = container.querySelector('.anim-roue-sol span.f')
    expect(f.textContent).toBe('C')
    expect(f.style.getPropertyValue('--c')).toBe('rgb(59,130,246)')
  })
})
