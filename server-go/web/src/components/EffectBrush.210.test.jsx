import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import EffectBrush, { brushToAction, DEFAULT_BRUSH, BRUSH_TIPS, INTENSITY_TIP } from './EffectBrush'

// #210 (v11.8.0, LOT-2, F2.2) — pinceau : couleurs nommées, libre (roue), éteint, gomme ; intensité ; info-bulles accessibles au clavier.
// Source : contrat §4.2 (couleurs, intensité 0-100, [eteint]), §11.8 (case vide = aucune action), maquette effect_editor §02.

describe('brushToAction', () => {
  it('couleur nommée : { target, color, intensity }', () => {
    expect(brushToAction('g2', { color: 'theme', intensity: 70, free: '#14B8A6' })).toEqual({ target: 'g2', color: 'theme', intensity: 70 })
  })
  it('couleur libre : le #RRGGBB choisi sur la roue', () => {
    expect(brushToAction('g1', { color: 'free', intensity: 100, free: '#00AAFF' })).toEqual({ target: 'g1', color: '#00AAFF', intensity: 100 })
  })
  it('éteint : { target, off: true } sans color ni intensity', () => {
    expect(brushToAction('g1', { color: 'off', intensity: 50, free: '#14B8A6' })).toEqual({ target: 'g1', off: true })
  })
  it('gomme : null (vide la case)', () => {
    expect(brushToAction('g1', { color: 'erase', intensity: 50, free: '#14B8A6' })).toBeNull()
  })
  it('l\'intensité est bornée à 0-100 et arrondie', () => {
    expect(brushToAction('g1', { color: 'scene', intensity: 140 }).intensity).toBe(100)
    expect(brushToAction('g1', { color: 'scene', intensity: -5 }).intensity).toBe(0)
    expect(brushToAction('g1', { color: 'scene', intensity: 33.4 }).intensity).toBe(33)
  })
})

describe('composant', () => {
  const mount = (brush = DEFAULT_BRUSH, over = {}) => {
    const onChange = vi.fn()
    render(<EffectBrush brush={brush} onChange={onChange} {...over} />)
    return onChange
  }

  it('sept pinceaux : thème, équipe, urgence, scène, couleur libre, éteint, gomme ; celui choisi est pressé', () => {
    mount()
    expect(BRUSH_TIPS.map(b => b.key)).toEqual(['theme', 'equipe', 'urgence', 'scene', 'free', 'off', 'erase'])
    const group = screen.getByRole('group', { name: 'Couleur du pinceau' })
    expect(group.querySelectorAll('button')).toHaveLength(7)
    expect(screen.getByRole('button', { name: /^Urgence/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /^Thème/ })).toHaveAttribute('aria-pressed', 'false')
  })

  it('chaque pinceau porte son info-bulle (nom accessible = libellé + définition), au clavier comme au survol', () => {
    mount()
    for (const b of BRUSH_TIPS) expect(screen.getByRole('button', { name: new RegExp(`^${b.label}`) })).toHaveAttribute('data-tip', b.tip)
    expect(screen.getByLabelText(/^Intensité —/)).toHaveAttribute('tabindex', '0')
    expect(screen.getByLabelText(/^Intensité —/)).toHaveAttribute('data-tip', INTENSITY_TIP)
  })

  it('choisir un pinceau et une intensité', () => {
    const onChange = mount()
    fireEvent.click(screen.getByRole('button', { name: /^Éteint/ }))
    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_BRUSH, color: 'off' })
    fireEvent.click(screen.getByRole('button', { name: '50 %' }))
    expect(onChange).toHaveBeenLastCalledWith({ ...DEFAULT_BRUSH, intensity: 50 })
    fireEvent.change(screen.getByLabelText('Autre intensité, de 0 à 100'), { target: { value: '140' } })
    expect(onChange).toHaveBeenLastCalledWith({ ...DEFAULT_BRUSH, intensity: 100 })
  })

  it('la roue n\'apparaît qu\'avec « couleur libre » ; un clic dans la roue change la couleur libre', () => {
    const { unmount } = render(<EffectBrush brush={DEFAULT_BRUSH} onChange={vi.fn()} />)
    expect(screen.queryByTestId('color-wheel')).toBeNull()
    unmount()
    const onChange = mount({ ...DEFAULT_BRUSH, color: 'free', free: '#FF0000' })
    expect(screen.getByTestId('color-wheel')).toBeInTheDocument()
    const code = screen.getByLabelText('Code de la couleur (#RRGGBB)')
    fireEvent.change(code, { target: { value: '#00FF00' } })
    fireEvent.blur(code)
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ color: 'free', free: '#00FF00' }))
  })

  it('désactivé (effet livré) : rien n\'est modifiable', () => {
    mount(DEFAULT_BRUSH, { disabled: true })
    for (const b of screen.getByRole('group', { name: 'Couleur du pinceau' }).querySelectorAll('button')) expect(b).toBeDisabled()
  })
})
