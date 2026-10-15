import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import DifficultyPicker from './DifficultyPicker'
import { DIFFICULTY_LEVELS } from '../utils/categoryUtils'

// ---------------------------------------------------------------------------
// DifficultyPicker — sélecteur de difficulté (★) unifié (#221, LOT-4, v11.2.2).
//
// Remplace `.memotion-diff-btn` (valeur de la carte MEMOTION), `.rafale-diff-btn`
// (question du réservoir), `.rafale-chip ★☆☆` (filtre ★ du réservoir),
// `.rafale-multi-chip` ★ (manche / carte RAFALE) et `.rafale-ai-star-chip`
// (modale IA RAFALE). Trois usages :
//   - single (strict)       : sites 6 et 9 — radio, re-clic sur l'active = no-op
//   - single + allowNone    : site 11 — re-clic désélectionne (onChange(null)), ghost ★☆☆
//   - multi                 : sites 3, 5, 15 — aria-pressed + ✓
//
// Contrat a11y : single → role="radiogroup" > role="radio" + aria-checked ;
// multi → <button aria-pressed> + ✓. Les tests ciblent rôles / texte, jamais les classes.
// Les chips sont dans l'ordre de `levels` (défaut [1, 2, 3]) ; le texte d'un chip
// de niveau n commence par n étoiles « ★ ».
// ---------------------------------------------------------------------------

vi.mock('./DifficultyPicker.css', () => ({}))

function renderSingle(overrides = {}) {
  const onChange = overrides.onChange ?? vi.fn()
  const utils = render(
    <DifficultyPicker
      mode="single"
      value={overrides.value ?? null}
      onChange={onChange}
      allowNone={overrides.allowNone}
      ghost={overrides.ghost}
      label={overrides.label}
      showModeBadge={overrides.showModeBadge}
    />
  )
  return { ...utils, onChange }
}

function renderMulti(overrides = {}) {
  const onToggle = overrides.onToggle ?? vi.fn()
  const utils = render(
    <DifficultyPicker
      mode="multi"
      value={overrides.value ?? []}
      onToggle={onToggle}
      onChange={overrides.onChange}
      levels={overrides.levels}
      label={overrides.label}
      showModeBadge={overrides.showModeBadge}
    />
  )
  return { ...utils, onToggle }
}

const texts = (nodes) => nodes.map(n => n.textContent.replace(/\s+/g, ''))

beforeEach(() => {
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Constante partagée
// ---------------------------------------------------------------------------

describe('DIFFICULTY_LEVELS — constante partagée (remplace RAFALE_DIFFICULTIES / DIFFICULTIES / [1,2,3] inline)', () => {
  it('vaut [1, 2, 3], exportée par utils/categoryUtils', () => {
    expect(DIFFICULTY_LEVELS).toEqual([1, 2, 3])
  })
})

// ---------------------------------------------------------------------------
// single strict (sites 6 et 9)
// ---------------------------------------------------------------------------

describe('DifficultyPicker — single strict', () => {
  it('rend un radiogroup de 3 radios ★, ★★, ★★★ (dans cet ordre)', () => {
    renderSingle()
    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
    expect(texts(screen.getAllByRole('radio'))).toEqual(['★', '★★', '★★★'])
  })

  it('libellé « Difficulté » par défaut + badge « une seule » dans le DOM', () => {
    renderSingle()
    expect(screen.getByText('Difficulté')).toBeInTheDocument()
    expect(screen.getByText('une seule')).toBeInTheDocument()
  })

  it('label="Valeur de la carte :" est rendu tel quel (contrainte C5, retour QUALIF v9.0.0.4)', () => {
    renderSingle({ label: 'Valeur de la carte :' })
    expect(screen.getByText('Valeur de la carte :')).toBeInTheDocument()
    expect(screen.queryByText('Difficulté')).not.toBeInTheDocument()
    expect(screen.getByText('une seule')).toBeInTheDocument()
  })

  it('showModeBadge=false : le badge n\'est pas rendu', () => {
    renderSingle({ showModeBadge: false })
    expect(screen.queryByText('une seule')).not.toBeInTheDocument()
  })

  it('clic sur un niveau non sélectionné appelle onChange avec le niveau (entier)', () => {
    const { onChange } = renderSingle({ value: 1 })
    fireEvent.click(screen.getAllByRole('radio')[2])
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith(3)
  })

  it('re-cliquer sur le niveau DÉJÀ sélectionné = aucun appel (radio strict, sites 6 et 9)', () => {
    const { onChange } = renderSingle({ value: 2 })
    fireEvent.click(screen.getAllByRole('radio')[1])
    expect(onChange).not.toHaveBeenCalled()
  })

  it('aria-checked : « true » sur le niveau sélectionné seulement', () => {
    renderSingle({ value: 2 })
    const radios = screen.getAllByRole('radio')
    expect(radios[0]).toHaveAttribute('aria-checked', 'false')
    expect(radios[1]).toHaveAttribute('aria-checked', 'true')
    expect(radios[2]).toHaveAttribute('aria-checked', 'false')
  })

  it('value=null : aucun radio coché', () => {
    renderSingle({ value: null })
    for (const radio of screen.getAllByRole('radio')) {
      expect(radio).toHaveAttribute('aria-checked', 'false')
    }
  })

  it('pas de ✓ en sélection unique, pas d\'étoiles fantômes par défaut', () => {
    renderSingle({ value: 2 })
    expect(screen.queryByText('✓')).not.toBeInTheDocument()
    expect(screen.getAllByRole('radio').every(r => !r.textContent.includes('☆'))).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// single + allowNone + ghost (site 11 : filtre ★ du réservoir)
// ---------------------------------------------------------------------------

describe('DifficultyPicker — single allowNone (+ ghost)', () => {
  it('badge « une seule ou aucune » (et non « une seule »)', () => {
    renderSingle({ allowNone: true })
    expect(screen.getByText('une seule ou aucune')).toBeInTheDocument()
    expect(screen.queryByText('une seule')).not.toBeInTheDocument()
  })

  it('re-cliquer sur le niveau actif appelle onChange(null) (désélection)', () => {
    const { onChange } = renderSingle({ allowNone: true, value: 2 })
    fireEvent.click(screen.getAllByRole('radio')[1])
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith(null)
  })

  it('clic sur un autre niveau appelle onChange(niveau)', () => {
    const { onChange } = renderSingle({ allowNone: true, value: 2 })
    fireEvent.click(screen.getAllByRole('radio')[0])
    expect(onChange).toHaveBeenCalledWith(1)
  })

  it('ghost : étoiles fantômes ★☆☆, ★★☆, ★★★', () => {
    renderSingle({ allowNone: true, ghost: true })
    expect(texts(screen.getAllByRole('radio'))).toEqual(['★☆☆', '★★☆', '★★★'])
  })

  it('ghost : aria-checked toujours exposé sur le niveau actif', () => {
    renderSingle({ allowNone: true, ghost: true, value: 3 })
    expect(screen.getAllByRole('radio')[2]).toHaveAttribute('aria-checked', 'true')
  })
})

// ---------------------------------------------------------------------------
// multi (sites 3, 5, 15)
// ---------------------------------------------------------------------------

describe('DifficultyPicker — multi', () => {
  it('rend 3 boutons aria-pressed, ni radiogroup ni radio', () => {
    renderMulti()
    expect(screen.queryByRole('radiogroup')).not.toBeInTheDocument()
    expect(screen.queryAllByRole('radio')).toHaveLength(0)
    expect(screen.getAllByRole('button', { pressed: false })).toHaveLength(3)
  })

  it('libellé « Difficultés » par défaut + badge « plusieurs possibles »', () => {
    renderMulti()
    expect(screen.getByText('Difficultés')).toBeInTheDocument()
    expect(screen.getByText('plusieurs possibles')).toBeInTheDocument()
  })

  it('label surchargé : « Difficultés des questions tirées » (site 5)', () => {
    renderMulti({ label: 'Difficultés des questions tirées' })
    expect(screen.getByText('Difficultés des questions tirées')).toBeInTheDocument()
    expect(screen.queryByText('Difficultés')).not.toBeInTheDocument()
  })

  it('clic sur un niveau appelle onToggle avec le niveau (et jamais onChange)', () => {
    const onChange = vi.fn()
    const { onToggle } = renderMulti({ onChange })
    fireEvent.click(screen.getAllByRole('button')[1])
    expect(onToggle).toHaveBeenCalledTimes(1)
    expect(onToggle).toHaveBeenCalledWith(2)
    expect(onChange).not.toHaveBeenCalled()
  })

  it('value TABLEAU : les niveaux présents sont aria-pressed="true"', () => {
    renderMulti({ value: [1, 3] })
    const buttons = screen.getAllByRole('button')
    expect(buttons[0]).toHaveAttribute('aria-pressed', 'true')
    expect(buttons[1]).toHaveAttribute('aria-pressed', 'false')
    expect(buttons[2]).toHaveAttribute('aria-pressed', 'true')
  })

  it('value SET : même rendu qu\'avec un tableau', () => {
    renderMulti({ value: new Set([2]) })
    const buttons = screen.getAllByRole('button')
    expect(buttons[0]).toHaveAttribute('aria-pressed', 'false')
    expect(buttons[1]).toHaveAttribute('aria-pressed', 'true')
    expect(buttons[2]).toHaveAttribute('aria-pressed', 'false')
  })

  it('✓ visible sur les niveaux actifs seulement', () => {
    renderMulti({ value: [1, 3] })
    expect(screen.getAllByText('✓')).toHaveLength(2)
    const buttons = screen.getAllByRole('button')
    expect(within(buttons[0]).getByText('✓')).toBeInTheDocument()
    expect(within(buttons[1]).queryByText('✓')).not.toBeInTheDocument()
  })

  it('aucune sélection : aucun ✓', () => {
    renderMulti({ value: [] })
    expect(screen.queryByText('✓')).not.toBeInTheDocument()
  })

  it('levels personnalisés : seuls les niveaux fournis sont rendus, onToggle reçoit le niveau réel', () => {
    const { onToggle } = renderMulti({ levels: [2, 3] })
    const buttons = screen.getAllByRole('button')
    expect(buttons).toHaveLength(2)
    expect(texts(buttons)).toEqual(['★★', '★★★'])
    fireEvent.click(buttons[0])
    expect(onToggle).toHaveBeenCalledWith(2)
  })
})
