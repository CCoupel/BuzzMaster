/**
 * #144 (v12.0.0, LOT-1.2) — carte « Roue de la Fortune » : spécification (contracts/roue.md §10, §11.3, maquette M4).
 * Complète WheelConfigCard.144.test.jsx (développeur). Principe vérifié : AUCUNE règle de construction côté
 * navigateur — l'aperçu et le pas retenu viennent du serveur.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, act, screen } from '@testing-library/react'
import WheelConfigCard from './WheelConfigCard'

const DEF = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }
const seg = (i, kind, v = 0) => ({ ID: `s${i}`, KIND: kind, VALUE: v })
const PREVIEW = { COUNT: 4, step: 10, SEGMENTS: [seg(0, 'value', 100), seg(1, 'bankrupt'), seg(2, 'value', 90), seg(3, 'lose_turn')] }
const json = (data, ok = true, status = 200) => ({ ok, status, json: async () => data, text: async () => (typeof data === 'string' ? data : JSON.stringify(data)) })
let calls
const respond = (fn) => { global.fetch = vi.fn(async (url, opts) => { calls.push({ url, method: opts?.method, body: opts?.body ? JSON.parse(opts.body) : null }); return fn(url, opts) }) }
const echo = (o) => json({ ...PREVIEW, step: JSON.parse(o.body).step })
const settle = async () => { await act(async () => { await vi.advanceTimersByTimeAsync(400) }) }
const previews = () => calls.filter(c => c.url === '/api/roue/wheel-preview')

beforeEach(() => { vi.useFakeTimers(); calls = [] })
afterEach(() => { cleanup(); vi.useRealTimers() })

describe('WheelConfigCard — spécification', () => {
  it('affiche les durées en secondes (ms ÷ 1000)', async () => {
    respond((u, o) => echo(o))
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    expect(screen.getByLabelText("Durée d'un tour de roue").value).toBe('2.5')
    expect(screen.getByLabelText('Durée maximale de rotation').value).toBe('10')
    expect(screen.getByLabelText('Durée du freinage').value).toBe('4')
  })

  it('l\'aperçu montre EXACTEMENT les cases du serveur (aucune règle recopiée)', async () => {
    // réponse volontairement différente de ce que produirait la règle : la carte doit l'afficher telle quelle
    respond(() => json({ COUNT: 3, step: 50, SEGMENTS: [seg(0, 'value', 7), seg(1, 'value', 3), seg(2, 'lose_turn')] }))
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    const cells = Array.from(container.querySelectorAll('.wheel-cfg-pc')).map(e => e.textContent)
    expect(cells).toHaveLength(3)
    expect(cells[0]).toMatch(/7/)
    expect(cells[1]).toMatch(/3/)
  })

  it('anti-rebond : des modifications rapides ne produisent qu\'un appel d\'aperçu', async () => {
    respond((u, o) => echo(o))
    render(<WheelConfigCard wheel={{ ...DEF, max_points: 100, step: 10 }} wheelDefault={DEF} />)
    await settle()
    calls = []
    const input = screen.getByLabelText('Points maximum')
    for (const v of ['200', '300', '400']) fireEvent.change(input, { target: { value: v } })
    await settle()
    expect(previews()).toHaveLength(1)
    expect(previews()[0].body.max_points).toBe(400)
  })

  it('champ vidé : aucun appel serveur, message, Enregistrer désactivé', async () => {
    respond((u, o) => echo(o))
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    calls = []
    fireEvent.change(screen.getByLabelText('Pas'), { target: { value: '' } })
    await settle()
    expect(previews()).toHaveLength(0)
    expect(screen.getByRole('alert')).toBeTruthy()
    expect(screen.getByText('Enregistrer').closest('button').disabled).toBe(true)
  })

  it('serveur injoignable : message, Enregistrer désactivé', async () => {
    respond(() => { throw new TypeError('network') })
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    expect(screen.getByRole('alert').textContent).toMatch(/indisponible|injoignable/i)
    expect(screen.getByText('Enregistrer').closest('button').disabled).toBe(true)
  })

  it('l\'enregistrement envoie la section wheel COMPLÈTE (10 clés, dont bonus_count depuis #309 et bonus_mode depuis #317), flip_interval_ms conservé tel quel', async () => {
    respond((url, o) => (url === '/game-config.json' ? json({ wheel: DEF }) : echo(o)))
    render(<WheelConfigCard wheel={{ ...DEF, flip_interval_ms: 900 }} wheelDefault={DEF} />)
    await settle()
    await act(async () => { fireEvent.click(screen.getByText('Enregistrer')); await vi.advanceTimersByTimeAsync(10) })
    const save = calls.find(c => c.url === '/game-config.json')
    expect(save.method).toBe('POST')
    expect(Object.keys(save.body.wheel).sort()).toEqual(
      ['bankrupt_count', 'bonus_count', 'bonus_mode', 'flip_interval_ms', 'lose_turn_count', 'max_points', 'spin_max_ms', 'spin_period_ms', 'step', 'stop_duration_ms'])
    expect(save.body.wheel.flip_interval_ms).toBe(900)
  })

  it('après enregistrement : onSaved reçoit la section retenue par le serveur', async () => {
    const onSaved = vi.fn()
    const kept = { ...DEF, step: 25, max_points: 100 }
    respond((url, o) => (url === '/game-config.json' ? json({ wheel: kept }) : echo(o)))
    render(<WheelConfigCard wheel={{ ...DEF, max_points: 100, step: 10 }} wheelDefault={DEF} onSaved={onSaved} />)
    await settle()
    await act(async () => { fireEvent.click(screen.getByText('Enregistrer')); await vi.advanceTimersByTimeAsync(10) })
    expect(onSaved).toHaveBeenCalledTimes(1)
    expect(onSaved.mock.calls[0][0]).toMatchObject({ step: 25, max_points: 100 })
  })

  it('échec de l\'enregistrement : message du serveur, onSaved non appelé', async () => {
    const onSaved = vi.fn()
    respond((url, o) => (url === '/game-config.json' ? json('wheel: la roue aurait 65 cases', false, 400) : echo(o)))
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} onSaved={onSaved} />)
    await settle()
    await act(async () => { fireEvent.click(screen.getByText('Enregistrer')); await vi.advanceTimersByTimeAsync(10) })
    expect(screen.getByRole('alert').textContent).toMatch(/65 cases/)
    expect(onSaved).not.toHaveBeenCalled()
  })

  it('« Roue par défaut » est désactivé sans wheel_default', async () => {
    respond((u, o) => echo(o))
    render(<WheelConfigCard wheel={DEF} wheelDefault={null} />)
    await settle()
    expect(screen.getByText(/Roue par défaut/).closest('button').disabled).toBe(true)
  })

  it('« Roue par défaut » déclenche un nouvel aperçu avec les paramètres par défaut', async () => {
    respond((u, o) => echo(o))
    render(<WheelConfigCard wheel={{ ...DEF, max_points: 100, step: 10, bankrupt_count: 0, lose_turn_count: 0 }} wheelDefault={DEF} />)
    await settle()
    calls = []
    fireEvent.click(screen.getByText(/Roue par défaut/))
    await settle()
    expect(previews().pop().body).toMatchObject({ max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1 })
  })
})
