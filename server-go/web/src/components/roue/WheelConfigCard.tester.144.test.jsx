/**
 * #144 (v12.0.0) — carte « Roue de la Fortune » : bouton « Tester » / « Arrêter » (contracts/roue.md §10.2, §11.3).
 * L'essai anime les deux aperçus existants avec les valeurs NON enregistrées ; montée → pleine vitesse → freinage → arrêt ;
 * aucun message WebSocket ni écriture ; durées et montée viennent de TIMING (aucune copie de règle dans la page).
 * Complète WheelConfigCard.trial.144.test.jsx (développeur : déroulement, arrêt automatique, interruption, aucun appel hors aperçu) :
 * ici seulement refus serveur, durées servies par TIMING, case d'arrêt commune aux deux aperçus, rejeu, démontage, reduced-motion.
 * Tirage de la case d'arrêt : Math.random (index = floor(r × n)).
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, act, screen } from '@testing-library/react'
import WheelConfigCard from './WheelConfigCard'

const DEF = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }
const seg = (i, kind, v = 0) => ({ ID: `s${i}`, KIND: kind, VALUE: v })
const TIMING = { SPIN_PERIOD_MS: 2500, SPIN_UP_MS: 5000, SPIN_MAX_MS: 10000, STOP_DURATION_MS: 4000 }
const PREVIEW = { COUNT: 4, step: 50, // même pas que la saisie : sinon la carte corrige le pas et relance un aperçu (qui interrompt l'essai)
   TIMING, SEGMENTS: [seg(0, 'value', 100), seg(1, 'bankrupt'), seg(2, 'value', 90), seg(3, 'lose_turn')] }
const json = (data, ok = true, status = 200) => ({ ok, status, json: async () => data, text: async () => JSON.stringify(data) })
let calls
const respond = (fn) => { global.fetch = vi.fn(async (url, opts) => { calls.push({ url, method: opts?.method, body: opts?.body ? JSON.parse(opts.body) : null }); return fn(url, opts) }) }
const ok = (over = {}) => respond(() => json({ ...PREVIEW, ...over }))
const advance = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }
const settle = async () => { await advance(400); await act(async () => { await Promise.resolve() }) }
const tester = () => screen.queryByRole('button', { name: /Tester/ })
const arreter = () => screen.queryByRole('button', { name: /Arrêter/ })
const click = (el) => act(async () => { fireEvent.click(el) })
const text = () => screen.getByTestId('wheel-trial-state').textContent // libellé d'état (aria-live)
let ws

beforeEach(() => {
  vi.useFakeTimers(); calls = []
  ws = vi.fn(); global.WebSocket = ws
  vi.spyOn(Math, 'random').mockReturnValue(0.55) // 4 cases → index 2 (« 90 »)
})
afterEach(() => { cleanup(); vi.useRealTimers(); vi.restoreAllMocks() })

describe('WheelConfigCard — Tester : disponibilité', () => {
  it('sans aperçu valide (refus serveur) : « Tester » absent ou grisé', async () => {
    respond(() => json({ CODE: 'ROUE_WHEEL_INVALID', MESSAGE: 'pas ≥ maximum' }, false, 400))
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    const b = tester()
    expect(b === null || b.disabled).toBe(true)
  })
})

describe('WheelConfigCard — Tester : déroulement (temps de TIMING)', () => {
  it('les durées viennent de TIMING, pas d\'un calcul de la page (montée de 1 234 ms servie par le serveur)', async () => {
    ok({ TIMING: { ...TIMING, SPIN_UP_MS: 1234, SPIN_MAX_MS: 3000, STOP_DURATION_MS: 1500 } })
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    await click(tester())
    await advance(1100)
    expect(text()).toMatch(/Mont.e…/)
    await advance(300)
    expect(text()).toMatch(/Pleine vitesse/)
    await advance(3100)
    expect(text()).toMatch(/Freinage/)
    await advance(1600)
    expect(text()).toMatch(/Arrêt sur 90/)
  })

  it('le bandeau et la vue d\'ensemble s\'arrêtent ensemble ; la roue ne bouge plus après l\'arrêt', async () => {
    ok()
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    const ring = () => container.querySelector('.wheel-cfg-ring').innerHTML
    const strip = () => container.querySelector('[data-testid="fortune-wheel"] [role="img"]')
    const rest = ring()
    await click(tester())
    await advance(5100)
    const a = ring(); await advance(300)
    expect(ring()).not.toBe(a) // la vue d'ensemble tourne à pleine vitesse
    await click(arreter())
    await advance(4300)
    expect(text()).toMatch(/Arrêt sur 90/)
    expect(Number(strip().getAttribute('data-pos'))).toBe(2) // même case que le libellé
    const stopped = ring(); await advance(500)
    expect(ring()).toBe(stopped)
    expect(stopped).not.toBe(rest) // arrêtée sur la case tirée, pas sur la case de départ
  })

  it('on peut rejouer l\'essai après l\'arrêt', async () => {
    ok()
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    await click(tester()); await advance(5100); await click(arreter()); await advance(4300)
    await click(tester())
    expect(text()).toMatch(/Mont.e…/)
  })
})

describe('WheelConfigCard — Tester : interruption', () => {
  it('démontage pendant l\'essai : minuteries annulées, aucune erreur', async () => {
    ok()
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    const { unmount } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    await click(tester()); await advance(1000)
    unmount()
    await advance(30000)
    expect(err).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })
})

describe('WheelConfigCard — Tester : prefers-reduced-motion', () => {
  it('les temps s\'enchaînent et la roue est placée directement sur la case tirée', async () => {
    ok()
    window.matchMedia = vi.fn().mockImplementation((q) => ({ matches: /reduce/.test(q), media: q, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} }))
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    await click(tester()); await advance(5100); await click(arreter()); await advance(4300)
    expect(text()).toMatch(/Arrêt sur 90/)
    expect(Number(container.querySelector('[data-testid="fortune-wheel"] [role="img"]').getAttribute('data-pos'))).toBe(2)
    delete window.matchMedia
  })
})
