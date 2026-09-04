/**
 * Tests — BackstagePage (onglet Quiz) : chips multi-sélection
 * Publics/Difficultés (#137 Batch 2b T2.2).
 *
 * Ré-adressé depuis QuestionsPage.quizChips.test.jsx (#215, extraction de la
 * section méta-quiz vers BackstagePage.jsx/QuizMetaForm.jsx — l'onglet Quiz
 * est actif par défaut). Mêmes assertions qu'avant l'extraction : sélection,
 * désélection, et le fait que la sélection locale n'est envoyée au serveur
 * qu'au clic sur "Enregistrer" — jamais au clic sur un chip lui-même
 * (contract §5bis, T2.5 : la génération IA doit utiliser gameState.quiz*,
 * pas un état de formulaire qui s'auto-enverrait à chaque interaction).
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import BackstagePage from './BackstagePage'

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, type, ...rest }) => (
    <button onClick={onClick} disabled={disabled} type={type || 'button'} {...rest}>
      {children}
    </button>
  ),
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
  CardHeader: ({ children }) => <div className="card-header">{children}</div>,
  CardBody: ({ children }) => <div className="card-body">{children}</div>,
}))

vi.mock('./BackstagePage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const makeBackstageMock = (overrides = {}) => ({
  sendMessage: vi.fn(),
  gameState: {
    phase: 'STOPPED',
    quizPopulations: [],
    quizDifficulties: [],
    quizHiddenFields: [],
    ...overrides.gameState,
  },
  newGame: vi.fn(),
  ...overrides,
})

describe('BackstagePage — onglet Quiz : chips multi-sélection Publics/Difficultés (#137 Batch 2b T2.2)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    global.fetch = vi.fn()
  })

  it('un public non sélectionné devient actif au clic (sélection)', () => {
    useGame.mockReturnValue(makeBackstageMock())
    render(<BackstagePage />)

    const chip = screen.getByRole('button', { name: 'Ado (13-17 ans)' })
    expect(chip.className).not.toMatch(/active/)

    fireEvent.click(chip)

    expect(chip.className).toMatch(/active/)
  })

  it('un public déjà sélectionné (via gameState) redevient inactif au clic (désélection)', () => {
    useGame.mockReturnValue(makeBackstageMock({ gameState: { quizPopulations: ['Ado (13-17 ans)'] } }))
    render(<BackstagePage />)

    const chip = screen.getByRole('button', { name: 'Ado (13-17 ans)' })
    expect(chip.className).toMatch(/active/)

    fireEvent.click(chip)

    expect(chip.className).not.toMatch(/active/)
  })

  it('une difficulté suit la même logique de bascule sélection/désélection', () => {
    useGame.mockReturnValue(makeBackstageMock({ gameState: { quizDifficulties: ['Moyen'] } }))
    render(<BackstagePage />)

    const chip = screen.getByRole('button', { name: 'Moyen' })
    expect(chip.className).toMatch(/active/)

    fireEvent.click(chip)
    expect(chip.className).not.toMatch(/active/)

    fireEvent.click(chip)
    expect(chip.className).toMatch(/active/)
  })

  it('cliquer un chip ne déclenche AUCUN envoi au serveur — seule "Enregistrer" le fait', () => {
    const sendMessage = vi.fn()
    useGame.mockReturnValue(makeBackstageMock({ sendMessage }))
    render(<BackstagePage />)

    fireEvent.click(screen.getByRole('button', { name: 'Ado (13-17 ans)' }))
    fireEvent.click(screen.getByRole('button', { name: 'Moyen' }))

    expect(sendMessage).not.toHaveBeenCalledWith('UPDATE_QUIZ_META', expect.anything())
  })

  it('"Enregistrer" envoie la sélection locale de chips (POPULATIONS/DIFFICULTIES), pas seulement ce qui était déjà dans gameState', () => {
    const sendMessage = vi.fn()
    useGame.mockReturnValue(makeBackstageMock({ sendMessage, gameState: { quizPopulations: [], quizDifficulties: [] } }))
    render(<BackstagePage />)

    // Sélectionne 2 publics et 1 difficulté avant tout enregistrement —
    // persistance dans l'état local du formulaire, pas encore dans gameState.
    fireEvent.click(screen.getByRole('button', { name: 'Ado (13-17 ans)' }))
    fireEvent.click(screen.getByRole('button', { name: 'Adulte (18-64 ans)' }))
    fireEvent.click(screen.getByRole('button', { name: 'Moyen' }))

    // Un seul bouton "Enregistrer" sur l'onglet Quiz (l'onglet Entracte,
    // qui en a un autre, n'est pas rendu) : plus besoin de scoper à
    // .quiz-meta-form comme avant l'extraction (#215).
    fireEvent.click(screen.getByText('Enregistrer'))

    expect(sendMessage).toHaveBeenCalledWith(
      'UPDATE_QUIZ_META',
      expect.objectContaining({
        POPULATIONS: expect.arrayContaining(['Ado (13-17 ans)', 'Adulte (18-64 ans)']),
        DIFFICULTIES: ['Moyen'],
      })
    )
    const [, payload] = sendMessage.mock.calls.find(c => c[0] === 'UPDATE_QUIZ_META')
    expect(payload.POPULATIONS).toHaveLength(2)
  })
})
