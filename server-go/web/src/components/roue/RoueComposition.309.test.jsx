/**
 * #309 (v12.1.0, LOT-2) — éditeur ROUE, section « 🎡 Roue — composition » : chaque paramètre est lié au global (🔗) ou personnalisé (✏️),
 * champ par champ (contracts/roue.md §16.2, §16.4 ; maquette admin_question_editor__roue_wheel_fields.html).
 * `values.<champ>` : nombre = personnalisé ; null / absent = suit le global (champ retiré à l'enregistrement).
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import RoueEditor from './RoueEditor'

afterEach(() => cleanup())

const GLOBAL = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 0 }
const base = { rouePhrase: '', rouePresentation: '', roueMode: 'TANT_QUE_JE_GAGNE', roueGainsRule: 'WINNER_ONLY', roueWheelLayout: 'strip', roueShowLetters: true }
const PREVIEW_OK = { COLS: 14, MAX_ROWS: 4, ROWS: ['COMBIEN POUR'], CONSONANTS: 12, LETTERS: 17 }
const seg = (i, kind, v = 0) => ({ ID: `s${i}`, KIND: kind, VALUE: v })
const WHEEL_OK = { COUNT: 4, step: 50, SEGMENTS: [seg(0, 'value', 1000), seg(1, 'bankrupt'), { ...seg(2, 'value', 800), BONUS: true }, seg(3, 'lose_turn')] }
const json = (data, ok = true, status = 200) => ({ ok, status, json: async () => data })

let calls, globalWheel, wheelHandler
beforeEach(() => {
  calls = []
  globalWheel = GLOBAL
  wheelHandler = () => json(WHEEL_OK)
  global.fetch = vi.fn(async (url, opts) => {
    calls.push({ url, body: opts?.body ? JSON.parse(opts.body) : null })
    if (url === '/api/roue/preview') return json(PREVIEW_OK)
    if (url === '/game-config.json') return json({ wheel: globalWheel })
    if (url === '/api/roue/wheel-preview') return wheelHandler(opts)
    return json({})
  })
})

function renderEditor(values = {}, extra = {}) {
  const onChange = vi.fn()
  const onValidityChange = vi.fn()
  const onWheelValidityChange = vi.fn() // validité de la COMPOSITION (la validité de la phrase est un autre signal)
  const utils = render(<RoueEditor values={{ ...base, ...values }} onChange={onChange} onValidityChange={onValidityChange} onWheelValidityChange={onWheelValidityChange} {...extra} />)
  return { onChange, onValidityChange, onWheelValidityChange, ...utils }
}
const param = (id) => screen.getByTestId(`roue-wparam-${id}`)
const input = (id) => param(id).querySelector('input')
const toggle = (id) => param(id).querySelector('button')
const lastWheelCall = () => [...calls].reverse().find(c => c.url === '/api/roue/wheel-preview')
const loaded = async () => { await waitFor(() => expect(input('max').value).toBe('1000'), { timeout: 2000 }) }

describe('RoueEditor — composition : pré-remplissage avec le global', () => {
  it('critical — question neuve ou v12.0 : les 5 paramètres affichent le global, liés, non modifiables', async () => {
    renderEditor()
    await loaded()
    expect(['max', 'step', 'B', 'P', 'X'].map(id => input(id).value)).toEqual(['1000', '50', '2', '1', '0'])
    for (const id of ['max', 'step', 'B', 'P', 'X']) {
      expect(input(id).disabled, id).toBe(true)
      expect(param(id).textContent, id).toMatch(/global/)
      expect(param(id).textContent, id).not.toMatch(/personnalisé/)
    }
    expect(screen.getByTestId('roue-wheel-status').textContent).toMatch(/Tous les paramètres suivent les Paramètres/)
  })

  it('deux groupes : Points (maximum, pas) et Cases spéciales (Banqueroute, Perdu, BONUS)', async () => {
    const { container } = renderEditor()
    await loaded()
    const groups = Array.from(container.querySelectorAll('.roue-wgroup'))
    expect(groups.map(g => g.querySelector('legend').textContent)).toEqual(['Points', 'Cases spéciales'])
    expect(within(groups[0]).getAllByRole('spinbutton')).toHaveLength(2)
    expect(within(groups[1]).getAllByRole('spinbutton')).toHaveLength(3)
  })

  it('le global suit les Paramètres : une autre valeur globale est affichée telle quelle', async () => {
    globalWheel = { max_points: 500, step: 25, bankrupt_count: 0, lose_turn_count: 3, bonus_count: 2 }
    renderEditor()
    await waitFor(() => expect(input('max').value).toBe('500'), { timeout: 2000 })
    expect(['max', 'step', 'B', 'P', 'X'].map(id => input(id).value)).toEqual(['500', '25', '0', '3', '2'])
  })

  it('global sans bonus_count (config v12.0 / LOT-1) : BONUS global = 0', async () => {
    globalWheel = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1 }
    renderEditor()
    await loaded()
    expect(input('X').value).toBe('0')
  })

  it('une question v12.0 (champs absents) : aucun appel inutile, aucun champ personnalisé', async () => {
    const { onChange } = renderEditor({ roueWheelMaxPoints: undefined })
    await loaded()
    expect(onChange).not.toHaveBeenCalled()
  })
})

describe('RoueEditor — composition : personnaliser et revenir au global, champ par champ', () => {
  it('✏️ délie un seul champ, avec la valeur globale comme point de départ', async () => {
    const { onChange } = renderEditor()
    await loaded()
    fireEvent.click(toggle('B'))
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith('roueWheelBankruptCount', 2)
  })

  it('chaque champ envoie son propre nom de champ', async () => {
    const { onChange } = renderEditor()
    await loaded()
    const expected = { max: ['roueWheelMaxPoints', 1000], step: ['roueWheelStep', 50], B: ['roueWheelBankruptCount', 2], P: ['roueWheelLoseTurnCount', 1], X: ['roueBonusCount', 0] }
    for (const [id, [field, value]] of Object.entries(expected)) {
      onChange.mockClear()
      fireEvent.click(toggle(id))
      expect(onChange, id).toHaveBeenCalledWith(field, value)
    }
  })

  it('un champ personnalisé est éditable et marqué « personnalisé » ; les autres restent liés', async () => {
    renderEditor({ roueWheelBankruptCount: 3 })
    await loaded()
    expect(input('B').disabled).toBe(false)
    expect(input('B').value).toBe('3')
    expect(param('B').textContent).toMatch(/personnalisé/)
    expect(param('B').textContent).toMatch(/global : 2/) // rappel de la valeur globale
    for (const id of ['max', 'step', 'P', 'X']) {
      expect(input(id).disabled, id).toBe(true)
      expect(param(id).textContent, id).not.toMatch(/personnalisé/)
    }
  })

  it('🔗 relie un champ au global : onChange(champ, null) — le champ sera retiré', async () => {
    const { onChange } = renderEditor({ roueWheelBankruptCount: 3, roueBonusCount: 2 })
    await loaded()
    fireEvent.click(toggle('B'))
    expect(onChange).toHaveBeenCalledWith('roueWheelBankruptCount', null)
    expect(onChange).toHaveBeenCalledTimes(1) // le BONUS personnalisé n'est pas touché
  })

  it('saisie : la valeur choisie est envoyée ; 0 est une valeur personnalisée valable', async () => {
    const { onChange } = renderEditor({ roueWheelBankruptCount: 3 })
    await loaded()
    fireEvent.change(input('B'), { target: { value: '0' } })
    expect(onChange).toHaveBeenCalledWith('roueWheelBankruptCount', 0)
    fireEvent.change(input('B'), { target: { value: '5' } })
    expect(onChange).toHaveBeenCalledWith('roueWheelBankruptCount', 5)
  })

  it('0 personnalisé est bien « personnalisé » (≠ global) : affiché lié ✏️ → 🔗, valeur 0 conservée', async () => {
    renderEditor({ roueBonusCount: 0 })
    await loaded()
    expect(param('X').textContent).toMatch(/personnalisé/)
    expect(input('X').value).toBe('0')
  })

  it('une valeur personnalisée égale au global reste personnalisée', async () => {
    renderEditor({ roueWheelMaxPoints: 1000 })
    await loaded()
    expect(param('max').textContent).toMatch(/personnalisé/)
  })

  it('BONUS borné à 4 dans la saisie', async () => {
    const { onChange } = renderEditor({ roueBonusCount: 1 })
    await loaded()
    fireEvent.change(input('X'), { target: { value: '9' } })
    expect(onChange).toHaveBeenCalledWith('roueBonusCount', 4)
  })

  it('les paramètres personnalisés sont signalés en résumé', async () => {
    renderEditor({ roueWheelStep: 100, roueBonusCount: 2 })
    await loaded()
    const status = screen.getByTestId('roue-wheel-status').textContent
    expect(status).toMatch(/Personnalisé pour cette question/)
    expect(status).toMatch(/Pas/)
    expect(status).toMatch(/BONUS/)
    expect(status).not.toMatch(/Banqueroute/)
    expect(status).toMatch(/le reste suit les Paramètres/)
  })

  it('question du LOT-1 (BONUS seul) : seul le BONUS est personnalisé, le reste suit le global', async () => {
    renderEditor({ roueBonusCount: 2 })
    await loaded()
    expect(param('X').textContent).toMatch(/personnalisé/)
    expect(input('X').value).toBe('2')
    for (const id of ['max', 'step', 'B', 'P']) expect(param(id).textContent, id).not.toMatch(/personnalisé/)
  })
})

describe('RoueEditor — composition : aperçu par le serveur', () => {
  it('l\'aperçu reçoit la composition EFFECTIVE (global + personnalisés), jamais calculée côté client', async () => {
    renderEditor({ roueWheelMaxPoints: 500, roueBonusCount: 2 })
    await waitFor(() => expect(lastWheelCall()).toBeTruthy(), { timeout: 2000 })
    expect(lastWheelCall().body).toEqual({ max_points: 500, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 2 })
  })

  it('tout global : l\'aperçu reçoit exactement le global', async () => {
    renderEditor()
    await waitFor(() => expect(lastWheelCall()).toBeTruthy(), { timeout: 2000 })
    expect(lastWheelCall().body).toEqual({ max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 0 })
  })

  it('l\'aperçu affiche les cases renvoyées par le serveur (dont BONUS) et un résumé', async () => {
    renderEditor({ roueBonusCount: 1 })
    await waitFor(() => expect(screen.getByTestId('roue-wheel-summary')).toBeTruthy(), { timeout: 2000 })
    expect(screen.getByTestId('roue-wheel-summary').textContent).toMatch(/4 cases/)
    expect(screen.getByTestId('roue-bonus-preview').textContent).toMatch(/🎁/) // marque BONUS (v12.2.0) : logo seul, jamais le mot
  })

  it('« Pas ajusté » : le pas corrigé par le serveur est signalé, jamais refusé', async () => {
    wheelHandler = () => json({ ...WHEEL_OK, step: 25, step_adjusted_from: 30 })
    renderEditor({ roueWheelMaxPoints: 100, roueWheelStep: 30 })
    await waitFor(() => expect(screen.getByRole('status').textContent).toMatch(/Pas ajusté à 25/), { timeout: 2000 })
    expect(screen.getByRole('status').textContent).toMatch(/30/)
  })

  it('refus du serveur (plus de 64 cases, pas ≥ maximum) : message du serveur, question non enregistrable', async () => {
    wheelHandler = () => json({ CODE: 'ROUE_WHEEL_INVALID', MESSAGE: 'la roue aurait 70 cases, la limite est de 64' }, false, 400)
    const { onWheelValidityChange } = renderEditor({ roueWheelBankruptCount: 70 })
    await loaded()
    fireEvent.click(toggle('P')) // l'utilisateur a touché la composition : refus « en cours de saisie »
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/70 cases/), { timeout: 2000 })
    expect(screen.getByRole('alert').textContent).toMatch(/ne peut pas être enregistrée/)
    await waitFor(() => expect(onWheelValidityChange).toHaveBeenLastCalledWith(false))
  })

  it('composition valable : validité signalée vraie', async () => {
    const { onWheelValidityChange } = renderEditor({ roueBonusCount: 1 })
    await waitFor(() => expect(onWheelValidityChange).toHaveBeenCalledWith(true), { timeout: 2000 })
    expect(onWheelValidityChange).not.toHaveBeenCalledWith(false)
  })

  it('composition devenue invalide à la réouverture (global changé, champ non touché) : signalée, la roue globale servira en jeu', async () => {
    wheelHandler = () => json({ CODE: 'ROUE_WHEEL_INVALID', MESSAGE: 'step doit être strictement inférieur à max_points' }, false, 400)
    const { onWheelValidityChange } = renderEditor({ roueWheelStep: 600 })
    await waitFor(() => expect(onWheelValidityChange).toHaveBeenLastCalledWith(false), { timeout: 2000 })
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/n'est plus valable avec les réglages globaux actuels/), { timeout: 2000 })
    expect(screen.getByRole('alert').textContent).toMatch(/roue globale sera utilisée/)
  })

  it('après une modification de l\'utilisateur, le refus n\'est plus présenté comme « devenue invalide »', async () => {
    wheelHandler = () => json({ CODE: 'ROUE_WHEEL_INVALID', MESSAGE: 'trop de cases' }, false, 400)
    renderEditor({ roueWheelBankruptCount: 70 })
    await loaded()
    fireEvent.click(toggle('P'))
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/Roue refusée/), { timeout: 2000 })
  })

  it('réglages globaux indisponibles : message, champs figés, aucun aperçu faux', async () => {
    global.fetch = vi.fn(async (url) => (url === '/api/roue/preview' ? json(PREVIEW_OK) : json({}, false, 500)))
    renderEditor()
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/indisponibles/), { timeout: 2000 })
    for (const id of ['max', 'step', 'B', 'P', 'X']) expect(toggle(id).disabled, id).toBe(true)
  })
})
