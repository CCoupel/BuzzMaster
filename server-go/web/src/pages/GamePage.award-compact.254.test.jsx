import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #254 (v11.2, lot V2-4) — attribution groupée : bloc compact (2 lignes, sans montant propre, icônes accessibles),
// niveau équipe OU joueur selon la question, montant = champ « Points » de la page, clic sur une réponse QCM
// (pénalité de la question, 0 = bouton désactivé avec la raison), commutateur « Tenir compte des indices »,
// base du barème. Maquettes VALIDÉES : admin_game_award__compact_block_hints_toggle,
// admin_game_award__team_or_player_level_hints_toggle. Contrat : websocket-actions.md §TEAM_POINTS_BATCH
// (« Niveau d'attribution », « Montant », « QCM : clic sur une réponse », « SET_CREDIT_POINTS ≤ 0 »).
// Scénarios front 10 à 16, 19, 22 à 31 du plan. Boîte noire par libellés / rôles ARIA.

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
// équipe → [joueur, couleur de réponse, indices au buzz]
const ROWS = [['Les Bleus', 'Paul', 'GREEN', 0], ['Les Rouges', 'Mamie', 'GREEN', 1], ['Les Jaunes', 'Jade', 'RED', 0], ['Les Verts', 'Vera', 'GREEN', 2]]
const TEAMS = Object.fromEntries(ROWS.map(([n], i) => [n, { NAME: n, SCORE: 10 + i, COLOR: [20 * i, 80, 160] }]))
const bumpers = (rows = ROWS) => Object.fromEntries(rows.map(([team, name, color, hints], i) => [`vj${i}`, {
  NAME: name, TEAM: team, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, SCORE: 0, TIME: 1000 + i, ANSWER_COLOR: color || '', HINTS_AT_BUZZ: hints,
}]))
const QCM = (over = {}) => ({ ID: '9', TYPE: 'QCM', QUESTION: 'Quel fleuve traverse Paris ?', ANSWER: 'La Seine', POINTS: '3', QCM_ANSWERS: ANSWERS, QCM_CORRECT: 'GREEN', QCM_HINTS_ENABLED: false, QCM_WRONG_PENALTY: 0, STATUS: 'REVEALED', ...over })

const mk = (over = {}) => {
  const { gameState: gsOver, ...rest } = over
  const question = gsOver?.question || QCM()
  return {
    gameState: { phase: 'REVEALED', question, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {}, qcmInvalidated: [], ...(gsOver || {}) },
    teams: TEAMS, bumpers: bumpers(), questions: { [question.ID]: { ...question, STATUS: 'REVEALED', ORDER: 1 } },
    awardedTeams: {}, awardedPlayers: {}, creditPoints: 3,
    startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
    setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
    sendMessage: vi.fn(() => true), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
    teamPointsBatch: vi.fn(() => true), teamPointsBatchResult: null, clearTeamPointsBatchResult: vi.fn(),
    bumperPointsBatch: vi.fn(() => true), bumperPointsBatchResult: null, clearBumperPointsBatchResult: vi.fn(),
    ...rest,
  }
}
// Parcours réel : sélection de la question (clic sur sa carte), puis passage en REVEALED (la base du barème est fixée à
// l'entrée en REVEALED). `before(game)` permet d'agir en STOPPED (ex. régler « Points » avant le REVEAL).
const show = (over = {}, before) => {
  const game = mk(over)
  useGame.mockReturnValue(mk({ ...over, gameState: { ...(over.gameState || {}), phase: 'STOPPED' } }))
  const utils = render(<GamePage />)
  const card = screen.queryByTestId(`question-card-${game.gameState.question.ID}`)
  if (card) fireEvent.click(card)
  if (before) before(game)
  useGame.mockReturnValue(game)
  utils.rerender(<GamePage />)
  return { game, ...utils }
}
const reshow = (utils, game, patch) => {
  const next = { ...game, ...patch, gameState: { ...game.gameState, ...(patch.gameState || {}) } }
  useGame.mockReturnValue(next)
  utils.rerender(<GamePage />)
  return next
}
const box = (name) => screen.getByRole('checkbox', { name: `Sélectionner ${name}` })
const queryBox = (name) => screen.queryByRole('checkbox', { name: `Sélectionner ${name}` })
const checkedOf = (names) => names.filter((n) => queryBox(n)?.checked)
const TEAM_NAMES = ROWS.map((r) => r[0])
const PLAYER_NAMES = ROWS.map((r) => r[1])
const action = () => screen.getByRole('button', { name: /^(Attribuer les points|Appliquer les pénalités) \(\d+\)$/ })
const queryAction = () => screen.queryByRole('button', { name: /^(Attribuer les points|Appliquer les pénalités) \(\d+\)$/ })
const pointsField = () => screen.getAllByLabelText('Points').find((el) => el.tagName === 'INPUT' && el.type === 'number')
const setPoints = (v) => fireEvent.change(pointsField(), { target: { value: String(v) } })
const answerBtn = (label) => screen.getByRole('button', { name: new RegExp(label) })
const hintSwitch = () => screen.queryByRole('switch', { name: 'Tenir compte des indices' })
// état d'un interrupteur : aria-checked s'il existe, sinon la propriété checked d'un <input role="switch">
const isOn = (el) => String(el.getAttribute('aria-checked') ?? el.checked)
const REASON = 'Aucune pénalité définie pour cette question'

describe('Bloc compact (#254, scénario 13)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — après le REVEAL : « Attribuer les points (0) » désactivé, AUCUNE saisie de montant dans le bloc, le champ « Points » de la page sert de montant', () => {
    show()
    expect(action().textContent).toBe('Attribuer les points (0)')
    expect(action().disabled).toBe(true)
    expect(screen.queryByLabelText('Points par équipe')).toBeNull()
    expect(screen.queryByLabelText('Augmenter les points par équipe')).toBeNull()
    expect(pointsField().value).toBe('3')
  })

  it('les icônes « Tout sélectionner » / « Tout désélectionner » ont un nom accessible (aria-label et title), sur la ligne du bouton', () => {
    show()
    const selectAll = screen.getByRole('button', { name: 'Tout sélectionner' })
    const clearAll = screen.getByRole('button', { name: 'Tout désélectionner' })
    expect(selectAll.getAttribute('title')).toBe('Tout sélectionner')
    expect(clearAll.getAttribute('title')).toBe('Tout désélectionner')
    // même ligne que le bouton d'action : ancêtre commun proche, bloc compact
    const row = action().parentElement
    expect(row.contains(selectAll) && row.contains(clearAll)).toBe(true)
  })

  it('« Tout sélectionner » ne coche pas les équipes déjà créditées ; « Tout désélectionner » vide la sélection', () => {
    show({ awardedTeams: { 'Les Jaunes': { POINTS: 2, TIMESTAMP: 1 } } })
    fireEvent.click(screen.getByRole('button', { name: 'Tout sélectionner' }))
    expect(checkedOf(TEAM_NAMES).sort()).toEqual(['Les Bleus', 'Les Rouges', 'Les Verts'])
    expect(queryBox('Les Jaunes').disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'Tout désélectionner' }))
    expect(checkedOf(TEAM_NAMES)).toEqual([])
  })

  it('avant le REVEAL : ni cases, ni bloc, ni réponses cliquables', () => {
    useGame.mockReturnValue(mk({ gameState: { phase: 'STARTED' } }))
    render(<GamePage />)
    expect(queryBox('Les Bleus')).toBeNull()
    expect(queryAction()).toBeNull()
    expect(screen.queryByRole('button', { name: 'Tout sélectionner' })).toBeNull()
  })
})

describe('Niveau d\'attribution : équipe ou joueur (scénario 10)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — niveau équipe (QCM par défaut) : cases sur les cartes d’équipe, aucune case de joueur, TEAM_POINTS_BATCH', () => {
    const { game } = show()
    for (const n of TEAM_NAMES) expect(queryBox(n)).not.toBeNull()
    for (const n of PLAYER_NAMES) expect(queryBox(n), n).toBeNull()
    fireEvent.click(box('Les Bleus'))
    fireEvent.click(box('Les Jaunes'))
    fireEvent.click(action())
    expect(game.bumperPointsBatch).not.toHaveBeenCalled()
    expect(game.teamPointsBatch).toHaveBeenCalledTimes(1)
    const [qid, entries] = game.teamPointsBatch.mock.calls[0]
    expect(qid).toBe('9')
    expect(entries).toEqual([{ team: 'Les Bleus', points: 3 }, { team: 'Les Jaunes', points: 3 }])
  })

  it('Critical — niveau joueur (POINTS_TARGET PLAYER) : cases sur les lignes de joueurs, aucune case d’équipe, BUMPER_POINTS_BATCH', () => {
    const { game } = show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) } })
    for (const n of PLAYER_NAMES) expect(queryBox(n), n).not.toBeNull()
    for (const n of TEAM_NAMES) expect(queryBox(n), n).toBeNull()
    fireEvent.click(box('Paul'))
    fireEvent.click(box('Jade'))
    expect(action().textContent).toBe('Attribuer les points (2)')
    fireEvent.click(action())
    expect(game.teamPointsBatch).not.toHaveBeenCalled()
    expect(game.bumperPointsBatch).toHaveBeenCalledTimes(1)
    const [qid, entries] = game.bumperPointsBatch.mock.calls[0]
    expect(qid).toBe('9')
    expect(entries).toEqual([{ id: 'vj0', points: 3 }, { id: 'vj2', points: 3 }])
  })

  it.each([
    ['SPEEDY', undefined, 'player'], ['SPEEDY', 'TEAM', 'team'], ['ARDOISE', undefined, 'team'], ['ARDOISE', 'PLAYER', 'player'],
    ['MEMORY', 'PLAYER', 'team'], ['MEMOTION', 'PLAYER', 'team'], ['RAFALE', 'PLAYER', 'team'],
  ])('Critical — %s avec POINTS_TARGET %s → niveau %s', (type, target, level) => {
    const q = { ID: '11', TYPE: type, QUESTION: 'Q', ANSWER: 'A', POINTS: '2', STATUS: 'REVEALED', ...(target ? { POINTS_TARGET: target } : {}),
      ...(type === 'MEMORY' ? { MEMORY_MODE: 'SOLO', MEMORY_PAIRS: [{}, {}] } : {}), ...(type === 'ARDOISE' ? { ARDOISE_KEYBOARD_TYPE: 'AZERTY' } : {}) }
    show({ gameState: { question: q } })
    const teamBoxes = TEAM_NAMES.filter((n) => queryBox(n)).length
    const playerBoxes = PLAYER_NAMES.filter((n) => queryBox(n)).length
    if (level === 'team') { expect(teamBoxes).toBe(4); expect(playerBoxes).toBe(0) } else { expect(playerBoxes).toBe(4); expect(teamBoxes).toBe(0) }
  })

  it('(15) joueurs déjà crédités : grisés, non cochables, « ✓ déjà crédité ±N », ignorés par « Tout sélectionner »', () => {
    show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) }, awardedPlayers: { vj1: { POINTS: -2, TIMESTAMP: 1, TEAM: 'Les Rouges' } } })
    expect(queryBox('Mamie').disabled).toBe(true)
    expect(queryBox('Mamie').checked).toBe(false)
    expect(screen.getByText(/✓ déjà crédité −2|✓ déjà crédité -2/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Tout sélectionner' }))
    expect(checkedOf(PLAYER_NAMES).sort()).toEqual(['Jade', 'Paul', 'Vera'])
  })
})

describe('QCM : clic sur une réponse (scénario 11)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — bonne réponse : sélectionne les équipes de cette couleur, « Points » = base, « Attribuer les points (3) »', () => {
    show()
    fireEvent.click(answerBtn('La Seine'))
    expect(checkedOf(TEAM_NAMES).sort()).toEqual(['Les Bleus', 'Les Rouges', 'Les Verts'])
    expect(pointsField().value).toBe('3')
    expect(action().textContent).toBe('Attribuer les points (3)')
    expect(action().disabled).toBe(false)
  })

  it('niveau joueur : la bonne réponse sélectionne les JOUEURS de cette couleur (hors déjà crédités)', () => {
    show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) }, awardedPlayers: { vj0: { POINTS: 3, TIMESTAMP: 1, TEAM: 'Les Bleus' } } })
    fireEvent.click(answerBtn('La Seine'))
    expect(checkedOf(PLAYER_NAMES).sort()).toEqual(['Mamie', 'Vera'])
  })

  it('Critical (19) — mauvaise réponse avec QCM_WRONG_PENALTY = −3 : désélection puis sélection de ses équipes, « Points » = −3, « Appliquer les pénalités (1) » actif', () => {
    show({ gameState: { question: QCM({ QCM_WRONG_PENALTY: -3 }) } })
    fireEvent.click(answerBtn('La Seine'))
    fireEvent.click(answerBtn('La Loire'))
    expect(checkedOf(TEAM_NAMES)).toEqual(['Les Jaunes'])
    expect(pointsField().value).toBe('-3')
    expect(action().textContent).toBe('Appliquer les pénalités (1)')
    expect(action().disabled).toBe(false)
  })

  it('Critical (19) — pénalité de la question envoyée dans le lot (−3 pour chaque équipe sélectionnée)', () => {
    const { game } = show({ gameState: { question: QCM({ QCM_WRONG_PENALTY: -3 }) } })
    fireEvent.click(answerBtn('La Loire'))
    fireEvent.click(action())
    expect(game.teamPointsBatch).toHaveBeenCalledTimes(1)
    expect(game.teamPointsBatch.mock.calls[0][1]).toEqual([{ team: 'Les Jaunes', points: -3 }])
  })

  it.each([
    ['0', { QCM_WRONG_PENALTY: 0 }], ['champ absent', { QCM_WRONG_PENALTY: undefined }], ['valeur positive invalide (5)', { QCM_WRONG_PENALTY: 5 }], ['hors bornes (−150)', { QCM_WRONG_PENALTY: -150 }],
  ])('Critical (19) — mauvaise réponse sans pénalité (%s) : sélection faite, « Points » = 0, bouton désactivé, raison visible, aucun lot', (_n, over) => {
    const { game } = show({ gameState: { question: QCM(over) } })
    fireEvent.click(answerBtn('La Loire'))
    expect(checkedOf(TEAM_NAMES)).toEqual(['Les Jaunes'])
    expect(pointsField().value).toBe('0')
    expect(action().textContent).toBe('Appliquer les pénalités (1)')
    expect(action().disabled).toBe(true)
    expect(screen.getByText(REASON)).toBeTruthy()
    fireEvent.click(action())
    expect(game.teamPointsBatch).not.toHaveBeenCalled()
  })

  it('(19) saisie manuelle −1 → bouton actif ; saisie 2 → « Attribuer les points » actif ; puis bonne réponse → points de la question', () => {
    const { game } = show()
    fireEvent.click(answerBtn('La Loire'))
    setPoints(-1)
    expect(action().disabled).toBe(false)
    expect(action().textContent).toMatch(/^Appliquer les pénalités/)
    expect(screen.queryByText(REASON)).toBeNull()
    setPoints(2)
    expect(action().disabled).toBe(false)
    expect(action().textContent).toBe('Attribuer les points (1)')
    fireEvent.click(answerBtn('La Seine'))
    expect(pointsField().value).toBe('3')
    expect(action().textContent).toBe('Attribuer les points (3)')
    expect(game.teamPointsBatch).not.toHaveBeenCalled()
  })

  it('(11) le libellé « Attribuer les points » revient après la bonne réponse ou une nouvelle question', () => {
    const utils = show({ gameState: { question: QCM({ QCM_WRONG_PENALTY: -2 }) } })
    fireEvent.click(answerBtn('La Loire'))
    expect(action().textContent).toMatch(/^Appliquer les pénalités/)
    fireEvent.click(answerBtn('La Seine'))
    expect(action().textContent).toMatch(/^Attribuer les points/)
    fireEvent.click(answerBtn('La Loire'))
    expect(action().textContent).toMatch(/^Appliquer les pénalités/)
    // nouvelle question : sélection (STOPPED) puis REVEAL, comme en partie
    const q2 = QCM({ ID: '12', QCM_WRONG_PENALTY: -2 })
    const base = { ...utils.game, questions: { 12: { ...q2, STATUS: 'REVEALED', ORDER: 1 } } }
    reshow(utils, base, { gameState: { question: q2, phase: 'STOPPED' } })
    const card = screen.queryByTestId('question-card-12')
    if (card) fireEvent.click(card)
    reshow(utils, base, { gameState: { question: q2, phase: 'REVEALED' } })
    expect(queryAction()?.textContent ?? 'Attribuer les points (0)').toMatch(/^Attribuer les points/)
    expect(pointsField().value).toBe('3')
  })
})

describe('Montant négatif et tablettes de l\'anim (scénario 12)', () => {
  beforeEach(() => { useGame.mockReset() })
  const creditCalls = (game) => game.sendMessage.mock.calls.filter((c) => c[0] === 'SET_CREDIT_POINTS').map((c) => c[1].POINTS)

  it('Critical — pendant la pénalité (−3, 0 ou saisie négative) AUCUN SET_CREDIT_POINTS ≤ 0 n’est envoyé ; le retour positif l’est (envoi différé de 400 ms)', () => {
    vi.useFakeTimers()
    try {
      const { game } = show({ gameState: { question: QCM({ QCM_WRONG_PENALTY: -3 }) } })
      fireEvent.click(answerBtn('La Loire'))
      setPoints(-1)
      act(() => { vi.advanceTimersByTime(600) })
      setPoints(0)
      act(() => { vi.advanceTimersByTime(600) })
      expect(creditCalls(game).filter((p) => !(p > 0))).toEqual([])
      fireEvent.click(answerBtn('La Seine'))
      setPoints(4)
      act(() => { vi.advanceTimersByTime(600) })
      expect(creditCalls(game).filter((p) => !(p > 0))).toEqual([])
      expect(creditCalls(game)).toContain(4)
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('Base du barème (scénario 27)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — « Points » réglé à 6 AVANT le REVEAL : à l’entrée en REVEALED le champ vaut 6 ; mauvaise réponse (−2) puis bonne réponse → retour à 6, pas à 3', () => {
    show({ gameState: { question: QCM({ QCM_WRONG_PENALTY: -2 }) } }, () => setPoints(6))
    expect(pointsField().value).toBe('6')
    fireEvent.click(answerBtn('La Loire'))
    expect(pointsField().value).toBe('-2')
    fireEvent.click(answerBtn('La Seine'))
    expect(pointsField().value).toBe('6')
    expect(action().textContent).toBe('Attribuer les points (3)')
  })

  it('le lot envoyé après réglage à 6 porte 6 (et non les points de la question)', () => {
    const { game } = show({}, () => setPoints(6))
    fireEvent.click(box('Les Bleus'))
    fireEvent.click(action())
    expect(game.teamPointsBatch.mock.calls[0][1]).toEqual([{ team: 'Les Bleus', points: 6 }])
  })

  it('(28) changement de question : « Points » reprend la nouvelle base', () => {
    const utils = show()
    const q2 = QCM({ ID: '12', POINTS: '5' })
    useGame.mockReturnValue(mk({ gameState: { question: q2, phase: 'STOPPED' } }))
    utils.rerender(<GamePage />)
    const card = screen.queryByTestId('question-card-12')
    if (card) fireEvent.click(card)
    useGame.mockReturnValue(mk({ gameState: { question: q2 } }))
    utils.rerender(<GamePage />)
    expect(pointsField().value).toBe('5')
  })
})

describe('Commutateur « Tenir compte des indices » (scénarios 22 à 26, 28, 29, 31)', () => {
  beforeEach(() => { useGame.mockReset() })
  const HINTS = { gameState: { question: QCM({ QCM_HINTS_ENABLED: true }), qcmInvalidated: ['RED', 'BLUE'] } }
  const previews = () => Object.fromEntries(TEAM_NAMES.map((n) => [n, box(n).closest('[class*="team"], div')?.textContent.match(/[+−-]\d+/)?.[0] ?? null]))
  const sendAll = (game) => {
    fireEvent.click(screen.getByRole('button', { name: 'Tout sélectionner' }))
    fireEvent.click(action())
    return Object.fromEntries(game.teamPointsBatch.mock.calls[0][1].map((e) => [e.team, e.points]))
  }

  it('Critical (22, 31) — visible, activé par défaut, nom accessible ; bonne réponse : lot +3 / +2 / +1 (Bleus 0 indice, Rouges 1, Verts 2), Jaunes (mauvaise couleur) hors sélection', () => {
    const { game } = show(HINTS)
    const sw = hintSwitch()
    expect(sw).not.toBeNull()
    expect(isOn(sw)).toBe('true')
    fireEvent.click(answerBtn('La Seine'))
    expect(checkedOf(TEAM_NAMES).sort()).toEqual(['Les Bleus', 'Les Rouges', 'Les Verts'])
    fireEvent.click(action())
    const sent = Object.fromEntries(game.teamPointsBatch.mock.calls[0][1].map((e) => [e.team, e.points]))
    expect(sent).toEqual({ 'Les Bleus': 3, 'Les Rouges': 2, 'Les Verts': 1 })
  })

  it('(23) désactivé : +3 partout ; réactivé : +3 / +2 / +1', () => {
    const { game } = show(HINTS)
    fireEvent.click(answerBtn('La Seine'))
    fireEvent.click(hintSwitch())
    expect(isOn(hintSwitch())).toBe('false')
    fireEvent.click(action())
    expect(game.teamPointsBatch.mock.calls[0][1].map((e) => e.points)).toEqual([3, 3, 3])
  })

  it('(23) la ligne 1 dit « montant uniforme » quand le commutateur est désactivé, « ajusté selon les indices de chaque équipe » quand il est activé', () => {
    show(HINTS)
    fireEvent.click(answerBtn('La Seine'))
    expect(screen.getByText(/barème de la question, ajusté selon les indices de chaque équipe/)).toBeTruthy()
    fireEvent.click(hintSwitch())
    expect(screen.getByText(/montant uniforme/)).toBeTruthy()
  })

  it('Critical (24) — champ forcé à 6 après le REVEAL : activé → 6 / 4 / 2 ; désactivé → 6 partout ; libellé « montant saisi, ajusté… »', () => {
    const { game } = show(HINTS)
    fireEvent.click(answerBtn('La Seine'))
    setPoints(6)
    expect(screen.getByText(/montant saisi, ajusté selon les indices de chaque équipe/)).toBeTruthy()
    fireEvent.click(action())
    expect(game.teamPointsBatch.mock.calls[0][1].map((e) => e.points)).toEqual([6, 4, 2])
  })

  it('(24) forcé à 6, désactivé → 6 partout', () => {
    const { game } = show(HINTS)
    fireEvent.click(answerBtn('La Seine'))
    setPoints(6)
    fireEvent.click(hintSwitch())
    fireEvent.click(action())
    expect(game.teamPointsBatch.mock.calls[0][1].map((e) => e.points)).toEqual([6, 6, 6])
  })

  it('(30) l’aperçu affiché sur chaque carte est exactement le montant envoyé', () => {
    const { game } = show(HINTS)
    fireEvent.click(answerBtn('La Seine'))
    const shown = {}
    for (const n of ['Les Bleus', 'Les Rouges', 'Les Verts']) {
      const m = box(n).closest('div').parentElement.textContent.match(/\+(\d+)/)
      shown[n] = m ? Number(m[1]) : null
    }
    fireEvent.click(action())
    const sent = Object.fromEntries(game.teamPointsBatch.mock.calls[0][1].map((e) => [e.team, e.points]))
    expect(shown).toEqual(sent)
  })

  it.each([
    ['QCM sans indices activés', { gameState: { question: QCM({ QCM_HINTS_ENABLED: false }), qcmInvalidated: ['RED'] } }],
    ['QCM avec indices mais aucun révélé', { gameState: { question: QCM({ QCM_HINTS_ENABLED: true }), qcmInvalidated: [] } }],
    ['SPEEDY', { gameState: { question: { ID: '20', TYPE: 'SPEEDY', QUESTION: 'Q', ANSWER: 'A', POINTS: '3', STATUS: 'REVEALED' }, qcmInvalidated: ['RED'] } }],
    ['ARDOISE', { gameState: { question: { ID: '21', TYPE: 'ARDOISE', QUESTION: 'Q', ANSWER: 'A', POINTS: '3', ARDOISE_KEYBOARD_TYPE: 'AZERTY', STATUS: 'REVEALED' }, qcmInvalidated: ['RED'] } }],
    ['RAFALE', { gameState: { question: { ID: '22', TYPE: 'RAFALE', QUESTION: 'Q', ANSWER: 'A', POINTS: '3', STATUS: 'REVEALED' }, qcmInvalidated: ['RED'] } }],
  ])('Critical (26) — commutateur masqué : %s', (_n, over) => {
    show(over)
    expect(hintSwitch()).toBeNull()
  })

  it('Critical (26) — montant ≤ 0 : commutateur masqué, pénalité −2 uniforme quel que soit l’état précédent du commutateur', () => {
    const { game } = show({ gameState: { question: QCM({ QCM_HINTS_ENABLED: true, QCM_WRONG_PENALTY: -2 }), qcmInvalidated: ['RED', 'BLUE'] } })
    fireEvent.click(hintSwitch()) // désactivé avant la pénalité
    fireEvent.click(answerBtn('La Loire'))
    expect(hintSwitch()).toBeNull()
    expect(screen.getByText(/pénalité de la question, uniforme/)).toBeTruthy()
    fireEvent.click(action())
    expect(game.teamPointsBatch.mock.calls[0][1]).toEqual([{ team: 'Les Jaunes', points: -2 }])
  })

  it('(26 / option A) mauvaise réponse avec 0, 1 ou 2 indices révélés, y compris une réponse éliminée : même pénalité', () => {
    for (const invalidated of [[], ['RED'], ['RED', 'BLUE']]) {
      useGame.mockReset()
      const { game, unmount } = show({ gameState: { question: QCM({ QCM_HINTS_ENABLED: true, QCM_WRONG_PENALTY: -2 }), qcmInvalidated: invalidated } })
      fireEvent.click(answerBtn('La Loire')) // « La Loire » (RED) : éliminée dès que RED est révélée
      fireEvent.click(action())
      expect(game.teamPointsBatch.mock.calls[0][1], JSON.stringify(invalidated)).toEqual([{ team: 'Les Jaunes', points: -2 }])
      unmount()
    }
  })

  it('(28) le commutateur est remis à activé à chaque nouvelle question ; un clic sur une réponse ou une saisie ne le change pas', () => {
    const utils = show(HINTS)
    fireEvent.click(hintSwitch())
    expect(isOn(hintSwitch())).toBe('false')
    fireEvent.click(answerBtn('La Seine'))
    setPoints(5)
    expect(isOn(hintSwitch())).toBe('false')
    const q2 = QCM({ ID: '12', QCM_HINTS_ENABLED: true })
    useGame.mockReturnValue(mk({ gameState: { question: q2, phase: 'STOPPED', qcmInvalidated: [] } }))
    utils.rerender(<GamePage />)
    const card = screen.queryByTestId('question-card-12')
    if (card) fireEvent.click(card)
    useGame.mockReturnValue(mk({ gameState: { question: q2, qcmInvalidated: ['RED'] } }))
    utils.rerender(<GamePage />)
    expect(isOn(hintSwitch())).toBe('true')
  })

  it('Critical (29) — niveau joueur : mêmes montants par joueur (HINTS_AT_BUZZ de chacun), lot BUMPER_POINTS_BATCH = aperçus', () => {
    const { game } = show({ gameState: { question: QCM({ QCM_HINTS_ENABLED: true, POINTS_TARGET: 'PLAYER' }), qcmInvalidated: ['RED', 'BLUE'] } })
    expect(hintSwitch()).not.toBeNull()
    fireEvent.click(answerBtn('La Seine'))
    fireEvent.click(action())
    const sent = Object.fromEntries(game.bumperPointsBatch.mock.calls[0][1].map((e) => [e.id, e.points]))
    expect(sent).toEqual({ vj0: 3, vj1: 2, vj3: 1 })
  })

  it('(29) joueur sans buzz sélectionné à la main : coefficient des indices actuellement révélés', () => {
    const silent = [['Les Bleus', 'Paul', 'GREEN', 0], ['Les Rouges', 'Mamie', '', 0]]
    const g = mk({ gameState: { question: QCM({ QCM_HINTS_ENABLED: true, POINTS_TARGET: 'PLAYER' }), qcmInvalidated: ['RED', 'BLUE'] } })
    g.bumpers = { ...bumpers(silent), vj1: { ...bumpers(silent).vj1, TIME: 0 } }
    useGame.mockReturnValue({ ...g, gameState: { ...g.gameState, phase: 'STOPPED' } })
    const utils = render(<GamePage />)
    const card = screen.queryByTestId('question-card-9')
    if (card) fireEvent.click(card)
    useGame.mockReturnValue(g)
    utils.rerender(<GamePage />)
    fireEvent.click(box('Mamie'))
    fireEvent.click(action())
    expect(g.bumperPointsBatch.mock.calls[0][1]).toEqual([{ id: 'vj1', points: 1 }]) // 3 × 0,33 → 1
  })
})

describe('Résultat du serveur : messages de la maquette (scénario 10, 16)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('niveau équipe : « ✓ Points attribués à 3 équipes. » puis sélection vidée', () => {
    const utils = show()
    fireEvent.click(answerBtn('La Seine'))
    const g = { ...utils.game, teamPointsBatchResult: { status: 'APPLIED', questionId: '9', applied: ['Les Bleus', 'Les Rouges', 'Les Verts'].map((team) => ({ team, points: 3 })), skipped: [], teams: [] } }
    reshow(utils, g, {})
    expect(screen.getByText(/Points attribués à 3 équipes/)).toBeTruthy()
    expect(checkedOf(TEAM_NAMES)).toEqual([])
  })

  it('niveau joueur : « Points attribués à 2 joueurs. », et « Pénalité −2 appliquée à 1 joueur. »', () => {
    const utils = show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) } })
    const g = { ...utils.game, bumperPointsBatchResult: { status: 'APPLIED', questionId: '9', applied: [{ id: 'vj0', points: 3 }, { id: 'vj1', points: 3 }], skipped: [], players: [] } }
    const next = reshow(utils, g, {})
    expect(screen.getByText(/Points attribués à 2 joueurs/)).toBeTruthy()
    reshow(utils, next, { bumperPointsBatchResult: { status: 'APPLIED', questionId: '9', applied: [{ id: 'vj2', points: -2 }], skipped: [], players: [] } })
    expect(screen.getByText(/Pénalité −2 appliquée à 1 joueur/)).toBeTruthy()
  })

  it('Critical — UNKNOWN_PLAYER : joueurs inconnus retirés de la sélection, message', () => {
    const utils = show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) } })
    fireEvent.click(answerBtn('La Seine'))
    expect(box('Paul').checked).toBe(true)
    reshow(utils, utils.game, { bumperPointsBatchResult: { status: 'REFUSED', reason: 'UNKNOWN_PLAYER', questionId: '9', applied: [], skipped: [], players: ['vj0'] } })
    expect(screen.getByText(/Joueur introuvable/)).toBeTruthy()
    expect(box('Paul').checked).toBe(false)
    expect(box('Mamie').checked).toBe(true)
  })

  it('envoi impossible au niveau joueur : « Connexion perdue », bouton réactivé', () => {
    const { game } = show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) }, bumperPointsBatch: vi.fn(() => false) })
    fireEvent.click(box('Paul'))
    fireEvent.click(action())
    expect(screen.getByText(/Connexion perdue/)).toBeTruthy()
    expect(action().disabled).toBe(false)
    expect(game.bumperPointsBatch).toHaveBeenCalledTimes(1)
  })

  it('Critical — double clic au niveau joueur : bouton désactivé pendant l’envoi, un seul lot', () => {
    const { game } = show({ gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) } })
    fireEvent.click(box('Paul'))
    fireEvent.click(action())
    expect(action().disabled).toBe(true)
    fireEvent.click(action())
    expect(game.bumperPointsBatch).toHaveBeenCalledTimes(1)
  })
})

describe('Niveau joueur : équipe déjà créditée au niveau équipe (revue #254, scénario 7)', () => {
  beforeEach(() => { useGame.mockReset() })
  const LEVEL_PLAYER = { gameState: { question: QCM({ POINTS_TARGET: 'PLAYER' }) } }
  const TEAM_CREDIT = { awardedTeams: { 'Les Bleus': { POINTS: 2, TIMESTAMP: 1, TEAM_LEVEL: true } } }

  it('Critical — les joueurs d’une équipe créditée au niveau équipe sont grisés, non cochables, « ✓ équipe déjà créditée » ; « Tout sélectionner » les ignore', () => {
    show({ ...LEVEL_PLAYER, ...TEAM_CREDIT })
    expect(queryBox('Paul').disabled).toBe(true)
    expect(queryBox('Paul').checked).toBe(false)
    expect(screen.getByText(/✓ équipe déjà créditée/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Tout sélectionner' }))
    expect(checkedOf(PLAYER_NAMES).sort()).toEqual(['Jade', 'Mamie', 'Vera'])
  })

  it('une équipe créditée seulement par ses joueurs (TEAM_LEVEL false) ne grise pas ses autres joueurs', () => {
    show({ ...LEVEL_PLAYER, awardedTeams: { 'Les Bleus': { POINTS: 2, TIMESTAMP: 1, TEAM_LEVEL: false } }, awardedPlayers: { vj9: { POINTS: 2, TIMESTAMP: 1, TEAM: 'Les Bleus' } } })
    expect(queryBox('Paul').disabled).toBe(false)
  })

  it('résultat : « N joueurs ignorés (équipe déjà créditée) » pour TEAM_ALREADY_AWARDED', () => {
    const utils = show(LEVEL_PLAYER)
    reshow(utils, utils.game, { bumperPointsBatchResult: { status: 'APPLIED', questionId: '9', applied: [{ id: 'vj1', points: 3 }], skipped: [{ id: 'vj0', reason: 'TEAM_ALREADY_AWARDED' }, { id: 'vj3', reason: 'TEAM_ALREADY_AWARDED' }], players: [] } })
    expect(screen.getByText(/2 joueurs ignorés \(équipe déjà créditée\)/)).toBeTruthy()
  })
})

