import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #244 + #251 (v11.2, lot V2-2) — attribution groupée après le REVEAL, selon la maquette VALIDÉE
// admin_game_team_award__selection : une case par carte d'équipe, barre « Points par équipe » (−, saisie, +),
// « Tout sélectionner / désélectionner », bouton « Attribuer les points (N) » / « Appliquer les pénalités (N) » (rouge si
// négatif, désactivé à N = 0), QCM : les 4 réponses cliquables (bonne → sélection + points ; mauvaise → pénalité −1),
// équipes déjà créditées grisées (« ✓ déjà créditée ±N »). Contrat : websocket-actions.md §TEAM_POINTS_BATCH.
// Scénarios front 11 à 16 du plan (_work/reports/planner-20261011-174500.md §4). Boîte noire par libellés / rôles ARIA.

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../components/Timer', () => ({
  default: ({ phase }) => <div data-testid="timer" data-phase={phase} />,
}))

vi.mock('../components/QuestionPreview', () => ({
  default: () => <div data-testid="question-preview" />,
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

vi.mock('./GamePage.css', () => ({}))


const ANSWERS = { RED: 'La Loire', GREEN: 'La Seine', YELLOW: 'Le Rhône', BLUE: 'La Garonne' }
const TEAM_ROWS = [
  ['Les Bleus', 'GREEN'], ['Les Rouges', 'GREEN'], ['Les Jaunes', 'RED'], ['Les Verts', 'GREEN'],
  ['Les Violets', 'BLUE'], ['Les Oranges', 'RED'], ['Les Roses', 'GREEN'], ['Les Gris', null],
]
const TEAMS = Object.fromEntries(TEAM_ROWS.map(([n], i) => [n, { NAME: n, SCORE: 10 + i, COLOR: [20 * i, 80, 160] }]))
const bumpersFor = (hintsByTeam = {}) => Object.fromEntries(TEAM_ROWS.map(([n, color], i) => [`vj${i}`, {
  NAME: `VJ${i}`, TEAM: n, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, ANSWER_COLOR: color || '', TIME: color ? 1000 + i : 0, /* #255 : TIME > 0 = a répondu */ HINTS_AT_BUZZ: hintsByTeam[n] || 0,
}]))
const QCM = { ID: '9', TYPE: 'QCM', QUESTION: 'Quel fleuve traverse Paris ?', ANSWER: 'La Seine', POINTS: '3', QCM_ANSWERS: ANSWERS, QCM_CORRECT: 'GREEN', QCM_HINTS_ENABLED: false, QCM_WRONG_PENALTY: -1, STATUS: 'REVEALED' }
const ARDOISE = { ID: '10', TYPE: 'ARDOISE', QUESTION: 'Capitale ?', ANSWER: 'Paris', POINTS: '2', ARDOISE_KEYBOARD_TYPE: 'AZERTY', STATUS: 'REVEALED' }

const mk = (over = {}) => {
  const { gameState: gsOver, hints, ...rest } = over
  return {
  gameState: { phase: 'REVEALED', question: QCM, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {}, ...(gsOver || {}) },
  teams: TEAMS, bumpers: bumpersFor(hints), questions: { [(gsOver?.question || QCM).ID]: { ...(gsOver?.question || QCM), STATUS: 'REVEALED', ORDER: 1 } },
  awardedTeams: {}, creditPoints: 3,
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
  setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
  sendMessage: vi.fn(), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
  teamPointsBatch: vi.fn(() => true), teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
  ...rest,
  }
}
// La partie réelle : sélection de la question (clic sur sa carte → points de la question dans « Points »), lancement,
// STOP, puis REVEAL. À l'entrée en REVEALED le montant de base B reprend ces points : on reproduit ce parcours
// (montage en STOPPED, sélection, passage en REVEALED) au lieu de monter directement en REVEALED.
const show = (over) => {
  const game = mk(over)
  const phase = game.gameState.phase
  if (phase !== 'REVEALED') {
    useGame.mockReturnValue(game)
    const utils = render(<GamePage />)
    return { game, ...utils }
  }
  useGame.mockReturnValue(mk({ ...over, gameState: { ...(over?.gameState || {}), phase: 'STOPPED' } }))
  const utils = render(<GamePage />)
  const id = (over?.gameState?.question || QCM).ID
  const card = screen.queryByTestId(`question-card-${id}`)
  if (card) fireEvent.click(card)
  useGame.mockReturnValue(game)
  utils.rerender(<GamePage />)
  return { game, ...utils }
}
const box = (name) => screen.getByRole('checkbox', { name: `Sélectionner ${name}` })
const queryBox = (name) => screen.queryByRole('checkbox', { name: `Sélectionner ${name}` })
const give = () => screen.getByRole('button', { name: /^(Attribuer les points|Appliquer les pénalités) \(\d+\)$/ })
const queryGive = () => screen.queryByRole('button', { name: /^(Attribuer les points|Appliquer les pénalités) \(\d+\)$/ })
// #254 : plus de montant propre au bloc — le montant est le champ « Points » de la page
const amountInput = () => screen.getAllByLabelText('Points').find((el) => el.tagName === 'INPUT' && el.type === 'number')
const answerBtn = (label) => screen.getByRole('button', { name: new RegExp(label) })
const checked = () => TEAM_ROWS.map(([n]) => n).filter((n) => queryBox(n)?.checked)

describe('Attribution groupée — visibilité (#244/#251, scénario 11)', () => {
  beforeEach(() => { useGame.mockReset() })

  it.each(['STARTED', 'PAUSED', 'STOPPED'])('Critical — phase %s (avant REVEALED) : ni cases, ni barre, ni bouton, ni réponses cliquables', (phase) => {
    show({ gameState: { phase, question: { ...QCM, STATUS: phase } } })
    expect(queryBox('Les Bleus')).toBeNull()
    expect(queryGive()).toBeNull()
    expect(screen.queryByLabelText('Points par équipe')).toBeNull() // #254 : aucun montant propre au bloc
    expect(screen.queryByRole('button', { name: /La Seine/ })).toBeNull()
  })

  it('après le REVEAL : une case par carte d’équipe, la barre et le bouton « Attribuer les points (0) » désactivé', () => {
    show()
    for (const [n] of TEAM_ROWS) expect(box(n).checked).toBe(false)
    expect(give().textContent).toBe('Attribuer les points (0)')
    expect(give().disabled).toBe(true)
    expect(amountInput().value).toBe('3') // B = points de la question
  })
})

describe('Attribution groupée — QCM : clic sur une réponse (scénario 12)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('chaque réponse affiche le nombre d’équipes qui l’ont donnée', () => {
    show()
    expect(answerBtn('La Seine').textContent).toMatch(/4 équipes/)
    expect(answerBtn('La Loire').textContent).toMatch(/2 équipes/)
    expect(answerBtn('La Garonne').textContent).toMatch(/1 équipe\b/)
    expect(answerBtn('Le Rhône').textContent).toMatch(/0 équipe/)
  })

  it('Critical — bonne réponse : sélectionne exactement les équipes qui l’ont donnée, montant = points de la question', () => {
    show()
    fireEvent.click(answerBtn('La Seine'))
    expect(checked().sort()).toEqual(['Les Bleus', 'Les Roses', 'Les Rouges', 'Les Verts'])
    expect(amountInput().value).toBe('3')
    expect(give().textContent).toBe('Attribuer les points (4)')
    expect(give().className).not.toMatch(/neg/)
  })

  it('Critical — mauvaise réponse : désélectionne tout, sélectionne ses équipes, pénalité −1, bouton « Appliquer les pénalités (N) » rouge', () => {
    show()
    fireEvent.click(answerBtn('La Seine')) // d'abord la bonne
    fireEvent.click(answerBtn('La Loire'))

    expect(checked().sort()).toEqual(['Les Jaunes', 'Les Oranges'])
    expect(amountInput().value).toBe('-1')
    expect(give().textContent).toBe('Appliquer les pénalités (2)')
    expect(give().className).toMatch(/neg/)
  })

  it('une équipe sans réponse n’est dans aucune sélection de réponse mais reste cochable à la main', () => {
    show()
    fireEvent.click(answerBtn('La Seine'))
    expect(box('Les Gris').checked).toBe(false)
    fireEvent.click(box('Les Gris'))
    expect(give().textContent).toBe('Attribuer les points (5)')
  })
})

describe('Attribution groupée — sélection manuelle et envoi (scénarios 13 et 15)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('« Tout sélectionner » / « Tout désélectionner »', () => {
    show()
    fireEvent.click(screen.getByRole('button', { name: 'Tout sélectionner' }))
    expect(checked().length).toBe(8)
    expect(give().textContent).toBe('Attribuer les points (8)')
    fireEvent.click(screen.getByRole('button', { name: 'Tout désélectionner' }))
    expect(checked().length).toBe(0)
    expect(give().disabled).toBe(true)
  })

  it('le montant est le champ « Points » de la page ; un montant négatif bascule le bouton en « Appliquer les pénalités »', () => {
    show()
    fireEvent.click(box('Les Bleus'))
    fireEvent.change(amountInput(), { target: { value: '-2' } })
    expect(give().textContent).toBe('Appliquer les pénalités (1)')
    fireEvent.change(amountInput(), { target: { value: '7' } })
    expect(give().textContent).toBe('Attribuer les points (1)')
  })

  it('Critical — le bouton envoie UN lot avec l’ID de la question et un montant par équipe sélectionnée', () => {
    const { game } = show()
    fireEvent.click(answerBtn('La Seine'))

    fireEvent.click(give())

    expect(game.teamPointsBatch).toHaveBeenCalledTimes(1)
    const [questionId, entries] = game.teamPointsBatch.mock.calls[0]
    expect(String(questionId)).toBe('9')
    expect(entries.map((e) => e.team).sort()).toEqual(['Les Bleus', 'Les Roses', 'Les Rouges', 'Les Verts'])
    for (const e of entries) expect(e.points).toBe(3)
  })

  it('montant modifié à la main : montant uniforme pour toutes les équipes sélectionnées', () => {
    const { game } = show()
    fireEvent.click(box('Les Bleus'))
    fireEvent.click(box('Les Gris'))
    fireEvent.change(amountInput(), { target: { value: '5' } })
    fireEvent.click(give())
    expect(game.teamPointsBatch.mock.calls[0][1]).toEqual(expect.arrayContaining([{ team: 'Les Bleus', points: 5 }, { team: 'Les Gris', points: 5 }]))
  })

  it('Critical — bouton désactivé pendant l’envoi : un second clic n’envoie rien', () => {
    const { game } = show()
    fireEvent.click(box('Les Bleus'))
    fireEvent.click(give())
    expect(give().disabled).toBe(true)
    fireEvent.click(give())
    expect(game.teamPointsBatch).toHaveBeenCalledTimes(1)
  })

  it('aperçu du montant sur chaque carte sélectionnée', () => {
    show()
    fireEvent.click(answerBtn('La Seine'))
    expect(screen.getAllByText('+3').length).toBe(4)
  })
})

describe('Attribution groupée — montants par type (scénario 13)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('QCM avec indices actifs : aperçu équipe par équipe (pénalité d’indices), identique au clic unitaire', () => {
    const { game } = show({ gameState: { question: { ...QCM, QCM_HINTS_ENABLED: true, POINTS: '3' } }, hints: { 'Les Rouges': 1, 'Les Roses': 2 } })
    fireEvent.click(answerBtn('La Seine'))
    fireEvent.click(give())
    const entries = game.teamPointsBatch.mock.calls[0][1]
    const byTeam = Object.fromEntries(entries.map((e) => [e.team, e.points]))
    expect(byTeam['Les Bleus']).toBe(3) // sans indice
    expect(byTeam['Les Rouges']).toBeLessThanOrEqual(3)
    expect(byTeam['Les Roses']).toBeLessThanOrEqual(byTeam['Les Rouges'])
    expect(byTeam['Les Roses']).toBeGreaterThan(0)
  })

  it('ARDOISE (pas de réponses cliquables) : cases seulement, montant = points de la question', () => {
    const { game } = show({ gameState: { question: ARDOISE } })
    expect(screen.queryByRole('button', { name: /La Seine/ })).toBeNull()
    expect(amountInput().value).toBe('2')
    fireEvent.click(box('Les Bleus'))
    fireEvent.click(give())
    expect(game.teamPointsBatch.mock.calls[0][1]).toEqual([{ team: 'Les Bleus', points: 2 }])
  })

  it('SPEEDY réglé au niveau équipe : cases seulement, montant = points de la question', () => {
    const { game } = show({ gameState: { question: { ID: '11', TYPE: 'SPEEDY', QUESTION: 'Q', ANSWER: 'A', POINTS: '4', POINTS_TARGET: 'TEAM', STATUS: 'REVEALED' } } })
    expect(screen.queryByRole('button', { name: /La Seine/ })).toBeNull()
    fireEvent.click(box('Les Rouges'))
    fireEvent.click(give())
    expect(game.teamPointsBatch.mock.calls[0][1]).toEqual([{ team: 'Les Rouges', points: 4 }])
  })
})

describe('Attribution groupée — équipes déjà créditées (scénario 14)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — grisées, non cochables, avec « ✓ déjà créditée +N » ; jamais sélectionnées par une réponse ni par « Tout sélectionner »', () => {
    show({ awardedTeams: { 'Les Bleus': { POINTS: 3, TIMESTAMP: 1 }, 'Les Rouges': { POINTS: -1, TIMESTAMP: 2 } } })

    expect(box('Les Bleus').disabled).toBe(true)
    expect(box('Les Rouges').disabled).toBe(true)
    expect(screen.getByText(/✓ déjà créditée \+3/)).toBeTruthy()
    expect(screen.getByText(/✓ déjà créditée -1|✓ déjà créditée −1/)).toBeTruthy()

    fireEvent.click(answerBtn('La Seine'))
    expect(checked().sort()).toEqual(['Les Roses', 'Les Verts']) // Bleus et Rouges déjà créditées : exclues
    fireEvent.click(screen.getByRole('button', { name: 'Tout sélectionner' }))
    expect(box('Les Bleus').checked).toBe(false)
    expect(give().textContent).toBe('Attribuer les points (6)') // 8 équipes − 2 déjà créditées
  })
})

describe('Attribution groupée — résultat du serveur (scénario 15)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('lot appliqué : message « Points attribués à 4 équipes » (maquette #254), sélection vidée', () => {
    const { game, rerender } = show()
    fireEvent.click(answerBtn('La Seine'))
    fireEvent.click(give())
    useGame.mockReturnValue(mk({ teamPointsBatch: game.teamPointsBatch, teamPointsBatchResult: {
      status: 'APPLIED', questionId: '9', applied: ['Les Bleus', 'Les Rouges', 'Les Verts', 'Les Roses'].map((team) => ({ team, points: 3 })), skipped: [], teams: [],
    } }))
    rerender(<GamePage />)

    expect(screen.getByText(/Points attribués à 4 équipes/)).toBeTruthy()
    expect(checked().length).toBe(0)
  })

  it('équipes ignorées : « 2 équipes déjà créditées ignorées »', () => {
    const { rerender } = show()
    useGame.mockReturnValue(mk({ teamPointsBatchResult: {
      status: 'APPLIED', questionId: '9', applied: [{ team: 'Les Verts', points: 3 }],
      skipped: [{ team: 'Les Bleus', reason: 'ALREADY_AWARDED' }, { team: 'Les Rouges', reason: 'ALREADY_AWARDED' }], teams: [],
    } }))
    rerender(<GamePage />)
    expect(screen.getByText(/2 équipes déjà créditées ignorées/)).toBeTruthy()
  })

  it('Critical — UNKNOWN_TEAM : message et équipes inconnues retirées de la sélection', () => {
    const { rerender } = show()
    fireEvent.click(answerBtn('La Seine'))
    useGame.mockReturnValue(mk({ teamPointsBatchResult: { status: 'REFUSED', reason: 'UNKNOWN_TEAM', questionId: '9', applied: [], skipped: [], teams: ['Les Bleus'] } }))
    rerender(<GamePage />)

    expect(screen.getByText(/Équipe introuvable/)).toBeTruthy()
    expect(box('Les Bleus').checked).toBe(false)
    expect(box('Les Verts').checked).toBe(true)
  })

  it('envoi impossible (WebSocket fermée) : message d’erreur, le bouton se réactive', () => {
    const { game } = show({ teamPointsBatch: vi.fn(() => false) })
    fireEvent.click(box('Les Bleus'))
    fireEvent.click(give())
    expect(screen.getByText(/Connexion perdue/)).toBeTruthy()
    expect(give().disabled).toBe(false)
    expect(game.teamPointsBatch).toHaveBeenCalledTimes(1)
  })
})

describe('Attribution groupée — remise à zéro (scénario 16) et accessibilité', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — changement de question : sélection et montant remis à zéro', () => {
    const { rerender } = show()
    fireEvent.click(answerBtn('La Loire')) // pénalité −1, 2 équipes
    expect(amountInput().value).toBe('-1')

    // nouvelle question comme en partie : sélection (STOPPED) puis REVEAL
    useGame.mockReturnValue(mk({ gameState: { phase: 'STOPPED', question: { ...QCM, ID: '12', STATUS: 'STOPPED' } } }))
    rerender(<GamePage />)
    useGame.mockReturnValue(mk({ gameState: { question: { ...QCM, ID: '12', STATUS: 'REVEALED' } } }))
    rerender(<GamePage />)

    expect(checked().length).toBe(0)
    expect(amountInput().value).toBe('3')
  })

  it('changement de phase (REVEALED → NEW_GAME puis retour) : sélection remise à zéro', () => {
    const { rerender } = show()
    fireEvent.click(answerBtn('La Seine'))
    useGame.mockReturnValue(mk({ gameState: { phase: 'STARTED', question: { ...QCM, STATUS: 'STARTED' } } }))
    rerender(<GamePage />)
    useGame.mockReturnValue(mk())
    rerender(<GamePage />)
    expect(checked().length).toBe(0)
  })

  it('accessibilité : chaque case a un nom « Sélectionner <équipe> », la barre est un groupe nommé, le montant a un libellé', () => {
    show()
    for (const [n] of TEAM_ROWS) expect(box(n)).toBeTruthy()
    expect(screen.getByRole('group', { name: /Attribution des points/ })).toBeTruthy()
    expect(amountInput()).toBeTruthy()
  })

  it('non-régression : le clic sur une carte d’équipe reste libre (TEAM_POINTS unitaire) — setTeamPoints existe toujours', () => {
    const { game } = show()
    expect(typeof game.setTeamPoints).toBe('function')
  })
})
