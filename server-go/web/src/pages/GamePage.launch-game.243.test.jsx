import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, within, act } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #243 (v11.2, lot V2-1) — bouton « LANCER LA PARTIE » + popup de purge, selon la maquette VALIDÉE
// admin_new_game__purge_popup_details (D1 : option « Questions », D2 : « Joueurs des tablettes » coche et
// verrouille « Scores »). Contrats : websocket-actions.md §NEW_GAME / §NEW_GAME_REFUSED.
// Boîte noire par libellés de la maquette et rôles ARIA.

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

vi.mock('./GamePage.css', () => ({}))


const TEAMS = Object.fromEntries(Array.from({ length: 10 }, (_, i) => [`Équipe ${i + 1}`, { NAME: `Équipe ${i + 1}`, SCORE: 0, COLOR: [10 * i, 50, 90] }]))
const BUMPERS = {
  ...Object.fromEntries(Array.from({ length: 10 }, (_, i) => [`vj${i}`, { NAME: `VJ${i}`, TEAM: `Équipe ${i + 1}`, IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: i < 8, SCORE: 0 }])),
  buz1: { NAME: 'Buzzer 1', TEAM: 'Équipe 1', IS_VIRTUAL: false, CONNECTED: true, SCORE: 0 },
  buz2: { NAME: 'Buzzer 2', TEAM: 'Équipe 2', IS_VIRTUAL: false, CONNECTED: true, SCORE: 0 },
}
const QUESTIONS = Object.fromEntries(Array.from({ length: 30 }, (_, i) => [`${i + 1}`, { ID: `${i + 1}`, STATUS: i < 12 ? 'REVEALED' : 'AVAILABLE', ORDER: i + 1 }]))

const mk = (over = {}) => ({
  gameState: { phase: 'REVEALED', question: { ID: '2', STATUS: 'REVEALED' }, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], ...(over.gameState || {}) },
  teams: TEAMS, bumpers: BUMPERS, questions: QUESTIONS,
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
  setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
  sendMessage: vi.fn(), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
  ...over,
})
const open = (over) => {
  const game = mk(over)
  useGame.mockReturnValue(game)
  const utils = render(<GamePage />)
  fireEvent.click(screen.getByText('LANCER LA PARTIE'))
  return { game, ...utils, dlg: screen.getByRole('dialog') }
}
const box = (dlg, name) => within(dlg).getByRole('checkbox', { name: new RegExp(`^${name}`) })
const goBtn = (dlg) => within(dlg).getByRole('button', { name: /^Lancer (la partie|sans rien effacer)$/ })

describe('Popup « Lancer la partie » (#243)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — rien n’est envoyé à l’ouverture ni sur « Annuler » ; la popup est un dialogue modal accessible (titre + description)', () => {
    const { game, dlg } = open()
    expect(game.newGame).not.toHaveBeenCalled()
    expect(dlg.getAttribute('aria-modal')).toBe('true')
    expect(dlg.getAttribute('aria-labelledby')).toBeTruthy()
    expect(document.getElementById(dlg.getAttribute('aria-labelledby')).textContent).toBe('Lancer la partie')
    expect(dlg.getAttribute('aria-describedby')).toBeTruthy()

    fireEvent.click(within(dlg).getByRole('button', { name: 'Annuler' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(game.newGame).not.toHaveBeenCalled()
  })

  it('Escape ferme la popup sans rien envoyer', () => {
    const { game, dlg } = open()
    fireEvent.keyDown(dlg, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(game.newGame).not.toHaveBeenCalled()
  })

  it('Critical — cases à l’ouverture : Scores et Questions cochées, Joueurs et Équipes décochées ; Scores non verrouillée', () => {
    const { dlg } = open()
    expect(box(dlg, 'Scores').checked).toBe(true)
    expect(box(dlg, 'Questions').checked).toBe(true)
    expect(box(dlg, 'Joueurs des tablettes').checked).toBe(false)
    expect(box(dlg, 'Équipes').checked).toBe(false)
    expect(box(dlg, 'Scores').disabled).toBe(false)
  })

  it('compteurs dérivés de l’état : équipes, joueurs, questions jouées sur total, tablettes connectées', () => {
    const { dlg } = open()
    const t = dlg.textContent
    expect(t).toMatch(/Scores\s*\(10 équipes, 12 joueurs\)/)
    expect(t).toMatch(/Questions\s*\(12 jouées sur 30\)/)
    expect(t).toMatch(/Joueurs des tablettes\s*\(\d+ inscrits, 8 connectés\)/)
    expect(t).toMatch(/Joueurs des tablettes\s*\(10 inscrits, 8 connectés\)/) // « N inscrits, M connectés » (harmonisation, revue #243)
    expect(t).toMatch(/Équipes\s*\(10\)/)
  })

  it('Critical (D2) — cocher « Joueurs des tablettes » coche ET verrouille « Scores », avec la phrase d’explication', () => {
    const { dlg } = open()
    fireEvent.click(box(dlg, 'Scores')) // décocher d'abord les scores
    expect(box(dlg, 'Scores').checked).toBe(false)

    fireEvent.click(box(dlg, 'Joueurs des tablettes'))

    expect(box(dlg, 'Scores').checked).toBe(true)
    expect(box(dlg, 'Scores').disabled).toBe(true)
    expect(dlg.textContent).toMatch(/🔒 Obligatoire quand les joueurs des tablettes sont supprimés : leurs points ne peuvent pas être conservés sans eux\./)
  })

  it('Critical (D2) — décocher « Joueurs des tablettes » rend « Scores » de nouveau modifiable (elle reste cochée)', () => {
    const { dlg } = open()
    fireEvent.click(box(dlg, 'Joueurs des tablettes'))
    fireEvent.click(box(dlg, 'Joueurs des tablettes'))
    expect(box(dlg, 'Scores').disabled).toBe(false)
    expect(box(dlg, 'Scores').checked).toBe(true)
    fireEvent.click(box(dlg, 'Scores'))
    expect(box(dlg, 'Scores').checked).toBe(false)
  })

  it('Critical — la confirmation par défaut envoie les QUATRE booléens (Scores + Questions)', () => {
    const { game, dlg } = open()
    fireEvent.click(goBtn(dlg))
    expect(game.newGame).toHaveBeenCalledTimes(1)
    expect(game.newGame).toHaveBeenCalledWith({ purgeScores: true, purgePlayers: false, purgeTeams: false, resetQuestions: true })
  })

  it('Critical (D2) — jamais de payload « joueurs sans scores » : Joueurs coché ⇒ purgeScores toujours true', () => {
    const { game, dlg } = open()
    fireEvent.click(box(dlg, 'Scores')) // scores décochés…
    fireEvent.click(box(dlg, 'Joueurs des tablettes')) // …puis joueurs cochés
    fireEvent.click(goBtn(dlg))
    const sent = game.newGame.mock.calls[0][0]
    expect(sent.purgePlayers).toBe(true)
    expect(sent.purgeScores).toBe(true)
  })

  it('tout coché : les quatre à true ; seule « Questions » décochée : resetQuestions false', () => {
    const a = open()
    fireEvent.click(box(a.dlg, 'Joueurs des tablettes'))
    fireEvent.click(box(a.dlg, 'Équipes'))
    fireEvent.click(goBtn(a.dlg))
    expect(a.game.newGame).toHaveBeenCalledWith({ purgeScores: true, purgePlayers: true, purgeTeams: true, resetQuestions: true })
    a.unmount()

    const b = open()
    fireEvent.click(box(b.dlg, 'Questions'))
    fireEvent.click(goBtn(b.dlg))
    expect(b.game.newGame).toHaveBeenCalledWith({ purgeScores: true, purgePlayers: false, purgeTeams: false, resetQuestions: false })
  })

  it('rien de coché : « Lancer sans rien effacer » et quatre booléens à false', () => {
    const { game, dlg } = open()
    fireEvent.click(box(dlg, 'Scores'))
    fireEvent.click(box(dlg, 'Questions'))
    const btn = within(dlg).getByRole('button', { name: 'Lancer sans rien effacer' })
    fireEvent.click(btn)
    expect(game.newGame).toHaveBeenCalledWith({ purgeScores: false, purgePlayers: false, purgeTeams: false, resetQuestions: false })
  })

  it('avertissement rouge (role=alert) et bouton rouge dès qu’une option destructrice est cochée, avec les comptes', () => {
    const { dlg } = open()
    expect(within(dlg).queryByRole('alert')).toBeNull()
    expect(goBtn(dlg).className).not.toMatch(/danger/)

    fireEvent.click(box(dlg, 'Joueurs des tablettes'))
    expect(within(dlg).getByRole('alert').textContent).toMatch(/Irréversible : les 10 joueurs des tablettes seront supprimés\./)
    expect(goBtn(dlg).className).toMatch(/danger/)

    fireEvent.click(box(dlg, 'Équipes'))
    expect(within(dlg).getByRole('alert').textContent).toMatch(/les 10 joueurs des tablettes et les 10 équipes seront supprimés/)
  })

  it('« Conservé : … » liste ce qui n’est pas coché ; « Dans tous les cas » rappelle l’arrêt de la question', () => {
    const { dlg } = open()
    expect(dlg.textContent).toMatch(/Conservé : joueurs des tablettes, équipes\./)
    fireEvent.click(box(dlg, 'Questions'))
    expect(dlg.textContent).toMatch(/Conservé : questions déjà jouées, joueurs des tablettes, équipes\./)
    expect(dlg.textContent).toMatch(/Dans tous les cas : la question en cours s'arrête\./)
  })

  it('avertissement « question en cours » seulement pendant une question', () => {
    const idle = open()
    expect(idle.dlg.textContent).not.toMatch(/Une question est en cours/)
    idle.unmount()
    useGame.mockReset()
    const busy = open({ gameState: { phase: 'STARTED' } })
    expect(busy.dlg.textContent).toMatch(/⚠ Une question est en cours : elle sera interrompue\./)
  })

  it('Critical — le bouton de confirmation est désactivé dès l’envoi (pas de double NEW_GAME) et « Annuler » aussi', () => {
    const { game, dlg } = open()
    const btn = goBtn(dlg)
    fireEvent.click(btn)
    fireEvent.click(btn)
    expect(game.newGame).toHaveBeenCalledTimes(1)
    expect(goBtn(screen.getByRole('dialog')).disabled).toBe(true)
  })

  it('la popup se ferme quand la phase passe à NEW_GAME après l’envoi', () => {
    const { game, rerender, dlg } = open()
    fireEvent.click(goBtn(dlg))
    useGame.mockReturnValue(mk({ gameState: { phase: 'NEW_GAME', question: null }, newGame: game.newGame }))
    rerender(<GamePage />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('NEW_GAME_REFUSED : message d’erreur dans la popup, qui reste ouverte, bouton de nouveau actif', () => {
    const { game, rerender, dlg } = open()
    fireEvent.click(goBtn(dlg))
    useGame.mockReturnValue(mk({ newGame: game.newGame, newGameRefused: { reason: 'PLAYERS_REQUIRE_SCORES' } }))
    rerender(<GamePage />)

    const d = screen.getByRole('dialog')
    expect(within(d).getAllByRole('alert').map((a) => a.textContent).join(' ')).toMatch(/Lancement refusé/)
    expect(goBtn(d).disabled).toBe(false)
  })

  it('accessibilité : chaque case a un nom accessible (libellé), focus placé dans la popup à l’ouverture, retour du focus sur le bouton à la fermeture', () => {
    const { dlg } = open()
    expect(dlg.contains(document.activeElement)).toBe(true)
    for (const n of ['Scores', 'Questions', 'Joueurs des tablettes', 'Équipes']) expect(box(dlg, n)).toBeTruthy()
    fireEvent.click(within(dlg).getByRole('button', { name: 'Annuler' }))
    expect(document.activeElement.textContent).toBe('LANCER LA PARTIE')
  })

  it('le libellé historique « NOUVELLE PARTIE » a disparu de la page', () => {
    useGame.mockReturnValue(mk())
    render(<GamePage />)
    expect(screen.queryByText('NOUVELLE PARTIE')).toBeNull()
  })
})

describe('Popup « Lancer la partie » — robustesse (revue #243)', () => {
  beforeEach(() => { useGame.mockReset(); vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('(f) Critical — si l’envoi échoue (WebSocket fermée : newGame retourne false), rien n’est « envoyé » : message « Connexion perdue », boutons actifs', () => {
    const newGame = vi.fn(() => false)
    const { dlg } = open({ newGame })
    fireEvent.click(goBtn(dlg))

    const d = screen.getByRole('dialog')
    expect(d.textContent).toMatch(/Connexion perdue/)
    expect(goBtn(d).disabled).toBe(false)
    expect(within(d).getByRole('button', { name: 'Annuler' }).disabled).toBe(false)
    act(() => { vi.advanceTimersByTime(10000) })
    expect(screen.getByRole('dialog').textContent).not.toMatch(/Pas de réponse du serveur/) // jamais parti : pas de garde de 5 s
  })

  it('(g) Critical — NEW_GAME jamais confirmé : au bout de 5 s « Pas de réponse du serveur » et les boutons sont réactivés', () => {
    const { dlg } = open({ newGame: vi.fn(() => true) })
    fireEvent.click(goBtn(dlg))
    expect(goBtn(screen.getByRole('dialog')).disabled).toBe(true)

    act(() => { vi.advanceTimersByTime(4900) })
    expect(goBtn(screen.getByRole('dialog')).disabled).toBe(true) // pas avant 5 s
    act(() => { vi.advanceTimersByTime(200) })

    const d = screen.getByRole('dialog')
    expect(d.textContent).toMatch(/Pas de réponse du serveur/)
    expect(goBtn(d).disabled).toBe(false)
    expect(within(d).getByRole('button', { name: 'Annuler' }).disabled).toBe(false)
  })

  it('(h) NEW_GAME_REFUSED { ENTRACTE_ACTIVE } : message « Impossible pendant une entracte », popup ouverte, boutons actifs', () => {
    const { game, rerender, dlg } = open({ newGame: vi.fn(() => true) })
    fireEvent.click(goBtn(dlg))
    useGame.mockReturnValue(mk({ newGame: game.newGame, newGameRefused: { reason: 'ENTRACTE_ACTIVE' } }))
    rerender(<GamePage />)

    const d = screen.getByRole('dialog')
    expect(within(d).getAllByRole('alert').map((a) => a.textContent).join(' ')).toMatch(/Impossible pendant une entracte : sortez d'abord de l'entracte/)
    expect(goBtn(d).disabled).toBe(false)
  })

  it('(i) Critical — deuxième lancement alors que la phase vaut DÉJÀ NEW_GAME : la popup ne se ferme pas avant un changement observé', () => {
    const base = { gameState: { phase: 'NEW_GAME', question: null }, newGame: vi.fn(() => true) }
    const first = mk(base)
    useGame.mockReturnValue(first)
    const { rerender } = render(<GamePage />)
    fireEvent.click(screen.getByText('LANCER LA PARTIE'))
    fireEvent.click(goBtn(screen.getByRole('dialog')))

    // même état rediffusé (mêmes objets) : rien n'est observé → la popup reste ouverte
    rerender(<GamePage />)
    expect(screen.queryByRole('dialog')).not.toBeNull()

    // données renouvelées après la purge (nouvel objet bumpers/questions) → lancement observé : fermeture
    useGame.mockReturnValue(mk({ ...base, bumpers: { ...BUMPERS }, questions: { ...QUESTIONS } }))
    rerender(<GamePage />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('(j) compteurs « N inscrits, M connectés » : inscrits = tous les VJoueurs, connectés = ceux en ligne', () => {
    const { dlg } = open()
    expect(dlg.textContent).toMatch(/Joueurs des tablettes\s*\(10 inscrits, 8 connectés\)/)
  })

  it('refus pendant une entracte : le message ne dépend pas de la case cochée (aucune case n’est modifiée par le refus)', () => {
    const { game, rerender, dlg } = open({ newGame: vi.fn(() => true) })
    fireEvent.click(box(dlg, 'Équipes'))
    fireEvent.click(goBtn(dlg))
    useGame.mockReturnValue(mk({ newGame: game.newGame, newGameRefused: { reason: 'ENTRACTE_ACTIVE' } }))
    rerender(<GamePage />)
    expect(box(screen.getByRole('dialog'), 'Équipes').checked).toBe(true) // sélection conservée pour relancer
  })

  it('(k) Critical — réponse du serveur APRÈS la garde de 5 s (à 6 s) : la popup se ferme et aucun second NEW_GAME n’est envoyé', () => {
    const newGame = vi.fn(() => true)
    const { dlg, rerender } = open({ newGame })
    fireEvent.click(goBtn(dlg))
    act(() => { vi.advanceTimersByTime(5100) })
    expect(screen.getByRole('dialog').textContent).toMatch(/Pas de réponse du serveur/) // la garde a débloqué la popup
    expect(newGame).toHaveBeenCalledTimes(1)

    act(() => { vi.advanceTimersByTime(900) }) // t = 6 s : le serveur répond enfin
    useGame.mockReturnValue(mk({ gameState: { phase: 'NEW_GAME', question: null }, newGame }))
    rerender(<GamePage />)

    expect(screen.queryByRole('dialog')).toBeNull()
    expect(newGame).toHaveBeenCalledTimes(1)
  })
})
