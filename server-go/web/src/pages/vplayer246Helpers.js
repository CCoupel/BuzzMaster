// Aides communes des tests de spécification #246 (VPlayerPage) — pas un test.
import { vi } from 'vitest'

export function createStorage() {
  let store = {}
  return {
    getItem: (k) => (Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
    clear: () => { store = {} },
    dump: () => ({ ...store }),
  }
}

export const PAUL = 'vjoueur_Paul_20261006_140000'
export const OLD = 'vjoueur_Mamie_20260601_100000'

export function seedSession(ls, { name = 'Paul', id = PAUL, session = '1' } = {}) {
  ls.setItem('vplayer_name', name)
  ls.setItem('vplayer_session', session)
  if (id) ls.setItem('vplayer_id', id)
}

// Valeur renvoyée par le mock de useGame(), pour une partie ARDOISE (ou autre).
export function makeGame(send, overrides = {}) {
  const id = overrides.id ?? PAUL
  return {
    sendMessage: send,
    sendClientTrace: overrides.sendClientTrace ?? vi.fn(),
    gameState: {
      phase: 'STARTED',
      timer: 30,
      question: { ID: 'q1', TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' },
      ARDOISE_ANSWERS: {},
      enrollmentActive: false,
      ...overrides.gameState,
    },
    bumpers: overrides.bumpers ?? {
      [id]: { NAME: 'Paul', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'Les Bleus', CONNECTED: true },
    },
    teams: { 'Les Bleus': { NAME: 'Les Bleus', COLOR: [0, 0, 255] } },
    status: overrides.status ?? 'connected',
    playerConnectStatus: overrides.playerConnectStatus ?? null,
    clearPlayerConnectStatus: vi.fn(),
    playerEvictedStatus: overrides.playerEvictedStatus ?? null,
    clearPlayerEvictedStatus: vi.fn(),
    ardoiseFinalAck: overrides.ardoiseFinalAck ?? null,
  }
}

export const BOUND = (id = PAUL, name = 'Paul') => ({ status: 'connected', id, name })

// Messages CLIENT_TRACE émis par la page, quelle que soit la voie (sendMessage direct ou aide du hook).
export function traceKinds(send, sendClientTrace) {
  const direct = send.mock.calls.filter((c) => c[0] === 'CLIENT_TRACE').map((c) => c[1]?.KIND)
  const viaHelper = (sendClientTrace?.mock?.calls ?? []).map((c) => c[0])
  return [...direct, ...viaHelper]
}

export function sent(send, action) {
  return send.mock.calls.filter((c) => c[0] === action).map((c) => c[1])
}

export function clickKey(container, label) {
  const btn = [...container.querySelectorAll('button.ardoise-key')].find((b) => b.textContent === label)
  if (!btn) throw new Error(`touche ${label} introuvable`)
  return btn
}
