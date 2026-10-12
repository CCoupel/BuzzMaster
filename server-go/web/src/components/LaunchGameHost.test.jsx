import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import LaunchGameHost, { useLaunchGame, LaunchLockBanner, useLaunchLock } from './LaunchGameHost'
import { useGame } from '../hooks/GameContext'

// #253 — hôte « Lancer la partie » : verrou serveur (OPEN / renouvellement 15 s / CLOSE), popup en
// portail, bandeau des autres postes, refus d'ouverture.

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))

const LOCK_OFF = { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 }
const mk = (over = {}) => ({
  gameState: { phase: 'REVEALED', entracte: false, LAUNCH_LOCK: LOCK_OFF },
  teams: { A: {} }, bumpers: {}, questions: {},
  connectionId: 'me',
  newGame: vi.fn(() => true), newGameRefused: null, clearNewGameRefused: vi.fn(),
  launchLock: vi.fn(() => true), launchLockRefused: null, clearLaunchLockRefused: vi.fn(),
  actionRefused: null, clearActionRefused: vi.fn(),
  ...over,
})
const owned = (g) => ({ ...g, gameState: { ...g.gameState, LAUNCH_LOCK: { ACTIVE: true, OWNER: 'me', SINCE: 1, EXPIRES_AT: 46000 } } })
const other = (g) => ({ ...g, gameState: { ...g.gameState, LAUNCH_LOCK: { ACTIVE: true, OWNER: 'someone-else', SINCE: 1, EXPIRES_AT: 46000 } } })

function Opener() {
  const { openLaunch, lockedByOther } = useLaunchGame()
  return <button onClick={openLaunch} aria-disabled={lockedByOther}>ouvrir</button>
}
const tree = () => (
  <LaunchGameHost>
    <Opener />
    <LaunchLockBanner />
  </LaunchGameHost>
)
const sentLocks = (g) => g.launchLock.mock.calls.map(c => c[0])

describe('LaunchGameHost (#253)', () => {
  beforeEach(() => { vi.useFakeTimers(); useGame.mockReset() })
  afterEach(() => vi.useRealTimers())

  it('ouverture : envoie OPEN, la popup ne s\'affiche qu\'à réception du verrou à son nom, en portail sur <body>', () => {
    const g = mk(); useGame.mockReturnValue(g)
    const { rerender, container } = render(tree())
    fireEvent.click(screen.getByText('ouvrir'))
    expect(sentLocks(g)).toEqual(['OPEN'])
    expect(screen.queryByRole('dialog')).toBeNull() // pas encore d'état verrouillé à notre nom
    useGame.mockReturnValue(owned(g)); rerender(tree())
    const dlg = screen.getByRole('dialog')
    expect(dlg).toBeTruthy()
    expect(container.contains(dlg)).toBe(false) // hors de l'arbre React de la page : portail sur document.body
    expect(document.body.contains(dlg)).toBe(true)
  })

  it('verrou non reçu en 1 s : message d\'erreur, pas de popup', () => {
    const g = mk(); useGame.mockReturnValue(g)
    render(tree())
    fireEvent.click(screen.getByText('ouvrir'))
    act(() => { vi.advanceTimersByTime(1100) })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.getByRole('alert').textContent).toMatch(/le serveur n'a pas répondu/)
  })

  it('renouvellement OPEN toutes les 15 s tant que la popup est ouverte', () => {
    const g = owned(mk()); useGame.mockReturnValue(g)
    const { rerender } = render(tree())
    useGame.mockReturnValue(mk()); rerender(tree())
    fireEvent.click(screen.getByText('ouvrir'))
    useGame.mockReturnValue(g); rerender(tree())
    const n0 = sentLocks(g).length
    act(() => { vi.advanceTimersByTime(15000) })
    act(() => { vi.advanceTimersByTime(15000) })
    expect(sentLocks(g).slice(n0)).toEqual(['OPEN', 'OPEN'])
  })

  it('Annuler envoie CLOSE et ferme ; fermeture de la page (pagehide) envoie CLOSE', () => {
    const base = mk()
    useGame.mockReturnValue(base)
    const { rerender } = render(tree())
    fireEvent.click(screen.getByText('ouvrir'))
    useGame.mockReturnValue(owned(base)); rerender(tree())
    fireEvent.click(screen.getByText('Annuler'))
    expect(sentLocks(base).at(-1)).toBe('CLOSE')
    expect(screen.queryByRole('dialog')).toBeNull()

    base.launchLock.mockClear()
    fireEvent.click(screen.getByText('ouvrir'))
    useGame.mockReturnValue(owned(base)); rerender(tree())
    window.dispatchEvent(new Event('pagehide'))
    expect(sentLocks(base)).toContain('CLOSE')
  })

  it('verrou perdu popup ouverte (expiration de sécurité) : message « La fenêtre a expiré », confirmation désactivée', () => {
    const base = mk(); useGame.mockReturnValue(base)
    const { rerender } = render(tree())
    fireEvent.click(screen.getByText('ouvrir'))
    useGame.mockReturnValue(owned(base)); rerender(tree())
    useGame.mockReturnValue(base); rerender(tree()) // verrou levé par le serveur
    expect(screen.getByRole('dialog').textContent).toMatch(/La fenêtre a expiré/)
    expect(screen.getByRole('button', { name: /Lancer (la partie|sans rien effacer)/ }).disabled).toBe(true)
  })

  it('entracte active : message, aucun OPEN envoyé', () => {
    const g = mk({ gameState: { phase: 'REVEALED', entracte: true, LAUNCH_LOCK: LOCK_OFF } }); useGame.mockReturnValue(g)
    render(tree())
    fireEvent.click(screen.getByText('ouvrir'))
    expect(g.launchLock).not.toHaveBeenCalled()
    expect(screen.getByRole('alert').textContent).toBe("Impossible pendant une entracte : sortez d'abord de l'entracte.")
  })

  it('verrou tenu par un autre poste : aucun OPEN, entrée grisée (aria-disabled), bandeau affiché', () => {
    const g = other(mk()); useGame.mockReturnValue(g)
    render(tree())
    expect(screen.getByText('ouvrir').getAttribute('aria-disabled')).toBe('true')
    expect(screen.getByRole('status').textContent).toMatch(/Lancement d'une nouvelle partie en cours sur un autre poste : actions de jeu suspendues/)
    fireEvent.click(screen.getByText('ouvrir'))
    expect(g.launchLock).not.toHaveBeenCalled()
  })

  it('bandeau absent hors verrou et chez le propriétaire', () => {
    const g = mk(); useGame.mockReturnValue(g)
    const { rerender } = render(tree())
    expect(screen.queryByRole('status')).toBeNull()
    useGame.mockReturnValue(owned(g)); rerender(tree())
    expect(screen.queryByText(/sur un autre poste/)).toBeNull()
  })

  it('LAUNCH_LOCK_REFUSED : message (autre poste / entracte), consommé', () => {
    const g = mk({ launchLockRefused: { reason: 'LOCKED_BY_OTHER' } }); useGame.mockReturnValue(g)
    render(tree())
    expect(screen.getByRole('alert').textContent).toMatch(/déjà ouverte sur un autre poste/)
    expect(g.clearLaunchLockRefused).toHaveBeenCalled()
  })

  it('ACTION_REFUSED : message court « action suspendue »', () => {
    const g = mk({ actionRefused: { action: 'START', reason: 'LAUNCH_IN_PROGRESS' } }); useGame.mockReturnValue(g)
    render(tree())
    expect(screen.getByRole('alert').textContent).toMatch(/Action suspendue/)
    expect(g.clearActionRefused).toHaveBeenCalled()
  })
})

describe('useLaunchLock (#253)', () => {
  beforeEach(() => useGame.mockReset())
  function Probe() {
    const l = useLaunchLock()
    return <span data-testid="p">{`${l.active}|${l.isOwner}|${l.lockedByOther}`}</span>
  }
  it.each([
    ['inactif', mk(), 'false|false|false'],
    ['propriétaire', owned(mk()), 'true|true|false'],
    ['autre poste', other(mk()), 'true|false|true'],
    ['connexion inconnue : jamais propriétaire', owned(mk({ connectionId: null })), 'true|false|true'],
  ])('%s', (_, g, expected) => {
    useGame.mockReturnValue(g)
    render(<Probe />)
    expect(screen.getByTestId('p').textContent).toBe(expected)
  })
})
