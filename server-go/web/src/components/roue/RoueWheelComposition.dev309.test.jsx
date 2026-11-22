// #309 (v12.1.0) — éditeur ROUE : composition de la roue par question, « champ lié au global, à délier » (contrat roue.md §16, maquette B).
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import RoueEditor from './RoueEditor'

const GLOBAL = { max_points: 1000, step: 50, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 0 }
const base = { rouePhrase: '', rouePresentation: '', roueMode: 'TANT_QUE_JE_GAGNE', roueGainsRule: 'WINNER_ONLY', roueWheelLayout: 'strip', roueShowLetters: true,
  roueWheelMaxPoints: null, roueWheelStep: null, roueWheelBankruptCount: null, roueWheelLoseTurnCount: null, roueBonusCount: null }
const ok = (b) => ({ ok: true, status: 200, json: async () => b })
let previewBody = { COUNT: 23, step: 50, SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 1000 }, { ID: 's1', KIND: 'value', VALUE: 800, BONUS: true }] }
let previewReply
const previews = () => global.fetch.mock.calls.filter(c => c[0] === '/api/roue/wheel-preview').map(c => JSON.parse(c[1].body))

beforeEach(() => {
  previewReply = () => ok(previewBody)
  global.fetch = vi.fn(async (url) => (url === '/game-config.json' ? ok({ wheel: GLOBAL }) : url === '/api/roue/wheel-preview' ? previewReply() : ok({})))
})
afterEach(() => { vi.restoreAllMocks() })

const renderEd = (values = {}) => {
  const onChange = vi.fn(); const onWheelValidityChange = vi.fn()
  return { ...render(<RoueEditor values={{ ...base, ...values }} onChange={onChange} onValidityChange={() => {}} onWheelValidityChange={onWheelValidityChange} />), onChange, onWheelValidityChange }
}

describe('RoueEditor — composition de la roue (#309)', () => {
  it('5 paramètres pré-remplis avec le global, liés (désactivés), deux groupes', async () => {
    const { container } = renderEd()
    await waitFor(() => expect(container.querySelector('#roue-w-max').value).toBe('1000'))
    expect(container.querySelector('#roue-w-step').value).toBe('50')
    expect(container.querySelector('#roue-w-B').value).toBe('2')
    expect(container.querySelector('#roue-w-P').value).toBe('1')
    expect(container.querySelector('#roue-w-X').value).toBe('0')
    expect(container.querySelector('#roue-w-max').disabled).toBe(true)
    expect(screen.getByText('Points')).toBeTruthy()
    expect(screen.getByText('Cases spéciales')).toBeTruthy()
    expect(screen.getByTestId('roue-wheel-status').textContent).toMatch(/suivent les Paramètres/)
  })

  it('✏️ délie avec la valeur globale comme départ ; 🔗 relie (null = champ retiré)', async () => {
    const { container, onChange } = renderEd()
    await waitFor(() => expect(container.querySelector('#roue-w-step').value).toBe('50'))
    fireEvent.click(screen.getByRole('button', { name: /Personnaliser : Pas/ }))
    expect(onChange).toHaveBeenCalledWith('roueWheelStep', 50)
    onChange.mockClear()
    renderEd({ roueWheelStep: 25 })
    fireEvent.click(await screen.findByRole('button', { name: /Revenir au global : Pas/ }))
    expect(onChange).not.toHaveBeenCalled() // l'autre rendu a son propre onChange
  })

  it('paramètre personnalisé : champ actif, badge « personnalisé », 0 est une valeur valable', async () => {
    const { container, onChange } = renderEd({ roueWheelBankruptCount: 0 })
    await waitFor(() => expect(container.querySelector('#roue-w-B')).toBeTruthy())
    expect(container.querySelector('#roue-w-B').disabled).toBe(false)
    expect(container.querySelector('#roue-w-B').value).toBe('0')
    expect(screen.getByTestId('roue-wparam-B').textContent).toMatch(/personnalisé/)
    fireEvent.click(screen.getByRole('button', { name: /Revenir au global : .*Banqueroute/ }))
    expect(onChange).toHaveBeenCalledWith('roueWheelBankruptCount', null)
    fireEvent.change(container.querySelector('#roue-w-B'), { target: { value: '3' } })
    expect(onChange).toHaveBeenCalledWith('roueWheelBankruptCount', 3)
  })

  it('BONUS borné à 0–4', async () => {
    const { container, onChange } = renderEd({ roueBonusCount: 1 })
    await waitFor(() => expect(container.querySelector('#roue-w-X')).toBeTruthy())
    fireEvent.change(container.querySelector('#roue-w-X'), { target: { value: '9' } })
    expect(onChange).toHaveBeenCalledWith('roueBonusCount', 4)
  })

  it('l\'aperçu reçoit la composition effective (question + global)', async () => {
    renderEd({ roueWheelStep: 25, roueBonusCount: 3 })
    await waitFor(() => expect(previews()).toHaveLength(1), { timeout: 2000 })
    expect(previews()[0]).toEqual({ max_points: 1000, step: 25, bankrupt_count: 2, lose_turn_count: 1, bonus_count: 3 })
  })

  it('« Pas ajusté » quand le serveur corrige le pas', async () => {
    previewBody = { COUNT: 23, step: 25, step_adjusted_from: 30, SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 1000 }] }
    renderEd({ roueWheelStep: 30 })
    await waitFor(() => expect(screen.getByText(/Pas ajusté à/)).toBeTruthy(), { timeout: 2000 })
  })

  it('refus (plus de 64 cases) : message serveur, composition non valide signalée', async () => {
    previewReply = () => ({ ok: false, status: 400, json: async () => ({ CODE: 'ROUE_WHEEL_INVALID', MESSAGE: 'la roue aurait 70 cases, la limite est de 64' }) })
    const { onWheelValidityChange } = renderEd({ roueWheelStep: 10 })
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/70 cases/), { timeout: 2000 })
    expect(onWheelValidityChange).toHaveBeenLastCalledWith(false)
  })

  it('à la réouverture, une composition devenue invalide est signalée « plus valable »', async () => {
    previewReply = () => ({ ok: false, status: 400, json: async () => ({ CODE: 'ROUE_WHEEL_INVALID', MESSAGE: 'le pas (600) doit être inférieur aux points maximum (500)' }) })
    renderEd({ roueWheelStep: 600 })
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/n'est plus valable/), { timeout: 2000 })
  })
})

describe('roueRefusalText — refus de composition à l\'enregistrement (#309)', () => {
  it('ROUE_WHEEL_INVALID : message précis du serveur conservé', async () => {
    const { roueRefusalText } = await import('./RoueEditor')
    expect(roueRefusalText('ROUE_WHEEL_INVALID: la roue aurait 70 cases, la limite est de 64')).toBe('la roue aurait 70 cases, la limite est de 64')
    expect(roueRefusalText('ROUE_BONUS_COUNT_INVALID:')).toMatch(/BONUS/)
  })
})

describe('aide sous « Forme de la roue » (#309)', () => {
  it('renvoie à « Roue — composition », plus aux seuls Paramètres', () => {
    const { container } = renderEd()
    expect(container.textContent).toContain('se règlent plus bas, dans « Roue — composition »')
    expect(container.textContent).not.toContain('se règlent dans les Paramètres.')
  })
})
