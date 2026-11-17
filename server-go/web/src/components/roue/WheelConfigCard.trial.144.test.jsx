import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, act, fireEvent, screen } from '@testing-library/react'
import WheelConfigCard from './WheelConfigCard'

/**
 * Essai « Tester » de la carte de réglage (roue.md §10.2) : montée → pleine vitesse → freinage → arrêt, libellés et boutons,
 * aucun message WebSocket, interrompu par un changement de réglage, durées issues de `TIMING`.
 */
afterEach(() => { cleanup(); vi.useRealTimers(); vi.restoreAllMocks() })
beforeEach(() => { vi.useFakeTimers({ shouldAdvanceTime: false }) })

const wheel = { max_points: 100, step: 10, bankrupt_count: 1, lose_turn_count: 1, spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }
const SEG = Array.from({ length: 12 }, (_, i) => ({ ID: `s${i}`, KIND: i === 5 ? 'bankrupt' : 'value', VALUE: 10 * (i + 1) }))
const TIMING = { SPIN_PERIOD_MS: 2500, SPIN_UP_MS: 5000, SPIN_MAX_MS: 10000, STOP_DURATION_MS: 4000 }
const mockFetch = (timing = TIMING) => vi.spyOn(globalThis, 'fetch').mockImplementation(async (url, init) => {
  if (String(url).includes('wheel-preview')) return { ok: true, json: async () => ({ COUNT: 12, step: 10, SEGMENTS: SEG, ...(timing ? { TIMING: timing } : {}) }) }
  return { ok: true, json: async () => ({}), text: async () => '' }
})
const run = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }
const setup = async (timing) => {
  const f = mockFetch(timing)
  const r = render(<WheelConfigCard wheel={wheel} wheelDefault={wheel} />)
  await run(400) // délai de l'aperçu + réponse
  await act(async () => { await Promise.resolve() })
  return { ...r, f }
}
const btn = (name) => screen.getByRole('button', { name })
const state = () => screen.getByTestId('wheel-trial-state').textContent

describe('essai de la roue — déroulement (roue.md §10.2)', () => {
  it('repos : « Tester » actif, « Arrêter » grisé, aucun libellé', async () => {
    await setup()
    expect(btn(/Tester/).disabled).toBe(false)
    expect(btn(/Arrêter/).disabled).toBe(true)
    expect(state()).toBe('')
  })

  it('montée (SPIN_UP_MS de TIMING) : « Montée… », les deux boutons grisés ; puis pleine vitesse : « Arrêter » actif + temps restant', async () => {
    await setup()
    fireEvent.click(btn(/Tester/))
    expect(state()).toBe('Montée…')
    expect(btn(/Tester/).disabled).toBe(true)
    expect(btn(/Arrêter/).disabled).toBe(true)
    await run(4900)
    expect(state()).toBe('Montée…')
    await run(200)
    expect(state()).toMatch(/Pleine vitesse/)
    expect(btn(/Arrêter/).disabled).toBe(false)
    expect(btn(/Tester/).disabled).toBe(true)
  })

  it('« Arrêter » : freinage (tout grisé), puis « Arrêt sur … » et « Tester » de nouveau actif', async () => {
    await setup()
    fireEvent.click(btn(/Tester/))
    await run(5100)
    fireEvent.click(btn(/Arrêter/))
    expect(state()).toBe('Freinage')
    expect(btn(/Tester/).disabled).toBe(true)
    expect(btn(/Arrêter/).disabled).toBe(true)
    await run(4200)
    expect(state()).toMatch(/^Arrêt sur /)
    expect(btn(/Tester/).disabled).toBe(false)
    expect(btn(/Arrêter/).disabled).toBe(true)
  })

  it('sans clic : arrêt automatique à SPIN_MAX_MS compté depuis la pleine vitesse', async () => {
    await setup()
    fireEvent.click(btn(/Tester/))
    await run(5000 + 9800)
    expect(state()).toMatch(/Pleine vitesse/)
    await run(400)
    expect(state()).toBe('Freinage')
    await run(4200)
    expect(state()).toMatch(/^Arrêt sur /)
  })

  it('les deux aperçus s\'animent ensemble : le bandeau et la vue d\'ensemble (rotor) existent et le rotor tourne', async () => {
    await setup()
    const rotor = screen.getByTestId('wheel-cfg-ring-rotor')
    const before = rotor.getAttribute('transform')
    fireEvent.click(btn(/Tester/))
    await run(4000) // en montée : la roue a bougé
    expect(rotor.getAttribute('transform')).not.toBe(before)
  })

  it('modifier un réglage pendant l\'essai l\'interrompt : repos, boutons d\'origine', async () => {
    await setup()
    fireEvent.click(btn(/Tester/))
    await run(6000)
    expect(state()).toMatch(/Pleine vitesse/)
    fireEvent.change(screen.getByLabelText(/Durée du freinage/), { target: { value: '3' } })
    expect(state()).toBe('')
    expect(btn(/Arrêter/).disabled).toBe(true)
  })

  it('aucun message hors aperçu : seul appel réseau = wheel-preview', async () => {
    const { f } = await setup()
    fireEvent.click(btn(/Tester/))
    await run(5100)
    fireEvent.click(btn(/Arrêter/))
    await run(4200)
    for (const [url] of f.mock.calls) expect(String(url)).toMatch(/wheel-preview/)
  })

  it('aperçu sans TIMING (serveur ancien) : « Tester » grisé', async () => {
    await setup(null)
    expect(btn(/Tester/).disabled).toBe(true)
  })

  it('les durées saisies partent dans la requête d\'aperçu ; TIMING fait foi pour l\'essai', async () => {
    const { f } = await setup()
    const bodies = f.mock.calls.filter(([u]) => String(u).includes('wheel-preview')).map(([, i]) => JSON.parse(i.body))
    expect(bodies[0]).toMatchObject({ spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000 })
  })
})
