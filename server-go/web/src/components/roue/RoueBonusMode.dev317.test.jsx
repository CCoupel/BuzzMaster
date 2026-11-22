// #316/#317 (v12.2.0) — mode du BONUS : interrupteur « une seule fois par manche » (carte des Paramètres et éditeur de question),
// « 🎲 Autre tirage », information de migration (contrat roue.md §17.5 / §17.7, maquette B). Aucune règle de placement côté client.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react'
import RoueEditor from './RoueEditor'
import WheelConfigCard from './WheelConfigCard'

const ok = (b) => ({ ok: true, status: 200, json: async () => b, text: async () => JSON.stringify(b) })
const GLOBAL = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 2, bonus_mode: 'EPHEMERAL', spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }
const PREVIEW = { COUNT: 22, step: 50, SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 1000, BONUS: false }, { ID: 's1', KIND: 'value', VALUE: 800, BONUS: true }] }
const base = { rouePhrase: '', rouePresentation: '', roueMode: 'TANT_QUE_JE_GAGNE', roueGainsRule: 'WINNER_ONLY', roueWheelLayout: 'strip', roueShowLetters: true,
  roueWheelMaxPoints: null, roueWheelStep: null, roueWheelBankruptCount: null, roueWheelLoseTurnCount: null, roueBonusCount: null, roueBonusMode: null }
const previews = () => global.fetch.mock.calls.filter(c => c[0] === '/api/roue/wheel-preview')

afterEach(() => cleanup())

describe('Carte Paramètres — interrupteur du mode (#317)', () => {
  let saved
  beforeEach(() => {
    vi.useFakeTimers(); saved = null
    global.fetch = vi.fn(async (url, opts) => {
      if (url === '/game-config.json') { saved = JSON.parse(opts.body); return ok({ wheel: saved.wheel }) }
      return ok(PREVIEW)
    })
  })
  afterEach(() => vi.useRealTimers())
  const settle = async () => { await act(async () => { await vi.advanceTimersByTimeAsync(400) }) }

  it('interrupteur coché (éphémère) par défaut ; absent du serveur = éphémère', async () => {
    const { bonus_mode: _m, ...noMode } = GLOBAL
    render(<WheelConfigCard wheel={noMode} wheelDefault={GLOBAL} />)
    await settle()
    expect(screen.getByRole('switch', { name: /une seule fois par manche/ }).checked).toBe(true)
  })

  it('grisé à 0 BONUS ; décocher = PERMANENT, enregistré dans wheel.bonus_mode', async () => {
    render(<WheelConfigCard wheel={{ ...GLOBAL, bonus_count: 0 }} wheelDefault={GLOBAL} />)
    await settle()
    expect(screen.getByRole('switch').disabled).toBe(true)
    cleanup()
    render(<WheelConfigCard wheel={GLOBAL} wheelDefault={GLOBAL} />)
    await settle()
    fireEvent.click(screen.getByRole('switch'))
    await settle()
    await act(async () => { fireEvent.click(screen.getByText('Enregistrer')); await vi.advanceTimersByTimeAsync(10) })
    expect(saved.wheel.bonus_mode).toBe('PERMANENT')
  })

  it('aperçu : la case marquée est dorée avec 🎁 seul, montant au survol', async () => {
    const { container } = render(<WheelConfigCard wheel={GLOBAL} wheelDefault={GLOBAL} />)
    await settle()
    const cell = container.querySelector('.wheel-cfg-pc.bonus')
    expect(cell.textContent).toBe('2🎁')
    expect(cell.getAttribute('title')).toBe('800, BONUS')
  })
})

describe('Éditeur de question — mode du BONUS (#317)', () => {
  beforeEach(() => {
    global.fetch = vi.fn(async (url) => (url === '/game-config.json' ? ok({ wheel: GLOBAL }) : url === '/api/roue/wheel-preview' ? ok(PREVIEW) : ok({})))
  })
  const renderEd = (values = {}) => {
    const onChange = vi.fn()
    return { onChange, ...render(<RoueEditor values={{ ...base, ...values }} onChange={onChange} onValidityChange={() => {}} />) }
  }
  const mode = () => screen.getByTestId('roue-wmode')

  it('suit le global par défaut (lié, interrupteur non modifiable, coché pour éphémère)', async () => {
    renderEd()
    await waitFor(() => expect(mode().querySelector('input').checked).toBe(true))
    expect(mode().textContent).toMatch(/global/)
    expect(mode().querySelector('input').disabled).toBe(true)
  })

  it('✏️ personnalise avec le mode global comme départ ; décocher → PERMANENT ; 🔗 → null', async () => {
    const { onChange } = renderEd()
    await waitFor(() => expect(mode().querySelector('input').disabled).toBe(true))
    fireEvent.click(mode().querySelector('button'))
    expect(onChange).toHaveBeenCalledWith('roueBonusMode', 'EPHEMERAL')
    cleanup()
    const r = renderEd({ roueBonusMode: 'EPHEMERAL' })
    await waitFor(() => expect(mode().querySelector('input').disabled).toBe(false))
    fireEvent.click(mode().querySelector('input'))
    expect(r.onChange).toHaveBeenCalledWith('roueBonusMode', 'PERMANENT')
    fireEvent.click(mode().querySelector('button'))
    expect(r.onChange).toHaveBeenCalledWith('roueBonusMode', null)
  })

  it('grisé quand le nombre de BONUS effectif est 0', async () => {
    renderEd({ roueBonusCount: 0 })
    await waitFor(() => expect(screen.getByTestId('roue-wmode').className).toMatch(/off/))
    expect(mode().querySelector('input').disabled).toBe(true)
  })

  it('« 🎲 Autre tirage » : nouvel appel wheel-preview (le serveur tire, aucun placement côté client)', async () => {
    renderEd()
    await waitFor(() => expect(previews().length).toBe(1), { timeout: 2000 })
    fireEvent.click(await screen.findByTestId('roue-redraw'))
    await waitFor(() => expect(previews().length).toBe(2), { timeout: 2000 })
  })

  it('refus ROUE_BONUS_COUNT_INVALID de l\'aperçu : message affiché, question non enregistrable', async () => {
    global.fetch = vi.fn(async (url) => (url === '/game-config.json' ? ok({ wheel: GLOBAL }) : { ok: false, status: 400, json: async () => ({ CODE: 'ROUE_BONUS_COUNT_INVALID', MESSAGE: 'Trop de BONUS : 1 au plus pour cette roue' }) }))
    const validity = vi.fn()
    render(<RoueEditor values={{ ...base, roueBonusCount: 4 }} onChange={() => {}} onValidityChange={() => {}} onWheelValidityChange={validity} />)
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/Trop de BONUS/), { timeout: 2000 })
    expect(validity).toHaveBeenLastCalledWith(false)
  })

  it('question v12.1 (BONUS sans mode, à l\'ouverture) : information de migration', async () => {
    renderEd({ roueBonusCount: 2 })
    await waitFor(() => expect(screen.getByTestId('roue-bonus-migration').textContent).toMatch(/2 BONUS.*2 cases de moins.*éphémère/), { timeout: 2000 })
  })
})
