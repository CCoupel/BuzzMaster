import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act, within } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import LaunchGameHost, { useLaunchGame, LaunchLockBanner } from './LaunchGameHost'
import { useGame } from '../hooks/GameContext'

// #253 (v11.2, lot V2-4) — popup « Lancer la partie » en SUPERPOSITION plein écran (maquette VALIDÉE
// admin_new_game__launch_overlay) : portail à la racine du document (hors du bloc de la prévisualisation TV,
// cause du cloisonnement constaté en QUALIF 11.2.0.5), cycle de verrou OPEN / renouvellement 15 s / CLOSE,
// expiration, blocage de l'écran, bandeau des autres postes, accessibilité.

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./LaunchGameDialog.css', () => ({}))
vi.mock('./LaunchGameHost.css', () => ({}))

const here = dirname(fileURLToPath(import.meta.url))
const css = (f) => { try { return readFileSync(resolve(here, f), 'utf8') } catch { return '' } }

const idle = { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 }
const mine = { ACTIVE: true, OWNER: 'me', SINCE: 1000, EXPIRES_AT: 46000 }
const other = { ACTIVE: true, OWNER: 'someone-else', SINCE: 1000, EXPIRES_AT: 46000 }
const mk = (lock = idle, over = {}) => ({
  gameState: { phase: 'STOPPED', entracte: false, LAUNCH_LOCK: lock },
  connectionId: 'me', launchLock: vi.fn(() => true), launchLockRefused: null, clearLaunchLockRefused: vi.fn(),
  actionRefused: null, clearActionRefused: vi.fn(),
  teams: { A: {}, B: {} }, bumpers: {}, questions: {}, newGame: vi.fn(() => true), newGameRefused: null, clearNewGameRefused: vi.fn(),
  ...over,
})

function Opener() {
  const { openLaunch } = useLaunchGame()
  return <button onClick={openLaunch}>Ouvrir</button>
}
// Page factice : un bloc « prévisualisation TV » avec transform (piège classique du position: fixed).
const Page = () => (
  <LaunchGameHost>
    <nav data-testid="navbar"><Opener /></nav>
    <div data-testid="tv-preview" style={{ transform: 'scale(0.5)', overflow: 'hidden' }}>aperçu TV</div>
    <section data-testid="teams-column">Équipes</section>
  </LaunchGameHost>
)

let game
const show = (lock, over) => { game = mk(lock, over); useGame.mockReturnValue(game); return render(<Page />) }
const reshow = (utils, lock, over) => { game = { ...mk(lock, over), launchLock: game.launchLock }; useGame.mockReturnValue(game); utils.rerender(<Page />) }

beforeEach(() => { vi.useFakeTimers() })
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks() })

describe('Popup en superposition (#253)', () => {
  it('Critical — la popup est rendue à la RACINE du document (portail), jamais dans le bloc de prévisualisation TV ni la colonne Équipes', () => {
    show(mine)
    fireEvent.click(screen.getByText('Ouvrir'))
    const dlg = screen.getByRole('dialog')
    expect(dlg.closest('[data-testid="tv-preview"]')).toBeNull()
    expect(dlg.closest('[data-testid="teams-column"]')).toBeNull()
    expect(dlg.closest('[data-testid="navbar"]')).toBeNull()
    // le fond assombri est un enfant direct de <body>
    let node = dlg
    while (node.parentElement && node.parentElement !== document.body) node = node.parentElement
    expect(node.parentElement).toBe(document.body)
  })

  it('Critical — le fond couvre tout l’écran : position fixed, inset 0, au-dessus de la barre de navigation', () => {
    const dialogCss = css('./LaunchGameDialog.css') + css('./LaunchGameHost.css')
    const rule = dialogCss.match(/\.lg-backdrop\s*\{[^}]*\}/)?.[0] || ''
    expect(rule).toMatch(/position:\s*fixed/)
    expect(rule).toMatch(/inset:\s*0|top:\s*0[^}]*left:\s*0[^}]*(right:\s*0|width:\s*100(vw|%))/)
    const z = Number(rule.match(/z-index:\s*(\d+)/)?.[1])
    const navZ = Number(css('./Navbar.css').match(/\.navbar\s*\{[^}]*z-index:\s*(\d+)/)?.[1])
    expect(z).toBeGreaterThan(navZ)
  })

  it('la popup n’apparaît qu’à réception du verrou à notre nom (jamais avant)', () => {
    const utils = show(idle)
    fireEvent.click(screen.getByText('Ouvrir'))
    expect(game.launchLock).toHaveBeenCalledWith('OPEN')
    expect(screen.queryByRole('dialog')).toBeNull()
    reshow(utils, mine)
    expect(screen.getByRole('dialog')).toBeTruthy()
  })

  it('sans réponse du serveur au bout de 1 s : message d’erreur, pas de popup', () => {
    show(idle)
    fireEvent.click(screen.getByText('Ouvrir'))
    act(() => { vi.advanceTimersByTime(1100) })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('alert')).toBeTruthy()
  })

  it('sans verrou à notre nom (verrou d’un autre poste), aucune popup', () => {
    show(other)
    fireEvent.click(screen.getByText('Ouvrir'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(game.launchLock).not.toHaveBeenCalledWith('OPEN')
  })
})

describe('Cycle de vie du verrou côté popup (#253)', () => {
  const openMine = () => { const u = show(mine); fireEvent.click(screen.getByText('Ouvrir')); return u }

  it('Critical — renouvellement : un OPEN toutes les 15 s tant que la popup est ouverte', () => {
    openMine()
    const opens = () => game.launchLock.mock.calls.filter((c) => c[0] === 'OPEN').length
    expect(opens()).toBe(1)
    act(() => { vi.advanceTimersByTime(15000) })
    expect(opens()).toBe(2)
    act(() => { vi.advanceTimersByTime(30000) })
    expect(opens()).toBe(4)
  })

  it('« Annuler » envoie CLOSE et ferme ; le renouvellement s’arrête', () => {
    openMine()
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Annuler' }))
    expect(game.launchLock).toHaveBeenCalledWith('CLOSE')
    expect(screen.queryByRole('dialog')).toBeNull()
    const before = game.launchLock.mock.calls.length
    act(() => { vi.advanceTimersByTime(60000) })
    expect(game.launchLock.mock.calls.length).toBe(before)
  })

  it('Échap envoie CLOSE et ferme', () => {
    openMine()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(game.launchLock).toHaveBeenCalledWith('CLOSE')
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('Critical — fermeture de la page (pagehide) : CLOSE envoyé', () => {
    openMine()
    act(() => { window.dispatchEvent(new Event('pagehide')) })
    expect(game.launchLock).toHaveBeenCalledWith('CLOSE')
  })

  it('Critical — expiration de sécurité : le verrou n’est plus à notre nom → message « La fenêtre a expiré », lancement impossible, plus de renouvellement', () => {
    const utils = openMine()
    reshow(utils, idle)
    const dlg = screen.getByRole('dialog')
    expect(dlg.textContent).toMatch(/La fenêtre a expiré/)
    expect(dlg.textContent).toMatch(/rouvrez « Lancer la partie »/)
    expect(within(dlg).getByRole('button', { name: /^Lancer (la partie|sans rien effacer)$/ }).disabled).toBe(true)
    const before = game.launchLock.mock.calls.filter((c) => c[0] === 'OPEN').length
    act(() => { vi.advanceTimersByTime(30000) })
    expect(game.launchLock.mock.calls.filter((c) => c[0] === 'OPEN').length).toBe(before)
  })

  it('le lancement envoie NEW_GAME avec les quatre options (le serveur lève le verrou)', () => {
    openMine()
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^Lancer la partie$/ }))
    expect(game.newGame).toHaveBeenCalledWith({ purgeScores: true, purgePlayers: false, purgeTeams: false, resetQuestions: true })
  })
})

describe('Accessibilité (#253)', () => {
  it('dialogue modal nommé, focus dans la popup à l’ouverture, retour du focus sur l’élément déclencheur à la fermeture', () => {
    show(mine)
    const opener = screen.getByText('Ouvrir')
    opener.focus()
    fireEvent.click(opener)
    const dlg = screen.getByRole('dialog')
    expect(dlg.getAttribute('aria-modal')).toBe('true')
    expect(dlg.getAttribute('aria-labelledby')).toBeTruthy()
    expect(dlg.contains(document.activeElement)).toBe(true)
    fireEvent.click(within(dlg).getByRole('button', { name: 'Annuler' }))
    expect(document.activeElement).toBe(opener)
  })

  it('les erreurs et l’expiration sont annoncées (role alert)', () => {
    const utils = show(mine)
    fireEvent.click(screen.getByText('Ouvrir'))
    reshow(utils, idle)
    expect(within(screen.getByRole('dialog')).getAllByRole('alert').length).toBeGreaterThan(0)
  })
})

describe('Bandeau des autres postes (#253)', () => {
  const banner = () => screen.queryByText(/Lancement d'une nouvelle partie en cours sur un autre poste : actions de jeu suspendues\./)

  it('Critical — popup ouverte ailleurs : bandeau « actions de jeu suspendues » (role status) ; rien au poste propriétaire ni au repos', () => {
    useGame.mockReturnValue(mk(other)); const a = render(<LaunchGameHost><LaunchLockBanner /></LaunchGameHost>)
    expect(banner()).toBeTruthy()
    expect(screen.getByRole('status')).toBeTruthy()
    a.unmount()
    useGame.mockReturnValue(mk(mine)); const b = render(<LaunchGameHost><LaunchLockBanner /></LaunchGameHost>)
    expect(banner()).toBeNull()
    b.unmount()
    useGame.mockReturnValue(mk(idle)); render(<LaunchGameHost><LaunchLockBanner /></LaunchGameHost>)
    expect(banner()).toBeNull()
  })

  it('bandeau disparu quand le verrou est levé', () => {
    useGame.mockReturnValue(mk(other)); const u = render(<LaunchGameHost><LaunchLockBanner /></LaunchGameHost>)
    expect(banner()).toBeTruthy()
    useGame.mockReturnValue(mk(idle)); u.rerender(<LaunchGameHost><LaunchLockBanner /></LaunchGameHost>)
    expect(banner()).toBeNull()
  })

  it('ACTION_REFUSED (LAUNCH_IN_PROGRESS) : message court, puis consommé', () => {
    const g = mk(other, { actionRefused: { action: 'TEAM_POINTS', reason: 'LAUNCH_IN_PROGRESS' } })
    useGame.mockReturnValue(g)
    render(<LaunchGameHost><span /></LaunchGameHost>)
    expect(screen.getByRole('alert').textContent).toMatch(/suspendue/i)
    expect(g.clearActionRefused).toHaveBeenCalled()
  })
})
