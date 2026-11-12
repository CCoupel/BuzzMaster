import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import ColorWheel, { hsvToHex, hexToHsv, readRecents } from './ColorWheel'

// #210 (v11.8.0, LOT-2, F2.2) — roue chromatique maison (aucune dépendance) : roue teinte / saturation, luminosité, code #RRGGBB,
// couleurs récentes par navigateur. Source : maquette effect_editor §01 « Pinceau », plan F2.2.

beforeEach(() => { try { localStorage.clear() } catch { /* ignoré */ } })

describe('conversions', () => {
  it.each([['#FF0000', 0], ['#00FF00', 120], ['#0000FF', 240]])('hsv ⇄ hex : %s', (hex, h) => {
    expect(hexToHsv(hex).h).toBeCloseTo(h, 0)
    expect(hsvToHex(h, 1, 1)).toBe(hex)
  })
  it('aller-retour sur quelques couleurs', () => {
    for (const hex of ['#14B8A6', '#4F46E5', '#E53935', '#FFFFFF', '#000000']) {
      const { h, s, v } = hexToHsv(hex)
      expect(hsvToHex(h, s, v)).toBe(hex)
    }
  })
})

describe('composant', () => {
  const wheelRect = { left: 0, top: 0, width: 100, height: 100, right: 100, bottom: 100 }
  const click = (x, y) => {
    const w = screen.getByRole('img', { name: /Roue de couleur/ })
    w.getBoundingClientRect = () => wheelRect
    fireEvent.click(w, { clientX: x, clientY: y })
  }

  it('affiche la couleur reçue dans le champ code', () => {
    render(<ColorWheel value="#14B8A6" onChange={vi.fn()} />)
    expect(screen.getByLabelText('Code de la couleur (#RRGGBB)')).toHaveValue('#14B8A6')
  })

  it('clic sur la roue : une couleur « posée » (commit) avec la teinte visée', () => {
    const onChange = vi.fn()
    render(<ColorWheel value="#FF0000" onChange={onChange} />)
    click(100, 50) // à droite du centre, au bord : teinte 0° (rouge), saturation maximale
    const [hex, meta] = onChange.mock.calls[0]
    expect(hex).toBe('#FF0000')
    expect(meta).toEqual({ commit: true })
    click(50, 100) // en bas : teinte 90°
    expect(onChange.mock.calls[1][0]).toMatch(/^#[0-9A-F]{6}$/)
    expect(onChange.mock.calls[1][0]).not.toBe('#FF0000')
  })

  it('curseur de luminosité : change la couleur sans la « poser » (pas de commit)', () => {
    const onChange = vi.fn()
    render(<ColorWheel value="#FF0000" onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('Luminosité de la couleur', { selector: 'input' }), { target: { value: '50' } })
    expect(onChange).toHaveBeenCalledWith('#800000', { commit: false })
    expect(readRecents()).toEqual([])
  })

  it('code saisi : accepté s\'il est valide (avec ou sans #), ignoré sinon', () => {
    const onChange = vi.fn()
    render(<ColorWheel value="#FF0000" onChange={onChange} />)
    const input = screen.getByLabelText('Code de la couleur (#RRGGBB)')
    fireEvent.change(input, { target: { value: '00aaff' } })
    fireEvent.blur(input)
    expect(onChange).toHaveBeenCalledWith('#00AAFF', { commit: true })
    fireEvent.change(input, { target: { value: '#zz' } })
    fireEvent.blur(input)
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(input).toHaveValue('#FF0000')
  })

  it('Entrée valide la saisie', () => {
    const onChange = vi.fn()
    render(<ColorWheel value="#FF0000" onChange={onChange} />)
    const input = screen.getByLabelText('Code de la couleur (#RRGGBB)')
    fireEvent.change(input, { target: { value: '#112233' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onChange).toHaveBeenCalledWith('#112233', { commit: true })
  })

  it('couleurs récentes : mémorisées par navigateur, la plus récente d\'abord, sans doublon, 8 au plus', () => {
    const { rerender } = render(<ColorWheel value="#FF0000" onChange={vi.fn()} />)
    const input = screen.getByLabelText('Code de la couleur (#RRGGBB)')
    for (const c of ['#111111', '#222222', '#111111', ...Array.from({ length: 9 }, (_, i) => `#0000${(i + 10).toString(16).toUpperCase()}0`.slice(0, 7))]) {
      fireEvent.change(input, { target: { value: c } })
      fireEvent.blur(input)
    }
    const list = readRecents()
    expect(list.length).toBeLessThanOrEqual(8)
    expect(new Set(list).size).toBe(list.length)
    rerender(<ColorWheel value="#FF0000" onChange={vi.fn()} />)
    expect(screen.getAllByLabelText(/Couleur récente/).length).toBe(list.length)
  })

  it('clic sur une couleur récente : la reprend (commit)', () => {
    localStorage.setItem('buzzcontrol.effects.recentColors', JSON.stringify(['#ABCDEF']))
    const onChange = vi.fn()
    render(<ColorWheel value="#FF0000" onChange={onChange} />)
    fireEvent.click(screen.getByLabelText('Couleur récente #ABCDEF'))
    expect(onChange).toHaveBeenCalledWith('#ABCDEF', { commit: true })
  })

  it('stockage indisponible ou corrompu : le composant s\'affiche quand même', () => {
    localStorage.setItem('buzzcontrol.effects.recentColors', '{pas du json')
    render(<ColorWheel value="#FF0000" onChange={vi.fn()} />)
    expect(screen.getByTestId('color-wheel')).toBeInTheDocument()
    expect(screen.queryAllByLabelText(/Couleur récente/)).toHaveLength(0)
  })

  it('valeur invalide reçue : couleur par défaut, pas de plantage', () => {
    render(<ColorWheel value="rouge" onChange={vi.fn()} />)
    expect(screen.getByLabelText('Code de la couleur (#RRGGBB)')).toHaveValue('#14B8A6')
  })
})
