/**
 * Tests for GamePage — panneau de pré-lancement RAFALE (v8.0.0, #16/#107).
 *
 * Historique : catégorie unique depuis le bugfix 2026-08-29 (contrat
 * rafale.md §3.3, SHA 8f8ff92d) — le panneau affichait un compteur
 * "N catégorie(s)" (RAFALE_CATEGORIES, multi, retiré), remplacé par un seul
 * CategoryBadge.
 *
 * ⚠️ [CHANGED] #216 (milestone v9.0.0, Lot 2, GamePage.jsx:997-1031) —
 * réouverture ASSUMÉE : le panneau affiche désormais un chip par catégorie
 * ET un chip par difficulté (RAFALE_CATEGORIES/RAFALE_DIFFICULTIES),
 * exactement le même principe "chips multiples" que QuestionCard.jsx
 * (contracts/rafale.md §3.3, réouverture documentée du bugfix). Les 2 tests
 * qui affirmaient explicitement le badge/chip UNIQUE ("CATEGORY définie :
 * affiche un SEUL CategoryBadge", "transmet la CATEGORY unique à
 * RafalePoolAlert (pas un tableau)") sont réécrits ci-dessous — pas
 * supprimés — pour vérifier le nouveau contrat multi.
 *
 * Mocks calqués sur GamePage.categories.test.jsx (même scaffold) — la
 * différence : useCategories() est mocké ici (RafalePoolAlert et le badge
 * de catégorie du panneau RAFALE en dépendent tous les deux via
 * customCategories), RafalePoolAlert lui-même est stubbé (déjà couvert
 * isolément par components/RafalePoolAlert.rafale.test.jsx — inutile de
 * dupliquer ses 3 états d'alerte/son format de requête ici) mais capture
 * désormais les props `categories`/`difficulties` reçues pour vérifier le
 * câblage GamePage → RafalePoolAlert.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(() => ({ categories: [], loading: false, error: null, refetch: vi.fn() })),
}))

vi.mock('../components/Timer', () => ({
  default: () => <div data-testid="timer" />,
}))

vi.mock('../components/QuestionPreview', () => ({
  default: () => <div data-testid="question-preview" />,
}))

vi.mock('../components/TeamCard', () => ({
  default: ({ name }) => <div data-testid={`team-card-${name}`} />,
  OtaAllModal: () => null,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div data-testid={`question-card-${question.ID}`} onClick={() => onClick && onClick(question)} />
  ),
  CATEGORIES: {
    GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
    HISTORY: { label: 'Histoire', icon: '📜', color: '#eab308' },
  },
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, variant, size, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

// RafalePoolAlert : déjà couvert isolément (components/RafalePoolAlert.
// rafale.test.jsx) — stub léger ici, ce fichier teste UNIQUEMENT le
// panneau GamePage lui-même (chips catégories/difficultés multiples,
// #216). Les props categories/difficulties reçues sont exposées en
// attributs data-* pour vérifier le câblage sans dupliquer le
// comportement interne de RafalePoolAlert.
vi.mock('../components/RafalePoolAlert', () => ({
  default: ({ categories, difficulties }) => (
    <div
      data-testid="rafale-pool-alert-stub"
      data-categories={JSON.stringify(categories)}
      data-difficulties={JSON.stringify(difficulties)}
    />
  ),
}))

vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import GamePage from './GamePage'

// ⚠️ `...overrides` spreadé EN PREMIER : sinon un `overrides.gameState`
// PARTIEL (ex. { question: {...} } seul, sans `phase`) écraserait
// entièrement la clé `gameState` fusionnée juste en dessous, faisant
// disparaître `phase: 'STOPPED'` silencieusement (bug réel identifié et
// corrigé dans GamePage.rafaleStartGate.test.jsx — inoffensif ICI
// puisqu'aucun test de ce fichier ne dépend de `phase`/`isPlaying`, mais
// corrigé par cohérence pour ne pas re-piéger un futur test ajouté ici).
const makeGameMock = (overrides = {}) => ({
  ...overrides,
  gameState: {
    phase: 'STOPPED',
    question: { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', RAFALE_CATEGORIES: ['HISTORY'], RAFALE_DIFFICULTIES: [2], RAFALE_MODE: 'SOLO', RAFALE_QUESTION_TIME: 3 },
    remote: 'GAME',
    timer: 0,
    totalTime: 120,
    MEMORY_PARTICIPATING_TEAMS: [],
    ...overrides.gameState,
  },
  teams: overrides.teams ?? {},
  bumpers: overrides.bumpers ?? {},
  questions: overrides.questions ?? {
    '1': { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', ORDER: 1, CATEGORY: 'HISTORY' },
  },
  startGame: vi.fn(),
  stopGame: vi.fn(),
  pauseGame: vi.fn(),
  continueGame: vi.fn(),
  revealAnswer: vi.fn(),
  selectQuestion: vi.fn(),
  setRemoteDisplay: vi.fn(),
  setBumperPoints: vi.fn(),
  setTeamPoints: vi.fn(),
  forceReady: vi.fn(),
  simulateButton: vi.fn(),
  simulatePong: vi.fn(),
  sendMessage: vi.fn(),
  rafaleSetTeams: vi.fn(),
  rafaleAnswer: overrides.rafaleAnswer ?? null,
  rafaleValidate: overrides.rafaleValidate ?? vi.fn(),
  rafaleInvalidate: overrides.rafaleInvalidate ?? vi.fn(),
})

describe('GamePage — panneau de pré-lancement RAFALE : catégorie unique (bugfix 2026-08-29, SHA 8f8ff92d)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('question RAFALE sélectionnée, phase STOPPED : le panneau de configuration de manche est affiché', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-panel')).not.toBeNull()
  })

  it('RAFALE_CATEGORIES à 1 élément : affiche un chip catégorie (#216, cas mono via la liste)', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<GamePage />)

    const panel = container.querySelector('.rafale-admin-panel')
    expect(panel.querySelectorAll('.category-badge').length).toBe(1)
    expect(screen.getByText('Histoire')).toBeInTheDocument()
  })

  it('RAFALE_CATEGORIES à PLUSIEURS éléments : un chip par catégorie (#216)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', RAFALE_CATEGORIES: ['HISTORY', 'GEOGRAPHY'], RAFALE_DIFFICULTIES: [2] } },
    }))
    const { container } = render(<GamePage />)

    const panel = container.querySelector('.rafale-admin-panel')
    expect(panel.querySelectorAll('.category-badge').length).toBe(2)
    expect(screen.getByText('Histoire')).toBeInTheDocument()
    expect(screen.getByText('Geographie')).toBeInTheDocument()
  })

  it('RAFALE_CATEGORIES vide/absent : aucun badge de catégorie affiché', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', RAFALE_CATEGORIES: [], RAFALE_DIFFICULTIES: [1] } },
    }))
    const { container } = render(<GamePage />)

    const panel = container.querySelector('.rafale-admin-panel')
    expect(panel.querySelectorAll('.category-badge').length).toBe(0)
  })

  it('RAFALE_DIFFICULTIES à PLUSIEURS éléments : un chip étoilé par difficulté (#216)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', RAFALE_CATEGORIES: ['HISTORY'], RAFALE_DIFFICULTIES: [1, 3] } },
    }))
    const { container } = render(<GamePage />)

    const panel = container.querySelector('.rafale-admin-panel')
    const chips = Array.from(panel.querySelectorAll('.rafale-admin-chip')).map(el => el.textContent)
    expect(chips.some(c => c.trim() === '★')).toBe(true)   // difficulté 1
    expect(chips.some(c => c.trim() === '★★★')).toBe(true) // difficulté 3
  })

  it('affiche aussi le mode et le temps par question (autres chips du panneau, non-régression)', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<GamePage />)

    const panel = container.querySelector('.rafale-admin-panel')
    const chips = Array.from(panel.querySelectorAll('.rafale-admin-chip')).map(el => el.textContent)
    expect(chips.some(c => c === 'SOLO')).toBe(true)
    expect(chips.some(c => c.includes('3s/question'))).toBe(true)
  })

  it('transmet RAFALE_CATEGORIES/RAFALE_DIFFICULTIES (tableaux) à RafalePoolAlert — pas des scalaires (#216)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', RAFALE_CATEGORIES: ['HISTORY', 'GEOGRAPHY'], RAFALE_DIFFICULTIES: [1, 2] } },
    }))
    const { container } = render(<GamePage />)

    const stub = container.querySelector('[data-testid="rafale-pool-alert-stub"]')
    expect(JSON.parse(stub.dataset.categories)).toEqual(['HISTORY', 'GEOGRAPHY'])
    expect(JSON.parse(stub.dataset.difficulties)).toEqual([1, 2])
  })

  it('question non-RAFALE : le panneau RAFALE n\'est jamais affiché (non-régression)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'SPEEDY', STATUS: 'STOPPED', CATEGORY: 'HISTORY' } },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-panel')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// .rafale-admin-live (maquette rafale-v8.html §9.3, SHA 01eb5ce9) — /admin
// affichait RIEN pendant la manche (STARTED) avant ce lot ; le panneau de
// pré-lancement se cache dès isPlaying (voir describe précédent). Nouveau
// bloc, visible UNIQUEMENT en isPlaying, montrant le même encart coloré
// équipe + question + réponse que /anim (mêmes classes .rafale-anim-qcard*
// — pas de duplication de test de rendu détaillé, juste la présence et le
// bon contenu ici, la structure de l'encart lui-même étant déjà exhaustive
// dans AnimPage.rafale.test.jsx).
// ---------------------------------------------------------------------------

describe('GamePage — .rafale-admin-live : question+réponse pendant la manche (bugfix SHA 01eb5ce9)', () => {
  it('phase STOPPED (avant lancement) : .rafale-admin-live absent — c\'est le panneau de pré-lancement qui s\'affiche', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED' } }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-live')).toBeNull()
    expect(container.querySelector('.rafale-admin-panel')).not.toBeNull()
  })

  it('phase STARTED (manche en cours) : .rafale-admin-live visible, panneau de pré-lancement caché', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
      },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-live')).not.toBeNull()
    expect(container.querySelector('.rafale-admin-panel')).toBeNull()
  })

  it('affiche la question courante (RAFALE_CURRENT_QUESTION)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
      },
    }))
    render(<GamePage />)

    expect(screen.getByText('Capitale de l\'Italie ?')).toBeInTheDocument()
  })

  it('rafaleAnswer.ID correspond à la question courante : affiche la réponse (RAFALE_ANSWER déjà diffusé à admin, contrat §2.3 — simple choix d\'affichage)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
      },
      rafaleAnswer: { ID: 'r-042', ANSWER: 'Rome' },
    }))
    render(<GamePage />)

    expect(screen.getByText('Rome')).toBeInTheDocument()
  })

  it('rafaleAnswer.ID NE correspond PAS à la question courante (pas encore reçu pour cette question) : aucune réponse affichée', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
      },
      rafaleAnswer: { ID: 'r-999', ANSWER: 'Berlin' },
    }))
    render(<GamePage />)

    expect(screen.queryByText('Berlin')).not.toBeInTheDocument()
  })

  it('équipe active définie (mode multi) : son nom apparaît dans l\'encart', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Q?', CATEGORY: 'HISTORY', DIFFICULTY: 1 },
        RAFALE_CURRENT_TEAM: 'Équipe A',
        RAFALE_CURRENT_TEAM_COLOR: [99, 102, 241],
      },
    }))
    const { container } = render(<GamePage />)

    const chip = container.querySelector('.rafale-anim-qcard-team')
    expect(chip).not.toBeNull()
    expect(chip.textContent).toContain('Équipe A')
  })

  it('question non-RAFALE, phase STARTED : .rafale-admin-live n\'est jamais affiché (non-régression)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY', STATUS: 'STARTED' } },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-live')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Boutons RÉPONSE VALIDE/INVALIDE sur /admin (contrat §5.1, SHA fe4d5bcc) —
// réutilise AnimRafaleActions TEL QUEL (composant déjà testé isolément,
// components/AnimRafaleActions.rafale.test.jsx) : ce bloc vérifie
// uniquement le CÂBLAGE dans GamePage (onValidate/onInvalidate ->
// rafaleValidate/rafaleInvalidate, disabled -> RAFALE_SUBPHASE), pas le
// rendu détaillé du composant lui-même. AnimRafaleActions rendu RÉEL (pas
// mocké) — léger, et c'est la façon la plus directe de prouver que le
// câblage fonctionne réellement de bout en bout (le bug découvert sur
// AnimPage.jsx au cycle précédent — props jamais transmises — est
// exactement le type de défaut qu'un mock de complaisance ne peut pas
// détecter).
// ---------------------------------------------------------------------------

describe('GamePage — RAFALE : boutons RÉPONSE VALIDE/INVALIDE sur /admin (SHA fe4d5bcc)', () => {
  it('phase STARTED, RAFALE_SUBPHASE=QUESTION : les 2 boutons sont rendus et cliquables', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', RAFALE_SUBPHASE: 'QUESTION' },
    }))
    const { container } = render(<GamePage />)

    const buttons = container.querySelectorAll('.anim-rafale-action-btn')
    expect(buttons).toHaveLength(2)
    for (const btn of buttons) expect(btn.disabled).toBe(false)
  })

  it('clic sur RÉPONSE VALIDE appelle rafaleValidate()', () => {
    const rafaleValidate = vi.fn()
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', RAFALE_SUBPHASE: 'QUESTION' },
      rafaleValidate,
    }))
    const { container } = render(<GamePage />)

    fireEvent.click(container.querySelectorAll('.anim-rafale-action-btn')[0])
    expect(rafaleValidate).toHaveBeenCalledTimes(1)
  })

  it('clic sur RÉPONSE INVALIDE appelle rafaleInvalidate()', () => {
    const rafaleInvalidate = vi.fn()
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', RAFALE_SUBPHASE: 'QUESTION' },
      rafaleInvalidate,
    }))
    const { container } = render(<GamePage />)

    fireEvent.click(container.querySelectorAll('.anim-rafale-action-btn')[1])
    expect(rafaleInvalidate).toHaveBeenCalledTimes(1)
  })

  it('RAFALE_SUBPHASE != QUESTION (ex. ROUND_END) : les 2 boutons sont désactivés, aucun clic n\'a d\'effet', () => {
    const rafaleValidate = vi.fn()
    const rafaleInvalidate = vi.fn()
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', RAFALE_SUBPHASE: 'ROUND_END' },
      rafaleValidate,
      rafaleInvalidate,
    }))
    const { container } = render(<GamePage />)

    const buttons = container.querySelectorAll('.anim-rafale-action-btn')
    for (const btn of buttons) {
      expect(btn.disabled).toBe(true)
      fireEvent.click(btn)
    }
    expect(rafaleValidate).not.toHaveBeenCalled()
    expect(rafaleInvalidate).not.toHaveBeenCalled()
  })

  it('phase STOPPED (avant lancement) : les boutons ne sont pas rendus (pas de .rafale-admin-live, panneau de pré-lancement à la place)', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED' } }))
    const { container } = render(<GamePage />)

    expect(container.querySelectorAll('.anim-rafale-action-btn')).toHaveLength(0)
  })
})

// ---------------------------------------------------------------------------
// Sélecteur d'équipes RAFALE — reflet fidèle de RAFALE_PARTICIPATING_TEAMS
// après un rejeu (v8.0.0, #199, bugfix backend SHA a7b70057/a8b125ec :
// Ready() ne réinitialisait pas la sélection sur un rejeu de la MÊME
// question). `selectedRafaleTeams` (GamePage.jsx) est une lecture DIRECTE de
// `gameState.RAFALE_PARTICIPATING_TEAMS` — aucun état local React ne la
// duplique/cache — donc l'UI ne PEUT PAS être désynchronisée du serveur par
// construction : ce test le prouve en simulant le push WebSocket exact
// qu'un rejeu produit désormais (nouveau GameState avec la liste vidée) et
// vérifie que le sélecteur suit, sans action de l'utilisateur.
// ---------------------------------------------------------------------------

describe('GamePage — sélecteur RAFALE : reflète RAFALE_PARTICIPATING_TEAMS après rejeu (#199, SHA a7b70057)', () => {
  const bumpersFor = (teamNames) =>
    Object.fromEntries(teamNames.map((name, i) => [`MAC-${i}`, { TEAM: name, NAME: `P${i}`, SCORE: 0 }]))

  function chipsRow(container) {
    return container.querySelector('.memory-team-selector .memory-chips-row')
  }

  it('manche précédente : "red" et "blue" apparaissent en chips SÉLECTIONNÉES', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'PREPARE',
        question: { ID: '1', TYPE: 'RAFALE', CATEGORY: 'HISTORY', RAFALE_MODE: 'CHACUN_SON_TOUR' },
        RAFALE_PARTICIPATING_TEAMS: ['red', 'blue'],
      },
      teams: { red: { READY: true }, blue: { READY: true } },
      bumpers: bumpersFor(['red', 'blue']),
    }))
    const { container } = render(<GamePage />)

    const row = chipsRow(container)
    const selectedNames = Array.from(row.querySelectorAll('.memory-team-chip.selected .chip-name')).map(el => el.textContent)
    expect(selectedNames.sort()).toEqual(['blue', 'red'])
  })

  it('après rejeu (nouveau GameState poussé par le serveur, RAFALE_PARTICIPATING_TEAMS vidée) : plus AUCUNE chip sélectionnée, red/blue repassent en "disponible"', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'PREPARE',
        question: { ID: '1', TYPE: 'RAFALE', CATEGORY: 'HISTORY', RAFALE_MODE: 'CHACUN_SON_TOUR' },
        RAFALE_PARTICIPATING_TEAMS: ['red', 'blue'],
      },
      teams: { red: { READY: true }, blue: { READY: true } },
      bumpers: bumpersFor(['red', 'blue']),
    }))
    const { container, rerender } = render(<GamePage />)
    expect(chipsRow(container).querySelectorAll('.memory-team-chip.selected')).toHaveLength(2)

    // Simule EXACTEMENT ce que le fix serveur produit sur un rejeu : un
    // nouveau GameState (même question ID "1") avec RAFALE_PARTICIPATING_TEAMS
    // réinitialisée à vide, poussé via WebSocket — aucune interaction
    // utilisateur, useGame() renvoie juste un nouvel objet gameState.
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'PREPARE',
        question: { ID: '1', TYPE: 'RAFALE', CATEGORY: 'HISTORY', RAFALE_MODE: 'CHACUN_SON_TOUR' },
        RAFALE_PARTICIPATING_TEAMS: [],
      },
      teams: { red: { READY: true }, blue: { READY: true } },
      bumpers: bumpersFor(['red', 'blue']),
    }))
    rerender(<GamePage />)

    const row = chipsRow(container)
    expect(row.querySelectorAll('.memory-team-chip.selected')).toHaveLength(0)
    const availableNames = Array.from(row.querySelectorAll('.memory-team-chip.available .chip-name')).map(el => el.textContent)
    expect(availableNames.sort()).toEqual(['blue', 'red'])
  })

  it('après rejeu, START reste cohérent avec le sélecteur vide : désactivé (rafaleTeamsBlocked), pas de désync bouton/chips', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'READY',
        question: { ID: '1', TYPE: 'RAFALE', CATEGORY: 'HISTORY', RAFALE_MODE: 'CHACUN_SON_TOUR' },
        RAFALE_PARTICIPATING_TEAMS: [],
      },
      teams: { red: { READY: true } },
      bumpers: bumpersFor(['red']),
    }))
    const { container } = render(<GamePage />)

    expect(chipsRow(container).querySelectorAll('.memory-team-chip.selected')).toHaveLength(0)
    const startBtn = screen.getByText('START')
    expect(startBtn.disabled).toBe(true)
  })
})
