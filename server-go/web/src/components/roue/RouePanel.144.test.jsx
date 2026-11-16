import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen } from '@testing-library/react'
import RouePanel from './RouePanel'
import { makeRoueActions } from './roueState'

afterEach(() => cleanup())

const gs = (over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_PHRASE: 'COMBIEN POUR CE CHIEN', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_GAINS_RULE: 'WINNER_ONLY', ROUE_PRESENTATION: 'Chanson' },
  ROUE_SUBPHASE: 'TURN', ROUE_STEP: 1, ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_NEXT_TEAM: 'Bleus', ROUE_NEXT_DEFAULT: '',
  ROUE_TEAM_BANK: { Rouges: 1300, Bleus: 0 }, ROUE_TURN_GAIN: 0, ROUE_CONSONANTS_LEFT: true, ROUE_LETTERS: [],
  ROUE_WHEEL: { SEGMENTS: [{ KIND: 'value', VALUE: 500 }], CURRENT: 0, SPIN_MAX_MS: 10000, STOP: null },
  ROUE_LAST_EVENT: { SEQ: 0, KIND: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }, ROUE_AWARDS: {}, ROUE_WINNER: '',
  ...over,
})
const setup = (state, extra = {}) => {
  const send = vi.fn()
  const utils = render(<RouePanel gameState={state} actions={makeRoueActions(send)} teamList={[]} {...extra} />)
  return { send, ...utils }
}

describe('RouePanel — commandes admin (#144, M2)', () => {
  it('étape 1 : « Lancer la roue » envoie ROUE_SPIN ; bandeau des 7 étapes sur ROUE_STEP', () => {
    const { send, container } = setup(gs())
    fireEvent.click(screen.getByText(/Lancer la roue/, { selector: 'button' }))
    expect(send).toHaveBeenCalledWith('ROUE_SPIN', {})
    expect(container.querySelectorAll('.roue-step').length).toBe(7)
    expect(container.querySelector('.roue-step.cur').textContent).toMatch(/^1/)
  })
  it('étape 2 : « Forcer l\'arrêt » → ROUE_STOP', () => {
    const { send } = setup(gs({ ROUE_SUBPHASE: 'SPINNING', ROUE_STEP: 2 }))
    fireEvent.click(screen.getByText(/Forcer l'arrêt/))
    expect(send).toHaveBeenCalledWith('ROUE_STOP', {})
  })
  it('étape 4 : sélection d\'une consonne puis validation → ROUE_LETTER ; voyelles non cliquables', () => {
    const { send } = setup(gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4, ROUE_LETTERS: [{ LETTER: 'T', TEAM: 'Bleus', HIT: true }] }))
    expect(screen.getByRole('button', { name: 'T' }).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'C' }))
    fireEvent.click(screen.getByText('Valider la lettre C'))
    expect(send).toHaveBeenCalledWith('ROUE_LETTER', { LETTER: 'C' })
    expect(screen.getByRole('button', { name: 'A' }).disabled).toBe(true)
  })
  it('clavier physique : lettre puis Entrée', () => {
    const { send } = setup(gs({ ROUE_SUBPHASE: 'LETTER', ROUE_STEP: 4 }))
    fireEvent.keyDown(window, { key: 'k' })
    fireEvent.keyDown(window, { key: 'Enter' })
    expect(send).toHaveBeenCalledWith('ROUE_LETTER', { LETTER: 'K' })
  })
  it('étape 5 : « Retourner les cases » → ROUE_FLIP', () => {
    const { send } = setup(gs({ ROUE_SUBPHASE: 'LIT', ROUE_STEP: 5, ROUE_LAST_EVENT: { LETTER: 'C', COUNT: 3, VALUE: 500, GAIN: 1500 } }))
    fireEvent.click(screen.getByText(/Retourner les cases/))
    expect(send).toHaveBeenCalledWith('ROUE_FLIP', {})
  })
  it('étape 6 : un seul « Réponse validée » ; deux boutons d\'étape 7, le conseillé en avant', () => {
    const { send, container } = setup(gs({ ROUE_SUBPHASE: 'ANSWER', ROUE_STEP: 6, ROUE_NEXT_DEFAULT: 'NEXT' }))
    expect(screen.getAllByText(/Réponse validée/).length).toBe(1)
    fireEvent.click(screen.getByText(/Réponse validée/))
    expect(send).toHaveBeenCalledWith('ROUE_ANSWER', {})
    const same = screen.getByText(/Tour suivant/)
    const next = screen.getByText(/Équipe suivante/)
    expect(next.className).toMatch(/primary/)
    expect(same.className).not.toMatch(/primary/)
    fireEvent.click(next)
    fireEvent.click(same)
    expect(send).toHaveBeenCalledWith('ROUE_NEXT', { HAND: 'NEXT' })
    expect(send).toHaveBeenCalledWith('ROUE_NEXT', { HAND: 'SAME' })
    expect(container.querySelectorAll('.roue-btn.primary').length).toBe(1)
  })
  it('SOLO : « Équipe suivante » et « Passer la main » indisponibles', () => {
    setup(gs({ ROUE_SUBPHASE: 'TURN_END', ROUE_STEP: 7, question: { TYPE: 'ROUE', ROUE_MODE: 'SOLO' } }))
    expect(screen.queryByText(/Équipe suivante/)).toBeNull()
    expect(screen.getByText(/Passer la main/).disabled).toBe(true)
  })
  it('« Dévoiler » demande une confirmation avant ROUE_REVEAL', () => {
    const { send } = setup(gs())
    fireEvent.click(screen.getByText(/Dévoiler la phrase/))
    expect(send).not.toHaveBeenCalled()
    fireEvent.click(screen.getByText(/Confirmer/))
    expect(send).toHaveBeenCalledWith('ROUE_REVEAL', {})
  })
  it('refus serveur affiché (voyelle non disponible)', () => {
    setup(gs({ ROUE_SUBPHASE: 'LETTER' }), { error: { CODE: 'VOWEL_NOT_AVAILABLE', MESSAGE: 'x', SEQ: 1 } })
    expect(screen.getByRole('alert').textContent).toBe('Voyelle non disponible')
  })
  it('PREPARE : choix des équipes → ROUE_SET_TEAMS', () => {
    const send = vi.fn()
    render(<RouePanel gameState={gs({ phase: 'PREPARE', ROUE_TEAMS: ['Rouges'] })} actions={makeRoueActions(send)}
      teamList={[{ name: 'Rouges', color: '#f00', ready: true }, { name: 'Bleus', color: '#00f', ready: true }]} waitReasons={['sélectionnez au moins deux équipes participantes']} />)
    fireEvent.click(screen.getByLabelText(/Bleus/))
    expect(send).toHaveBeenCalledWith('ROUE_SET_TEAMS', { TEAMS: ['Rouges', 'Bleus'] })
    expect(screen.getByText(/au moins deux équipes/)).toBeTruthy()
  })
  it('SOLVED : message de gain', () => {
    setup(gs({ ROUE_SUBPHASE: 'SOLVED', ROUE_WINNER: 'Rouges', ROUE_AWARDS: { Rouges: 1300 } }))
    expect(screen.getByText(/Les Rouges ont trouvé la phrase/).textContent).toMatch(/1\s?300/)
  })
})
