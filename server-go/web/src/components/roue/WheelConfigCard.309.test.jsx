/**
 * #309 (v12.1.0, LOT-2) — carte « Roue de la Fortune » des Paramètres : 5 paramètres en deux groupes, BONUS global (contracts/roue.md §16.1 ;
 * maquette docs/mockup/v12.1.0/ui/admin_settings__wheel_bonus.html). Principe inchangé : aucune règle de construction côté navigateur.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, act, screen, within } from '@testing-library/react'
import WheelConfigCard from './WheelConfigCard'

const DEF = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 0, spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }
const seg = (i, kind, v = 0) => ({ ID: `s${i}`, KIND: kind, VALUE: v })
const PREVIEW = { COUNT: 4, step: 50, SEGMENTS: [seg(0, 'value', 100), seg(1, 'bankrupt'), { ...seg(2, 'value', 800), BONUS: true }, seg(3, 'lose_turn')] }
const json = (data, ok = true, status = 200) => ({ ok, status, json: async () => data, text: async () => JSON.stringify(data) })
let calls
const respond = (fn) => { global.fetch = vi.fn(async (url, opts) => { calls.push({ url, method: opts?.method, body: opts?.body ? JSON.parse(opts.body) : null }); return fn(url, opts) }) }
const settle = async () => { await act(async () => { await vi.advanceTimersByTimeAsync(400) }) }
const previews = () => calls.filter(c => c.url === '/api/roue/wheel-preview')

beforeEach(() => { vi.useFakeTimers(); calls = []; respond(() => json(PREVIEW)) })
afterEach(() => { cleanup(); vi.useRealTimers() })

describe('WheelConfigCard — 5 paramètres en deux groupes (#309)', () => {
  it('deux groupes : « Points » (maximum, pas) et « Cases spéciales » (Banqueroute, Perdu, BONUS)', async () => {
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    const points = screen.getByTestId('wheel-group-Points')
    const special = screen.getByTestId('wheel-group-Cases spéciales')
    expect(within(points).getByLabelText('Points maximum')).toBeTruthy()
    expect(within(points).getByLabelText('Pas')).toBeTruthy()
    expect(within(special).getByLabelText(/Banqueroute/)).toBeTruthy()
    expect(within(special).getByLabelText(/Perdu/)).toBeTruthy()
    expect(within(special).getByLabelText(/BONUS/)).toBeTruthy()
    expect(within(points).queryByLabelText(/BONUS/)).toBeNull()
  })

  it('le texte d\'aide dit que ce sont les réglages par défaut, personnalisables par question', async () => {
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    expect(container.textContent).toMatch(/Réglages par défaut des questions ROUE/)
    expect(container.textContent).toMatch(/chaque question peut en personnaliser un ou plusieurs/)
  })

  it('BONUS global : valeur du serveur affichée ; absent (config v12.0 / LOT-1) = 0', async () => {
    const { rerender } = render(<WheelConfigCard wheel={{ ...DEF, bonus_count: 3 }} wheelDefault={DEF} />)
    await settle()
    expect(screen.getByLabelText(/BONUS/).value).toBe('3')
    const old = { ...DEF }
    delete old.bonus_count
    rerender(<WheelConfigCard key="old" wheel={old} wheelDefault={old} />)
    await settle()
    expect(screen.getByLabelText(/BONUS/).value).toBe('0')
  })

  it('l\'aperçu envoie bonus_count au serveur et montre la case BONUS qu\'il renvoie', async () => {
    const { container } = render(<WheelConfigCard wheel={{ ...DEF, bonus_count: 2 }} wheelDefault={DEF} />)
    await settle()
    expect(previews()[0].body).toMatchObject({ max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 2 })
    expect(container.textContent).toMatch(/BONUS/)
  })

  it('modifier le BONUS relance l\'aperçu avec la nouvelle valeur (une seule requête, anti-rebond)', async () => {
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    calls = []
    const input = screen.getByLabelText(/BONUS/)
    for (const v of ['1', '2', '3']) fireEvent.change(input, { target: { value: v } })
    await settle()
    expect(previews()).toHaveLength(1)
    expect(previews()[0].body.bonus_count).toBe(3)
  })

  it('l\'enregistrement envoie bonus_count dans la section wheel de POST /game-config.json', async () => {
    respond((url) => (url === '/game-config.json' ? json({ wheel: { ...DEF, bonus_count: 2 } }) : json(PREVIEW)))
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} onSaved={vi.fn()} />)
    await settle()
    fireEvent.change(screen.getByLabelText(/BONUS/), { target: { value: '2' } })
    await settle()
    const save = screen.getAllByRole('button').find(b => /Enregistrer/.test(b.textContent))
    expect(save.disabled).toBe(false)
    await act(async () => { fireEvent.click(save); await vi.advanceTimersByTimeAsync(50) })
    const post = calls.find(c => c.url === '/game-config.json' && c.method === 'POST')
    expect(post, 'POST /game-config.json attendu').toBeTruthy()
    expect(post.body.wheel.bonus_count).toBe(2)
  })

  it('« Roue par défaut » remet le BONUS de wheel_default (0)', async () => {
    render(<WheelConfigCard wheel={{ ...DEF, bonus_count: 3 }} wheelDefault={DEF} />)
    await settle()
    const reset = screen.getAllByRole('button').find(b => /Roue par défaut/.test(b.textContent))
    expect(reset.textContent).toMatch(/\(1000 · 50 · 2 · 1 · 0\)/)
    fireEvent.click(reset)
    await settle()
    expect(screen.getByLabelText(/BONUS/).value).toBe('0')
  })

  it('refus du serveur (plus de 64 cases avec les BONUS) : message du serveur affiché, Enregistrer désactivé', async () => {
    respond(() => json({ CODE: 'ROUE_WHEEL_INVALID', MESSAGE: 'la roue aurait plus de 64 cases avec ces cases BONUS' }, false, 400))
    const { container } = render(<WheelConfigCard wheel={{ ...DEF, bonus_count: 4 }} wheelDefault={DEF} />)
    await settle()
    expect(container.textContent).toMatch(/plus de 64 cases/)
    const save = screen.getAllByRole('button').find(b => /Enregistrer/.test(b.textContent))
    expect(save.disabled).toBe(true)
  })

  it('aucune règle de construction côté client : les cases affichées sont celles du serveur', async () => {
    respond(() => json({ COUNT: 3, step: 50, SEGMENTS: [seg(0, 'value', 7), { ...seg(1, 'value', 800), BONUS: true }, seg(2, 'lose_turn')] }))
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    const cells = Array.from(container.querySelectorAll('.wheel-cfg-pc')).map(e => e.textContent)
    expect(cells).toHaveLength(3)
    expect(cells[0]).toMatch(/7/)
    expect(cells[1]).toBe('2🎁') // marque BONUS : logo seul (rang + 🎁), jamais le montant ni le mot
  })
})
