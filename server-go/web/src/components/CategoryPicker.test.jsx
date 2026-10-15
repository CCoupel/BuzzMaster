import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import CategoryPicker from './CategoryPicker'

// ---------------------------------------------------------------------------
// CategoryPicker — sélecteur de catégorie unifié (#221, LOT-4, v11.2.2).
//
// Remplace CategorySelector (sélection unique + création inline), CategoryFilterBar
// (filtre à icônes rondes) et les chips « catégories cibles » texte des sites
// RAFALE / modales IA. Trois présentations :
//   - mode="single"            : radiogroup, une catégorie (re-clic = désélection), « + » optionnel
//   - mode="multi"             : boutons aria-pressed + ✓ sur les actives, renderExtra
//   - variant="filter"         : icônes rondes + bouton × (useCategoryFilter)
//
// Contrat a11y (plan §« API des composants », DECISIONS.md `admin_pickers`) :
// les tests ciblent rôles / aria-* / title / texte, JAMAIS les classes CSS.
//   - single : role="radiogroup" > role="radio" + aria-checked ; chip title = libellé
//   - multi / filter : <button aria-pressed> ; filter : title = libellé
//   - badge de mode rendu dans le DOM : « une seule » / « plusieurs possibles »
//
// Ce fichier transpose les cas de CategoryFilterBar.test.jsx (section filter) ;
// les 12 cas `allowCreate` de CategorySelector.test.jsx vivent dans
// CategoryPicker.create.test.jsx. Les deux anciens fichiers sont supprimés dans
// le même commit.
// ---------------------------------------------------------------------------

vi.mock('./CategoryPicker.css', () => ({}))

const CUSTOM_CATEGORIES = [
  { key: 'CUSTOM_1', name: 'Custom Un', isCustom: true, imageURL: '/img/custom1.png' },
]

const STANDARD_LABELS = [
  'Geographie', 'Divertissement', 'Histoire', 'Arts & Litterature',
  'Sciences & Nature', 'Sports & Loisirs', 'Gastronomie', 'Animaux',
]
const STANDARD_KEYS = [
  'GEOGRAPHY', 'ENTERTAINMENT', 'HISTORY', 'ARTS', 'SCIENCE', 'SPORTS', 'FOOD', 'ANIMALS',
]

// Forme RÉELLE de GET /api/categories (internal/server/http.go, CategoryInfo) :
// les 8 catégories codées en dur en miroir (isCustom:false) + les vraies
// catégories personnalisées (isCustom:true), PAS pré-filtrée. Cause racine du
// bug #197 (QUALIF 8.0.0.2, « le pool RAFALE liste 2 lignes identiques »).
const RAW_API_CATEGORIES_RESPONSE = [
  { key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', isCustom: false, color: '#3b82f6' },
  { key: 'ENTERTAINMENT', name: 'Divertissement', imageURL: '', isCustom: false, color: '#ec4899' },
  { key: 'HISTORY', name: 'Histoire', imageURL: '', isCustom: false, color: '#eab308' },
  { key: 'ARTS', name: 'Arts & Litterature', imageURL: '', isCustom: false, color: '#a855f7' },
  { key: 'SCIENCE', name: 'Sciences & Nature', imageURL: '', isCustom: false, color: '#22c55e' },
  { key: 'SPORTS', name: 'Sports & Loisirs', imageURL: '', isCustom: false, color: '#f97316' },
  { key: 'FOOD', name: 'Gastronomie', imageURL: '', isCustom: false, color: '#991b1b' },
  { key: 'ANIMALS', name: 'Animaux', imageURL: '', isCustom: false, color: '#78716c' },
  { key: 'CUSTOM_MASCOTS', name: 'Mascottes', imageURL: '/files/categories/mascots.png', isCustom: true, color: '' },
]
const RAW_KEYS = RAW_API_CATEGORIES_RESPONSE.map(c => c.key)

function renderSingle(overrides = {}) {
  const onChange = overrides.onChange ?? vi.fn()
  const utils = render(
    <CategoryPicker
      mode="single"
      value={overrides.value ?? ''}
      onChange={onChange}
      customCategories={overrides.customCategories ?? []}
      onRefetchCategories={overrides.onRefetchCategories}
      allowCreate={overrides.allowCreate}
      label={overrides.label}
      showModeBadge={overrides.showModeBadge}
    />
  )
  return { ...utils, onChange }
}

function renderMulti(overrides = {}) {
  const onToggle = overrides.onToggle ?? vi.fn()
  const utils = render(
    <CategoryPicker
      mode="multi"
      value={overrides.value ?? []}
      onToggle={onToggle}
      onChange={overrides.onChange}
      customCategories={overrides.customCategories ?? []}
      renderExtra={overrides.renderExtra}
      label={overrides.label}
      showModeBadge={overrides.showModeBadge}
    />
  )
  return { ...utils, onToggle }
}

function renderFilter(overrides = {}) {
  const onToggle = overrides.onToggle ?? vi.fn()
  const onClear = overrides.onClear ?? vi.fn()
  const utils = render(
    <CategoryPicker
      variant="filter"
      mode="multi"
      value={overrides.value ?? new Set()}
      availableCategories={overrides.availableCategories ?? ['HISTORY', 'SCIENCE']}
      customCategories={overrides.customCategories ?? []}
      onToggle={onToggle}
      onClear={onClear}
      showLabel={overrides.showLabel}
      allowCreate={overrides.allowCreate}
    />
  )
  return { ...utils, onToggle, onClear }
}

let consoleErrorSpy

beforeEach(() => {
  vi.clearAllMocks()
  consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(() => {
  consoleErrorSpy.mockRestore()
})

/** React signale les clés dupliquées via console.error ("... same key ..."). */
function expectNoDuplicateKeyWarning() {
  const duplicated = consoleErrorSpy.mock.calls.some(args => String(args[0]).includes('same key'))
  expect(duplicated).toBe(false)
}

// ---------------------------------------------------------------------------
// Rendu commun : badge de mode et libellé
// ---------------------------------------------------------------------------

describe('CategoryPicker — badge de mode et libellé (présents dans le DOM, pas seulement en couleur)', () => {
  it('single : libellé « Catégorie » par défaut + badge « une seule »', () => {
    renderSingle()
    expect(screen.getByText('Catégorie')).toBeInTheDocument()
    expect(screen.getByText('une seule')).toBeInTheDocument()
  })

  it('multi : libellé « Catégories » par défaut + badge « plusieurs possibles »', () => {
    renderMulti()
    expect(screen.getByText('Catégories')).toBeInTheDocument()
    expect(screen.getByText('plusieurs possibles')).toBeInTheDocument()
  })

  it('filter : libellé « Filtrer » par défaut + badge « plusieurs possibles »', () => {
    renderFilter()
    expect(screen.getByText('Filtrer')).toBeInTheDocument()
    expect(screen.getByText('plusieurs possibles')).toBeInTheDocument()
  })

  it('label surchargé : remplace le libellé par défaut (ex. « Catégories des questions tirées »)', () => {
    renderMulti({ label: 'Catégories des questions tirées' })
    expect(screen.getByText('Catégories des questions tirées')).toBeInTheDocument()
    expect(screen.queryByText('Catégories')).not.toBeInTheDocument()
    expect(screen.getByText('plusieurs possibles')).toBeInTheDocument()
  })

  it('showModeBadge=false : le badge n\'est pas rendu', () => {
    renderSingle({ showModeBadge: false })
    expect(screen.queryByText('une seule')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// #197 — garde isCustom INTERNE au composant (cause racine du doublon)
// ---------------------------------------------------------------------------

describe('CategoryPicker — non-régression #197 : liste BRUTE de GET /api/categories → chaque catégorie une seule fois', () => {
  it('single : 8 miroirs isCustom:false + 1 custom → exactement 9 radios, chaque libellé une seule fois', () => {
    renderSingle({ customCategories: RAW_API_CATEGORIES_RESPONSE })

    expect(screen.getAllByRole('radio')).toHaveLength(9)
    for (const label of [...STANDARD_LABELS, 'Mascottes']) {
      expect(screen.getAllByTitle(label)).toHaveLength(1)
    }
    expectNoDuplicateKeyWarning()
  })

  it('multi : 8 miroirs isCustom:false + 1 custom → exactement 9 boutons, chaque libellé une seule fois', () => {
    renderMulti({ customCategories: RAW_API_CATEGORIES_RESPONSE })

    expect(screen.getAllByRole('button')).toHaveLength(9)
    for (const label of [...STANDARD_LABELS, 'Mascottes']) {
      expect(screen.getAllByRole('button', { name: new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')) })).toHaveLength(1)
    }
    expectNoDuplicateKeyWarning()
  })

  it('filter : toutes les clés de la réponse brute disponibles → 9 pastilles, chaque titre une seule fois', () => {
    renderFilter({ availableCategories: RAW_KEYS, customCategories: RAW_API_CATEGORIES_RESPONSE })

    expect(screen.getAllByRole('button')).toHaveLength(9)
    for (const label of [...STANDARD_LABELS, 'Mascottes']) {
      expect(screen.getAllByTitle(label)).toHaveLength(1)
    }
    expectNoDuplicateKeyWarning()
  })

  it('la vraie catégorie personnalisée (isCustom:true) apparaît, une seule fois, dans les trois présentations', () => {
    const { unmount } = renderSingle({ customCategories: RAW_API_CATEGORIES_RESPONSE })
    expect(screen.getAllByTitle('Mascottes')).toHaveLength(1)
    unmount()

    const second = renderMulti({ customCategories: RAW_API_CATEGORIES_RESPONSE })
    expect(screen.getAllByRole('button', { name: /Mascottes/ })).toHaveLength(1)
    second.unmount()

    renderFilter({ availableCategories: ['CUSTOM_MASCOTS'], customCategories: RAW_API_CATEGORIES_RESPONSE })
    expect(screen.getAllByTitle('Mascottes')).toHaveLength(1)
  })

  it('sélectionner une catégorie standard depuis une réponse API brute appelle onChange une seule fois avec la bonne clé', () => {
    const { onChange } = renderSingle({ customCategories: RAW_API_CATEGORIES_RESPONSE })
    fireEvent.click(screen.getByTitle('Histoire'))
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith('HISTORY')
  })

  it('une clé sans meta connue est silencieusement ignorée, sans crash (multi : value inconnue ; filter : availableCategories inconnue)', () => {
    const { unmount } = renderMulti({ value: ['CLE_INCONNUE_XYZ', 'HISTORY'] })
    expect(screen.getAllByRole('button')).toHaveLength(8)
    expect(screen.getByRole('button', { name: /Histoire/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.queryByText('CLE_INCONNUE_XYZ')).not.toBeInTheDocument()
    unmount()

    renderFilter({ availableCategories: ['HISTORY', 'CLE_INCONNUE_XYZ'] })
    expect(screen.getAllByRole('button')).toHaveLength(1)
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
    expect(screen.queryByText('CLE_INCONNUE_XYZ')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// mode="single"
// ---------------------------------------------------------------------------

describe('CategoryPicker — mode single', () => {
  it('rend un radiogroup avec un radio par catégorie standard (8 codées en dur)', () => {
    renderSingle()
    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
    expect(screen.getAllByRole('radio')).toHaveLength(8)
  })

  it('rend aussi les catégories custom passées en props', () => {
    renderSingle({ customCategories: CUSTOM_CATEGORIES })
    expect(screen.getByTitle('Custom Un')).toBeInTheDocument()
    expect(screen.getAllByRole('radio')).toHaveLength(9)
  })

  it('clic sur une catégorie appelle onChange avec sa clé', () => {
    const { onChange } = renderSingle()
    fireEvent.click(screen.getByTitle('Histoire'))
    expect(onChange).toHaveBeenCalledWith('HISTORY')
  })

  it('re-cliquer sur la catégorie DÉJÀ sélectionnée appelle onChange(\'\') (toggle off conservé)', () => {
    const { onChange } = renderSingle({ value: 'HISTORY' })
    fireEvent.click(screen.getByTitle('Histoire'))
    expect(onChange).toHaveBeenCalledWith('')
  })

  it('aria-checked : « true » sur la catégorie sélectionnée seulement', () => {
    renderSingle({ value: 'SCIENCE' })
    expect(screen.getByTitle('Sciences & Nature')).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByTitle('Histoire')).toHaveAttribute('aria-checked', 'false')
  })

  it('value="" (aucune sélection) : aucun radio coché', () => {
    renderSingle({ value: '' })
    for (const radio of screen.getAllByRole('radio')) {
      expect(radio).toHaveAttribute('aria-checked', 'false')
    }
  })

  it('allowCreate par défaut = false : aucun bouton « + », jamais de formulaire inline possible', () => {
    renderSingle()
    expect(screen.queryByTitle('Créer une catégorie')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// mode="multi"
// ---------------------------------------------------------------------------

describe('CategoryPicker — mode multi', () => {
  it('pas de radiogroup ni de radio : des boutons aria-pressed', () => {
    renderMulti()
    expect(screen.queryByRole('radiogroup')).not.toBeInTheDocument()
    expect(screen.queryAllByRole('radio')).toHaveLength(0)
    expect(screen.getAllByRole('button', { pressed: false })).toHaveLength(8)
  })

  it('clic sur une catégorie appelle onToggle avec sa clé (et jamais onChange)', () => {
    const onChange = vi.fn()
    const { onToggle } = renderMulti({ onChange })
    fireEvent.click(screen.getByRole('button', { name: /Histoire/ }))
    expect(onToggle).toHaveBeenCalledTimes(1)
    expect(onToggle).toHaveBeenCalledWith('HISTORY')
    expect(onChange).not.toHaveBeenCalled()
  })

  it('value fournie sous forme de TABLEAU : les clés présentes sont aria-pressed="true"', () => {
    renderMulti({ value: ['HISTORY', 'SCIENCE'] })
    expect(screen.getByRole('button', { name: /Histoire/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /Sciences/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /Geographie/ })).toHaveAttribute('aria-pressed', 'false')
  })

  it('value fournie sous forme de SET : même rendu qu\'avec un tableau', () => {
    renderMulti({ value: new Set(['HISTORY', 'SCIENCE']) })
    expect(screen.getByRole('button', { name: /Histoire/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /Sciences/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getAllByRole('button', { pressed: true })).toHaveLength(2)
  })

  it('✓ visible sur les chips actives, absent des inactives', () => {
    renderMulti({ value: ['HISTORY', 'SCIENCE'] })
    expect(screen.getAllByText('✓')).toHaveLength(2)
    expect(within(screen.getByRole('button', { name: /Histoire/ })).getByText('✓')).toBeInTheDocument()
    expect(within(screen.getByRole('button', { name: /Geographie/ })).queryByText('✓')).not.toBeInTheDocument()
  })

  it('aucune sélection : aucun ✓', () => {
    renderMulti({ value: [] })
    expect(screen.queryByText('✓')).not.toBeInTheDocument()
  })

  it('renderExtra(key) est rendu DANS la chip de la catégorie (compteur du réservoir, site 14)', () => {
    const renderExtra = vi.fn(key => <span data-testid={`extra-${key}`}>{key === 'HISTORY' ? 42 : 0}</span>)
    renderMulti({ renderExtra })

    const histoire = screen.getByRole('button', { name: /Histoire/ })
    expect(within(histoire).getByTestId('extra-HISTORY')).toHaveTextContent('42')
    expect(renderExtra).toHaveBeenCalledWith('HISTORY')
    expect(screen.getAllByTestId(/^extra-/)).toHaveLength(8)
  })
})

// ---------------------------------------------------------------------------
// variant="filter"
// ---------------------------------------------------------------------------

describe('CategoryPicker — variant filter (ex-CategoryFilterBar)', () => {
  it('availableCategories vide : ne rend RIEN (retourne null)', () => {
    const { container } = renderFilter({ availableCategories: [] })
    expect(container.firstChild).toBeNull()
  })

  it('rend une pastille par catégorie disponible, et seulement celles-là', () => {
    renderFilter({ availableCategories: ['HISTORY', 'SCIENCE', 'SPORTS'] })
    expect(screen.getAllByRole('button')).toHaveLength(3)
    expect(screen.queryByTitle('Geographie')).not.toBeInTheDocument()
  })

  it('l\'ordre de availableCategories est conservé', () => {
    renderFilter({ availableCategories: ['SCIENCE', 'HISTORY', 'GEOGRAPHY'] })
    const titles = screen.getAllByRole('button').map(b => b.getAttribute('title'))
    expect(titles).toEqual(['Sciences & Nature', 'Histoire', 'Geographie'])
  })

  it('sans availableCategories : toutes les catégories sont proposées', () => {
    render(<CategoryPicker variant="filter" mode="multi" value={new Set()} onToggle={vi.fn()} onClear={vi.fn()} />)
    expect(screen.getAllByRole('button')).toHaveLength(STANDARD_KEYS.length)
  })

  it('title = libellé de la catégorie, y compris pour une catégorie custom valide', () => {
    renderFilter({ availableCategories: ['HISTORY', 'CUSTOM_1'], customCategories: CUSTOM_CATEGORIES })
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
    expect(screen.getByTitle('Custom Un')).toBeInTheDocument()
  })

  it('showLabel=true : le libellé texte est affiché à côté de l\'icône', () => {
    renderFilter({ availableCategories: ['HISTORY'], showLabel: true })
    expect(screen.getByText('Histoire')).toBeInTheDocument()
  })

  it('showLabel=false : libellé masqué, la pastille (icône) reste présente avec son title', () => {
    renderFilter({ availableCategories: ['HISTORY'], showLabel: false })
    expect(screen.queryByText('Histoire')).not.toBeInTheDocument()
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
  })

  it('clic sur une pastille appelle onToggle avec sa clé', () => {
    const { onToggle } = renderFilter()
    fireEvent.click(screen.getByTitle('Histoire'))
    expect(onToggle).toHaveBeenCalledWith('HISTORY')
  })

  it('aria-pressed reflète le Set de sélection (useCategoryFilter().selectedCategories)', () => {
    renderFilter({ value: new Set(['HISTORY']) })
    expect(screen.getByTitle('Histoire')).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByTitle('Sciences & Nature')).toHaveAttribute('aria-pressed', 'false')
  })

  it('aucun filtre actif : pas de bouton ×', () => {
    renderFilter({ value: new Set() })
    expect(screen.queryByTitle('Réinitialiser les filtres')).not.toBeInTheDocument()
  })

  it('au moins un filtre actif : le bouton × apparaît et appelle onClear', () => {
    const { onClear } = renderFilter({ value: new Set(['HISTORY']) })
    fireEvent.click(screen.getByTitle('Réinitialiser les filtres'))
    expect(onClear).toHaveBeenCalledTimes(1)
  })

  it('le « + » de création n\'existe pas en filter, même avec allowCreate (réservé au mode single)', () => {
    renderFilter({ allowCreate: true })
    expect(screen.queryByTitle('Créer une catégorie')).not.toBeInTheDocument()
  })
})
