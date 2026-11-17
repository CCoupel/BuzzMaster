import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, act, screen } from '@testing-library/react'
import WheelConfigCard from './WheelConfigCard'

const DEF = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }
const seg = (i, kind, v = 0) => ({ ID: `s${i}`, KIND: kind, VALUE: v })
const PREVIEW = { COUNT: 4, step: 10, SEGMENTS: [seg(0, 'value', 100), seg(1, 'bankrupt'), seg(2, 'value', 90), seg(3, 'lose_turn')] }

let calls
// L'aperçu simulé renvoie le pas demandé (aucune correction) sauf si le test répond autrement
const echo = (o) => json({ ...PREVIEW, step: JSON.parse(o.body).step })
const respond = (fn) => { global.fetch = vi.fn(async (url, opts) => { calls.push({ url, body: opts?.body ? JSON.parse(opts.body) : null }); return fn(url, opts) }) }
const json = (data, ok = true, status = 200) => ({ ok, status, json: async () => data, text: async () => JSON.stringify(data) })
const settle = async () => { await act(async () => { await vi.advanceTimersByTimeAsync(400) }) }

beforeEach(() => { vi.useFakeTimers(); calls = [] })
afterEach(() => { cleanup(); vi.useRealTimers() })

describe('WheelConfigCard — carte « Roue de la Fortune » (#144, M4)', () => {
  it('aperçu par POST /api/roue/wheel-preview avec les 4 paramètres, résumé issu de la réponse', async () => {
    respond((u, o) => echo(o))
    render(<WheelConfigCard wheel={{ ...DEF, max_points: 100, step: 10, bankrupt_count: 1, lose_turn_count: 1 }} wheelDefault={DEF} />)
    await settle()
    expect(calls[0].url).toBe('/api/roue/wheel-preview')
    expect(calls[0].body).toMatchObject({ max_points: 100, step: 10, bankrupt_count: 1, lose_turn_count: 1 })
    expect(screen.getByTestId('wheel-config-card').querySelector('.wheel-cfg-sum').textContent).toMatch(/4 cases/)
    expect(screen.getByTestId('wheel-config-card').querySelectorAll('.wheel-cfg-pc').length).toBe(4)
  })

  it('pas corrigé : le champ prend la valeur retenue et le message « Pas ajusté à » s\'affiche', async () => {
    respond((url, o) => {
      const b = JSON.parse(o.body)
      return json(b.step === 30 ? { ...PREVIEW, step: 25, step_adjusted_from: 30 } : { ...PREVIEW, step: b.step })
    })
    render(<WheelConfigCard wheel={{ ...DEF, max_points: 100, step: 30, bankrupt_count: 1, lose_turn_count: 1 }} wheelDefault={DEF} />)
    await settle()
    expect(screen.getByLabelText('Pas').value).toBe('25')
    expect(screen.getByRole('status').textContent).toMatch(/Pas ajusté à 25/)
  })

  it('refus 400 : message affiché, Enregistrer désactivé', async () => {
    respond(() => json({ CODE: 'ROUE_WHEEL_INVALID', MESSAGE: 'le pas doit être plus petit que le maximum' }, false, 400))
    render(<WheelConfigCard wheel={{ ...DEF, step: 1000 }} wheelDefault={DEF} />)
    await settle()
    expect(screen.getByRole('alert').textContent).toMatch(/plus petit que le maximum/)
    expect(screen.getByText('Enregistrer').closest('button').disabled).toBe(true)
  })

  it('« Roue par défaut » reprend wheel_default', async () => {
    respond((u, o) => echo(o))
    render(<WheelConfigCard wheel={{ ...DEF, max_points: 100, step: 10 }} wheelDefault={DEF} />)
    await settle()
    fireEvent.click(screen.getByText(/Roue par défaut/))
    expect(screen.getByLabelText('Points maximum').value).toBe('1000')
    expect(screen.getByLabelText('Pas').value).toBe('50')
  })

  it('enregistrement : POST /game-config.json { wheel } avec durées en millisecondes', async () => {
    respond((url, o) => (url === '/game-config.json' ? json({ wheel: { ...DEF, spin_max_ms: 12000 } }) : echo(o)))
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    fireEvent.change(screen.getByLabelText('Durée maximale de rotation'), { target: { value: '12' } })
    await settle()
    await act(async () => { fireEvent.click(screen.getByText('Enregistrer')); await vi.advanceTimersByTimeAsync(10) })
    const save = calls.find(c => c.url === '/game-config.json')
    expect(save.body.wheel).toMatchObject({ max_points: 1000, step: 50, spin_max_ms: 12000, stop_duration_ms: 4000, flip_interval_ms: 600 })
    expect(screen.getByText('Roue enregistrée.')).toBeTruthy()
  })
})
