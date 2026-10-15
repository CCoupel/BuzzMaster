import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import PointsRuleControl from './PointsRuleControl'

// ---------------------------------------------------------------------------
// PointsRuleControl — ligne « Points : » de l'éditeur de carte MEMOTION
// (#190, LOT-5, v11.2.2, contrat question-types.md §6.2, maquette
// admin_question_editor__memotion_card_points_rule).
//
// Contrat DOM (plan #190 §« components/PointsRuleControl.jsx ») :
//   div[role=group][aria-label="Barème de points de la carte"]
//     span « Points : » + select[aria-label="Mode de barème"] (défaut en premier,
//     libellé « (défaut) ») + input[type=number][min=1][max=999] seulement pour
//     FIXED / PER_UNIT (aria-label « Montant » / « Points par unité »,
//     aria-invalid si vide ou hors bornes) + aide « → … » + avertissement
//     (notice) + erreur (error).
// `value = { mode, value }` ; `mode: ''` = défaut du type = POINTS_RULE absent.
// Les tests ciblent rôles / aria-* / libellés, jamais les classes CSS.
// ---------------------------------------------------------------------------

vi.mock('./PointsRuleControl.css', () => ({}))

function renderControl(overrides = {}) {
  const onChange = overrides.onChange ?? vi.fn()
  const utils = render(
    <PointsRuleControl
      cardType={overrides.cardType ?? 'SPEEDY'}
      value={overrides.value ?? { mode: '', value: '' }}
      onChange={onChange}
      difficulty={overrides.difficulty ?? 2}
      motionConfig={overrides.motionConfig}
      unitsTotal={overrides.unitsTotal}
      notice={overrides.notice}
      error={overrides.error}
      idPrefix="motion-card-mc-1"
    />
  )
  return { ...utils, onChange }
}

const modeSelect = () => screen.getByRole('combobox', { name: 'Mode de barème' })
const optionLabels = () => within(modeSelect()).getAllByRole('option').map(o => o.textContent)

beforeEach(() => {
  vi.clearAllMocks()
})

describe('PointsRuleControl — structure et accessibilité', () => {
  it('rend un groupe nommé « Barème de points de la carte » avec le libellé « Points : » et le select « Mode de barème »', () => {
    renderControl()
    const group = screen.getByRole('group', { name: 'Barème de points de la carte' })
    expect(within(group).getByText('Points :')).toBeInTheDocument()
    expect(within(group).getByRole('combobox', { name: 'Mode de barème' })).toBeInTheDocument()
  })
})

describe('PointsRuleControl — options filtrées par type (défaut en premier)', () => {
  it.each(['SPEEDY', 'QCM'])('carte %s (binaire) : exactement « Étoiles (défaut) » puis « Montant fixe »', (cardType) => {
    renderControl({ cardType })
    expect(optionLabels()).toEqual(['Étoiles (défaut)', 'Montant fixe'])
  })

  it.each(['MEMORY', 'RAFALE'])('carte %s (progression) : exactement les 4 options, « Prorata des étoiles (défaut) » en premier', (cardType) => {
    renderControl({ cardType })
    expect(optionLabels()).toEqual([
      'Prorata des étoiles (défaut)',
      'Étoiles pleines (tout ou rien)',
      'Montant fixe',
      'Par unité',
    ])
  })

  it('binaire : ni « Par unité » ni « Prorata » ne sont proposés', () => {
    renderControl({ cardType: 'QCM' })
    expect(within(modeSelect()).queryByRole('option', { name: /Par unité|Prorata/ })).not.toBeInTheDocument()
  })

  it('mode: \'\' (défaut) : l\'option « (défaut) » est sélectionnée', () => {
    renderControl({ cardType: 'MEMORY', value: { mode: '', value: '' } })
    expect(modeSelect().selectedOptions[0].textContent).toBe('Prorata des étoiles (défaut)')
  })

  it('mode FIXED : « Montant fixe » est sélectionnée', () => {
    renderControl({ cardType: 'QCM', value: { mode: 'FIXED', value: 10 } })
    expect(modeSelect().selectedOptions[0].textContent).toBe('Montant fixe')
  })

  it('mode STARS sur type à progression : « Étoiles pleines (tout ou rien) » est sélectionnée', () => {
    renderControl({ cardType: 'MEMORY', value: { mode: 'STARS', value: '' } })
    expect(modeSelect().selectedOptions[0].textContent).toBe('Étoiles pleines (tout ou rien)')
  })
})

describe('PointsRuleControl — champ de montant (FIXED / PER_UNIT seulement)', () => {
  it.each([
    ['défaut', { mode: '', value: '' }],
    ['étoiles pleines', { mode: 'STARS', value: '' }],
  ])('mode %s : aucun champ de montant', (_label, value) => {
    renderControl({ cardType: 'MEMORY', value })
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument()
  })

  it('FIXED : champ « Montant » (nombre, min 1, max 999) avec la valeur saisie', () => {
    renderControl({ cardType: 'QCM', value: { mode: 'FIXED', value: 10 } })
    const input = screen.getByRole('spinbutton', { name: 'Montant' })
    expect(input).toHaveAttribute('type', 'number')
    expect(input).toHaveAttribute('min', '1')
    expect(input).toHaveAttribute('max', '999')
    expect(input.value).toBe('10')
  })

  it('PER_UNIT : champ « Points par unité »', () => {
    renderControl({ cardType: 'RAFALE', value: { mode: 'PER_UNIT', value: 2 } })
    expect(screen.getByRole('spinbutton', { name: 'Points par unité' })).toBeInTheDocument()
  })

  it.each([
    ['vide', ''],
    ['zéro', 0],
    ['au-dessus de 999', 1000],
    ['non entier', 2.5],
  ])('valeur %s : aria-invalid="true"', (_label, value) => {
    renderControl({ cardType: 'QCM', value: { mode: 'FIXED', value } })
    expect(screen.getByRole('spinbutton', { name: 'Montant' })).toHaveAttribute('aria-invalid', 'true')
  })

  it.each([1, 10, 999])('valeur %s (bornes comprises) : pas d\'aria-invalid', (value) => {
    renderControl({ cardType: 'QCM', value: { mode: 'FIXED', value } })
    expect(screen.getByRole('spinbutton', { name: 'Montant' })).not.toHaveAttribute('aria-invalid', 'true')
  })
})

describe('PointsRuleControl — onChange', () => {
  it('choisir « Montant fixe » depuis le défaut : { mode: \'FIXED\', value: \'\' } (montant à saisir)', () => {
    const { onChange } = renderControl({ cardType: 'QCM' })
    fireEvent.change(modeSelect(), { target: { value: 'FIXED' } })
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith({ mode: 'FIXED', value: '' })
  })

  it('choisir « Par unité » sur MEMORY : mode PER_UNIT', () => {
    const { onChange } = renderControl({ cardType: 'MEMORY' })
    fireEvent.change(modeSelect(), { target: { value: 'PER_UNIT' } })
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ mode: 'PER_UNIT' }))
  })

  it('choisir « Étoiles pleines » sur MEMORY : mode STARS, sans montant', () => {
    const { onChange } = renderControl({ cardType: 'MEMORY' })
    fireEvent.change(modeSelect(), { target: { value: 'STARS' } })
    expect(onChange).toHaveBeenCalledWith({ mode: 'STARS', value: '' })
  })

  it('revenir au défaut : mode \'\' (POINTS_RULE absent à l\'envoi), le montant n\'est plus conservé', () => {
    const { onChange } = renderControl({ cardType: 'QCM', value: { mode: 'FIXED', value: 10 } })
    fireEvent.change(modeSelect(), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith({ mode: '', value: '' })
  })

  it('saisir un montant : value passée en NOMBRE, mode conservé', () => {
    const { onChange } = renderControl({ cardType: 'QCM', value: { mode: 'FIXED', value: '' } })
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Montant' }), { target: { value: '25' } })
    expect(onChange).toHaveBeenCalledWith({ mode: 'FIXED', value: 25 })
  })

  it('effacer le montant : value \'\' (et non 0)', () => {
    const { onChange } = renderControl({ cardType: 'QCM', value: { mode: 'FIXED', value: 10 } })
    fireEvent.change(screen.getByRole('spinbutton', { name: 'Montant' }), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith({ mode: 'FIXED', value: '' })
  })
})

describe('PointsRuleControl — aide « → … » (describePointsRule, maquette)', () => {
  it('défaut binaire ★★ : « → 3 pts (★★) »', () => {
    renderControl({ cardType: 'SPEEDY', difficulty: 2 })
    expect(screen.getByText('→ 3 pts (★★)')).toBeInTheDocument()
  })

  it('suit le barème de la question (motionConfig) : ★★ = 4 pts', () => {
    renderControl({ cardType: 'SPEEDY', difficulty: 2, motionConfig: { POINTS_1_STAR: 1, POINTS_2_STAR: 4, POINTS_3_STAR: 6 } })
    expect(screen.getByText('→ 4 pts (★★)')).toBeInTheDocument()
  })

  it('Montant fixe 10 : « → 10 pts, quelle que soit la valeur ★ »', () => {
    renderControl({ cardType: 'QCM', value: { mode: 'FIXED', value: 10 } })
    expect(screen.getByText('→ 10 pts, quelle que soit la valeur ★')).toBeInTheDocument()
  })

  it('MEMORY ★★★ par défaut, 8 paires : « → jusqu\'à 5 pts, au prorata des paires trouvées (8 paires) »', () => {
    renderControl({ cardType: 'MEMORY', difficulty: 3, unitsTotal: 8 })
    expect(screen.getByText('→ jusqu\'à 5 pts, au prorata des paires trouvées (8 paires)')).toBeInTheDocument()
  })

  it('RAFALE par défaut : « → jusqu\'à … pts, au prorata des bonnes réponses »', () => {
    renderControl({ cardType: 'RAFALE', difficulty: 2 })
    expect(screen.getByText('→ jusqu\'à 3 pts, au prorata des bonnes réponses')).toBeInTheDocument()
  })

  it('Par unité 2 sur RAFALE : « → 2 pts × bonnes réponses »', () => {
    renderControl({ cardType: 'RAFALE', value: { mode: 'PER_UNIT', value: 2 } })
    expect(screen.getByText('→ 2 pts × bonnes réponses')).toBeInTheDocument()
  })

  it('Par unité sur MEMORY : « → 2 pts × paires trouvées »', () => {
    renderControl({ cardType: 'MEMORY', value: { mode: 'PER_UNIT', value: 2 }, unitsTotal: 8 })
    expect(screen.getByText('→ 2 pts × paires trouvées')).toBeInTheDocument()
  })

  it('Par unité sans montant : « → N pts × bonnes réponses »', () => {
    renderControl({ cardType: 'RAFALE', value: { mode: 'PER_UNIT', value: '' } })
    expect(screen.getByText('→ N pts × bonnes réponses')).toBeInTheDocument()
  })

  it('l\'aide suit la difficulté de la carte', () => {
    const { rerender } = renderControl({ cardType: 'SPEEDY', difficulty: 1 })
    expect(screen.getByText('→ 1 pt (★)')).toBeInTheDocument()
    rerender(
      <PointsRuleControl cardType="SPEEDY" value={{ mode: '', value: '' }} onChange={vi.fn()} difficulty={3} idPrefix="motion-card-mc-1" />
    )
    expect(screen.getByText('→ 5 pts (★★★)')).toBeInTheDocument()
  })
})

describe('PointsRuleControl — avertissement et erreur', () => {
  it('notice rendue (annonce polie role=status) quand fournie', () => {
    renderControl({ notice: 'Le barème « Par unité » n\'existe pas pour ce type de carte : remis à « Étoiles (défaut) ».' })
    expect(screen.getByRole('status')).toHaveTextContent(/Par unité.*remis à « Étoiles \(défaut\) »/)
  })

  it('error rendue (role=alert) quand fournie', () => {
    renderControl({
      cardType: 'RAFALE',
      value: { mode: 'PER_UNIT', value: '' },
      error: 'Indiquez un montant par unité (1 à 999) — la carte ne peut pas être enregistrée sans.',
    })
    expect(screen.getByRole('alert')).toHaveTextContent('Indiquez un montant par unité (1 à 999)')
  })

  it('ni notice ni error : ni status ni alert', () => {
    renderControl()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
