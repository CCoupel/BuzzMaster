/**
 * #144 (v12.0.0) — carte « Roue de la Fortune » : bouton « Tester » / « Arrêter » (contracts/roue.md §10.2, §11.3).
 * L'essai anime les deux aperçus existants avec les valeurs NON enregistrées ; montée → pleine vitesse → freinage → arrêt ;
 * aucun message WebSocket ni écriture ; durées et montée viennent de TIMING (aucune copie de règle dans la page).
 * Hypothèse d'implémentation : le tirage de la case d'arrêt utilise Math.random (tirage uniforme côté client).
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, act, screen } from '@testing-library/react'
import WheelConfigCard from './WheelConfigCard'

const DEF = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }
const seg = (i, kind, v = 0) => ({ ID: `s${i}`, KIND: kind, VALUE: v })
const TIMING = { SPIN_PERIOD_MS: 2500, SPIN_UP_MS: 5000, SPIN_MAX_MS: 10000, STOP_DURATION_MS: 4000 }
const PREVIEW = { COUNT: 4, step: 10, TIMING, SEGMENTS: [seg(0, 'value', 100), seg(1, 'bankrupt'), seg(2, 'value', 90), seg(3, 'lose_turn')] }
const json = (data, ok = true, status = 200) => ({ ok, status, json: async () => data, text: async () => JSON.stringify(data) })
let calls
const respond = (fn) => { global.fetch = vi.fn(async (url, opts) => { calls.push({ url, method: opts?.method, body: opts?.body ? JSON.parse(opts.body) : null }); return fn(url, opts) }) }
const ok = (over = {}) => respond(() => json({ ...PREVIEW, ...over }))
const advance = async (ms) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }
const settle = () => advance(400)
const tester = () => screen.queryByRole('button', { name: /Tester/ })
const arreter = () => screen.queryByRole('button', { name: /Arrêter/ })
const click = (el) => act(async () => { fireEvent.click(el) })
const text = (c) => c.textContent
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
  it('aperçu valide : « Tester » actif, « Arrêter » grisé ou absent, aucun libellé d\'état', async () => {
    ok()
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    expect(tester().disabled).toBe(false)
    expect(arreter() === null || arreter().disabled).toBe(true)
    expect(text(container)).not.toMatch(/Montée…|Pleine vitesse|Freinage/)
  })
})

describe('WheelConfigCard — Tester : déroulement (temps de TIMING)', () => {
  it('Montée → Pleine vitesse → Arrêter → Freinage → Arrêt sur la case tirée ; boutons selon le tableau §10.2', async () => {
    ok()
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    await click(tester())
    expect(text(container)).toMatch(/Montée…/)
    expect(tester().disabled).toBe(true)
    expect(arreter().disabled).toBe(true) // jamais d'arrêt pendant la montée
    await advance(2500)
    expect(text(container)).toMatch(/Montée…/)
    expect(arreter().disabled).toBe(true)
    await advance(2600) // SPIN_UP_MS = 5000 écoulé
    expect(text(container)).toMatch(/Pleine vitesse/)
    expect(tester().disabled).toBe(true)
    expect(arreter().disabled).toBe(false)
    await click(arreter())
    expect(text(container)).toMatch(/Freinage/)
    expect(tester().disabled).toBe(true); expect(arreter().disabled).toBe(true)
    await advance(4200) // STOP_DURATION_MS
    expect(text(container)).toMatch(/Arrêt sur 90/)
    expect(tester().disabled).toBe(false); expect(arreter().disabled).toBe(true)
  })

  it('sans clic sur « Arrêter » : freinage à la fin de SPIN_MAX_MS compté depuis la pleine vitesse', async () => {
    ok()
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    await click(tester())
    await advance(5100) // pleine vitesse
    await advance(9000)
    expect(text(container)).toMatch(/Pleine vitesse/) // 9 s < SPIN_MAX_MS
    await advance(1300)
    expect(text(container)).toMatch(/Freinage/)
    await advance(4200)
    expect(text(container)).toMatch(/Arrêt sur 90/)
  })

  it('les durées viennent de TIMING, pas d\'un calcul de la page (montée de 1 234 ms servie par le serveur)', async () => {
    ok({ TIMING: { ...TIMING, SPIN_UP_MS: 1234, SPIN_MAX_MS: 3000, STOP_DURATION_MS: 1500 } })
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    await click(tester())
    await advance(1100)
    expect(text(container)).toMatch(/Montée…/)
    await advance(300)
    expect(text(container)).toMatch(/Pleine vitesse/)
    await advance(3100)
    expect(text(container)).toMatch(/Freinage/)
    await advance(1600)
    expect(text(container)).toMatch(/Arrêt sur 90/)
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
    expect(text(container)).toMatch(/Arrêt sur 90/)
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
    expect(text(container)).toMatch(/Montée…/)
  })
})

describe('WheelConfigCard — Tester : sans effet sur le jeu', () => {
  it('aucun WebSocket, aucune écriture : seul appel réseau = wheel-preview', async () => {
    ok()
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    await click(tester()); await advance(5100); await click(arreter()); await advance(4300)
    expect(ws).not.toHaveBeenCalled()
    expect(calls.every(c => c.url === '/api/roue/wheel-preview')).toBe(true)
    expect(calls.some(c => c.url === '/game-config.json')).toBe(false)
  })
  it('les durées saisies (non enregistrées) partent dans la requête d\'aperçu', async () => {
    ok()
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    fireEvent.change(screen.getByLabelText("Durée d'un tour de roue"), { target: { value: '3' } })
    fireEvent.change(screen.getByLabelText('Durée maximale de rotation'), { target: { value: '12' } })
    fireEvent.change(screen.getByLabelText('Durée du freinage'), { target: { value: '5' } })
    await settle()
    const last = calls.filter(c => c.url === '/api/roue/wheel-preview').pop().body
    expect(last).toMatchObject({ spin_period_ms: 3000, spin_max_ms: 12000, stop_duration_ms: 5000 })
  })
})

describe('WheelConfigCard — Tester : interruption', () => {
  it('modifier un réglage pendant l\'essai l\'interrompt : plus d\'état, aperçus fixes, « Tester » actif', async () => {
    ok()
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    await click(tester()); await advance(5100)
    expect(text(container)).toMatch(/Pleine vitesse/)
    fireEvent.change(screen.getByLabelText('Points maximum'), { target: { value: '500' } })
    await settle()
    expect(text(container)).not.toMatch(/Montée…|Pleine vitesse|Freinage/)
    expect(tester().disabled).toBe(false)
    const frozen = container.querySelector('.wheel-cfg-ring').innerHTML
    await advance(15000)
    expect(container.querySelector('.wheel-cfg-ring').innerHTML).toBe(frozen)
    expect(text(container)).not.toMatch(/Arrêt sur/) // la minuterie annulée ne déclenche aucun freinage
  })
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
    expect(text(container)).toMatch(/Arrêt sur 90/)
    expect(Number(container.querySelector('[data-testid="fortune-wheel"] [role="img"]').getAttribute('data-pos'))).toBe(2)
    delete window.matchMedia
  })
})
