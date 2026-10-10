import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import LaunchGameButton, { getLaunchCounts } from './LaunchGameDialog'

// #243 — popup « Lancer la partie » (maquette admin_new_game__purge_popup_details).

const TEAMS = Object.fromEntries(Array.from({ length: 10 }, (_, i) => [`E${i}`, {}]))
const BUMPERS = {
  ...Object.fromEntries(Array.from({ length: 10 }, (_, i) => [`v${i}`, { IS_VPLAYER: true, TEAM: `E${i}`, CONNECTED: true }])),
  ...Object.fromEntries(Array.from({ length: 4 }, (_, i) => [`b${i}`, { IS_VPLAYER: false, TEAM: 'E0', CONNECTED: true }])),
}
const QUESTIONS = Object.fromEntries(Array.from({ length: 30 }, (_, i) => [`${i}`, { ID: `${i}`, STATUS: i < 12 ? 'REVEALED' : 'AVAILABLE' }]))

const setup = (props = {}) => {
  const onLaunch = vi.fn()
  const onClearRefused = vi.fn()
  const utils = render(
    <LaunchGameButton onLaunch={onLaunch} teams={TEAMS} bumpers={BUMPERS} questions={QUESTIONS}
      phase="REVEALED" refused={null} onClearRefused={onClearRefused} {...props} />
  )
  fireEvent.click(screen.getByText('LANCER LA PARTIE'))
  return { onLaunch, onClearRefused, ...utils }
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

describe('LaunchGameButton / popup (#243)', () => {
  it('le clic ouvre la popup (role dialog, aria) sans rien envoyer ; défaut = Scores + Questions cochées', () => {
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

  it('Échap et Annuler ferment sans envoyer', () => {
    const { onLaunch } = setup()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    fireEvent.click(screen.getByText('LANCER LA PARTIE'))
    fireEvent.click(screen.getByText('Annuler'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(onLaunch).not.toHaveBeenCalled()
  })

  it('NEW_GAME_REFUSED : message, la popup reste ouverte et la confirmation se réactive', () => {
    const onLaunch = vi.fn()
    const props = { onLaunch, teams: TEAMS, bumpers: BUMPERS, questions: QUESTIONS, phase: 'REVEALED', onClearRefused: vi.fn() }
    const { rerender } = render(<LaunchGameButton {...props} refused={null} />)
    fireEvent.click(screen.getByText('LANCER LA PARTIE'))
    fireEvent.click(go())
    expect(go().disabled).toBe(true)
    rerender(<LaunchGameButton {...props} refused={{ reason: 'PLAYERS_REQUIRE_SCORES' }} />)
    const dlg = screen.getByRole('dialog')
    expect(within(dlg).getByText(/Lancement refusé/)).toBeTruthy()
    expect(go().disabled).toBe(false)
  })

  it('se ferme quand la phase passe à NEW_GAME après l\'envoi', () => {
    const props = { onLaunch: vi.fn(), teams: TEAMS, bumpers: BUMPERS, questions: QUESTIONS, onClearRefused: vi.fn(), refused: null }
    const { rerender } = render(<LaunchGameButton {...props} phase="REVEALED" />)
    fireEvent.click(screen.getByText('LANCER LA PARTIE'))
    fireEvent.click(go())
    rerender(<LaunchGameButton {...props} phase="NEW_GAME" />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
