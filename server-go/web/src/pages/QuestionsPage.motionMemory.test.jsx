import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// QuestionsPage — sous-éditeur MEMORY d'une carte MEMOTION (#187, v7.1.0).
// Même conventions de mock que QuestionsPage.ardoise.test.jsx.
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn((questions) => ({
    selectedCategories: new Set(),
    availableCategories: [],
    filteredQuestions: questions || [],
    toggleCategoryFilter: vi.fn(),
    clearCategoryFilters: vi.fn(),
  })),
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

vi.mock('../components/CategoryBalance', () => ({
  default: () => null,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div data-testid={`qcard-${question.ID}`} onClick={() => onClick && onClick(question)} />
  ),
  CATEGORIES: {
    CULTURE: { label: 'Culture', icon: '🎭', color: '#8b5cf6' },
  },
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'

const makeQPageMock = (overrides = {}) => ({
  questions: overrides.questions ?? {},
  fsInfo: { used: 0, total: 100 },
  deleteQuestion: vi.fn(),
  sendMessage: vi.fn(),
  gameState: {
    phase: 'STOPPED',
    question: null,
    ...overrides.gameState,
  },
  newGame: vi.fn(),
  ...overrides,
})

// Sélectionne MEMOTION puis retourne le conteneur de la PREMIÈRE carte
// (déjà présente par défaut, `formData.motionCards` initial à 2 cartes).
const renderAndSelectMemotion = () => {
  const result = render(<QuestionsPage />)
  const memotionBtn = screen.getByText(/memotion/i).closest('button')
  fireEvent.click(memotionBtn)
  const firstCard = result.container.querySelectorAll('.memotion-card-item')[0]
  return { ...result, firstCard }
}

// Bascule le type de la première carte MEMOTION vers MEMORY, via son
// sélecteur de type propre (.memotion-card-type-selector), PAS le sélecteur
// de type de question en tête de formulaire (qui porte aussi un bouton
// "memory").
const selectCardTypeMemory = (firstCard) => {
  const memoryBtn = Array.from(firstCard.querySelectorAll('.memotion-card-type-selector .type-btn'))
    .find(btn => btn.classList.contains('memory'))
  fireEvent.click(memoryBtn)
  return memoryBtn
}

describe('QuestionsPage — sélecteur de type de carte MEMOTION inclut MEMORY (#187)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
  })

  it('le sélecteur de type d\'une carte MEMOTION propose SPEEDY, QCM ET MEMORY (nestable: true)', () => {
    const { firstCard } = renderAndSelectMemotion()
    const typeButtons = firstCard.querySelectorAll('.memotion-card-type-selector .type-btn')
    const classes = Array.from(typeButtons).map(b => b.className)
    expect(classes.some(c => c.includes('speedy') || (!c.includes('qcm') && !c.includes('memory') && !c.includes('ardoise')))).toBe(true)
    expect(classes.some(c => c.includes('qcm'))).toBe(true)
    expect(classes.some(c => c.includes('memory'))).toBe(true)
    // ARDOISE reste non nestable (#186 fermée « not planned »)
    expect(classes.some(c => c.includes('ardoise'))).toBe(false)
  })
})

describe('QuestionsPage — sous-éditeur MEMORY d\'une carte MEMOTION (#187)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
  })

  it('sélectionner MEMORY pour une carte affiche la grille de paires (2 paires par défaut)', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    expect(firstCard.querySelectorAll('.memory-pair-item').length).toBe(2)
  })

  it('masque l\'upload "Image question" pour une carte MEMORY (MediaSlots §7 : recto + N paires, pas de slot question)', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    expect(firstCard.querySelector('.memotion-face-verso')).not.toBeNull() // texte question reste
    expect(Array.from(firstCard.querySelectorAll('label')).some(l => l.textContent.includes('Image question'))).toBe(false)
  })

  it('n\'affiche pas la grille QCM (QcmAnswersEditor) pour une carte MEMORY', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    expect(firstCard.querySelector('.qcm-answers-section')).toBeNull()
  })

  it('affiche un message "pas de face REVEAL" spécifique MEMORY (grille = la carte)', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    expect(firstCard.querySelector('.memotion-card-no-reveal-hint').textContent).toMatch(/grille de paires/i)
  })

  it('"+ Ajouter une paire" ajoute une 3e paire à LA CARTE (pas à l\'hôte question)', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    const addBtn = Array.from(firstCard.querySelectorAll('button')).find(b => b.textContent.includes('Ajouter une paire'))
    fireEvent.click(addBtn)
    expect(firstCard.querySelectorAll('.memory-pair-item').length).toBe(3)
  })

  it('le bouton de suppression d\'une paire retire bien UNE paire (min 2 conservé)', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    // 2 paires par défaut -> pas de bouton de suppression visible (garde-fou min 2)
    expect(firstCard.querySelectorAll('.memory-pair-item .memory-remove-btn').length).toBe(0)
    const addBtn = Array.from(firstCard.querySelectorAll('button')).find(b => b.textContent.includes('Ajouter une paire'))
    fireEvent.click(addBtn)
    expect(firstCard.querySelectorAll('.memory-pair-item .memory-remove-btn').length).toBe(3)
    fireEvent.click(firstCard.querySelector('.memory-pair-item .memory-remove-btn'))
    expect(firstCard.querySelectorAll('.memory-pair-item').length).toBe(2)
  })

  it('les 3 réglages de points (Points par paire/Pénalité erreur/Bonus completion) sont désactivés', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    const disabledLabels = ['Points par paire', 'Penalite erreur', 'Bonus completion']
    disabledLabels.forEach(labelText => {
      const item = Array.from(firstCard.querySelectorAll('.memory-config-item')).find(el => el.textContent.includes(labelText))
      expect(item).not.toBeUndefined()
      expect(item.querySelector('input').disabled).toBe(true)
    })
  })

  it('les réglages actifs (Temps memorisation, Delai retournement) restent modifiables', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    const activeLabels = ['Delai retournement', 'Temps memorisation', 'Delai reveal']
    activeLabels.forEach(labelText => {
      const item = Array.from(firstCard.querySelectorAll('.memory-config-item')).find(el => el.textContent.includes(labelText))
      expect(item.querySelector('input').disabled).toBe(false)
    })
  })

  it('aucun champ VALUE/barème n\'est exposé (STARS_PRORATA, contrat §6.2 — pas de saisie)', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    expect(screen.getAllByText(/prorata/i).length).toBeGreaterThan(0) // note explicative présente
    expect(firstCard.querySelector('input[placeholder*="VALUE" i]')).toBeNull()
  })

  it('le verrou de type se déclenche dès qu\'une paire porte du texte (contrat §3.2)', () => {
    const { firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)
    const textInput = firstCard.querySelector('.memory-pair-item .memory-card-text-input')
    fireEvent.change(textInput, { target: { value: 'Napoléon' } })
    const lockReason = firstCard.querySelector('.motion-card-lock-reason')
    expect(lockReason).not.toBeNull()
    expect(lockReason.textContent).toMatch(/MEMORY/)
  })
})

describe('QuestionsPage — sérialisation MEMORY_PAIRS d\'une carte MEMOTION (#187)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('motion_cards inclut MEMORY_MODE=SOLO, MEMORY_PAIRS et MEMORY_CONFIG pour une carte MEMORY', async () => {
    const sendMessage = vi.fn().mockResolvedValue(undefined)
    useGame.mockReturnValue(makeQPageMock({ sendMessage }))
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ID: '99' }) })

    const { container, firstCard } = renderAndSelectMemotion()
    selectCardTypeMemory(firstCard)

    // Remplit la 1ère carte (MEMORY) avec un thème + 2 paires complètes
    fireEvent.change(firstCard.querySelector('input[placeholder="Theme / Titre..."]'), { target: { value: 'Paires' } })
    const pairTextInputs = firstCard.querySelectorAll('.memory-pair-item .memory-card-text-input')
    fireEvent.change(pairTextInputs[0], { target: { value: 'Napoléon' } })
    fireEvent.change(pairTextInputs[1], { target: { value: '1804' } })
    fireEvent.change(pairTextInputs[2], { target: { value: 'De Gaulle' } })
    fireEvent.change(pairTextInputs[3], { target: { value: '1958' } })

    // Renseigne aussi le thème de la 2e carte (SPEEDY par défaut) — validation
    // MEMOTION exige au moins 2 cartes avec thème (handleSubmit).
    const secondCard = container.querySelectorAll('.memotion-card-item')[1]
    fireEvent.change(secondCard.querySelector('input[placeholder="Theme / Titre..."]'), { target: { value: 'Autre carte' } })

    fireEvent.change(screen.getByPlaceholderText('Entrez la question...'), { target: { value: 'Manche MEMOTION' } })

    const submitBtn = container.querySelector('.submit-btn')
    fireEvent.click(submitBtn)

    // #187 note : useCategories() (hook réel, non mocké — même choix que
    // QuestionsPage.ardoise.test.jsx) émet son propre fetch('/api/categories')
    // au montage, AVANT la soumission — filtrer sur l'appel '/questions'
    // plutôt que supposer que la soumission est le premier appel.
    expect(global.fetch).toHaveBeenCalled()
    const submitCall = global.fetch.mock.calls.find(call => call[0] === '/questions')
    expect(submitCall).not.toBeUndefined()
    const formData = submitCall[1]?.body
    expect(formData).toBeInstanceOf(FormData)
    const motionCards = JSON.parse(formData.get('motion_cards'))
    const memoryCard = motionCards.find(c => c.TYPE === 'MEMORY')
    expect(memoryCard).not.toBeUndefined()
    expect(memoryCard.MEMORY_MODE).toBe('SOLO')
    expect(memoryCard.MEMORY_PAIRS).toHaveLength(2)
    expect(memoryCard.MEMORY_PAIRS[0].CARD1.TEXT).toBe('Napoléon')
    expect(memoryCard.MEMORY_PAIRS[0].CARD2.TEXT).toBe('1804')
    expect(memoryCard.MEMORY_CONFIG).toMatchObject({
      FLIP_DELAY: 3,
      POINTS_PER_PAIR: 10,
      ERROR_PENALTY: 0,
      COMPLETION_BONUS: 0,
      USE_TIMER: true,
      MEMORIZE_TIME: 5,
      SHOW_DURING_MEMORIZE: true,
      REVEAL_DELAY: 0.5,
    })
  })
})
