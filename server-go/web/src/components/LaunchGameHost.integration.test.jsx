import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { GameProvider } from '../hooks/GameContext'
import LaunchGameHost, { useLaunchGame } from './LaunchGameHost'

// #253 — intégration hook réel + faux serveur : HELLO_ACK { CLIENT_ID } puis OPEN → état verrouillé à
// ce CLIENT_ID → popup ouverte ; un verrou au nom d'un autre CLIENT_ID n'ouvre rien.

let ws
class FakeServerSocket {
  constructor() { ws = this; this.readyState = 0; this.sent = [] }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3 }
}
FakeServerSocket.OPEN = 1
const server = (ACTION, MSG) => act(() => { ws.onmessage({ data: JSON.stringify({ ACTION, MSG }) }) })
const lock = (OWNER, ACTIVE = true) => ({ ACTIVE, OWNER, SINCE: 1, EXPIRES_AT: 46000 })

function Opener() {
  const { openLaunch } = useLaunchGame()
  return <button onClick={openLaunch}>ouvrir</button>
}
const mount = () => render(
  <GameProvider endpoint="/ws/admin">
    <LaunchGameHost><Opener /></LaunchGameHost>
  </GameProvider>
)
// HELLO d'un client admin : le serveur répond HELLO_ACK en premier, puis l'état initial
const hello = (clientId) => {
  act(() => { ws.readyState = 1; ws.onopen() })
  server('HELLO_ACK', { CLIENT_ID: clientId })
  server('UPDATE', { GAME: { PHASE: 'REVEALED', LAUNCH_LOCK: lock('', false) } })
}

describe('LaunchGameHost — intégration HELLO_ACK + LAUNCH_LOCK (#253)', () => {
  beforeEach(() => { vi.useFakeTimers(); vi.stubGlobal('WebSocket', FakeServerSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  it('HELLO_ACK puis OPEN : le serveur verrouille au nom de CE CLIENT_ID → la popup s\'ouvre', () => {
    mount()
    hello('10.0.0.5:5001')
    fireEvent.click(screen.getByText('ouvrir'))
    expect(ws.sent.filter(m => m.ACTION === 'LAUNCH_LOCK').map(m => m.MSG)).toEqual([{ STATE: 'OPEN' }])
    expect(screen.queryByRole('dialog')).toBeNull()
    server('UPDATE', { GAME: { PHASE: 'REVEALED', LAUNCH_LOCK: lock('10.0.0.5:5001') } })
    expect(screen.getByRole('dialog')).toBeTruthy()
  })

  it('verrou au nom d\'une autre connexion : aucune popup, OPEN non envoyé (entrée grisée)', () => {
    mount()
    hello('10.0.0.5:5001')
    server('UPDATE', { GAME: { PHASE: 'REVEALED', LAUNCH_LOCK: lock('10.0.0.9:6000') } })
    fireEvent.click(screen.getByText('ouvrir'))
    expect(ws.sent.filter(m => m.ACTION === 'LAUNCH_LOCK')).toEqual([])
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('reconnexion : nouveau CLIENT_ID, l\'ancien verrou ne revient pas au nouvel identifiant', () => {
    mount()
    hello('10.0.0.5:5001')
    fireEvent.click(screen.getByText('ouvrir'))
    server('UPDATE', { GAME: { PHASE: 'REVEALED', LAUNCH_LOCK: lock('10.0.0.5:5001') } })
    expect(screen.getByRole('dialog')).toBeTruthy()
    // coupure puis reconnexion : le serveur a levé le verrou (déconnexion), nouvel identifiant
    server('HELLO_ACK', { CLIENT_ID: '10.0.0.5:5002' })
    server('UPDATE', { GAME: { PHASE: 'REVEALED', LAUNCH_LOCK: lock('', false) } })
    expect(screen.getByRole('dialog').textContent).toMatch(/La fenêtre a expiré/)
  })
})
