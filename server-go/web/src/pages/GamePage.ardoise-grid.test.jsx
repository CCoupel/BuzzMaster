import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import GamePage from './GamePage'

// ---------------------------------------------------------------------------
// Tests : grille responsive du panneau ARDOISE (#116)
//
// Plan : _work/reports/plan-20260801-140000.md.
//
// **Non-régression prioritaire (risque impact "Élevé" du plan)** : F1-F3 ne
// touchent QUE le conteneur, le modificateur de rang et l'habillage du
// texte — jamais `sortedArdoiseEntries` ni `formatArdoiseDelay` (#117). La
// première moitié de ce fichier reverrouille explicitement ces deux points
// dans le cadre de #116, en plus de la couverture déjà existante dans
// GamePage.ardoise-order.test.jsx (#117, inchangée).
//
// Limite technique assumée : les tests montés (`render(<GamePage />)`)
// mockent `./GamePage.css` (comme GamePage.ardoise-order.test.jsx) — jsdom
// n'applique donc jamais la vraie feuille de style, et `getComputedStyle`/
// `toHaveStyle` ne peuvent rien dire d'une règle externe telle que
// `display: grid`. Les exigences F1/F3 qui portent sur des propriétés CSS
// (grid, auto-fill, overflow-wrap, absence de troncature) sont donc
// vérifiées par lecture directe du texte source de GamePage.css — fiable
// pour « la règle existe et contient/omet telle propriété », complémentaire
// (pas un substitut) à la vérification visuelle prévue en QA à 2/6/16
// équipes et sur tablette.
// ---------------------------------------------------------------------------

describe('GamePage.css — règles du panneau ARDOISE en grille (#116, F1/F3)', () => {
  const cssPath = path.join(path.dirname(fileURLToPath(import.meta.url)), 'GamePage.css')
  const cssSource = fs.readFileSync(cssPath, 'utf-8')

  // The whole ARDOISE panel section, start to end marker. Scoping every
  // lookup below to THIS slice (not the full file) matters for real: the
  // stylesheet also has an unrelated, pre-existing `.ardoise-answers-list`
  // rule elsewhere (a different/legacy block, `.ardoise-team-answer` etc.,
  // outside this panel) — searching the whole file would silently match
  // that one instead of the panel's own rule.
  const panelStart = cssSource.indexOf('ARDOISE zone réponses')
  const panelEnd = cssSource.indexOf('Individual admin card')
  const panelSource = panelStart >= 0 && panelEnd > panelStart
    ? cssSource.slice(panelStart, panelEnd)
    : (() => { throw new Error('ARDOISE panel CSS section markers not found — GamePage.css structure changed') })()

  // Same panel source with /* ... */ comments stripped — needed for the
  // truncation-absence checks below: dev-frontend's own explanatory comment
  // on .ardoise-answer-text literally contains the word "line-clamp" while
  // documenting the decision NOT to add it (F3) — a plain substring search
  // on the commented source would flag that comment as a violation.
  const panelSourceNoComments = panelSource.replace(/\/\*[\s\S]*?\*\//g, '')

  // Extracts a single rule's declaration block by exact selector, scoped to
  // the panel section. Requiring whitespace-then-brace right after the
  // selector disambiguates e.g. `.ardoise-answer-text` from
  // `.ardoise-answer-text-row` or `.ardoise-answer-text.no-answer` (chained
  // modifier, no space before `.`).
  function ruleBody(selector) {
    const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const match = panelSource.match(new RegExp(escaped + '\\s*\\{([^}]*)\\}'))
    return match ? match[1] : null
  }

  it('F1 : .ardoise-answers-list passe en display:grid', () => {
    const rule = ruleBody('.ardoise-answers-list')
    expect(rule).not.toBeNull()
    expect(rule).toMatch(/display:\s*grid/)
  })

  it('F1 : les colonnes utilisent auto-fill (pas auto-fit) — cellules stables à 2-3 équipes', () => {
    const rule = ruleBody('.ardoise-answers-list')
    expect(rule).toMatch(/grid-template-columns:[^;]*auto-fill/)
    expect(rule).not.toMatch(/auto-fit/)
  })

  it('F1 : align-items: start — une réponse longue n\'étire pas ses voisines de rangée', () => {
    const rule = ruleBody('.ardoise-answers-list')
    expect(rule).toMatch(/align-items:\s*start/)
  })

  it('F2 : un modificateur existe pour la cellule de rang 1 (.ardoise-answer-row.rank-first)', () => {
    const rule = ruleBody('.ardoise-answer-row.rank-first')
    expect(rule).not.toBeNull()
  })

  it('F3 : overflow-wrap: anywhere sur .ardoise-answer-text (empêche le débordement sur chaîne sans espace)', () => {
    const rule = ruleBody('.ardoise-answer-text')
    expect(rule).toMatch(/overflow-wrap:\s*anywhere/)
  })

  it('F3 : aucune troncature nulle part dans le panneau ARDOISE (ni text-overflow, ni line-clamp)', () => {
    expect(panelSourceNoComments).not.toMatch(/text-overflow/)
    expect(panelSourceNoComments).not.toMatch(/line-clamp/)
  })
})

// ---------------------------------------------------------------------------
// Mocks pour le rendu — identiques à GamePage.ardoise-order.test.jsx (#117),
// dupliqués ici (pas d'import croisé entre fichiers de test) pour garder ce
// fichier autonome et directement lisible comme la couverture #116.
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn((questions) => ({
    selectedCategories: new Set(),
    availableCategories: [],
    filteredQuestions: questions ? Object.values(questions) : [],
    toggleCategoryFilter: vi.fn(),
    clearCategoryFilters: vi.fn(),
  })),
}))

vi.mock('../components/Timer', () => ({
  default: ({ phase }) => <div data-testid="timer" data-phase={phase} />,
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
    <div
      data-testid={`question-card-${question.ID}`}
      onClick={() => onClick && onClick(question)}
    />
  ),
  CATEGORIES: {},
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

vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color || '#888')),
}))

vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'

const makeTeamBumper = (teamName, mac) => ({
  [mac]: {
    TEAM: teamName,
    NAME: `Buzzer-${teamName}`,
    SCORE: 0,
    IS_VIRTUAL: false,
    IS_VPLAYER: true,
    CONNECTED: true,
    COLOR: [99, 102, 241],
  },
})

const THREE_TEAMS = {
  'Équipe A': { NAME: 'Équipe A', SCORE: 0, COLOR: [255, 0, 0] },
  'Équipe B': { NAME: 'Équipe B', SCORE: 0, COLOR: [0, 255, 0] },
  'Équipe C': { NAME: 'Équipe C', SCORE: 0, COLOR: [0, 0, 255] },
}

const THREE_BUMPERS = {
  ...makeTeamBumper('Équipe A', 'AA:00:00:00:00:01'),
  ...makeTeamBumper('Équipe B', 'AA:00:00:00:00:02'),
  ...makeTeamBumper('Équipe C', 'AA:00:00:00:00:03'),
}

const makeGameMock = (overrides = {}) => {
  const { gameState: gameStateOverride, teams, bumpers, questions, ...otherOverrides } = overrides
  return {
    gameState: {
      phase: 'STARTED',
      question: {
        ID: '10',
        TYPE: 'ARDOISE',
        QUESTION: 'Quelle est la capitale de la France ?',
        ANSWER: 'Paris',
        POINTS: '2',
        ARDOISE_KEYBOARD_TYPE: 'AZERTY',
      },
      remote: 'GAME',
      timer: 15,
      totalTime: 30,
      MEMORY_PARTICIPATING_TEAMS: [],
      ARDOISE_ANSWERS: {},
      gameTime: 1000000,
      ...(gameStateOverride || {}),
    },
    teams: teams ?? THREE_TEAMS,
    bumpers: bumpers ?? THREE_BUMPERS,
    questions: questions ?? {
      '10': { ID: '10', STATUS: 'STARTED', ORDER: 1 },
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
    ...otherOverrides,
  }
}

function readPanelRows(container) {
  const rows = container.querySelectorAll('.ardoise-answer-row')
  return Array.from(rows).map((row) => {
    const nameEl = row.querySelector(
      '.ardoise-answer-team-name span:not(.ardoise-team-dot):not(.ardoise-answer-rank):not(.ardoise-answer-delay)'
    )
    const rankEl = row.querySelector('.ardoise-answer-rank')
    const delayEl = row.querySelector('.ardoise-answer-delay')
    return {
      teamName: nameEl ? nameEl.textContent : '',
      hasAnswer: row.classList.contains('has-answer'),
      isRankFirst: row.classList.contains('rank-first'),
      rank: rankEl ? rankEl.textContent : null,
      delayText: delayEl ? delayEl.textContent : null,
    }
  })
}

describe('GamePage — panneau ARDOISE en grille : contenu et non-régressions (#116)', () => {
  it('non-régression #117 : l\'ordre suit toujours STARTED_AT, indépendant de l\'ordre de la liste d\'équipes', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Réponse A', STARTED_AT: 3000000, SUBMITTED_AT: 3000000 },
          'Équipe B': { TEXT: 'Réponse B', STARTED_AT: 1000000, SUBMITTED_AT: 1000000 },
          'Équipe C': { TEXT: 'Réponse C', STARTED_AT: 2000000, SUBMITTED_AT: 2000000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const rows = readPanelRows(container)
    expect(rows.map(r => r.teamName)).toEqual(['Équipe B', 'Équipe C', 'Équipe A'])
  })

  it('non-régression #117 : le délai reste formaté à 3 décimales', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        gameTime: 1000000,
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Paris', STARTED_AT: 5732000, SUBMITTED_AT: 5732000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.ardoise-answer-delay').textContent).toBe('4.732 s')
  })

  it('non-régression : les équipes sans réponse restent en fin de liste', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          'Équipe C': { TEXT: 'Réponse C', STARTED_AT: 2000000, SUBMITTED_AT: 2000000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const rows = readPanelRows(container)
    expect(rows[0].teamName).toBe('Équipe C')
    expect(rows[0].hasAnswer).toBe(true)
    expect(rows.slice(1).every(r => !r.hasAnswer)).toBe(true)
  })

  it('non-régression : le bouton d\'attribution de points reste fonctionnel en REVEALED', () => {
    const setTeamPoints = vi.fn()
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'REVEALED',
        question: {
          ID: '10', TYPE: 'ARDOISE', QUESTION: '?', ANSWER: 'Paris',
          POINTS: '3', ARDOISE_KEYBOARD_TYPE: 'AZERTY',
        },
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Réponse A', STARTED_AT: 1000000, SUBMITTED_AT: 1000000 },
        },
      },
      setTeamPoints,
    }))
    const { container } = render(<GamePage />)

    fireEvent.click(container.querySelector('.ardoise-points-btn'))
    expect(setTeamPoints).toHaveBeenCalledWith('Équipe A', 3)
  })

  it('F2 : la cellule de rang 1 porte le modificateur rank-first, les suivantes ne le portent pas', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Réponse A', STARTED_AT: 3000000, SUBMITTED_AT: 3000000 },
          'Équipe B': { TEXT: 'Réponse B', STARTED_AT: 1000000, SUBMITTED_AT: 1000000 },
          'Équipe C': { TEXT: 'Réponse C', STARTED_AT: 2000000, SUBMITTED_AT: 2000000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const rows = readPanelRows(container)
    expect(rows[0]).toMatchObject({ teamName: 'Équipe B', isRankFirst: true })
    expect(rows[1]).toMatchObject({ teamName: 'Équipe C', isRankFirst: false })
    expect(rows[2]).toMatchObject({ teamName: 'Équipe A', isRankFirst: false })
  })

  it('F2 : rank-first n\'est jamais posé sur une équipe sans réponse', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { ARDOISE_ANSWERS: {} } }))
    const { container } = render(<GamePage />)

    const rows = readPanelRows(container)
    expect(rows.every(r => !r.isRankFirst)).toBe(true)
  })

  it('F3 : une réponse longue et sans espace est rendue intégralement, sans troncature ni suffixe de coupe', () => {
    const longWord = 'Constantinoplitainementparlant'.repeat(4) // 124 chars, no spaces
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: longWord, STARTED_AT: 1000000, SUBMITTED_AT: 1000000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const textEl = container.querySelector('.ardoise-answer-text')
    expect(textEl.textContent).toBe(longWord)
    expect(textEl.textContent.length).toBe(longWord.length)
    expect(textEl.textContent).not.toMatch(/…|\.\.\.$/)
  })

  it('chaque cellule contient rang, pastille, nom, texte et délai', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        gameTime: 1000000,
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Paris', STARTED_AT: 2000000, SUBMITTED_AT: 2000000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const row = container.querySelector('.ardoise-answer-row.has-answer')
    expect(row.querySelector('.ardoise-answer-rank')).not.toBeNull()
    expect(row.querySelector('.ardoise-team-dot')).not.toBeNull()
    expect(row.querySelector('.ardoise-answer-text').textContent).toBe('Paris')
    expect(row.querySelector('.ardoise-answer-delay')).not.toBeNull()
  })
})
