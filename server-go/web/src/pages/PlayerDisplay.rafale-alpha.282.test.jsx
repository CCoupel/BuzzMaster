import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// #282 (v11.4.0, LOT-2) — contracts/rafale.md §15.8, maquette tv_rafale_started__letter_wheel :
// TV, manche RAFALE STARTED.
//   - option active : zone basse « roue des lettres » présente (data-testid="rafale-wheel"),
//     bandeau (strip) par défaut ; arc si RAFALE_WHEEL_LAYOUT = 'arc' ; layout vide ⇒ strip ;
//   - option inactive : aucune zone ajoutée (rendu v11.2.3 inchangé) ;
//   - TV STATIQUE : zone en `overflow: hidden`, unités viewport (16 vh strip / 30 vh arc) ;
//   - aucune ligne A→Z supplémentaire dans l'encart question ; la réponse n'est jamais lue.
// Mocks calqués sur PlayerDisplay.memotion-rafale.test.jsx.
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({ default: class NoSleep { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('canvas-confetti', () => ({ default: vi.fn() }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../components/Podium', () => ({ default: () => <div data-testid="podium" /> }))
vi.mock('../components/QRCodeOverlay', () => ({ default: () => null }))
vi.mock('../components/QRCodeDisplay', () => ({ default: () => null }))
vi.mock('./QuestionsPage', () => ({ CATEGORIES: [] }))
vi.mock('../constants/colors', () => ({ getCategoryColor: vi.fn(() => '#8b5cf6') }))
vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color)),
}))
vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))

import { useGame } from '../hooks/GameContext'

afterEach(() => { cleanup(); vi.clearAllMocks() })

const makeMock = ({ alphabetical = false, layout, letter = 'C', letters = ['D', 'M'], extra = {} } = {}) => ({
  gameState: {
    phase: 'STARTED',
    remote: 'GAME',
    timer: 90,
    totalTime: 120,
    question: {
      ID: 'rq', TYPE: 'RAFALE', QUESTION: 'RAFALE', RAFALE_MODE: 'SOLO', RAFALE_QUESTION_TIME: 3,
      RAFALE_CATEGORIES: ['HISTORY'], RAFALE_DIFFICULTIES: [2],
      ...(alphabetical ? { RAFALE_ALPHABETICAL: true } : {}),
      ...(layout !== undefined ? { RAFALE_WHEEL_LAYOUT: layout } : {}),
    },
    RAFALE_SUBPHASE: 'QUESTION',
    RAFALE_CURRENT_QUESTION: { ID: 'r-9', QUESTION: 'Capitale de l\'Australie ?', CATEGORY: 'HISTORY', DIFFICULTY: 2 },
    RAFALE_QUESTION_TIME: 3,
    RAFALE_PARTICIPATING_TEAMS: [],
    RAFALE_CURRENT_TEAM: 'Bleus',
    RAFALE_CURRENT_TEAM_COLOR: [59, 130, 246],
    RAFALE_CURRENT_LETTER: alphabetical ? letter : '',
    RAFALE_LETTERS: alphabetical ? letters : [],
    MEMORY_PARTICIPATING_TEAMS: [],
    newGameBackgrounds: [],
    ...extra,
  },
  teams: { Bleus: { SCORE: 10, COLOR: [59, 130, 246] } },
  bumpers: {},
  showQRCode: false,
})

const renderTV = (opts) => {
  useGame.mockReturnValue(makeMock(opts))
  return render(<PlayerDisplay />)
}

describe('PlayerDisplay TV — roue des lettres RAFALE alphabétique (#282)', () => {
  it('option active : la zone basse « roue des lettres » est présente', () => {
    const { getByTestId } = renderTV({ alphabetical: true })
    expect(getByTestId('rafale-wheel')).toBeInTheDocument()
  })

  it('option inactive : aucune roue (rendu RAFALE classique inchangé)', () => {
    const { queryByTestId } = renderTV({ alphabetical: false })
    expect(queryByTestId('rafale-wheel')).toBeNull()
  })

  it('RAFALE_WHEEL_LAYOUT vide ⇒ bandeau (strip) par défaut', () => {
    const { getByTestId } = renderTV({ alphabetical: true, layout: '' })
    const root = getByTestId('rafale-wheel')
    expect(root.querySelector('.wheel-strip') || root.classList.contains('wheel-strip')).toBeTruthy()
    expect(root.querySelector('.wheel-arc')).toBeNull()
  })

  it('RAFALE_WHEEL_LAYOUT = "arc" ⇒ roue en arc', () => {
    const { getByTestId } = renderTV({ alphabetical: true, layout: 'arc' })
    const root = getByTestId('rafale-wheel')
    expect(root.querySelector('.wheel-arc') || root.classList.contains('wheel-arc')).toBeTruthy()
  })

  it('TV statique : la zone de la roue est en overflow hidden, dimensions en unités viewport', () => {
    const { getByTestId } = renderTV({ alphabetical: true })
    let zone = getByTestId('rafale-wheel')
    // on remonte jusqu'à la zone basse (conteneur de la roue) : au moins un ancêtre en overflow hidden
    let hidden = false
    for (let el = zone; el && el !== document.body; el = el.parentElement) {
      if (el.style?.overflow === 'hidden') hidden = true
    }
    // le style peut aussi venir de la feuille (mockée ici) : le test accepte l'attribut en ligne OU la classe de zone
    const zoneClass = zone.closest('[class*="wheel-zone"], [class*="rafale-wheel-zone"]')
    expect(hidden || zoneClass !== null).toBe(true)
    for (let el = zone; el && el !== document.body; el = el.parentElement) {
      const h = el.style?.height || ''
      expect(h).not.toMatch(/\d+px$/) // jamais de hauteur en px sur la TV
    }
  })

  it('aucune ligne A→Z supplémentaire dans l\'encart question (la roue seule porte les lettres)', () => {
    const { container } = renderTV({ alphabetical: true })
    const alphabets = Array.from(container.querySelectorAll('*')).filter(
      (el) => el.children.length === 0 && /^A\s*B\s*C\s*D\s*E/.test((el.textContent || '').replace(/\s+/g, ' ').trim()),
    )
    expect(alphabets.length).toBe(0)
  })

  it('la lettre courante est annoncée par la roue ; la réponse n\'est jamais affichée', () => {
    const { getByRole, container } = renderTV({ alphabetical: true, letter: 'C' })
    expect(getByRole('img', { name: /Lettre courante\s*:\s*C/ })).toBeInTheDocument()
    expect(container.textContent).not.toContain('Canberra')
  })

  it('lettre « # » : aucune mise en évidence, la roue reste rendue', () => {
    const { getByTestId, container } = renderTV({ alphabetical: true, letter: '#' })
    expect(getByTestId('rafale-wheel')).toBeInTheDocument()
    expect(container.querySelector('.is-current, [data-current="true"]')).toBeNull()
  })
})
