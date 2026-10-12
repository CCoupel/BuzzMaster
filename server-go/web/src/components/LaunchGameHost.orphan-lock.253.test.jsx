import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import LaunchGameHost, { useLaunchGame } from './LaunchGameHost'
import { LaunchGameDialog } from './LaunchGameDialog'
import { useGame } from '../hooks/GameContext'

// #253 — corrections de revue : verrou orphelin (CLOSE), démontage hôte, expired ignoré une fois envoyé.
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./LaunchGameDialog.css', () => ({}))
vi.mock('./LaunchGameHost.css', () => ({}))

const idle = { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 }
const mine = { ACTIVE: true, OWNER: 'me', SINCE: 1000, EXPIRES_AT: 46000 }
const mk = (lock, launchLock) => ({
  gameState: { phase: 'STOPPED', entracte: false, LAUNCH_LOCK: lock },
  connectionId: 'me', launchLock, launchLockRefused: null, clearLaunchLockRefused: vi.fn(),
  actionRefused: null, clearActionRefused: vi.fn(),
  teams: {}, bumpers: {}, questions: {}, newGame: vi.fn(() => true), newGameRefused: null, clearNewGameRefused: vi.fn(),
})
function Opener() {
  const { openLaunch } = useLaunchGame()
  return <button onClick={openLaunch}>ouvrir</button>
}
const closes = (fn) => fn.mock.calls.filter((c) => c[0] === 'CLOSE').length

beforeEach(() => { vi.useFakeTimers() })
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks() })

describe('LaunchGameHost — verrou orphelin (#253)', () => {
  it('verrou pris à notre nom après l’attente de 1 s : CLOSE envoyé, une seule fois', () => {
    const launchLock = vi.fn(() => true)
    useGame.mockReturnValue(mk(idle, launchLock))
    const { rerender } = render(<LaunchGameHost><Opener /></LaunchGameHost>)
    fireEvent.click(screen.getByText('ouvrir'))
    act(() => { vi.advanceTimersByTime(1100) }) // délai écoulé
    expect(closes(launchLock)).toBe(0)
    useGame.mockReturnValue(mk(mine, launchLock))
    rerender(<LaunchGameHost><Opener /></LaunchGameHost>)
    expect(closes(launchLock)).toBe(1)
    rerender(<LaunchGameHost><Opener /></LaunchGameHost>)
    expect(closes(launchLock)).toBe(1)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('démontage de l’hôte popup ouverte : CLOSE au nettoyage', () => {
    const launchLock = vi.fn(() => true)
    useGame.mockReturnValue(mk(idle, launchLock))
    const { rerender, unmount } = render(<LaunchGameHost><Opener /></LaunchGameHost>)
    fireEvent.click(screen.getByText('ouvrir'))
    useGame.mockReturnValue(mk(mine, launchLock))
    rerender(<LaunchGameHost><Opener /></LaunchGameHost>)
    expect(closes(launchLock)).toBe(0)
    unmount()
    expect(closes(launchLock)).toBe(1)
  })

  it('démontage sans popup : aucun CLOSE', () => {
    const launchLock = vi.fn(() => true)
    useGame.mockReturnValue(mk(idle, launchLock))
    render(<LaunchGameHost><Opener /></LaunchGameHost>).unmount()
    expect(closes(launchLock)).toBe(0)
  })
})

describe('LaunchGamePopup — expired ignoré une fois NEW_GAME envoyé (#253)', () => {
  it('envoi en cours : pas de message d’expiration (le verrou est libéré par le serveur au lancement)', async () => {
    const { LaunchGamePopup } = await import('./LaunchGameDialog')
    const el = (expired) => (
      <LaunchGamePopup onLaunch={() => true} onClose={() => {}} teams={{}} bumpers={{}} questions={{}} phase="STOPPED" expired={expired} />
    )
    const { rerender } = render(el(false))
    fireEvent.click(screen.getByRole('button', { name: /lancer/i }))
    expect(screen.getByText(/Lancement de la partie/)).toBeTruthy()
    rerender(el(true))
    expect(document.body.textContent).not.toMatch(/expir/i)
  })
})
void LaunchGameDialog
