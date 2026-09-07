/**
 * Tests — QuizMetaForm : chips multi-sélection Publics/Difficultés (#137
 * Batch 2b T2.2).
 *
 * RÉ-ADRESSÉ depuis pages/QuestionsPage.quizChips.test.jsx (#215, milestone
 * v9.0.0) : le formulaire Quiz (nom/thème/publics/difficultés/langue/
 * objectif/notes + NOUVELLE PARTIE) a été extrait de QuestionsPage.jsx vers
 * ce composant autonome (props {gameState, sendMessage, onNewGame}, aucun
 * hook interne) — QuestionsPage ne le rend plus du tout, il vit désormais
 * dans l'onglet "Quiz" de BackstagePage.jsx. Assertions PRÉSERVÉES à
 * l'identique (sélection/désélection de chip, envoi différé au clic
 * "Enregistrer" seulement, payload POPULATIONS/DIFFICULTIES) — seul le
 * point de montage change : QuizMetaForm directement, plus besoin des mocks
 * useCategories/useCategoryFilter/QuestionCard (jamais utilisés par ce
 * composant, contrairement à la page complète d'origine).
 *
 * Contexte historique (_work/handoff/task-test-writer-review-batch2b-20260806-165427.md) :
 * les chips (motif repris des catégories d'AIGenerateModal.jsx) remplacent
 * les <select> à valeur unique v6.0.0 ; ce fichier verrouille : sélection,
 * désélection, et le fait que la sélection locale n'est envoyée au serveur
 * qu'au clic sur "Enregistrer" — jamais au clic sur un chip lui-même
 * (contract §5bis, T2.5 : la génération IA doit utiliser gameState.quiz*,
 * pas un état de formulaire qui s'auto-enverrait à chaque interaction).
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import QuizMetaForm from './QuizMetaForm'

vi.mock('./Button', () => ({
  default: ({ children, onClick, disabled, type, ...rest }) => (
    <button onClick={onClick} disabled={disabled} type={type || 'button'} {...rest}>
      {children}
    </button>
  ),
}))

vi.mock('./Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
  CardHeader: ({ children }) => <div className="card-header">{children}</div>,
  CardBody: ({ children }) => <div className="card-body">{children}</div>,
}))

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const makeGameState = (overrides = {}) => ({
  quizPopulations: [],
  quizDifficulties: [],
  quizHiddenFields: [],
  ...overrides,
})

describe('QuizMetaForm — chips multi-sélection Publics/Difficultés (#137 Batch 2b T2.2)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('un public non sélectionné devient actif au clic (sélection)', () => {
    render(<QuizMetaForm gameState={makeGameState()} sendMessage={vi.fn()} />)

    const chip = screen.getByRole('button', { name: 'Ado (13-17 ans)' })
    expect(chip.className).not.toMatch(/active/)

    fireEvent.click(chip)

    expect(chip.className).toMatch(/active/)
  })

  it('un public déjà sélectionné (via gameState) redevient inactif au clic (désélection)', () => {
    render(<QuizMetaForm gameState={makeGameState({ quizPopulations: ['Ado (13-17 ans)'] })} sendMessage={vi.fn()} />)

    const chip = screen.getByRole('button', { name: 'Ado (13-17 ans)' })
    expect(chip.className).toMatch(/active/)

    fireEvent.click(chip)

    expect(chip.className).not.toMatch(/active/)
  })

  it('une difficulté suit la même logique de bascule sélection/désélection', () => {
    render(<QuizMetaForm gameState={makeGameState({ quizDifficulties: ['Moyen'] })} sendMessage={vi.fn()} />)

    const chip = screen.getByRole('button', { name: 'Moyen' })
    expect(chip.className).toMatch(/active/)

    fireEvent.click(chip)
    expect(chip.className).not.toMatch(/active/)

    fireEvent.click(chip)
    expect(chip.className).toMatch(/active/)
  })

  it('cliquer un chip ne déclenche AUCUN envoi au serveur — seule "Enregistrer" le fait', () => {
    const sendMessage = vi.fn()
    render(<QuizMetaForm gameState={makeGameState()} sendMessage={sendMessage} />)

    fireEvent.click(screen.getByRole('button', { name: 'Ado (13-17 ans)' }))
    fireEvent.click(screen.getByRole('button', { name: 'Moyen' }))

    expect(sendMessage).not.toHaveBeenCalledWith('UPDATE_QUIZ_META', expect.anything())
  })

  it('"Enregistrer" envoie la sélection locale de chips (POPULATIONS/DIFFICULTIES), pas seulement ce qui était déjà dans gameState', () => {
    const sendMessage = vi.fn()
    const { container } = render(<QuizMetaForm gameState={makeGameState()} sendMessage={sendMessage} />)

    // Sélectionne 2 publics et 1 difficulté avant tout enregistrement —
    // persistance dans l'état local du formulaire, pas encore dans gameState.
    fireEvent.click(screen.getByRole('button', { name: 'Ado (13-17 ans)' }))
    fireEvent.click(screen.getByRole('button', { name: 'Adulte (18-64 ans)' }))
    fireEvent.click(screen.getByRole('button', { name: 'Moyen' }))

    const quizMetaForm = container.querySelector('.quiz-meta-form')
    fireEvent.click(within(quizMetaForm).getByText('Enregistrer'))

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

  it('NOUVELLE PARTIE : bouton rendu uniquement quand onNewGame est fourni, appelle le callback au clic (#215)', () => {
    const onNewGame = vi.fn()
    const { rerender } = render(<QuizMetaForm gameState={makeGameState()} sendMessage={vi.fn()} />)
    expect(screen.queryByText('NOUVELLE PARTIE')).toBeNull()

    rerender(<QuizMetaForm gameState={makeGameState()} sendMessage={vi.fn()} onNewGame={onNewGame} />)
    fireEvent.click(screen.getByText('NOUVELLE PARTIE'))
    expect(onNewGame).toHaveBeenCalledTimes(1)
  })
})
