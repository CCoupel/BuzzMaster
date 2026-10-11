import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, within, act } from '@testing-library/react'
import { LaunchGamePopup, getLaunchCounts } from './LaunchGameDialog'

// #243 — popup « Lancer la partie » (maquette admin_new_game__purge_popup_details).

const TEAMS = Object.fromEntries(Array.from({ length: 10 }, (_, i) => [`E${i}`, {}]))
const BUMPERS = {
  ...Object.fromEntries(Array.from({ length: 10 }, (_, i) => [`v${i}`, { IS_VPLAYER: true, TEAM: `E${i}`, CONNECTED: true }])),
  ...Object.fromEntries(Array.from({ length: 4 }, (_, i) => [`b${i}`, { IS_VPLAYER: false, TEAM: 'E0', CONNECTED: true }])),
}
const QUESTIONS = Object.fromEntries(Array.from({ length: 30 }, (_, i) => [`${i}`, { ID: `${i}`, STATUS: i < 12 ? 'REVEALED' : 'AVAILABLE' }]))

// #253 : la popup est contrôlée par le parent (LaunchGameHost) — affichée tant qu'elle est montée ;
// `onClose` est appelé à l'annulation / Échap / fermeture après lancement.
const setup = (props = {}) => {
  const onLaunch = vi.fn()
  const onClearRefused = vi.fn()
  const onClose = vi.fn()
  const utils = render(
    <LaunchGamePopup onLaunch={onLaunch} onClose={onClose} teams={TEAMS} bumpers={BUMPERS} questions={QUESTIONS}
      phase="REVEALED" refused={null} onClearRefused={onClearRefused} {...props} />
  )
  return { onLaunch, onClearRefused, onClose, ...utils }
}
const box = (name) => screen.getByRole('checkbox', { name: new RegExp(name) })
const go = () => screen.getByRole('button', { name: /Lancer (la partie|sans rien effacer)/ })

describe('getLaunchCounts', () => {
  it('compte équipes, joueurs, tablettes, questions jouées', () => {
    expect(getLaunchCounts(TEAMS, BUMPERS, QUESTIONS)).toEqual({
      teams: 10, players: 14, vplayers: 10, vplayersConnected: 10, played: 12, questions: 30,
    })
  })
})

describe('LaunchGamePopup (#243, #253)', () => {
  it('affiche la popup (role dialog, aria) sans rien envoyer ; défaut = Scores + Questions cochées', () => {
    const { onLaunch } = setup()
    const dlg = screen.getByRole('dialog')
    expect(dlg.getAttribute('aria-modal')).toBe('true')
    expect(dlg.getAttribute('aria-labelledby')).toBeTruthy()
    expect(box('Scores').checked).toBe(true)
    expect(box('Questions').checked).toBe(true)
    expect(box('Joueurs des tablettes').checked).toBe(false)
    expect(box('Équipes').checked).toBe(false)
    expect(dlg.textContent).toMatch(/\(10 équipes, 14 joueurs\)/)
    expect(dlg.textContent).toMatch(/\(12 jouées sur 30\)/)
    expect(dlg.textContent).toMatch(/Conservé : joueurs des tablettes, équipes\./)
    expect(onLaunch).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(box('Scores')) // focus dans la popup
  })

  it('Joueurs des tablettes coche ET verrouille Scores, avec l\'explication', () => {
    setup()
    fireEvent.click(box('Joueurs des tablettes'))
    expect(box('Scores').checked).toBe(true)
    expect(box('Scores').disabled).toBe(true)
    expect(screen.getByRole('dialog').textContent).toMatch(/Obligatoire quand les joueurs des tablettes sont supprimés/)
    expect(screen.getByRole('alert').textContent).toMatch(/Irréversible : les 10 joueurs des tablettes seront supprimés\./)
    fireEvent.click(box('Joueurs des tablettes'))
    expect(box('Scores').disabled).toBe(false)
  })

  it('envoie les quatre options (tout coché) et désactive la confirmation (pas de double envoi)', () => {
    const { onLaunch } = setup()
    fireEvent.click(box('Joueurs des tablettes'))
    fireEvent.click(box('Équipes'))
    expect(screen.getByRole('alert').textContent).toMatch(/les 10 joueurs des tablettes et les 10 équipes seront supprimés/)
    fireEvent.click(go())
    expect(onLaunch).toHaveBeenCalledWith({ purgeScores: true, purgePlayers: true, purgeTeams: true, resetQuestions: true })
    expect(go().disabled).toBe(true)
    fireEvent.click(go())
    expect(onLaunch).toHaveBeenCalledTimes(1)
  })

  it('rien coché : « Lancer sans rien effacer » envoie quatre faux', () => {
    const { onLaunch } = setup()
    fireEvent.click(box('Scores'))
    fireEvent.click(box('Questions'))
    expect(go().textContent).toBe('Lancer sans rien effacer')
    fireEvent.click(go())
    expect(onLaunch).toHaveBeenCalledWith({ purgeScores: false, purgePlayers: false, purgeTeams: false, resetQuestions: false })
  })

  it('question en cours : avertissement ; hors question : absent', () => {
    setup({ phase: 'STARTED' })
    expect(screen.getByRole('dialog').textContent).toMatch(/Une question est en cours : elle sera interrompue\./)
  })

  it('Échap et Annuler appellent onClose sans envoyer', () => {
    const { onLaunch, onClose } = setup()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByText('Annuler'))
    expect(onClose).toHaveBeenCalledTimes(2)
    expect(onLaunch).not.toHaveBeenCalled()
  })

  it('NEW_GAME_REFUSED : message, la popup reste ouverte et la confirmation se réactive', () => {
    const onLaunch = vi.fn()
    const onClose = vi.fn()
    const props = { onLaunch, onClose, teams: TEAMS, bumpers: BUMPERS, questions: QUESTIONS, phase: 'REVEALED', onClearRefused: vi.fn() }
    const { rerender } = render(<LaunchGamePopup {...props} refused={null} />)
    fireEvent.click(go())
    expect(go().disabled).toBe(true)
    rerender(<LaunchGamePopup {...props} refused={{ reason: 'PLAYERS_REQUIRE_SCORES' }} />)
    const dlg = screen.getByRole('dialog')
    expect(within(dlg).getByText(/Lancement refusé/)).toBeTruthy()
    expect(go().disabled).toBe(false)
  })

  it('se ferme quand la phase passe à NEW_GAME après l\'envoi', () => {
    const onClose = vi.fn()
    const props = { onLaunch: vi.fn(), onClose, teams: TEAMS, bumpers: BUMPERS, questions: QUESTIONS, onClearRefused: vi.fn(), refused: null }
    const { rerender } = render(<LaunchGamePopup {...props} phase="REVEALED" />)
    fireEvent.click(go())
    rerender(<LaunchGamePopup {...props} phase="NEW_GAME" />)
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})


describe('LaunchGamePopup — jamais figée (#243, revue)', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())
  const onCloseSpy = vi.fn()
  beforeEach(() => onCloseSpy.mockClear())
  const base = { onClose: onCloseSpy, teams: TEAMS, bumpers: BUMPERS, questions: QUESTIONS, onClearRefused: vi.fn(), refused: null }

  it('WebSocket fermée (onLaunch retourne false) : « Connexion perdue — réessayez », popup ouverte, confirmation active', () => {
    render(<LaunchGamePopup {...base} onLaunch={vi.fn(() => false)} phase="REVEALED" />)
    fireEvent.click(go())
    expect(screen.getByRole('dialog').textContent).toMatch(/Connexion perdue — réessayez/)
    expect(go().disabled).toBe(false)
  })

  it('aucune réponse en 5 s : la popup se débloque avec « Pas de réponse du serveur — réessayez »', () => {
    render(<LaunchGamePopup {...base} onLaunch={vi.fn(() => true)} phase="REVEALED" />)
    fireEvent.click(go())
    expect(go().disabled).toBe(true)
    act(() => { vi.advanceTimersByTime(4900) })
    expect(go().disabled).toBe(true)
    act(() => { vi.advanceTimersByTime(200) })
    expect(go().disabled).toBe(false)
    expect(screen.getByRole('dialog').textContent).toMatch(/Pas de réponse du serveur — réessayez/)
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' }) // Échap de nouveau possible
    expect(onCloseSpy).toHaveBeenCalledTimes(1)
  })

  it('NEW_GAME_REFUSED ENTRACTE_ACTIVE : message dédié, popup ouverte', () => {
    const { rerender } = render(<LaunchGamePopup {...base} onLaunch={vi.fn(() => true)} phase="REVEALED" />)
    fireEvent.click(go())
    rerender(<LaunchGamePopup {...base} onLaunch={vi.fn(() => true)} phase="REVEALED" refused={{ reason: 'ENTRACTE_ACTIVE' }} />)
    expect(screen.getByRole('dialog').textContent).toMatch(/Lancement refusé par le serveur : impossible pendant une entracte\. Sortez d'abord de l'entracte\./)
    expect(go().disabled).toBe(false)
  })

  it('phase déjà NEW_GAME à l\'envoi : pas de fermeture immédiate ; fermeture au renouvellement des données', () => {
    const onLaunch = vi.fn(() => true)
    const { rerender } = render(<LaunchGamePopup {...base} onLaunch={onLaunch} phase="NEW_GAME" />)
    fireEvent.click(go())
    expect(onCloseSpy).not.toHaveBeenCalled()
    rerender(<LaunchGamePopup {...base} onLaunch={onLaunch} phase="NEW_GAME" questions={{ ...QUESTIONS }} />)
    expect(onCloseSpy).toHaveBeenCalledTimes(1)
  })

  it('réponse tardive (6 s, après la garde de 5 s) : la popup se ferme, aucun second NEW_GAME', () => {
    const onLaunch = vi.fn(() => true)
    const { rerender } = render(<LaunchGamePopup {...base} onLaunch={onLaunch} phase="REVEALED" />)
    fireEvent.click(go())
    act(() => { vi.advanceTimersByTime(5100) })
    expect(go().disabled).toBe(false) // débloquée par la garde
    rerender(<LaunchGamePopup {...base} onLaunch={onLaunch} phase="NEW_GAME" />) // réponse du serveur à 6 s
    expect(onCloseSpy).toHaveBeenCalledTimes(1) // fermée par l'hôte, pas de second envoi
    expect(onLaunch).toHaveBeenCalledTimes(1)
  })

  it('refus puis changement de phase sans lien : la popup ne se ferme pas', () => {
    const onLaunch = vi.fn(() => true)
    const { rerender } = render(<LaunchGamePopup {...base} onLaunch={onLaunch} phase="REVEALED" />)
    fireEvent.click(go())
    rerender(<LaunchGamePopup {...base} onLaunch={onLaunch} phase="REVEALED" refused={{ reason: 'ENTRACTE_ACTIVE' }} />)
    rerender(<LaunchGamePopup {...base} onLaunch={onLaunch} phase="NEW_GAME" refused={{ reason: 'ENTRACTE_ACTIVE' }} />)
    expect(screen.queryByRole('dialog')).not.toBeNull()
  })
})
