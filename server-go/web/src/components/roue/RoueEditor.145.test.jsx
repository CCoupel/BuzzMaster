/**
 * #145 (v12.0.0, LOT-1.1) — RoueEditor : section d'édition d'une question ROUE (maquette M3, contrat §3, §11).
 * Composant contrôlé : `values` + `onChange(champ, valeur)` ; aperçu par POST /api/roue/preview (serveur).
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import RoueEditor from './RoueEditor'

const base = {
  rouePhrase: '', rouePresentation: '', roueMode: 'TANT_QUE_JE_GAGNE',
  roueGainsRule: 'WINNER_ONLY', roueWheelLayout: 'strip', roueShowLetters: true,
}
const OK_BODY = { COLS: 14, MAX_ROWS: 4, ROWS: ['COMBIEN POUR', 'CE CHIEN DANS', 'LA VITRINE'], CONSONANTS: 12, LETTERS: 17 }

function mockPreview(handler) {
  global.fetch = vi.fn(async (url, opts) => handler(url, opts))
}
const okResponse = (body) => ({ ok: true, status: 200, json: async () => body })
const errResponse = (body) => ({ ok: false, status: 400, json: async () => body })

function renderEditor(values = {}, extra = {}) {
  const onChange = vi.fn()
  const onValidityChange = vi.fn()
  const utils = render(<RoueEditor values={{ ...base, ...values }} onChange={onChange} onValidityChange={onValidityChange} {...extra} />)
  return { ...utils, onChange, onValidityChange }
}

beforeEach(() => { mockPreview(async () => okResponse(OK_BODY)) })
afterEach(() => { vi.restoreAllMocks() })

describe('RoueEditor — structure du formulaire (M3)', () => {
  it('propose phrase, présentation, 3 modes, 2 règles de gains, 2 formes de roue et l\'interrupteur des lettres', () => {
    const { container } = renderEditor()
    expect(screen.getByLabelText(/Phrase à deviner/)).toBeTruthy()
    expect(screen.getByLabelText(/Phrase de présentation/)).toBeTruthy()
    expect(container.querySelectorAll('input[name="roueMode"]')).toHaveLength(3)
    expect(container.querySelectorAll('input[name="roueGainsRule"]')).toHaveLength(2)
    expect(container.querySelectorAll('input[name="roueWheelLayout"]')).toHaveLength(2)
    expect(screen.getByRole('switch', { name: /lettres déjà proposées/i })).toBeTruthy()
  })

  it('les modes sont SOLO / CHACUN SON TOUR / TANT QUE JE GAGNE — sans « Maillon faible »', () => {
    const { container } = renderEditor()
    const values = Array.from(container.querySelectorAll('input[name="roueMode"]')).map(i => i.value)
    expect(values).toEqual(['SOLO', 'CHACUN_SON_TOUR', 'TANT_QUE_JE_GAGNE'])
    expect(container.textContent).not.toMatch(/maillon/i)
  })

  it('reflète les valeurs par défaut : TANT QUE JE GAGNE, gagnant seul, bandeau, lettres affichées', () => {
    const { container } = renderEditor()
    expect(container.querySelector('input[name="roueMode"][value="TANT_QUE_JE_GAGNE"]').checked).toBe(true)
    expect(container.querySelector('input[name="roueGainsRule"][value="WINNER_ONLY"]').checked).toBe(true)
    expect(container.querySelector('input[name="roueWheelLayout"][value="strip"]').checked).toBe(true)
    expect(screen.getByRole('switch', { name: /lettres déjà proposées/i }).getAttribute('aria-checked')).toBe('true')
  })

  it('ne propose ni Réponse, ni Points, ni Temps (type sans ces champs)', () => {
    renderEditor()
    expect(screen.queryByLabelText(/^Réponse/)).toBeNull()
    expect(screen.queryByLabelText(/^Points/)).toBeNull()
    expect(screen.queryByLabelText(/^Temps/)).toBeNull()
  })
})

describe('RoueEditor — champs contrôlés', () => {
  it('saisir la phrase appelle onChange(rouePhrase, …)', () => {
    const { onChange } = renderEditor()
    fireEvent.change(screen.getByLabelText(/Phrase à deviner/), { target: { value: 'Bonjour' } })
    expect(onChange).toHaveBeenCalledWith('rouePhrase', 'Bonjour')
  })

  it.each([
    ['roueMode', 'SOLO'], ['roueMode', 'CHACUN_SON_TOUR'], ['roueGainsRule', 'ALL_TEAMS'], ['roueWheelLayout', 'arc'],
  ])('choisir %s = %s appelle onChange', (name, value) => {
    const { container, onChange } = renderEditor()
    fireEvent.click(container.querySelector(`input[name="${name}"][value="${value}"]`))
    expect(onChange).toHaveBeenCalledWith(name, value)
  })

  it('l\'interrupteur des lettres inverse la valeur', () => {
    const { onChange } = renderEditor({ roueShowLetters: true })
    fireEvent.click(screen.getByRole('switch', { name: /lettres déjà proposées/i }))
    expect(onChange).toHaveBeenCalledWith('roueShowLetters', false)
  })

  it('interrupteur désactivé : aria-checked=false', () => {
    renderEditor({ roueShowLetters: false })
    expect(screen.getByRole('switch', { name: /lettres déjà proposées/i }).getAttribute('aria-checked')).toBe('false')
  })

  it('présentation limitée à 80 caractères avec compteur', () => {
    renderEditor({ rouePresentation: 'Chanson de Line Renaud' })
    const input = screen.getByLabelText(/Phrase de présentation/)
    expect(input.getAttribute('maxlength')).toBe('80')
    expect(document.body.textContent).toMatch(/22 \/ 80/)
  })
})

describe('RoueEditor — aperçu serveur (POST /api/roue/preview)', () => {
  it('phrase vide : aucun appel serveur, saisie non enregistrable', async () => {
    const { onValidityChange } = renderEditor({ rouePhrase: '' })
    await new Promise(r => setTimeout(r, 400))
    expect(global.fetch).not.toHaveBeenCalled()
    expect(onValidityChange).toHaveBeenLastCalledWith(false)
  })

  it('phrase valide : appelle l\'aperçu avec {PHRASE}, affiche le décompte et signale « enregistrable »', async () => {
    const { onValidityChange, container } = renderEditor({ rouePhrase: 'Combien pour ce chien dans la vitrine' })
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true), { timeout: 2000 })
    const [url, opts] = global.fetch.mock.calls[0]
    expect(url).toBe('/api/roue/preview')
    expect(opts.method).toBe('POST')
    expect(JSON.parse(opts.body)).toEqual({ PHRASE: 'Combien pour ce chien dans la vitrine' })
    expect(document.body.textContent).toMatch(/3 lignes sur 4/)
    expect(document.body.textContent).toMatch(/12 consonnes différentes/)
    expect(document.body.textContent).toMatch(/17 lettres différentes/)
    // le plateau affiche les lettres du serveur
    expect(container.querySelectorAll('.roue-cell').length).toBe(56)
    expect(Array.from(container.querySelectorAll('.roue-cell')).filter(e => e.textContent.trim()).length).toBe(31)
  })

  it('phrase refusée : affiche le message serveur, non enregistrable, aucun plateau', async () => {
    mockPreview(async () => errResponse({ CODE: 'ROUE_PHRASE_TOO_LONG', MESSAGE: 'Phrase trop longue : 5 lignes, 4 au plus' }))
    const { onValidityChange, container } = renderEditor({ rouePhrase: 'x'.repeat(5) })
    await waitFor(() => expect(screen.getByRole('alert').textContent).toMatch(/Phrase trop longue/), { timeout: 2000 })
    expect(onValidityChange).toHaveBeenLastCalledWith(false)
    expect(Array.from(container.querySelectorAll('.roue-cell')).filter(e => e.textContent.trim()).length).toBe(0)
  })

  it.each(['ROUE_PHRASE_EMPTY', 'ROUE_PHRASE_INVALID_CHAR', 'ROUE_PHRASE_WORD_TOO_LONG', 'ROUE_PHRASE_TOO_LONG'])(
    'refus %s sans MESSAGE : un texte lisible, jamais le code seul', async (code) => {
      mockPreview(async () => errResponse({ CODE: code, MESSAGE: '' }))
      renderEditor({ rouePhrase: 'abc' })
      const alert = await screen.findByRole('alert', {}, { timeout: 2000 })
      expect(alert.textContent).not.toBe(`✗ ${code}`)
      expect(alert.textContent.length).toBeGreaterThan(10)
    })

  it('serveur injoignable : message d\'indisponibilité, non enregistrable', async () => {
    mockPreview(async () => { throw new TypeError('network') })
    const { onValidityChange } = renderEditor({ rouePhrase: 'abc' })
    await waitFor(() => expect(document.body.textContent).toMatch(/indisponible/i), { timeout: 2000 })
    expect(onValidityChange).toHaveBeenLastCalledWith(false)
  })

  it('anti-rebond : plusieurs frappes rapides ne produisent pas un appel par frappe', async () => {
    const onChange = vi.fn(); const onValidityChange = vi.fn()
    const { rerender } = render(<RoueEditor values={{ ...base, rouePhrase: 'a' }} onChange={onChange} onValidityChange={onValidityChange} />)
    for (const p of ['ab', 'abc', 'abcd']) {
      rerender(<RoueEditor values={{ ...base, rouePhrase: p }} onChange={onChange} onValidityChange={onValidityChange} />)
    }
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true), { timeout: 2000 })
    expect(global.fetch.mock.calls.length).toBe(1)
    expect(JSON.parse(global.fetch.mock.calls[0][1].body).PHRASE).toBe('abcd')
  })
})
