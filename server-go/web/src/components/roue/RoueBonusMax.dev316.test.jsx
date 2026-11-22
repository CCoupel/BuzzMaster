// #316 (v12.2.0) — le « N au plus » du serveur (contrat roue.md §17.3) est montré à l'utilisateur (éditeur et carte Paramètres).
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen, act } from '@testing-library/react'
import { roueRefusalText } from './RoueEditor'
import WheelConfigCard from './WheelConfigCard'

const MSG = 'Trop de BONUS : 2 au plus pour cette roue (un BONUS ne touche jamais une Banqueroute, un Perdu ni un autre BONUS).'
afterEach(() => cleanup())

describe('roueRefusalText — ROUE_BONUS_COUNT_INVALID', () => {
  it('affiche le détail du serveur (« N au plus »), comme ROUE_WHEEL_INVALID', () => {
    expect(roueRefusalText(`ROUE_BONUS_COUNT_INVALID: ${MSG}`)).toBe(MSG)
  })
  it('sans détail : texte générique', () => {
    expect(roueRefusalText('ROUE_BONUS_COUNT_INVALID')).toMatch(/BONUS/)
  })
})

describe('WheelConfigCard — refus d\'enregistrement (texte brut du serveur)', () => {
  const DEF = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 2, bonus_mode: 'EPHEMERAL', spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }
  beforeEach(() => {
    vi.useFakeTimers()
    global.fetch = vi.fn(async (url) => (url === '/game-config.json'
      ? { ok: false, status: 400, text: async () => `ROUE_BONUS_COUNT_INVALID: ${MSG}\n`, json: async () => ({}) }
      : { ok: true, status: 200, json: async () => ({ COUNT: 22, step: 50, SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 1000, BONUS: false }] }) }))
  })
  afterEach(() => vi.useRealTimers())

  it('le message affiché n\'a plus le préfixe technique ROUE_…:', async () => {
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await act(async () => { await vi.advanceTimersByTimeAsync(400) })
    await act(async () => { fireEvent.click(screen.getByText('Enregistrer')); await vi.advanceTimersByTimeAsync(10) })
    const toast = screen.getByRole('alert')
    expect(toast.textContent).toBe(MSG)
    expect(toast.textContent).not.toMatch(/ROUE_/)
  })
})
