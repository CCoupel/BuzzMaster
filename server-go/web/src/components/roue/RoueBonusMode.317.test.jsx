/**
 * #317 (v12.2.0, LOT-1) — mode du BONUS (éphémère / permanent) : éditeur de question et carte « Roue de la Fortune » des
 * Paramètres (contracts/roue.md §17.5 / §17.6 ; maquette admin_question_editor__roue_bonus_mode, version B).
 *
 * Décision utilisateur : le mode est une OPTION DU COMPTEUR BONUS — un interrupteur « une seule fois par manche » sous le
 * compteur (🔗 global / ✏️ personnalisé dans l'éditeur), grisé à 0 BONUS. Éphémère = interrupteur activé (défaut).
 * Aucune copie JS de la règle de placement : l'aperçu n'affiche que ce que renvoie le serveur (R5).
 * Fichier NEUF ; les tests v12.1 (RoueComposition.309, WheelConfigCard.309) appartiennent à dev-frontend.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen, waitFor, within, act } from '@testing-library/react'
import RoueEditor from './RoueEditor'
import WheelConfigCard from './WheelConfigCard'

afterEach(() => cleanup())

const json = (data, ok = true, status = 200) => ({ ok, status, json: async () => data, text: async () => JSON.stringify(data) })
const seg = (i, kind, v = 0, bonus = false) => ({ ID: `s${i}`, KIND: kind, VALUE: v, BONUS: bonus })
const wheelWith = (marks) => ({ COUNT: 6, step: 100, SEGMENTS: [seg(0, 'value', 1000), seg(1, 'bankrupt'), seg(2, 'value', 800, marks.includes(2)), seg(3, 'value', 100), seg(4, 'lose_turn'), seg(5, 'value', 500, marks.includes(5))] })

// ======================================================================================================
// Éditeur de question
// ======================================================================================================

const base = { rouePhrase: '', rouePresentation: '', roueMode: 'TANT_QUE_JE_GAGNE', roueGainsRule: 'WINNER_ONLY', roueWheelLayout: 'strip', roueShowLetters: true }
const PREVIEW_OK = { COLS: 14, MAX_ROWS: 4, ROWS: ['COMBIEN POUR'], CONSONANTS: 12, LETTERS: 17 }

let calls, globalWheel, wheelHandler
beforeEach(() => {
  calls = []
  globalWheel = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 2, bonus_mode: 'EPHEMERAL' }
  wheelHandler = () => json(wheelWith([2]))
  global.fetch = vi.fn(async (url, opts) => {
    calls.push({ url, method: opts?.method, body: opts?.body ? JSON.parse(opts.body) : null })
    if (url === '/api/roue/preview') return json(PREVIEW_OK)
    if (url === '/game-config.json' && (!opts || !opts.method || opts.method === 'GET')) return json({ wheel: globalWheel })
    if (url === '/game-config.json') return json({ wheel: globalWheel })
    if (url === '/api/roue/wheel-preview') return wheelHandler(opts)
    return json({})
  })
})

function renderEditor(values = {}) {
  const onChange = vi.fn()
  const onValidityChange = vi.fn()
  const onWheelValidityChange = vi.fn()
  const utils = render(<RoueEditor values={{ ...base, ...values }} onChange={onChange} onValidityChange={onValidityChange} onWheelValidityChange={onWheelValidityChange} />)
  return { onChange, onWheelValidityChange, ...utils }
}
const param = (id) => screen.getByTestId(`roue-wparam-${id}`)
const loaded = async () => { await waitFor(() => expect(param('max').querySelector('input').value).toBe('1000'), { timeout: 2000 }) }
const modeSwitch = () => within(param('X')).getByRole('switch', { name: /une seule fois par manche/i })
const modeButton = () => within(param('X')).getByRole('button', { name: /mode du BONUS/i })
const wheelCalls = () => calls.filter((c) => c.url === '/api/roue/wheel-preview')

describe('Éditeur — interrupteur du mode, sous le compteur BONUS', () => {
  it('critical — l\'interrupteur « une seule fois par manche » est DANS le paramètre BONUS (option du compteur, pas un 6ᵉ paramètre)', async () => {
    renderEditor()
    await loaded()
    expect(modeSwitch()).toBeTruthy()
    expect(modeButton()).toBeTruthy()
    // Les 5 paramètres de composition restent 5 : Points (2) + Cases spéciales (3).
    const section = screen.getByTestId('roue-wheel-section')
    expect(within(section).getAllByRole('spinbutton')).toHaveLength(5)
    expect(within(section).getAllByRole('switch')).toHaveLength(1)
  })

  it('suit le global par défaut : lié (🔗, interrupteur non modifiable), coché pour un global éphémère', async () => {
    renderEditor()
    await loaded()
    expect(modeSwitch().checked).toBe(true) // éphémère = une seule fois par manche
    expect(modeSwitch().disabled).toBe(true)
    expect(param('X').textContent).toMatch(/🔗/)
    expect(modeButton().textContent).toBe('✏️')
  })

  it('global permanent : interrupteur décoché (la marque reste)', async () => {
    globalWheel = { ...globalWheel, bonus_mode: 'PERMANENT' }
    renderEditor()
    await loaded()
    expect(modeSwitch().checked).toBe(false)
  })

  it('global sans bonus_mode (config v12.1) : éphémère', async () => {
    const { bonus_mode, ...old } = globalWheel
    globalWheel = old
    renderEditor()
    await loaded()
    expect(modeSwitch().checked).toBe(true)
  })

  it('✏️ personnalise le mode avec la valeur globale comme point de départ (onChange roueBonusMode)', async () => {
    const { onChange } = renderEditor()
    await loaded()
    fireEvent.click(modeButton())
    expect(onChange).toHaveBeenCalledWith('roueBonusMode', 'EPHEMERAL')
    onChange.mockClear()
    cleanup()
    globalWheel = { ...globalWheel, bonus_mode: 'PERMANENT' }
    const again = renderEditor()
    await loaded()
    fireEvent.click(modeButton())
    expect(again.onChange).toHaveBeenCalledWith('roueBonusMode', 'PERMANENT')
  })

  it('mode personnalisé : interrupteur modifiable, il envoie EPHEMERAL (activé) ou PERMANENT (désactivé)', async () => {
    const { onChange } = renderEditor({ roueBonusMode: 'PERMANENT' })
    await loaded()
    expect(modeSwitch().checked).toBe(false) // PERMANENT personnalisé, global éphémère
    expect(modeSwitch().disabled).toBe(false)
    expect(param('X').textContent).toMatch(/personnalisé/)
    expect(modeButton().textContent).toBe('🔗') // retour au global
    fireEvent.click(modeSwitch())
    expect(onChange).toHaveBeenCalledWith('roueBonusMode', 'EPHEMERAL')
  })

  it('🔗 revient au global : le champ est retiré (roueBonusMode = null)', async () => {
    const { onChange } = renderEditor({ roueBonusMode: 'PERMANENT' })
    await loaded()
    fireEvent.click(modeButton())
    expect(onChange).toHaveBeenCalledWith('roueBonusMode', null)
  })

  it('le mode personnalisé n\'écrase ni ne déplace les autres paramètres (chaque 🔗 / ✏️ reste indépendant)', async () => {
    renderEditor({ roueBonusMode: 'EPHEMERAL' })
    await loaded()
    for (const id of ['max', 'step', 'B', 'P']) {
      expect(param(id).textContent, id).toMatch(/global/)
      expect(param(id).textContent, id).not.toMatch(/personnalisé/)
    }
  })

  it('grisé à 0 BONUS (effectif) : interrupteur et ✏️ désactivés, même mode personnalisé', async () => {
    globalWheel = { ...globalWheel, bonus_count: 0 }
    renderEditor()
    await loaded()
    expect(modeSwitch().disabled).toBe(true)
    expect(modeButton().disabled).toBe(true)
    cleanup()
    renderEditor({ roueBonusMode: 'PERMANENT' })
    await loaded()
    expect(modeSwitch().disabled).toBe(true)
    expect(modeButton().disabled).toBe(true)
  })

  it('un BONUS personnalisé > 0 active le mode même quand le global est à 0', async () => {
    globalWheel = { ...globalWheel, bonus_count: 0 }
    renderEditor({ roueBonusCount: 2 })
    await loaded()
    expect(modeButton().disabled).toBe(false)
  })

  it('un BONUS personnalisé à 0 le grise même quand le global est > 0', async () => {
    renderEditor({ roueBonusCount: 0 })
    await loaded()
    expect(modeSwitch().disabled).toBe(true)
    expect(modeButton().disabled).toBe(true)
  })
})

describe('Éditeur — aperçu : un tirage du serveur, jamais calculé par le navigateur', () => {
  it('l\'aperçu affiche exactement les marques renvoyées par wheel-preview (BONUS par case)', async () => {
    wheelHandler = () => json(wheelWith([5]))
    const { container } = renderEditor()
    await loaded()
    await waitFor(() => expect(container.querySelector('[data-testid="roue-bonus-preview"] .wheel-cell.marked')).toBeTruthy(), { timeout: 2000 })
    const marked = Array.from(container.querySelectorAll('[data-testid="roue-bonus-preview"] .wheel-cell.marked'))
    // Un tirage : la case marquée est la 500 (index 5), pas la 800.
    expect(marked.every((c) => c.textContent.includes('500'))).toBe(true)
    expect(marked.some((c) => c.textContent.includes('800'))).toBe(false)
  })

  it('la requête d\'aperçu porte la composition effective (bonus_count compris), sans règle de placement locale', async () => {
    renderEditor({ roueBonusCount: 3 })
    await loaded()
    await waitFor(() => expect(wheelCalls().length).toBeGreaterThan(0), { timeout: 2000 })
    expect(wheelCalls().at(-1).body).toMatchObject({ max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 3 })
  })

  it('« 🎲 Autre tirage » refait un aperçu et montre les marques ailleurs ; un tirage ne modifie pas le formulaire', async () => {
    let n = 0
    wheelHandler = () => json(wheelWith(n++ === 0 ? [2] : [5]))
    const { container, onChange } = renderEditor()
    await loaded()
    await waitFor(() => expect(container.querySelector('[data-testid="roue-bonus-preview"] .wheel-cell.marked')).toBeTruthy(), { timeout: 2000 })
    const before = wheelCalls().length
    const lab = () => Array.from(container.querySelectorAll('[data-testid="roue-bonus-preview"] .wheel-cell.marked')).map((c) => c.textContent).join('|')
    expect(lab()).toMatch(/800/)
    onChange.mockClear()
    fireEvent.click(screen.getByRole('button', { name: /Autre tirage/i }))
    await waitFor(() => expect(wheelCalls().length).toBeGreaterThan(before), { timeout: 2000 })
    await waitFor(() => expect(lab()).toMatch(/500/), { timeout: 2000 })
    expect(onChange).not.toHaveBeenCalled()
  })

  it('refus du serveur ROUE_BONUS_COUNT_INVALID : message affiché (role alert), composition invalide signalée', async () => {
    wheelHandler = () => json({ CODE: 'ROUE_BONUS_COUNT_INVALID', MESSAGE: 'Trop de BONUS : 2 au plus pour cette roue (un BONUS ne touche jamais une Banqueroute, un Perdu ni un autre BONUS).' }, false, 400)
    const { onWheelValidityChange } = renderEditor({ roueBonusCount: 4 })
    await loaded()
    const alert = await screen.findByRole('alert', {}, { timeout: 2000 })
    expect(alert.textContent).toMatch(/BONUS/)
    await waitFor(() => expect(onWheelValidityChange).toHaveBeenLastCalledWith(false), { timeout: 2000 })
  })

  it('question v12.1 (ROUE_BONUS_COUNT sans mode) : information de migration, mode suivant le global', async () => {
    renderEditor({ roueBonusCount: 2 })
    await loaded()
    expect(screen.getByTestId('roue-bonus-migration').textContent).toMatch(/BONUS.*cases à points/)
    expect(modeSwitch().checked).toBe(true)
  })

  it('question neuve ou sans BONUS : aucune information de migration', async () => {
    renderEditor()
    await loaded()
    expect(screen.queryByTestId('roue-bonus-migration')).toBeNull()
    cleanup()
    renderEditor({ roueBonusCount: 0 })
    await loaded()
    expect(screen.queryByTestId('roue-bonus-migration')).toBeNull()
  })
})

// ======================================================================================================
// Carte « Roue de la Fortune » des Paramètres
// ======================================================================================================

const DEF = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 2, bonus_mode: 'EPHEMERAL', spin_period_ms: 2500, spin_max_ms: 10000, stop_duration_ms: 4000, flip_interval_ms: 600 }

describe('Carte Paramètres — interrupteur du mode sous le compteur BONUS', () => {
  let cardCalls
  const respond = (fn) => { global.fetch = vi.fn(async (url, opts) => { cardCalls.push({ url, method: opts?.method, body: opts?.body ? JSON.parse(opts.body) : null }); return fn(url, opts) }) }
  const settle = async () => { await act(async () => { await vi.advanceTimersByTimeAsync(400) }) }
  const cardSwitch = () => screen.getByRole('switch', { name: /une seule fois par manche/i })
  const saveBtn = () => screen.getAllByRole('button').find((b) => /Enregistrer/.test(b.textContent))

  beforeEach(() => { vi.useFakeTimers(); cardCalls = []; respond((url) => json(url === '/game-config.json' ? { wheel: DEF } : wheelWith([2]))) })
  afterEach(() => { cleanup(); vi.useRealTimers() })

  it('critical — éphémère par défaut : interrupteur activé ; absent de la config (v12.1) = éphémère aussi', async () => {
    const { rerender } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    expect(cardSwitch().checked).toBe(true)
    const old = { ...DEF }
    delete old.bonus_mode
    rerender(<WheelConfigCard key="old" wheel={old} wheelDefault={old} />)
    await settle()
    expect(cardSwitch().checked).toBe(true)
  })

  it('global permanent : interrupteur désactivé (décoché)', async () => {
    render(<WheelConfigCard wheel={{ ...DEF, bonus_mode: 'PERMANENT' }} wheelDefault={DEF} />)
    await settle()
    expect(cardSwitch().checked).toBe(false)
  })

  it('l\'interrupteur est sous le compteur BONUS (même paramètre) ; grisé à 0 BONUS', async () => {
    const { rerender } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} />)
    await settle()
    const counter = screen.getByLabelText(/BONUS/, { selector: 'input[type="number"]' })
    expect(counter.closest('.wheel-cfg-param').contains(cardSwitch())).toBe(true)
    expect(cardSwitch().disabled).toBe(false)
    rerender(<WheelConfigCard key="zero" wheel={{ ...DEF, bonus_count: 0 }} wheelDefault={DEF} />)
    await settle()
    expect(cardSwitch().disabled).toBe(true)
  })

  it('basculer puis Enregistrer : POST /game-config.json porte bonus_mode PERMANENT', async () => {
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} onSaved={vi.fn()} />)
    await settle()
    fireEvent.click(cardSwitch())
    await settle()
    expect(cardSwitch().checked).toBe(false)
    await act(async () => { fireEvent.click(saveBtn()); await vi.advanceTimersByTimeAsync(50) })
    const post = cardCalls.find((c) => c.url === '/game-config.json' && c.method === 'POST')
    expect(post).toBeTruthy()
    const wheel = post.body.wheel ?? post.body
    expect(wheel.bonus_mode).toBe('PERMANENT')
    expect(wheel.bonus_count).toBe(2)
  })

  it('un enregistrement sans toucher à l\'interrupteur n\'envoie jamais un mode inconnu (éphémère)', async () => {
    render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} onSaved={vi.fn()} />)
    await settle()
    await act(async () => { fireEvent.click(saveBtn()); await vi.advanceTimersByTimeAsync(50) })
    const post = cardCalls.find((c) => c.url === '/game-config.json' && c.method === 'POST')
    expect(post).toBeTruthy()
    const wheel = post.body.wheel ?? post.body
    expect(['EPHEMERAL', undefined]).toContain(wheel.bonus_mode)
  })

  it('aperçu : les marques sont celles du serveur (🎁 seul) ; refus ROUE_BONUS_COUNT_INVALID : message affiché, Enregistrer désactivé', async () => {
    const { container } = render(<WheelConfigCard wheel={DEF} wheelDefault={DEF} onSaved={vi.fn()} />)
    await settle()
    expect(container.textContent).toMatch(/🎁/)
    respond((url) => (url === '/api/roue/wheel-preview'
      ? json({ CODE: 'ROUE_BONUS_COUNT_INVALID', MESSAGE: 'Trop de BONUS : 2 au plus pour cette roue (un BONUS ne touche jamais une Banqueroute, un Perdu ni un autre BONUS).' }, false, 400)
      : json({ wheel: DEF })))
    fireEvent.change(screen.getByLabelText(/BONUS/, { selector: 'input[type="number"]' }), { target: { value: '4' } })
    await settle()
    expect(container.textContent).toMatch(/Trop de BONUS/)
    expect(saveBtn().disabled).toBe(true)
  })
})
