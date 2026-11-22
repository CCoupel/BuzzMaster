// Review LOT-1.2 M2 — TURN sans consonne restante, achat après l'arrêt possible : « Réponse validée » et boutons de fin de tour présents.
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import RouePanel from './RouePanel'

const gs = {
  phase: 'STARTED', question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE' }, ROUE_SUBPHASE: 'TURN', ROUE_STEP: 1,
  ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_NEXT_TEAM: 'Bleus', ROUE_TEAM_BANK: { Rouges: 900, Bleus: 0 },
  ROUE_CONSONANTS_LEFT: false, ROUE_BUY_VOWELS: true, ROUE_VOWEL_TIMING: 'AFTER', ROUE_VOWELS_BEFORE: 'MANY', ROUE_VOWELS_AFTER: 'MANY',
  ROUE_VOWEL_PRICE: 250, ROUE_VOWEL_GAIN: 'NONE', ROUE_VOWELS_LEFT: true, ROUE_TURN_VOWEL: false, ROUE_STOP_VOWEL: false, ROUE_LETTERS: [],
}
const mk = () => ({ spin: vi.fn(), answer: vi.fn(), next: vi.fn(), pass: vi.fn(), reveal: vi.fn(), flip: vi.fn(), stop: vi.fn(), letter: vi.fn(), buyVowel: vi.fn(), setTeams: vi.fn() })

describe('RouePanel — sans consonne, achat après l\'arrêt possible', () => {
  it('propose Lancer la roue, Réponse validée et Tour suivant / Équipe suivante', () => {
    const a = mk()
    render(<RouePanel gameState={gs} actions={a} />)
    expect(screen.getByRole('button', { name: /Lancer la roue/ })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /Réponse validée/ }))
    expect(a.answer).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button', { name: /Équipe suivante/ }))
    expect(a.next).toHaveBeenCalledWith('NEXT')
  })
})
